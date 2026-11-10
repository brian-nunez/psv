package psv_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"io"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/brian-nunez/psv"
)

// lockstep runs one stream through two analysers, one with Analyze and one
// with Watch, checks that Watch decides every frame exactly as Analyze does,
// and records when Watch's live measurement first sees flashing.
type lockstep struct {
	analyze, watch *psv.Analyzer
	frames         int
	decided        []psv.FrameResult
	// live is the first frame whose Building shows flashing pixels, per
	// kind: luminance, then red. Zero when none did.
	live [2]int
	// spent is the time spent in Watch.
	spent time.Duration
}

func newLockstep(t testing.TB, fps float64, w, h int, cfg psv.Config) *lockstep {
	t.Helper()

	a, err := psv.NewAnalyzer(fps, w, h, cfg)
	if err != nil {
		t.Fatal(err)
	}

	b, err := psv.NewAnalyzer(fps, w, h, cfg)
	if err != nil {
		t.Fatal(err)
	}

	return &lockstep{analyze: a, watch: b}
}

func (l *lockstep) add(t testing.TB, f *psv.Frame) {
	t.Helper()

	if _, err := l.analyze.Analyze(f); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	u, err := l.watch.Watch(f)
	l.spent += time.Since(start)

	if err != nil {
		t.Fatal(err)
	}

	l.frames++

	if u.Schema != psv.WatchSchema || u.Method != psv.WatchPixel || u.Frame != l.frames {
		t.Fatalf("update header %q %q frame %d, want %q %q frame %d",
			u.Schema, u.Method, u.Frame, psv.WatchSchema, psv.WatchPixel, l.frames)
	}

	for k, a := range []psv.Activity{u.Building.Luminance, u.Building.Red} {
		if a.Flashing > 0 && l.live[k] == 0 {
			l.live[k] = u.Frame
		}
	}

	l.collect(t, u)
}

func (l *lockstep) collect(t testing.TB, u psv.Update) {
	t.Helper()

	if len(u.Decided) > 0 && u.DecidedFrom != len(l.decided)+1 {
		t.Fatalf("frame %d: decided from frame %d, want %d", u.Frame, u.DecidedFrom, len(l.decided)+1)
	}

	l.decided = append(l.decided, u.Decided...)
}

// finish ends both analysers, checks that their decided frames are
// identical and returns the Analyze analysis.
func (l *lockstep) finish(t testing.TB) *psv.Analysis {
	t.Helper()

	analysis := l.analyze.Finish()

	u, err := l.watch.Stop()
	if err != nil {
		t.Fatal(err)
	}

	l.collect(t, u)

	if n := len(l.watch.Analysis().Frames); n != 0 {
		t.Errorf("Watch kept %d frames", n)
	}

	if len(l.decided) != len(analysis.Frames) {
		t.Fatalf("Watch decided %d frames, Analyze %d", len(l.decided), len(analysis.Frames))
	}

	for i := range l.decided {
		if !reflect.DeepEqual(l.decided[i], analysis.Frames[i]) {
			t.Fatalf("frame %d: Watch decided %+v, Analyze %+v", i+1, l.decided[i], analysis.Frames[i])
		}
	}

	return analysis
}

// liveLead checks that Watch saw the flashing of every kind that fails no
// later than the frame that completes its first failing sequence, and
// returns by how many frames the live measurement came before the decided
// failure reached the caller, per failing kind.
func liveLead(t testing.TB, analysis *psv.Analysis, l *lockstep) []int {
	t.Helper()

	var leads []int

	for k, name := range []string{"luminance", "red"} {
		failed := 0

		for i := range analysis.Frames {
			r := &analysis.Frames[i]
			if (k == 0 && r.Luminance.Failed) || (k == 1 && r.Red.Failed) {
				failed = i + 1

				break
			}
		}

		if failed == 0 {
			continue
		}

		// A frame's decided result reaches the caller in the Update of the
		// frame a second later.
		delivered := failed + analysis.FrameRate - 1

		switch live := l.live[k]; {
		case live == 0:
			t.Errorf("%s: fails at frame %d, but Watch never saw flashing pixels", name, failed)
		case live > failed:
			t.Errorf("%s: fails at frame %d, but Watch first saw flashing pixels at frame %d", name, failed, live)
		default:
			leads = append(leads, delivered-live)
		}
	}

	return leads
}

const clipsDir = "testdata/clips"

// TestWatchClips watches the generated clips: decided frames match Analyze,
// the failing clip is seen live before its failure is decided, and the
// still clips show no flashing.
func TestWatchClips(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found on PATH")
	}

	for _, c := range []struct {
		clip string
		fail bool
	}{
		{"fail.mp4", true},
		{"pass.mp4", false},
		{"long.mp4", false},
	} {
		t.Run(c.clip, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(clipsDir, c.clip)

			info, err := psv.Probe(context.Background(), path, psv.Tools{})
			if err != nil {
				t.Fatal(err)
			}

			dec, err := psv.Decode(context.Background(), path, info, psv.Tools{}, psv.DecodeOptions{})
			if err != nil {
				t.Fatal(err)
			}

			defer func() { _ = dec.Close() }()

			l := newLockstep(t, info.FrameRate.Float64(), info.Width, info.Height, psv.Config{})

			for {
				f, err := dec.Next()
				if errors.Is(err, io.EOF) {
					break
				}

				if err != nil {
					t.Fatal(err)
				}

				l.add(t, f)
			}

			analysis := l.finish(t)

			if got := !analysis.Verdict().Passed(); got != c.fail {
				t.Fatalf("failed %v, want %v", got, c.fail)
			}

			leads := liveLead(t, analysis, l)

			if !c.fail && l.live != [2]int{} {
				t.Errorf("still clip shows flashing pixels live from frames %v", l.live)
			}

			if c.fail && len(leads) != 2 {
				t.Fatalf("live leads %v, want luminance and red", leads)
			}

			for _, lead := range leads {
				// Seen in the frame that completes the failure, a full decision
				// delay ahead of the decided result.
				if lead < analysis.FrameRate-1 {
					t.Errorf("live measurement %d frames ahead of the decided failure, want at least %d", lead, analysis.FrameRate-1)
				}
			}

			t.Logf("%d frames, Watch %v a frame; live ahead of the decided failure by %v frames",
				l.frames, l.spent/time.Duration(l.frames), leads)
		})
	}
}

// strobe renders frame n of a synthetic screen recording that repeats every
// ten seconds: two seconds of full-frame white flashing at 10 Hz, a second of
// a red flash at 5 Hz in the top left quarter, then a slow 2 Hz flicker that
// passes, then calm.
func strobe(f *psv.Frame, n, rate int) {
	t := n % (10 * rate)
	half := f.Width / 2

	var on, red bool

	switch {
	case t < 2*rate:
		on = t*20/rate%2 == 0
	case t < 3*rate:
		on, red = t*10/rate%2 == 0, true
	case t < 6*rate:
		on = t*4/rate%2 == 0
	}

	for y := range f.Height {
		for x := range f.Width {
			i := (y*f.Width + x) * 3
			v := byte(16)

			switch {
			case on && red && x < half && y < f.Height/2:
				f.Pix[i], f.Pix[i+1], f.Pix[i+2] = 255, 0, 0

				continue
			case on && !red:
				v = 255
			}

			f.Pix[i], f.Pix[i+1], f.Pix[i+2] = v, v, v
		}
	}
}

// TestWatchLongSession watches a simulated screen recording at 60 fps for
// three hours and checks that memory stays flat while failures keep being
// decided and seen live. With -short, which make test runs under the race
// detector, it watches three minutes.
func TestWatchLongSession(t *testing.T) {
	const rate, w, h = 60, 96, 54

	minutes, warm := 180, 60*rate
	if testing.Short() {
		minutes, warm = 3, 30*rate
	}

	a, err := psv.NewAnalyzer(rate, w, h, psv.Config{})
	if err != nil {
		t.Fatal(err)
	}

	f := &psv.Frame{Width: w, Height: h, Pix: make([]byte, w*h*3)}

	heap := func() uint64 {
		runtime.GC()

		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		return m.HeapAlloc
	}

	var (
		failed, live, frames int
		before               uint64
	)

	for n := range minutes * 60 * rate {
		strobe(f, n, rate)

		u, err := a.Watch(f)
		if err != nil {
			t.Fatal(err)
		}

		frames++

		if u.Building.Luminance.Flashing > 0 || u.Building.Red.Flashing > 0 {
			live++
		}

		for _, r := range u.Decided {
			if r.Failed() {
				failed++
			}
		}

		if frames == warm {
			before = heap()
		}
	}

	after := heap()

	if n := len(a.Analysis().Frames); n != 0 {
		t.Errorf("Watch kept %d frames", n)
	}

	if failed == 0 || live == 0 {
		t.Fatalf("%d failing frames decided, %d frames flashing live; the session should fail", failed, live)
	}

	// The live heap after warming up must not grow with the session.
	const slack = 256 << 10
	if after > before+slack {
		t.Errorf("heap grew from %d to %d bytes over %d minutes", before, after, minutes)
	}

	t.Logf("%d frames: heap %d bytes after %d frames, %d at the end; %d failing frames, %d frames flashing live",
		frames, before, warm, after, failed, live)
}

func TestWatchUpdateJSON(t *testing.T) {
	a, err := psv.NewAnalyzer(30, 64, 36, psv.Config{})
	if err != nil {
		t.Fatal(err)
	}

	f := &psv.Frame{Width: 64, Height: 36, Pix: make([]byte, 64*36*3)}

	var u psv.Update

	for n := range 60 {
		strobe(f, n, 30)

		if u, err = a.Watch(f); err != nil {
			t.Fatal(err)
		}
	}

	if u.Building.Luminance.Peak == 0 || u.Building.Luminance.Mask == nil || len(u.Decided) != 1 {
		t.Fatalf("update %+v: want a live measurement and one decided frame", u)
	}

	b, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(b, []byte(`"schema":"psv.watch/v1"`)) || !bytes.Contains(b, []byte(`"method":"pixel"`)) {
		t.Errorf("JSON %s lacks the schema or method", b)
	}

	var back psv.Update
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(back, u) {
		t.Errorf("round trip: got %+v, want %+v", back, u)
	}
}

func TestWatchMeasuresFullFrameFlash(t *testing.T) {
	const rate, w, h = 30, 64, 36

	a, err := psv.NewAnalyzer(rate, w, h, psv.Config{})
	if err != nil {
		t.Fatal(err)
	}

	f := &psv.Frame{Width: w, Height: h, Pix: make([]byte, w*h*3)}
	maxTransitions := psv.DefaultThresholds().MaxTransitions

	// The whole frame toggles between black and white every third frame: a
	// transition in every frame n divisible by 3, ten a second.
	for n := range 3 * rate {
		v := byte(255 * (n / 3 % 2))
		for i := range f.Pix {
			f.Pix[i] = v
		}

		u, err := a.Watch(f)
		if err != nil {
			t.Fatal(err)
		}

		lum := u.Building.Luminance

		if n%3 != 0 || n == 0 {
			if lum.Area != 0 || lum.Mask != nil {
				t.Fatalf("frame %d: activity %+v without a transition", u.Frame, lum)
			}

			continue
		}

		// Transitions within the second ending at frame n.
		recent := 0

		for j := max(n-rate+1, 1); j <= n; j++ {
			if j%3 == 0 {
				recent++
			}
		}

		if lum.Area != 100 || lum.Peak != recent {
			t.Fatalf("frame %d: area %.1f peak %d, want 100 and %d", u.Frame, lum.Area, lum.Peak, recent)
		}

		for i, c := range lum.Mask {
			if int(c) != recent {
				t.Fatalf("frame %d: mask cell %d holds %d, want %d", u.Frame, i, c, recent)
			}
		}

		if recent > maxTransitions {
			if lum.Flashing != 100 || lum.Bounds != image.Rect(0, 0, w, h) {
				t.Fatalf("frame %d: flashing %.1f in %v, want 100 in the whole frame", u.Frame, lum.Flashing, lum.Bounds)
			}
		} else if lum.Flashing != 0 || !lum.Bounds.Empty() {
			t.Fatalf("frame %d: flashing %.1f in %v after %d transitions", u.Frame, lum.Flashing, lum.Bounds, recent)
		}
	}
}

func TestWatchModes(t *testing.T) {
	newAnalyzer := func() *psv.Analyzer {
		a, err := psv.NewAnalyzer(30, 8, 8, psv.Config{})
		if err != nil {
			t.Fatal(err)
		}

		return a
	}

	f := &psv.Frame{Width: 8, Height: 8, Pix: make([]byte, 8*8*3)}

	a := newAnalyzer()
	if _, err := a.Analyze(f); err != nil {
		t.Fatal(err)
	}

	if _, err := a.Watch(f); !errors.Is(err, psv.ErrMode) {
		t.Errorf("Watch after Analyze: %v, want ErrMode", err)
	}

	if _, err := a.Stop(); !errors.Is(err, psv.ErrMode) {
		t.Errorf("Stop after Analyze: %v, want ErrMode", err)
	}

	w := newAnalyzer()
	if _, err := w.Watch(f); err != nil {
		t.Fatal(err)
	}

	if _, err := w.Analyze(f); !errors.Is(err, psv.ErrMode) {
		t.Errorf("Analyze after Watch: %v, want ErrMode", err)
	}

	if _, err := w.Watch(&psv.Frame{Width: 4, Height: 4, Pix: make([]byte, 48)}); err == nil {
		t.Error("Watch accepted a frame of the wrong size")
	}

	u, err := w.Stop()
	if err != nil || len(u.Decided) != 1 || u.DecidedFrom != 1 {
		t.Errorf("Stop: %+v, %v; want frame 1 decided", u, err)
	}

	if u, err := w.Stop(); err != nil || len(u.Decided) != 0 || u.Schema != psv.WatchSchema {
		t.Errorf("second Stop: %+v, %v; want an empty update", u, err)
	}

	if _, err := w.Watch(f); !errors.Is(err, psv.ErrFinished) {
		t.Errorf("Watch after Stop: %v, want ErrFinished", err)
	}

	if n := len(w.Finish().Frames); n != 0 {
		t.Errorf("Finish after Watch returned %d frames", n)
	}

	if _, err := psv.NewAnalyzer(30, 8, 8, psv.Config{WatchMethod: "region"}); !errors.Is(err, psv.ErrWatchMethod) {
		t.Errorf("unknown method: %v, want ErrWatchMethod", err)
	}
}

// BenchmarkWatch measures a frame of full-screen flashing at psvguard's size
// (640×360), the worst case for the live measurement, against Analyze.
func BenchmarkWatch(b *testing.B) {
	const rate, w, h = 60, 640, 360

	frames := make([]*psv.Frame, 2*rate)
	for n := range frames {
		frames[n] = &psv.Frame{Width: w, Height: h, Pix: make([]byte, w*h*3)}
		strobe(frames[n], n, rate)
	}

	for _, m := range []string{"Analyze", "Watch"} {
		b.Run(m, func(b *testing.B) {
			a, err := psv.NewAnalyzer(rate, w, h, psv.Config{})
			if err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()

			for n := 0; b.Loop(); n++ {
				f := frames[n%len(frames)]

				if m == "Analyze" {
					_, err = a.Analyze(f)
				} else {
					_, err = a.Watch(f)
				}

				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
