package psv

import (
	"reflect"
	"testing"
)

// TestRebaseKeepsResults forces the detector to move its stored frame
// numbers back every few seconds, as it does every 2^30 frames, and checks
// that Watch still decides every frame, frame numbers included, exactly as
// an Analyze run that never rebases.
func TestRebaseKeepsResults(t *testing.T) {
	const rate, w, h = 30, 32, 18

	minutes := 20
	if testing.Short() {
		minutes = 2
	}

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

	for n := range minutes * 60 * rate {
		// Seven seconds of a 5 Hz flash, then calm for three, so that
		// failing sequences straddle the rebases.
		v := byte(16)
		if n%(10*rate) < 7*rate && n/3%2 == 1 {
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

	if live.detect.base == 0 || live.detect.frame > 7*rate {
		t.Fatalf("base %d, frame %d: the detector did not rebase", live.detect.base, live.detect.frame)
	}

	if len(decided) != len(want) {
		t.Fatalf("decided %d frames, want %d", len(decided), len(want))
	}

	failed := 0

	for i := range want {
		if want[i].Failed() {
			failed++
		}

		if !reflect.DeepEqual(decided[i], want[i]) {
			t.Fatalf("frame %d: got %+v, want %+v", i+1, decided[i], want[i])
		}
	}

	if failed == 0 {
		t.Fatal("no failing frames: the test pattern should fail")
	}
}
