package psv

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
)

// MaxFrameRate is the highest frame rate PSV analyses.
const MaxFrameRate = 240

var (
	// ErrFrameRate is returned for an unusable frame rate.
	ErrFrameRate = fmt.Errorf("psv: frame rate must be between 1 and %d fps", MaxFrameRate)
	// ErrFrameSize is returned for an unusable frame size.
	ErrFrameSize = errors.New("psv: invalid frame size")
	// ErrThresholds is returned for unusable thresholds.
	ErrThresholds = errors.New("psv: invalid thresholds")
)

// Config sets up an Analyzer.
type Config struct {
	// Thresholds are the detection parameters; the zero value selects
	// DefaultThresholds.
	Thresholds Thresholds
}

// Analyzer analyses a stream of frames. Whether a frame belongs to a
// hazardous sequence depends on the second of video after it, so each frame
// is decided one second after it is analysed. It is not safe for concurrent
// use.
type Analyzer struct {
	detect   *detector
	analysis Analysis
	finished bool
}

// NewAnalyzer starts the analysis of a video with the given frame rate and
// frame size. Fractional rates such as 29.97 are rounded.
func NewAnalyzer(fps float64, width, height int, cfg Config) (*Analyzer, error) {
	rate := int(math.Round(fps))
	if rate < 1 || rate > MaxFrameRate {
		return nil, fmt.Errorf("%w (got %.3f fps)", ErrFrameRate, fps)
	}

	if width < 1 || height < 1 {
		return nil, fmt.Errorf("%w: %dx%d", ErrFrameSize, width, height)
	}

	th := cfg.Thresholds
	if th == (Thresholds{}) {
		th = DefaultThresholds()
	}

	if th.MaxTransitions < 0 || th.MaxTransitions >= math.MaxUint8 ||
		th.AreaPercent <= 0 || th.AreaPercent > 100 || th.RegionWidth < 0 || th.RegionHeight < 0 {
		return nil, fmt.Errorf("%w: %+v", ErrThresholds, th)
	}

	s := newSampler(width, height, th.RedSaturation)

	return &Analyzer{
		detect: newDetector(rate, s, th),
		analysis: Analysis{
			FrameRate:  rate,
			Width:      width,
			Height:     height,
			Thresholds: th,
		},
	}, nil
}

// ErrFinished is returned when frames are analysed after Finish.
var ErrFinished = errors.New("psv: analysis is finished")

// Analyze analyses the next frame. The frame must have the size the analyzer
// was created with. The frames it decides are appended to Analysis().Frames;
// it returns how many.
func (a *Analyzer) Analyze(f *Frame) (int, error) {
	if a.finished {
		return 0, ErrFinished
	}

	if err := f.validate(a.analysis.Width, a.analysis.Height); err != nil {
		return 0, err
	}

	decided := a.detect.process(f)
	a.analysis.Frames = append(a.analysis.Frames, decided...)

	return len(decided), nil
}

// Finish decides the remaining frames and returns the complete analysis.
func (a *Analyzer) Finish() *Analysis {
	if !a.finished {
		a.finished = true
		a.analysis.Frames = append(a.analysis.Frames, a.detect.flush()...)
	}

	return &a.analysis
}

// Analysis returns the analysis of the frames decided so far.
func (a *Analyzer) Analysis() *Analysis {
	return &a.analysis
}

// Progress reports progress during AnalyzeFile: the number of frames read,
// starting at 1, and the expected total, or zero when unknown.
type Progress func(frame, total int)

// Options configures AnalyzeFile.
type Options struct {
	Config

	// FrameRate, when nonzero, converts the video to this frame rate before
	// analysis.
	FrameRate int
	// Deinterlace deinterlaces frames flagged as interlaced.
	Deinterlace bool
	// Tools locates FFmpeg.
	Tools Tools
	// Progress, when set, is called after each frame.
	Progress Progress
}

// AnalyzeFile analyses every frame of a video file.
func AnalyzeFile(ctx context.Context, path string, opts Options) (*Analysis, error) {
	info, err := Probe(ctx, path, opts.Tools)
	if err != nil {
		return nil, err
	}

	fps, total := info.FrameRate.Float64(), info.Frames
	if opts.FrameRate > 0 {
		fps = float64(opts.FrameRate)
		total = int(math.Round(info.Duration.Seconds() * fps))
	}

	a, err := NewAnalyzer(fps, info.Width, info.Height, opts.Config)
	if err != nil {
		return nil, err
	}

	dec, err := Decode(ctx, path, info, opts.Tools, DecodeOptions{
		FrameRate:   opts.FrameRate,
		Deinterlace: opts.Deinterlace,
	})
	if err != nil {
		return nil, err
	}

	runErr := a.run(ctx, dec, total, opts.Progress)
	if err := errors.Join(runErr, dec.Close()); err != nil {
		return nil, err
	}

	return a.Finish(), nil
}

func (a *Analyzer) run(ctx context.Context, dec *Decoder, total int, progress Progress) error {
	for n := 1; ; n++ {
		f, err := dec.Next()
		if errors.Is(err, io.EOF) {
			return ctx.Err()
		}

		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}

			return err
		}

		if _, err := a.Analyze(f); err != nil {
			return err
		}

		if progress != nil {
			progress(n, total)
		}
	}
}
