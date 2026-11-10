package psv_test

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/brian-nunez/psv"
	presets "github.com/brian-nunez/psv/internal/standards"
)

// examplesDir holds real-world videos, each with the psv.report/v1 report of
// its last accepted analysis. TestExamples decodes them with FFmpeg and checks
// the analysis still matches; `make examples` regenerates the reports after
// an intended change in results. Each video is also watched live, through
// Analyzer.Watch, and must give the same report.
//
// Decoding full-resolution footage is slow, so TestExamples runs only when
// PSV_TEST_EXAMPLES is set: `make test-examples` locally, or the Examples
// workflow in CI, which runs one job per video.
const examplesDir = "testdata/examples"

func TestExamples(t *testing.T) {
	if os.Getenv("PSV_TEST_EXAMPLES") == "" {
		t.Skip("set PSV_TEST_EXAMPLES=1, or run make test-examples, to decode the full-resolution example videos")
	}

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found on PATH")
	}

	dirs, err := filepath.Glob(filepath.Join(examplesDir, "*", "video.mp4"))
	if err != nil {
		t.Fatal(err)
	}

	if len(dirs) == 0 {
		t.Fatalf("no example videos in %s", examplesDir)
	}

	for _, video := range dirs {
		dir := filepath.Dir(video)

		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			want := readReport(t, filepath.Join(dir, "report.json"))

			th, err := presets.Lookup(want.Standard)
			if err != nil {
				t.Fatal(err)
			}

			if th != want.Thresholds {
				t.Fatalf("report thresholds %+v differ from preset %s %+v", want.Thresholds, want.Standard, th)
			}

			analysis, err := psv.AnalyzeFile(context.Background(), video, psv.Options{Config: psv.Config{Thresholds: th}})
			if err != nil {
				t.Fatal(err)
			}

			compareReport(t, "Analyze", analysis.Report(false), want)

			watched, leads := watchFile(t, video, th)
			compareReport(t, "Watch", watched.Report(false), want)
			t.Logf("live ahead of the decided failure by %v frames", leads)
		})
	}
}

func compareReport(t *testing.T, how string, got psv.Report, want *psv.Report) {
	t.Helper()

	for _, c := range []struct {
		name      string
		got, want any
	}{
		{"verdict", got.Verdict, want.Verdict},
		{"frame rate", got.FrameRate, want.FrameRate},
		{"frames", got.Frames, want.Frames},
		{"size", [2]int{got.Width, got.Height}, [2]int{want.Width, want.Height}},
		{"failures", got.Failures, want.Failures},
		{"defects", got.Defects, want.Defects},
	} {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s: %s: got %+v, want %+v", how, c.name, c.got, c.want)
		}
	}
}

// watchFile decodes a video and watches it live, rebuilding its analysis
// from the decided frames of each Update. It returns the analysis and how
// many frames the live measurement came before each decided failure.
func watchFile(t *testing.T, path string, th psv.Thresholds) (*psv.Analysis, []int) {
	t.Helper()

	ctx := context.Background()

	info, err := psv.Probe(ctx, path, psv.Tools{})
	if err != nil {
		t.Fatal(err)
	}

	dec, err := psv.Decode(ctx, path, info, psv.Tools{}, psv.DecodeOptions{})
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = dec.Close() }()

	a, err := psv.NewAnalyzer(info.FrameRate.Float64(), info.Width, info.Height, psv.Config{Thresholds: th})
	if err != nil {
		t.Fatal(err)
	}

	analysis := *a.Analysis()
	l := &lockstep{watch: a}

	for {
		f, err := dec.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			t.Fatal(err)
		}

		u, err := a.Watch(f)
		if err != nil {
			t.Fatal(err)
		}

		l.frames++

		for k, act := range []psv.Activity{u.Building.Luminance, u.Building.Red} {
			if act.Flashing > 0 && l.live[k] == 0 {
				l.live[k] = u.Frame
			}
		}

		analysis.Frames = append(analysis.Frames, u.Decided...)
	}

	u, err := a.Stop()
	if err != nil {
		t.Fatal(err)
	}

	analysis.Frames = append(analysis.Frames, u.Decided...)

	return &analysis, liveLead(t, &analysis, l)
}

func readReport(t *testing.T, path string) *psv.Report {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	rep, err := psv.DecodeReport(f)
	if err != nil {
		t.Fatal(err)
	}

	return rep
}
