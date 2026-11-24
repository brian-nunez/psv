package psv

import (
	"encoding/json"
	"math/big"
	"math/rand/v2"
	"os"
	"testing"
)

// The tests in this file keep the detector's per-pixel and per-region
// counting in step with the Lean model in proofs/, whose theorems state the
// guarantees the detector relies on. See proofs/README.md.
//
//   - TestModelVectors replays testdata/model/vectors.json, the answers of the
//     Lean model, against the real channel and against modelPixel and
//     modelRegion, the model's Go translation below.
//   - TestChannelMatchesModel and the fuzz targets compare the real channel
//     with that translation on random input, so that every input, not just
//     the vectors, is checked.

// modelPixel is the Lean model of one pixel (PsvModel.Pixel and
// PsvModel.flashing): its step state and the frames of its counted
// transitions, oldest first.
type modelPixel struct {
	rate, maxTransitions int
	stepDir, lastDir     int8
	frames               []int
}

// step is PsvModel.Pixel.counts and PsvModel.Pixel.step.
func (m *modelPixel) step(t int, dir int8) bool {
	counts := dir != 0 && dir != m.stepDir && dir != m.lastDir
	m.stepDir = dir

	if counts {
		m.lastDir = dir
		m.frames = append(m.frames, t)
	}

	return counts
}

// window is PsvModel.window: the counted transitions in frames lo..hi.
func (m *modelPixel) window(lo, hi int) int {
	n := 0

	for _, f := range m.frames {
		if f >= lo && f <= hi {
			n++
		}
	}

	return n
}

// recent is the count PsvModel.recent_eq_window proves channel.recent
// returns: the window of the second ending at the last transition.
func (m *modelPixel) recent() int {
	t := m.frames[len(m.frames)-1]

	return m.window(t+1-m.rate, t)
}

// flashing is PsvModel.flashing. Its windows start at frame 0 at the
// earliest, where channel.flashing also tries negative starts.
func (m *modelPixel) flashing(f, now int) bool {
	span := m.rate - 1

	for s := max(f-span, 0); s <= f; s++ {
		if m.window(s, min(s+span, now)) > m.maxTransitions {
			return true
		}
	}

	return false
}

// modelRegion is the Lean model of one region position's transitions
// (PsvModel.ringCount and PsvModel.ringSince): every recorded frame, newest
// first.
type modelRegion struct {
	rate, keep int
	newest     []int
}

func (m *modelRegion) record(f int) (count, since int) {
	m.newest = append([]int{f}, m.newest...)
	since = f

	for _, at := range m.newest[:min(m.keep, len(m.newest))] {
		if f < at+m.rate {
			count++
			since = min(since, at)
		}
	}

	return count, since
}

type modelVectors struct {
	Generator string `json:"generator"`
	Pixel     []struct {
		Rate           int    `json:"rate"`
		MaxTransitions int    `json:"maxTransitions"`
		Dirs           []int8 `json:"dirs"`
		Counted        []bool `json:"counted"`
		Recent         []int  `json:"recent"`
		Live           []bool `json:"live"`
		Decided        []bool `json:"decided"`
	} `json:"pixel"`
	Region []struct {
		Rate           int   `json:"rate"`
		MaxTransitions int   `json:"maxTransitions"`
		Frames         []int `json:"frames"`
		Counts         []int `json:"counts"`
		Since          []int `json:"since"`
	} `json:"region"`
}

// pixelAnswers is what one pixel's counting answers in one frame, as the
// detector asks for it.
type pixelAnswers struct {
	counted bool
	recent  int
	live    bool
	decided map[int]bool
}

// pixelFrame advances a 1×1 channel and its model by frame t, with step
// direction dir unless t is 0, and returns the answers of both: whether the
// step counts, channel.recent after it, the live decision for frame t, and
// the decisions for the frames in check.
func pixelFrame(c *channel, m *modelPixel, t int, dir int8, check []int) (got, want pixelAnswers) {
	got.recent, want.recent = -1, -1

	if t > 0 {
		got.counted = c.step(0, t, dir)
		want.counted = m.step(t, dir)

		if got.counted {
			got.recent = c.recent(0)
		}

		if want.counted {
			want.recent = m.recent()
		}
	}

	got.live, want.live = c.flashing(0, t, t), m.flashing(t, t)
	got.decided, want.decided = map[int]bool{}, map[int]bool{}

	for _, f := range check {
		got.decided[f], want.decided[f] = c.flashing(0, f, t), m.flashing(f, t)
	}

	return got, want
}

func equalAnswers(a, b pixelAnswers) bool {
	if a.counted != b.counted || a.recent != b.recent || a.live != b.live || len(a.decided) != len(b.decided) {
		return false
	}

	for f, v := range a.decided {
		if w, ok := b.decided[f]; !ok || w != v {
			return false
		}
	}

	return true
}

// checkPixel runs a pixel through the real channel and the model, asking for
// the decisions the detector asks for (the frame a second back, then the
// final second at the end) plus those in extra, and fails on any difference.
func checkPixel(t *testing.T, rate, maxTransitions int, dirs []int8, extra func(now int) []int) {
	t.Helper()

	c := newChannel(1, 1, 1, 1, rate, maxTransitions)
	m := &modelPixel{rate: rate, maxTransitions: maxTransitions}
	n := len(dirs)

	for now := 0; now <= n; now++ {
		var check []int
		if f := now - (rate - 1); f >= 0 {
			check = append(check, f)
		}

		if now == n {
			for f := max(n-rate+2, 0); f <= n; f++ {
				check = append(check, f)
			}
		}

		if extra != nil {
			check = append(check, extra(now)...)
		}

		var dir int8
		if now > 0 {
			dir = dirs[now-1]
		}

		got, want := pixelFrame(c, m, now, dir, check)
		if !equalAnswers(got, want) {
			t.Fatalf("rate %d, maxTransitions %d, frame %d: channel %+v, model %+v", rate, maxTransitions, now, got, want)
		}
	}
}

// checkRegion records region transitions at the given frames in the real
// channel and the model and fails on any difference.
func checkRegion(t *testing.T, rate, maxTransitions int, frames []int) {
	t.Helper()

	c := newChannel(1, 1, 1, 1, rate, maxTransitions)
	m := &modelRegion{rate: rate, keep: maxTransitions + 1}
	dir := int8(1)

	for _, f := range frames {
		count, since := c.record(0, dir, f)
		wantCount, wantSince := m.record(f)

		if count != wantCount || since != wantSince {
			t.Fatalf("rate %d, maxTransitions %d, frame %d: channel (%d, %d), model (%d, %d)",
				rate, maxTransitions, f, count, since, wantCount, wantSince)
		}

		dir = -dir
	}
}

// TestModelVectors checks the real channel and the Go model against the
// answers of the Lean model. Regenerate the vectors with make model-vectors.
func TestModelVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/model/vectors.json")
	if err != nil {
		t.Fatal(err)
	}

	var v modelVectors
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}

	if len(v.Pixel) == 0 || len(v.Region) == 0 {
		t.Fatal("no vectors")
	}

	for i, p := range v.Pixel {
		c := newChannel(1, 1, 1, 1, p.Rate, p.MaxTransitions)
		m := &modelPixel{rate: p.Rate, maxTransitions: p.MaxTransitions}
		n := len(p.Dirs)

		for now := 0; now <= n; now++ {
			var check []int
			if f := now - (p.Rate - 1); f >= 0 {
				check = append(check, f)
			}

			if now == n {
				for f := max(n-p.Rate+2, 0); f <= n; f++ {
					check = append(check, f)
				}
			}

			var dir int8
			if now > 0 {
				dir = p.Dirs[now-1]
			}

			got, model := pixelFrame(c, m, now, dir, check)

			want := pixelAnswers{recent: -1, live: p.Live[now], decided: map[int]bool{}}
			if now > 0 {
				want.counted, want.recent = p.Counted[now-1], p.Recent[now-1]
			}

			for _, f := range check {
				want.decided[f] = p.Decided[f]
			}

			if !equalAnswers(got, want) {
				t.Fatalf("pixel case %d (rate %d), frame %d: channel %+v, Lean %+v", i, p.Rate, now, got, want)
			}

			if !equalAnswers(model, want) {
				t.Fatalf("pixel case %d (rate %d), frame %d: Go model %+v, Lean %+v", i, p.Rate, now, model, want)
			}
		}
	}

	for i, r := range v.Region {
		c := newChannel(1, 1, 1, 1, r.Rate, r.MaxTransitions)
		m := &modelRegion{rate: r.Rate, keep: r.MaxTransitions + 1}
		dir := int8(1)

		for j, f := range r.Frames {
			count, since := c.record(0, dir, f)
			mCount, mSince := m.record(f)

			if count != r.Counts[j] || since != r.Since[j] {
				t.Fatalf("region case %d, frame %d: channel (%d, %d), Lean (%d, %d)", i, f, count, since, r.Counts[j], r.Since[j])
			}

			if mCount != r.Counts[j] || mSince != r.Since[j] {
				t.Fatalf("region case %d, frame %d: Go model (%d, %d), Lean (%d, %d)", i, f, mCount, mSince, r.Counts[j], r.Since[j])
			}

			dir = -dir
		}
	}
}

// randomDirs returns n step directions in runs of flashing, ramps, noise and
// stillness, so that pixels both flash and stay calm.
func randomDirs(rng *rand.Rand, rate, n int) []int8 {
	dirs := make([]int8, 0, n)

	for len(dirs) < n {
		length := 1 + rng.IntN(2*rate+2)

		switch rng.IntN(4) {
		case 0:
			q, dir := 1+rng.IntN(max(1, rate/3)), int8(1-2*rng.IntN(2))

			for i := range length {
				if i%q == 0 {
					dirs = append(dirs, dir)
					dir = -dir
				} else {
					dirs = append(dirs, 0)
				}
			}
		case 1:
			dir := int8(1 - 2*rng.IntN(2))
			for range length {
				dirs = append(dirs, dir)
			}
		case 2:
			for range length {
				dirs = append(dirs, int8(rng.IntN(3)-1))
			}
		default:
			dirs = append(dirs, make([]int8, length)...)
		}
	}

	return dirs[:n]
}

// TestChannelMatchesModel is a property test: on random pixels and regions
// at frame rates that use one to four history words, the real channel
// answers every question the detector can ask, at every frame, exactly as
// the model does.
func TestChannelMatchesModel(t *testing.T) {
	cases := 200
	if testing.Short() {
		cases = 40
	}

	rng := rand.New(rand.NewPCG(1, 2))

	for range cases {
		rate, maxTransitions := 1+rng.IntN(130), rng.IntN(10)
		n := 2*rate + rng.IntN(4*rate+10)

		// Every frame the detector may decide at this point: up to a second
		// back.
		all := func(now int) []int {
			var fs []int
			for f := max(now-(rate-1), 0); f <= now; f++ {
				fs = append(fs, f)
			}

			return fs
		}

		checkPixel(t, rate, maxTransitions, randomDirs(rng, rate, n), all)

		frames, f := make([]int, 0, 64), rng.IntN(rate)
		for range 3*maxTransitions + 20 {
			f += 1 + rng.IntN(2*rate)
			frames = append(frames, f)
		}

		checkRegion(t, rate, maxTransitions, frames)
	}
}

// FuzzPixelMatchesModel compares the real channel with the model on any
// sequence of steps, at any frame rate up to 130.
func FuzzPixelMatchesModel(f *testing.F) {
	f.Add(uint8(29), uint8(6), []byte{2, 1, 0, 1, 2, 1, 0, 1, 2, 1, 0, 1, 2, 1, 0, 1, 2})
	f.Add(uint8(0), uint8(0), []byte{2, 0, 2, 0, 2})
	f.Add(uint8(64), uint8(3), []byte{2, 2, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 2, 0})
	f.Add(uint8(99), uint8(7), []byte{2, 0, 2, 0, 2, 0, 2, 0, 2, 0, 2, 0, 2, 0, 2, 0, 2, 0, 2})

	f.Fuzz(func(t *testing.T, rateSeed, maxSeed uint8, steps []byte) {
		rate, maxTransitions := 1+int(rateSeed)%130, int(maxSeed)%10

		if len(steps) > 2000 {
			steps = steps[:2000]
		}

		dirs := make([]int8, len(steps))
		for i, b := range steps {
			dirs[i] = int8(b%3) - 1
		}

		// The live frame, the frame a second back, and one between.
		some := func(now int) []int {
			return []int{now, max(now-(rate-1), 0), now - now%rate}
		}

		checkPixel(t, rate, maxTransitions, dirs, some)
	})
}

// FuzzRegionMatchesModel compares channel.record with the model on any
// increasing sequence of frames.
func FuzzRegionMatchesModel(f *testing.F) {
	f.Add(uint8(29), uint8(6), []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 40, 0, 0})
	f.Add(uint8(0), uint8(0), []byte{0, 1, 2})
	f.Add(uint8(59), uint8(3), []byte{5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 100, 1})

	f.Fuzz(func(t *testing.T, rateSeed, maxSeed uint8, gaps []byte) {
		rate, maxTransitions := 1+int(rateSeed)%130, int(maxSeed)%10
		frames, frame := make([]int, 0, len(gaps)), 0

		for _, g := range gaps {
			frame += 1 + int(g)
			frames = append(frames, frame)
		}

		checkRegion(t, rate, maxTransitions, frames)
	})
}

// FuzzAgeMatchesShift checks age, the multi-word shift behind every
// history, against PsvModel.History.record's definition: the history as one
// number, shifted left and cut to its width.
func FuzzAgeMatchesShift(f *testing.F) {
	f.Add(uint8(0), uint16(1), []byte{1})
	f.Add(uint8(1), uint16(63), []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 1})
	f.Add(uint8(3), uint16(130), []byte{0x80, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})

	f.Fuzz(func(t *testing.T, wordSeed uint8, shift uint16, data []byte) {
		words := 1 + int(wordSeed)%4
		h := make([]uint64, words)

		for i, b := range data {
			if i/8 < words {
				h[i/8] |= uint64(b) << (8 * (i % 8))
			}
		}

		// The same history as one number, word 0 lowest.
		want := new(big.Int)
		for w := words - 1; w >= 0; w-- {
			want.Lsh(want, 64)
			want.Or(want, new(big.Int).SetUint64(h[w]))
		}

		width := uint(64 * words)

		want.Lsh(want, uint(shift))
		want.Mod(want, new(big.Int).Lsh(big.NewInt(1), width))

		age(h, int(shift))

		got := new(big.Int)
		for w := words - 1; w >= 0; w-- {
			got.Lsh(got, 64)
			got.Or(got, new(big.Int).SetUint64(h[w]))
		}

		if got.Cmp(want) != 0 {
			t.Fatalf("%d words, shift %d: got %x, want %x", words, shift, got, want)
		}
	})
}
