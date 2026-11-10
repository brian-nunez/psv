package psv

import (
	"errors"
	"image"
)

// WatchSchema identifies the format of an Update, the result of
// Analyzer.Watch. An incompatible change to Update is a new schema name.
const WatchSchema = "psv.watch/v1"

// WatchPixel is the method that measures Building from the transitions of
// single pixels, without regions; it is the default Config.WatchMethod.
const WatchPixel = "pixel"

var (
	// ErrMode is returned when Analyze and Watch are mixed on one Analyzer.
	ErrMode = errors.New("psv: Analyze and Watch cannot be mixed on one Analyzer")
	// ErrWatchMethod is returned for an unknown Config.WatchMethod.
	ErrWatchMethod = errors.New("psv: unknown watch method")
)

// Update is the result of watching one frame, in the WatchSchema format. It
// pairs what is known about the frame as it is seen, Building, with the
// frames decided by it, which match those of Analyze exactly.
type Update struct {
	// Schema is WatchSchema.
	Schema string `json:"schema"`
	// Method is the Config.WatchMethod that measured Building.
	Method string `json:"method"`
	// Frame is the number of the frame watched, from 1. It is zero in the
	// Update returned by Stop.
	Frame int `json:"frame"`
	// Building measures the flashing building up in this frame.
	Building Building `json:"building"`
	// Decided holds the frames decided by this frame: one frame per Update
	// once the first second has been seen, about a second behind Frame, and
	// the rest of the stream in the Update returned by Stop. They are the
	// final results.
	Decided []FrameResult `json:"decided,omitempty"`
	// DecidedFrom is the number of the first frame in Decided, or zero.
	DecidedFrom int `json:"decidedFrom,omitempty"`
}

// Building is the live measurement of one frame, available as the frame is
// watched. It is provisional: only Update.Decided is the verdict, and
// Building also measures flashing that never fails, such as flashing that
// covers too little of any region.
type Building struct {
	Luminance Activity `json:"luminance"`
	Red       Activity `json:"red"`
}

// Activity measures the pixels that start a transition of one kind in the
// frame being watched, each by its transitions within the second ending at
// that frame.
type Activity struct {
	// Peak is the most transitions within the last second, this one
	// included, of any of those pixels. Above Thresholds.MaxTransitions, a
	// pixel flashes.
	Peak int `json:"peak"`
	// Flashing is the share of the frame, in percent, of those pixels that
	// already flash. This is final: they count as flashing pixels in the
	// decided results, which can only add to them.
	Flashing float64 `json:"flashing"`
	// Area is the share of the frame, in percent, of all those pixels.
	Area float64 `json:"area"`
	// Bounds is the bounding box, in source pixels, of the pixels counted in
	// Flashing.
	Bounds image.Rectangle `json:"bounds"`
	// Mask locates the activity on the MaskColumns×MaskRows grid of
	// Flashes.Mask, row by row from the top left: each cell holds the Peak of
	// the pixels in it. It is nil when no pixel starts a transition.
	Mask []byte `json:"mask,omitempty"`
}

// Watch analyses the next frame of a live stream, such as a screen capture,
// and keeps no history: Analysis().Frames stays empty, so memory stays flat
// however long the stream runs. The Update measures the frame as it is seen
// and carries the frames it decides, which are identical to those Analyze
// would append. An Analyzer is used with either Analyze or Watch.
func (a *Analyzer) Watch(f *Frame) (Update, error) {
	if err := a.use(modeWatch); err != nil {
		return Update{}, err
	}

	if err := f.validate(a.analysis.Width, a.analysis.Height); err != nil {
		return Update{}, err
	}

	a.detect.live = true
	a.watched++

	u := a.update(a.detect.process(f))
	u.Frame = a.watched
	u.Building = a.detect.building

	return u, nil
}

// Stop ends a watch and returns the frames still waiting to be decided, the
// last second of the stream, as an Update with no Building. Later calls
// return an empty Update.
func (a *Analyzer) Stop() (Update, error) {
	if a.mode == modeAnalyze {
		return Update{}, ErrMode
	}

	if a.finished {
		return a.update(nil), nil
	}

	a.finished = true

	return a.update(a.detect.flush()), nil
}

// update wraps decided frames in an Update.
func (a *Analyzer) update(decided []FrameResult) Update {
	u := Update{Schema: WatchSchema, Method: a.method, Decided: decided}

	if len(decided) > 0 {
		u.DecidedFrom = a.decided + 1
		a.decided += len(decided)
	}

	return u
}
