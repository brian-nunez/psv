package psv_test

import (
	"context"
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
// an intended change in results.
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

			got := analysis.Report(false)

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
					t.Errorf("%s: got %+v, want %+v", c.name, c.got, c.want)
				}
			}
		})
	}
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
