package psv

import (
	"encoding/json"
	"math/big"
	"math/rand/v2"
	"os"
	"slices"
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
//     with that translation on random input: channels of several pixels and
//     region positions, frame rates up to MaxFrameRate, rings up to the
//     largest NewAnalyzer allows, and frame numbers moved back by rebase.
//   - TestDetectorScheduleMatchesModel runs the real detector and checks
//     that it decides each frame when the model says the decision is final,
//     from the right queued transitions, and that the live measurement
//     matches the model.

// modelPixel is the Lean model of one pixel (PsvModel.Pixel and
// PsvModel.flashing): its step state and, per frame from 0, the number of
// counted transitions up to that frame.
type modelPixel struct {
	rate, maxTransitions int
	stepDir, lastDir     int8
	last                 int
	upTo                 []int
}

func newModelPixel(rate, maxTransitions int) *modelPixel {
	return &modelPixel{rate: rate, maxTransitions: maxTransitions, last: -1}
}

// step is PsvModel.Pixel.counts and PsvModel.Pixel.step for frame t, the
// next frame; frame 0 has no step.
func (m *modelPixel) step(t int, dir int8) bool {
	counts := t > 0 && dir != 0 && dir != m.stepDir && dir != m.lastDir

	if t > 0 {
		m.stepDir = dir
	}

	if counts {
		m.lastDir = dir
	}

	m.mark(t, counts)

	return counts
}

// mark records whether the pixel made a counted transition in frame t, the
// next frame.
func (m *modelPixel) mark(t int, counted bool) {
	n := 0
	if t > 0 {
		n = m.upTo[t-1]
	}

	if counted {
		n++
		m.last = t
	}

	m.upTo = append(m.upTo, n)
}

// window is PsvModel.window: the counted transitions in frames lo..hi.
func (m *modelPixel) window(lo, hi int) int {
	lo, hi = max(lo, 0), min(hi, len(m.upTo)-1)
	if hi < lo {
		return 0
	}

	if lo == 0 {
		return m.upTo[hi]
	}

	return m.upTo[hi] - m.upTo[lo-1]
}

// recent is the count PsvModel.recent_eq_window proves channel.recent
// returns: the window of the second ending at the last transition.
func (m *modelPixel) recent() int {
	return m.window(m.last+1-m.rate, m.last)
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

// pixelAnswers is what one pixel's counting answers in one frame, as the
// detector asks for it.
type pixelAnswers struct {
	counted bool
	recent  int
	live    bool
	decided map[int]bool
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

// pixelRun is a channel of several pixels and their models, advanced frame
// by frame. When rebaseAt is set, it moves the channel's frame numbers back
// as detector.process does, while the models keep counting from frame 0.
type pixelRun struct {
	rate, maxTransitions int
	rebaseAt             int
	c                    *channel
	models               []*modelPixel
	base                 int
}

func newPixelRun(rate, maxTransitions, width, height, rebaseAt int) *pixelRun {
	r := &pixelRun{
		rate: rate, maxTransitions: maxTransitions, rebaseAt: rebaseAt,
		c: newChannel(width, height, 1, 1, rate, maxTransitions),
	}

	for range width * height {
		r.models = append(r.models, newModelPixel(rate, maxTransitions))
	}

	return r
}

// frame advances every pixel by frame now, pixel i by step direction
// dirs[i] unless now is 0, and returns the answers of the channel and of the
// model for each pixel: whether its step counts, channel.recent after a
// counted step, the live decision for frame now, and the decisions for the
// frames in check.
func (r *pixelRun) frame(now int, dirs []int8, check []int) (got, want []pixelAnswers) {
	if rel := now - r.base; r.rebaseAt > 0 && rel >= r.rebaseAt {
		if shift := (rel - 2*r.rate) / r.rate * r.rate; shift > 0 {
			r.c.rebase(shift)
			r.base += shift
		}
	}

	rel := now - r.base

	for i, m := range r.models {
		g, w := pixelAnswers{recent: -1}, pixelAnswers{recent: -1}

		if now > 0 {
			g.counted = r.c.step(i, rel, dirs[i])
		}

		w.counted = m.step(now, dirs[i])

		if g.counted {
			g.recent = r.c.recent(i)
		}

		if w.counted {
			w.recent = m.recent()
		}

		g.live, w.live = r.c.flashing(i, rel, rel), m.flashing(now, now)
		g.decided, w.decided = map[int]bool{}, map[int]bool{}

		for _, f := range check {
			g.decided[f], w.decided[f] = r.c.flashing(i, f-r.base, rel), m.flashing(f, now)
		}

		got, want = append(got, g), append(want, w)
	}

	return got, want
}

// decideAt returns the frames the detector decides once frame now of n is
// seen: the frame a second back, then the final second after the last frame.
func decideAt(rate, now, n int) []int {
	var fs []int
	if f := now - (rate - 1); f >= 0 {
		fs = append(fs, f)
	}

	if now == n {
		for f := max(n-rate+2, 0); f <= n; f++ {
			fs = append(fs, f)
		}
	}

	return fs
}

// checkPixels runs pixels with the given steps, all of one length, through a
// width×height channel and the model, asking for the decisions the detector
// asks for plus those in extra, and fails on any difference.
func checkPixels(t *testing.T, rate, maxTransitions, width int, dirs [][]int8, rebaseAt int, extra func(now int) []int) {
	t.Helper()

	r := newPixelRun(rate, maxTransitions, width, len(dirs)/width, rebaseAt)
	n := len(dirs[0])
	step := make([]int8, len(dirs))

	for now := 0; now <= n; now++ {
		check := decideAt(rate, now, n)
		if extra != nil {
			check = append(check, extra(now)...)
		}

		for i := range dirs {
			step[i] = 0
			if now > 0 {
				step[i] = dirs[i][now-1]
			}
		}

		got, want := r.frame(now, step, check)
		for i := range got {
			if !equalAnswers(got[i], want[i]) {
				t.Fatalf("rate %d, maxTransitions %d, pixel %d of %d, frame %d, base %d: channel %+v, model %+v",
					rate, maxTransitions, i, len(dirs), now, r.base, got[i], want[i])
			}
		}
	}
}

// checkRegions records region transitions in a channel of len(at[0]) region
// positions and the model, position p in frame f when at[f][p], and fails on
// any difference. When rebaseAt is set, it moves the channel's frame numbers
// back as detector.process does.
func checkRegions(t *testing.T, rate, maxTransitions int, at [][]bool, rebaseAt int) {
	t.Helper()

	positions := len(at[0])
	c := newChannel(positions, 1, 1, 1, rate, maxTransitions)
	models := make([]*modelRegion, positions)
	dirs := make([]int8, positions)

	for p := range models {
		models[p], dirs[p] = &modelRegion{rate: rate, keep: maxTransitions + 1}, 1
	}

	base := 0

	for f, row := range at {
		if rel := f - base; rebaseAt > 0 && rel >= rebaseAt {
			if shift := (rel - 2*rate) / rate * rate; shift > 0 {
				c.rebase(shift)
				base += shift
			}
		}

		for p, rec := range row {
			if !rec {
				continue
			}

			count, since := c.record(p, dirs[p], f-base)
			wantCount, wantSince := models[p].record(f)

			if count != wantCount || since+base != wantSince {
				t.Fatalf("rate %d, maxTransitions %d, position %d of %d, frame %d, base %d: channel (%d, %d), model (%d, %d)",
					rate, maxTransitions, p, positions, f, base, count, since+base, wantCount, wantSince)
			}

			dirs[p] = -dirs[p]
		}
	}
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

// TestModelVectors checks the real channel and the Go model against the
// answers of the Lean model. Each case runs at one pixel or position of a
// channel of four, the others idle, so that indexing beyond the first is
// checked too. Regenerate the vectors with make model-vectors.
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

	const pixels = 4

	for i, p := range v.Pixel {
		n := len(p.Dirs)
		if len(p.Counted) != n || len(p.Recent) != n || len(p.Live) != n+1 || len(p.Decided) != n+1 {
			t.Fatalf("pixel case %d: %d steps with %d counted, %d recent, %d live and %d decided answers",
				i, n, len(p.Counted), len(p.Recent), len(p.Live), len(p.Decided))
		}

		r := newPixelRun(p.Rate, p.MaxTransitions, 2, 2, 0)
		at := i % pixels
		step := make([]int8, pixels)

		for now := 0; now <= n; now++ {
			check := decideAt(p.Rate, now, n)

			step[at] = 0
			if now > 0 {
				step[at] = p.Dirs[now-1]
			}

			got, model := r.frame(now, step, check)

			want := pixelAnswers{recent: -1, live: p.Live[now], decided: map[int]bool{}}
			if now > 0 {
				want.counted, want.recent = p.Counted[now-1], p.Recent[now-1]
			}

			for _, f := range check {
				want.decided[f] = p.Decided[f]
			}

			if !equalAnswers(got[at], want) {
				t.Fatalf("pixel case %d (rate %d), frame %d: channel %+v, Lean %+v", i, p.Rate, now, got[at], want)
			}

			if !equalAnswers(model[at], want) {
				t.Fatalf("pixel case %d (rate %d), frame %d: Go model %+v, Lean %+v", i, p.Rate, now, model[at], want)
			}

			for j := range got {
				if !equalAnswers(got[j], model[j]) {
					t.Fatalf("pixel case %d (rate %d), frame %d, pixel %d: channel %+v, Go model %+v", i, p.Rate, now, j, got[j], model[j])
				}
			}
		}
	}

	for i, r := range v.Region {
		if len(r.Counts) != len(r.Frames) || len(r.Since) != len(r.Frames) {
			t.Fatalf("region case %d: %d frames with %d counts and %d since", i, len(r.Frames), len(r.Counts), len(r.Since))
		}

		c := newChannel(pixels, 1, 1, 1, r.Rate, r.MaxTransitions)
		m := &modelRegion{rate: r.Rate, keep: r.MaxTransitions + 1}
		pos, dir := i%pixels, int8(1)

		for j, f := range r.Frames {
			count, since := c.record(pos, dir, f)
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

// boundaryRates are the frame rates on each side of a change in the number
// of history words, (2*rate+63)/64, up to MaxFrameRate.
var boundaryRates = []int{1, 31, 32, 33, 63, 64, 65, 96, 97, 128, 129, 160, 161, 192, 193, 224, 225, 240}

// randomCase picks a frame rate, often on a history word boundary, and a
// transition limit, now and then up to the largest NewAnalyzer allows.
func randomCase(rng *rand.Rand) (rate, maxTransitions int) {
	rate = 1 + rng.IntN(MaxFrameRate)
	if rng.IntN(2) == 0 {
		rate = boundaryRates[rng.IntN(len(boundaryRates))]
	}

	maxTransitions = rng.IntN(10)
	if rng.IntN(8) == 0 {
		maxTransitions = rng.IntN(255)
	}

	return rate, maxTransitions
}

// TestChannelMatchesModel is a property test: on random channels of several
// pixels and region positions, at any frame rate and limit NewAnalyzer
// allows, over short and long streams, with and without rebasing, the real
// channel answers every question the detector asks exactly as the model
// does.
func TestChannelMatchesModel(t *testing.T) {
	cases := 160
	if testing.Short() {
		cases = 32
	}

	rng := rand.New(rand.NewPCG(1, 2))

	for range cases {
		rate, maxTransitions := randomCase(rng)

		// A short stream ends before the first second is seen, so only the
		// final flush decides frames.
		n := 2*rate + rng.IntN(3*rate+10)
		if rng.IntN(6) == 0 {
			n = rng.IntN(rate)
		}

		rebaseAt := 0
		if rng.IntN(2) == 0 {
			rebaseAt = 3*rate + rng.IntN(2*rate)
		}

		// Fewer pixels and questions at high rates, where each costs more.
		width, height := 1+rng.IntN(3), 1+rng.IntN(3)
		if rate > 64 {
			height = 1
		}

		extra := func(now int) []int {
			if rate > 64 {
				return []int{now - now%rate}
			}

			var fs []int
			for f := max(now-(rate-1), 0); f <= now; f++ {
				fs = append(fs, f)
			}

			return fs
		}

		dirs := make([][]int8, width*height)
		for i := range dirs {
			dirs[i] = randomDirs(rng, rate, n)
		}

		checkPixels(t, rate, maxTransitions, width, dirs, rebaseAt, extra)

		keep := maxTransitions + 1
		at, density := make([][]bool, 3*rate+3*keep+20), 1+rng.IntN(max(1, 2*rate/keep))
		positions := 1 + rng.IntN(4)

		for f := range at {
			at[f] = make([]bool, positions)
			for p := range at[f] {
				at[f][p] = rng.IntN(density) == 0
			}
		}

		checkRegions(t, rate, maxTransitions, at, rebaseAt)
	}
}

// FuzzPixelMatchesModel compares the real channel with the model on any
// steps of one to three pixels, at any frame rate and limit NewAnalyzer
// allows, rebasing when the top bit of shape is set.
func FuzzPixelMatchesModel(f *testing.F) {
	f.Add(uint8(29), uint8(6), uint8(0), []byte{2, 1, 0, 1, 2, 1, 0, 1, 2, 1, 0, 1, 2, 1, 0, 1, 2})
	f.Add(uint8(0), uint8(0), uint8(1), []byte{2, 0, 2, 0, 2})
	f.Add(uint8(64), uint8(3), uint8(2), []byte{2, 2, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 2, 0})
	f.Add(uint8(239), uint8(7), uint8(0x80), []byte{2, 0, 2, 0, 2, 0, 2, 0, 2, 0, 2, 0, 2, 0, 2, 0, 2, 0, 2})

	f.Fuzz(func(t *testing.T, rateSeed, maxSeed, shape uint8, steps []byte) {
		rate, maxTransitions := 1+int(rateSeed)%MaxFrameRate, int(maxSeed)%255
		pixels := 1 + int(shape&0x7f)%3

		rebaseAt := 0
		if shape&0x80 != 0 {
			rebaseAt = 3 * rate
		}

		steps = steps[:min(len(steps), 1500)]
		n := len(steps) / pixels

		dirs := make([][]int8, pixels)
		for i := range dirs {
			dirs[i] = make([]int8, n)
			for j := range n {
				dirs[i][j] = int8(steps[j*pixels+i]%3) - 1
			}
		}

		// The live frame, the frame a second back, and one between.
		some := func(now int) []int {
			return []int{now, max(now-(rate-1), 0), now - now%rate}
		}

		checkPixels(t, rate, maxTransitions, pixels, dirs, rebaseAt, some)
	})
}

// FuzzRegionMatchesModel compares channel.record with the model on any
// transitions of one to four region positions: each byte moves on 1 to 32
// frames and picks a position. The top bit of shape turns on rebasing.
func FuzzRegionMatchesModel(f *testing.F) {
	f.Add(uint8(29), uint8(6), uint8(0), []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0xf8, 0, 0})
	f.Add(uint8(0), uint8(0), uint8(1), []byte{0, 9, 18})
	f.Add(uint8(59), uint8(254), uint8(0x83), []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	f.Fuzz(func(t *testing.T, rateSeed, maxSeed, shape uint8, events []byte) {
		rate, maxTransitions := 1+int(rateSeed)%MaxFrameRate, int(maxSeed)%255
		positions := 1 + int(shape&0x7f)%4

		rebaseAt := 0
		if shape&0x80 != 0 {
			rebaseAt = 3 * rate
		}

		events = events[:min(len(events), 2000)]
		at, frame := [][]bool{}, 0

		for _, e := range events {
			frame += 1 + int(e>>3)
			for len(at) <= frame {
				at = append(at, make([]bool, positions))
			}

			at[frame][int(e&7)%positions] = true
		}

		if len(at) == 0 {
			return
		}

		checkRegions(t, rate, maxTransitions, at, rebaseAt)
	})
}

// FuzzAgeMatchesShift checks age, the multi-word shift behind every
// history, against PsvModel.History.record's definition: the history as one
// number, shifted left and cut to its width. Histories hold 1 to 8 words,
// as many as MaxFrameRate needs.
func FuzzAgeMatchesShift(f *testing.F) {
	f.Add(uint8(0), uint16(1), []byte{1})
	f.Add(uint8(1), uint16(63), []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 1})
	f.Add(uint8(7), uint16(130), []byte{0x80, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})

	f.Fuzz(func(t *testing.T, wordSeed uint8, shift uint16, data []byte) {
		words := 1 + int(wordSeed)%8
		h := make([]uint64, words)

		for i, b := range data {
			if i/8 < words {
				h[i/8] |= uint64(b) << (8 * (i % 8))
			}
		}

		number := func() *big.Int {
			n := new(big.Int)
			for w := words - 1; w >= 0; w-- {
				n.Lsh(n, 64)
				n.Or(n, new(big.Int).SetUint64(h[w]))
			}

			return n
		}

		want := number()
		want.Lsh(want, uint(shift))
		want.Mod(want, new(big.Int).Lsh(big.NewInt(1), uint(64*words)))

		age(h, int(shift))

		if got := number(); got.Cmp(want) != 0 {
			t.Fatalf("%d words, shift %d: got %x, want %x", words, shift, got, want)
		}
	})
}

// TestDetectorScheduleMatchesModel runs the real detector, through
// Analyzer.Watch, on frames whose pixels flash at their own rates, and checks
// its scheduling against the model:
//
//   - Building.Luminance measures the frame's transitions exactly as the
//     model counts them, so a pixel counted as flashing has passed the limit.
//   - Each frame is decided once, in order, a second after it is seen, or by
//     Stop at the end, from the transitions queued when it was seen.
//   - Each decision matches the model's final decision for that frame
//     (PsvModel.flashing_final), and every pixel counted as flashing live
//     is flashing when its frame is decided (PsvModel.live_flashing_is_final).
func TestDetectorScheduleMatchesModel(t *testing.T) {
	const w, h = 8, 4

	rng := rand.New(rand.NewPCG(3, 4))

	for _, rate := range []int{1, 2, 24, 30, 33, 60, 97} {
		a, err := NewAnalyzer(float64(rate), w, h, Config{})
		if err != nil {
			t.Fatal(err)
		}

		d, maxTransitions := a.detect, a.analysis.Thresholds.MaxTransitions
		n := 4*rate + 10

		// Each pixel flips between dark and bright every period frames, and
		// holds now and then, so that some pixels flash and some do not.
		periods, levels := make([]int, w*h), make([]byte, w*h)
		for p := range periods {
			periods[p], levels[p] = 1+rng.IntN(max(1, rate/2)), 16
		}

		models := make([]*modelPixel, w*h)
		for p := range models {
			models[p] = newModelPixel(rate, maxTransitions)
		}

		frame := &Frame{Width: w, Height: h, Pix: make([]byte, w*h*3)}
		queued := make([][]event, n)
		live := make([][]int32, n)
		decided := map[[2]int]bool{}
		next := 0

		// checkDecided checks the decisions for frame f, made knowing frames
		// up to now.
		checkDecided := func(f, now int) {
			if f != next {
				t.Fatalf("rate %d: decided frame %d, want %d", rate, f, next)
			}

			next++

			if q := d.luma.queue[f%rate]; !slices.Equal(q, queued[f]) {
				t.Fatalf("rate %d: frame %d decided from queued transitions %v, want %v", rate, f, q, queued[f])
			}

			for _, e := range queued[f] {
				decided[[2]int{f, int(e.pixel)}] = d.luma.flashing(int(e.pixel), f, now)
			}
		}

		for now := range n {
			for p := range levels {
				if now%periods[p] == 0 && rng.IntN(8) != 0 {
					levels[p] ^= 16 ^ 255
				}

				frame.Pix[3*p], frame.Pix[3*p+1], frame.Pix[3*p+2] = levels[p], levels[p], levels[p]
			}

			u, err := a.Watch(frame)
			if err != nil {
				t.Fatal(err)
			}

			var events []event
			for b := range d.bands {
				events = append(events, d.bands[b].luma...)
			}

			queued[now] = events
			counted := map[int32]bool{}

			for _, e := range events {
				counted[e.pixel] = true
			}

			peak, flashing := 0, 0

			for p, m := range models {
				m.mark(now, counted[int32(p)])

				if counted[int32(p)] {
					peak = max(peak, m.recent())

					if m.recent() > maxTransitions {
						flashing++

						live[now] = append(live[now], int32(p))
					}
				}
			}

			b, total := u.Building.Luminance, float64(w*h)
			if b.Peak != peak || b.Flashing != float64(flashing)*100/total || b.Area != float64(len(events))*100/total {
				t.Fatalf("rate %d, frame %d: building %+v, model peak %d, %d flashing of %d moved", rate, now, b, peak, flashing, len(events))
			}

			for i := range u.Decided {
				if want := now - (rate - 1); u.DecidedFrom-1+i != want {
					t.Fatalf("rate %d: frame %d decided frame %d, want %d", rate, now, u.DecidedFrom-1+i, want)
				}

				checkDecided(u.DecidedFrom-1+i, now)
			}
		}

		u, err := a.Stop()
		if err != nil {
			t.Fatal(err)
		}

		for i := range u.Decided {
			checkDecided(u.DecidedFrom-1+i, n-1)
		}

		if next != n {
			t.Fatalf("rate %d: decided %d frames, want %d", rate, next, n)
		}

		for key, got := range decided {
			if want := models[key[1]].flashing(key[0], n-1); got != want {
				t.Fatalf("rate %d, frame %d, pixel %d: decided flashing %t, final model %t", rate, key[0], key[1], got, want)
			}
		}

		for f, ps := range live {
			for _, p := range ps {
				if !decided[[2]int{f, int(p)}] {
					t.Fatalf("rate %d, frame %d, pixel %d: flashing live but not when decided", rate, f, p)
				}
			}
		}
	}
}
