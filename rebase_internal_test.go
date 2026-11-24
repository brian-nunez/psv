package psv

import (
	"math"
	"testing"
)

// TestRebaseKeepsUnusedRingSlotsEmpty checks that moving frame numbers back
// leaves the never-written slots of a region's ring empty. With 32-bit int,
// the never-written value math.MinInt32 minus the shift used to wrap to a
// frame far in the future, so the next transition counted a full ring and
// the region failed.
func TestRebaseKeepsUnusedRingSlotsEmpty(t *testing.T) {
	const rate, maxTransitions = 30, 6

	c := newChannel(1, 1, 1, 1, rate, maxTransitions)
	c.rebase(3 * rate)

	if count, since := c.record(0, 1, 100); count != 1 || since != 100 {
		t.Fatalf("first transition after a rebase: count %d, since %d, want 1, 100", count, since)
	}
}

// TestRebaseClampsLongIdleHistory checks a pixel idle for so long that its
// last transition clamped to math.MinInt32. Its next transition must start
// a fresh history. With 32-bit int, the frames since that transition used to
// overflow to a negative count and age panicked.
func TestRebaseClampsLongIdleHistory(t *testing.T) {
	for _, rate := range []int{30, 60, 240} {
		c := newChannel(1, 1, 1, 1, rate, 6)

		for w := range c.words {
			c.hist[w] = math.MaxUint64
		}

		c.updated[0] = math.MinInt32
		c.rebase(3 * rate)

		if c.updated[0] != math.MinInt32 {
			t.Fatalf("rate %d: updated %d after rebase, want math.MinInt32", rate, c.updated[0])
		}

		if !c.step(0, 2*rate, 1) {
			t.Fatalf("rate %d: the step did not count", rate)
		}

		if n := c.recent(0); n != 1 {
			t.Fatalf("rate %d: recent %d after a long idle, want 1", rate, n)
		}

		for w, v := range c.hist {
			if want := uint64(0); w == 0 && v != 1 || w > 0 && v != want {
				t.Fatalf("rate %d: history word %d is %#x, want only the new transition", rate, w, v)
			}
		}
	}
}

// TestRebaseBeforeFirstFlash watches a stream that stays calm past the
// first rebase and only then flashes, and checks every decided frame against
// an Analyze run that never rebases. With 32-bit int, the first rebase used
// to fill every region's unused ring slots with future frames, so the first
// flash failed early with the wrong transition count.
func TestRebaseBeforeFirstFlash(t *testing.T) {
	const rate, w, h = 30, 32, 18

	retain, err := NewAnalyzer(rate, w, h, Config{})
	if err != nil {
		t.Fatal(err)
	}

	live, err := NewAnalyzer(rate, w, h, Config{})
	if err != nil {
		t.Fatal(err)
	}

	live.detect.rebaseAt = 5 * rate
	f := &Frame{Width: w, Height: h, Pix: make([]byte, w*h*3)}

	var decided []FrameResult

	for n := range 20 * rate {
		// Calm for 8 seconds, past the first rebase at 5, then a 5 Hz flash.
		v := byte(16)
		if n >= 8*rate && n/3%2 == 1 {
			v = 255
		}

		for i := range f.Pix {
			f.Pix[i] = v
		}

		if _, err := retain.Analyze(f); err != nil {
			t.Fatal(err)
		}

		u, err := live.Watch(f)
		if err != nil {
			t.Fatal(err)
		}

		decided = append(decided, u.Decided...)
	}

	u, err := live.Stop()
	if err != nil {
		t.Fatal(err)
	}

	decided = append(decided, u.Decided...)
	want := retain.Finish().Frames

	if live.detect.base == 0 {
		t.Fatal("the detector did not rebase")
	}

	if len(decided) != len(want) {
		t.Fatalf("decided %d frames, want %d", len(decided), len(want))
	}

	for i := range want {
		if decided[i].Luminance.Transitions != want[i].Luminance.Transitions ||
			decided[i].Luminance.Since != want[i].Luminance.Since ||
			decided[i].Failed() != want[i].Failed() {
			t.Fatalf("frame %d: watched %d transitions since %d, failed %t; analysed %d since %d, failed %t", i+1,
				decided[i].Luminance.Transitions, decided[i].Luminance.Since, decided[i].Failed(),
				want[i].Luminance.Transitions, want[i].Luminance.Since, want[i].Failed())
		}
	}
}
