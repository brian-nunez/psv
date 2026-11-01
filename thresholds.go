package psv

import "image"

// Thresholds are the detection parameters.
type Thresholds struct {
	// LuminanceChange is the change in relative luminance, 0 to 1, that
	// makes a luminance transition.
	LuminanceChange float64 `json:"luminanceChange"`
	// DarkLimit is the relative luminance the darker side of a luminance
	// transition must be below.
	DarkLimit float64 `json:"darkLimit"`
	// RedChange is the change in u'v' chromaticity that makes a red
	// transition.
	RedChange float64 `json:"redChange"`
	// RedSaturation is the share of red, R/(R+G+B) in linear light, at which
	// a colour is saturated red.
	RedSaturation float64 `json:"redSaturation"`
	// RegionWidth and RegionHeight are the size, in pixels, of the regions in
	// which the flashing area is measured. Zero measures it over the whole
	// frame.
	RegionWidth  int `json:"regionWidth"`
	RegionHeight int `json:"regionHeight"`
	// AreaPercent is the share of a region, in percent, that must transition
	// together for the region to transition.
	AreaPercent float64 `json:"areaPercent"`
	// MaxTransitions is the most alternating transitions a region may make
	// within one second. A flash is a pair of opposing transitions, so six
	// allows three flashes.
	MaxTransitions int `json:"maxTransitions"`
}

// WCAG22 returns the thresholds of WCAG 2.2 Success Criterion 2.3.1: the
// general flash and red flash thresholds, with the flashing area measured in
// any 341×256-pixel region, the reference for a quarter of a 10° visual field.
func WCAG22() Thresholds {
	return Thresholds{
		LuminanceChange: 0.1,
		DarkLimit:       0.8,
		RedChange:       0.2,
		RedSaturation:   0.8,
		RegionWidth:     341,
		RegionHeight:    256,
		AreaPercent:     25,
		MaxTransitions:  6,
	}
}

// Trace24 returns the thresholds of the photosensitive epilepsy guidelines
// proposed by the Trace Research & Development Center in 2024. They match
// WCAG 2.2 but measure the flashing area in any 416×416-pixel region.
func Trace24() Thresholds {
	th := WCAG22()
	th.RegionWidth, th.RegionHeight = 416, 416

	return th
}

// Broadcast returns the thresholds of the broadcast guidelines: ITU-R
// BT.1702, the Ofcom guidance note on flashing images and ISO 9241-391. A
// flash of 20 cd/m² on a 200 cd/m² display is a change of 0.1 in relative
// luminance, and the flashing area is a quarter of the whole screen.
func Broadcast() Thresholds {
	th := WCAG22()
	th.RegionWidth, th.RegionHeight = 0, 0

	return th
}

// SceneChange returns the thresholds of the experimental scene change hazard
// defined by the Trace RERC PSE test media: luminance changes of at least 0.1
// over at least 80% of the screen, more than three alternating transitions in
// one second.
func SceneChange() Thresholds {
	th := Broadcast()
	th.AreaPercent = 80
	th.MaxTransitions = 3

	return th
}

// DefaultThresholds returns the WCAG 2.2 thresholds.
func DefaultThresholds() Thresholds {
	return WCAG22()
}

// Flashes is the flash measurement of one kind in one frame.
type Flashes struct {
	// Transitions is the number of alternating transitions within the last
	// second of the region that transitioned in this frame with the most of
	// them.
	Transitions int `json:"transitions"`
	// Since is the frame number of the first of those transitions.
	Since int `json:"since,omitempty"`
	// Region is that region, in source pixels.
	Region image.Rectangle `json:"region"`
	// Area is the share of that region, in percent, that transitioned
	// together in this frame.
	Area float64 `json:"area"`
	// Failed reports that the region flashed too often.
	Failed bool `json:"failed"`
	// Flashing is the bounding box, in source pixels, of the flashing pixels
	// that start a transition in this frame, wherever they are in the frame.
	// Unlike Region, it follows the shape of what flashes. It is set only
	// when Failed.
	Flashing image.Rectangle `json:"flashing"`
	// Mask locates those pixels on a MaskColumns×MaskRows grid over the
	// frame, row by row from the top left: each cell holds the share, 0 to
	// 100, of its pixels that are among them. It is set only when Failed.
	Mask []byte `json:"mask,omitempty"`
}

// MaskColumns and MaskRows are the size of the grid of Flashes.Mask.
const (
	MaskColumns = 64
	MaskRows    = 36
)

// FrameResult is the analysis of one frame.
type FrameResult struct {
	Luminance Flashes `json:"luminance"`
	Red       Flashes `json:"red"`
}

// Failed reports whether the frame completes a failing flash sequence.
func (r *FrameResult) Failed() bool {
	return r.Luminance.Failed || r.Red.Failed
}
