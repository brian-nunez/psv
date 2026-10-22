// Command psv is the Photosafety Validator command-line tool. It analyses a
// video for flashing that can trigger photosensitive seizures.
//
// It exits with status 0 when the video passes, 1 when it fails and 2 on
// error.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/brian-nunez/psv"
)

var version = "dev"

const (
	exitPass  = 0
	exitFail  = 1
	exitError = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("psv", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printf(stderr, "usage: psv [flags] <video>\n\nflags:\n")
		fs.PrintDefaults()
	}

	standard := fs.String("standard", "wcag22", "threshold preset: wcag22, trace24, broadcast or scene")
	fps := fs.Int("fps", 0, "convert the video to this frame rate before analysis")
	deinterlace := fs.Bool("deinterlace", false, "deinterlace frames flagged as interlaced")
	lumChange := fs.Float64("luminance-change", 0, "relative luminance change of a luminance transition, 0 to 1")
	darkLimit := fs.Float64("dark-limit", 0, "relative luminance the darker side of a transition must be below")
	redChange := fs.Float64("red-change", 0, "u'v' chromaticity change of a red transition")
	redSaturation := fs.Float64("red-saturation", 0, "R/(R+G+B) at which a colour is saturated red")
	area := fs.Float64("area", 0, "share of a region, in percent, that must flash together")
	region := fs.String("region", "", "size of the area region, WIDTHxHEIGHT pixels, or 0x0 for the whole frame")
	transitions := fs.Int("transitions", -1, "most alternating transitions allowed in any one second")
	format := fs.String("format", "text", "output format: text or json")
	frames := fs.Bool("frames", false, "include every frame's result in JSON output")
	progress := fs.Bool("progress", isTerminal(os.Stderr), "show progress on stderr")
	ffmpeg := fs.String("ffmpeg", "", "path to ffmpeg (default: found on PATH)")
	ffprobe := fs.String("ffprobe", "", "path to ffprobe (default: found on PATH)")
	showVersion := fs.Bool("version", false, "print the version and exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitPass
		}

		return exitError
	}

	if *showVersion {
		printf(stdout, "psv %s\n", version)

		return exitPass
	}

	if fs.NArg() != 1 || (*format != "text" && *format != "json") {
		fs.Usage()

		return exitError
	}

	th, err := thresholds(*standard, *lumChange, *darkLimit, *redChange, *redSaturation, *area, *region, *transitions)
	if err != nil {
		printf(stderr, "%v\n", err)

		return exitError
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	opts := psv.Options{
		Config:      psv.Config{Thresholds: th},
		FrameRate:   *fps,
		Deinterlace: *deinterlace,
		Tools:       psv.Tools{FFmpeg: *ffmpeg, FFprobe: *ffprobe},
	}

	if *progress {
		opts.Progress = func(frame, total int) {
			if total > 0 {
				printf(stderr, "\ranalysing frame %d of %d (%d%%)", frame, total, min(100, frame*100/total))
			} else {
				printf(stderr, "\ranalysing frame %d", frame)
			}
		}
	}

	analysis, err := psv.AnalyzeFile(ctx, fs.Arg(0), opts)

	if *progress {
		printf(stderr, "\n")
	}

	if err != nil {
		printf(stderr, "%v\n", err)

		return exitError
	}

	report := analysis.Report(*frames)

	if err := write(stdout, *format, &report); err != nil {
		printf(stderr, "%v\n", err)

		return exitError
	}

	if !report.Passed {
		return exitFail
	}

	return exitPass
}

// thresholds starts from a standard's preset and applies the flags that were
// set; zero and negative values keep the preset.
func thresholds(standard string, lumChange, darkLimit, redChange, redSaturation, area float64, region string, transitions int) (psv.Thresholds, error) {
	var th psv.Thresholds

	switch standard {
	case "wcag22":
		th = psv.WCAG22()
	case "trace24":
		th = psv.Trace24()
	case "broadcast":
		th = psv.Broadcast()
	case "scene":
		th = psv.SceneChange()
	default:
		return th, fmt.Errorf("unknown standard %q", standard)
	}

	for _, o := range []struct {
		v   float64
		dst *float64
	}{
		{lumChange, &th.LuminanceChange},
		{darkLimit, &th.DarkLimit},
		{redChange, &th.RedChange},
		{redSaturation, &th.RedSaturation},
		{area, &th.AreaPercent},
	} {
		if o.v > 0 {
			*o.dst = o.v
		}
	}

	if region != "" {
		if _, err := fmt.Sscanf(region, "%dx%d", &th.RegionWidth, &th.RegionHeight); err != nil {
			return th, fmt.Errorf("invalid region %q: %w", region, err)
		}
	}

	if transitions >= 0 {
		th.MaxTransitions = transitions
	}

	return th, nil
}

func write(w io.Writer, format string, report *psv.Report) error {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")

		return enc.Encode(report)
	}

	return report.WriteText(w)
}

// printf writes diagnostics; a failed write to the terminal is not an error
// worth reporting.
func printf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()

	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
