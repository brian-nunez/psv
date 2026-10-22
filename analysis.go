package psv

import "time"

// Verdict is the overall result of a video.
type Verdict int

const (
	// Pass means no region flashes more often than the thresholds allow.
	Pass Verdict = iota
	// Fail means some region flashes more often than the thresholds allow.
	Fail
)

func (v Verdict) String() string {
	if v == Fail {
		return "Failed"
	}

	return "Passed"
}

// MarshalText encodes the verdict by name.
func (v Verdict) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

// Passed reports whether the verdict is a pass.
func (v Verdict) Passed() bool {
	return v == Pass
}

// Defect is a run of frames that belong to failing flash sequences. Frame
// numbers start at 1.
type Defect struct {
	FirstFrame int  `json:"firstFrame"`
	LastFrame  int  `json:"lastFrame"`
	Luminance  bool `json:"luminance"`
	Red        bool `json:"red"`
}

// Failures counts the frames that complete a failing flash sequence.
type Failures struct {
	Luminance int `json:"luminance"`
	Red       int `json:"red"`
}

// Analysis is the complete analysis of a video.
type Analysis struct {
	// FrameRate is the frame rate the analysis used, in frames per second.
	FrameRate int
	// Width and Height are the size of the source frames.
	Width, Height int
	// Thresholds are the detection parameters used.
	Thresholds Thresholds
	// Frames holds one result per frame; Frames[0] is frame 1.
	Frames []FrameResult
}

// Duration returns the length of the analysed material.
func (a *Analysis) Duration() time.Duration {
	if a.FrameRate == 0 {
		return 0
	}

	return time.Duration(len(a.Frames)) * time.Second / time.Duration(a.FrameRate)
}

// FrameTime returns the start time of a frame; frame numbers start at 1.
func (a *Analysis) FrameTime(frame int) time.Duration {
	if a.FrameRate == 0 {
		return 0
	}

	return time.Duration(frame-1) * time.Second / time.Duration(a.FrameRate)
}

// Verdict returns Fail when any frame completes a failing flash sequence.
func (a *Analysis) Verdict() Verdict {
	for i := range a.Frames {
		if a.Frames[i].Failed() {
			return Fail
		}
	}

	return Pass
}

// Failures counts the frames that complete a failing luminance and red flash
// sequence.
func (a *Analysis) Failures() Failures {
	var f Failures

	for i := range a.Frames {
		if a.Frames[i].Luminance.Failed {
			f.Luminance++
		}

		if a.Frames[i].Red.Failed {
			f.Red++
		}
	}

	return f
}

// Defects returns the runs of frames covered by failing flash sequences,
// each sequence spanning from its first transition to the frame that
// completes it. Overlapping and adjacent sequences are merged.
func (a *Analysis) Defects() []Defect {
	var defects []Defect

	for i := range a.Frames {
		r := &a.Frames[i]
		frame := i + 1

		for _, f := range []struct {
			flashes *Flashes
			red     bool
		}{{&r.Luminance, false}, {&r.Red, true}} {
			if !f.flashes.Failed {
				continue
			}

			d := Defect{FirstFrame: f.flashes.Since, LastFrame: frame, Luminance: !f.red, Red: f.red}

			if n := len(defects); n > 0 && d.FirstFrame <= defects[n-1].LastFrame+1 {
				last := &defects[n-1]
				last.FirstFrame = min(last.FirstFrame, d.FirstFrame)
				last.LastFrame = max(last.LastFrame, d.LastFrame)
				last.Luminance = last.Luminance || d.Luminance
				last.Red = last.Red || d.Red

				continue
			}

			defects = append(defects, d)
		}
	}

	return defects
}
