package main

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brian-nunez/psv"
	"github.com/brian-nunez/psv/internal/ui"
)

// The small generated clips in testdata/clips: these tests check the CLI's
// wiring, not the detection, which the conformance suite and the example
// videos cover.
const clips = "../../testdata/clips"

var (
	passing = filepath.Join(clips, "pass.mp4")
	failing = filepath.Join(clips, "fail.mp4")
)

func needFFmpeg(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found on PATH")
	}
}

func runPSV(t *testing.T, stdin []byte, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	var out, errOut bytes.Buffer

	code = run(args, bytes.NewReader(stdin), &out, &errOut)

	return code, out.String(), errOut.String()
}

func TestAnalyzeExitStatus(t *testing.T) {
	needFFmpeg(t)

	cases := []struct {
		name string
		args []string
		code int
	}{
		{"pass", []string{"-progress=false", passing}, exitPass},
		{"pass, analyze command", []string{"analyze", "-progress=false", passing}, exitPass},
		{"fail", []string{"-progress=false", failing}, exitFail},
		{"missing file", []string{"-progress=false", "no-such-video.mp4"}, exitError},
		{"no video", []string{}, exitError},
		{"unknown flag", []string{"-nope", passing}, exitError},
		{"unknown standard", []string{"-standard", "nope", passing}, exitError},
		{"unknown format", []string{"-format", "xml", passing}, exitError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if code, _, stderr := runPSV(t, nil, c.args...); code != c.code {
				t.Errorf("exit %d, want %d; stderr %s", code, c.code, stderr)
			}
		})
	}
}

func TestAnalyzeJSONReport(t *testing.T) {
	needFFmpeg(t)

	code, out, stderr := runPSV(t, nil, "-progress=false", "-format", "json", passing)
	if code != exitPass {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	rep, err := psv.DecodeReport(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}

	if rep.Tool.Name != "psv" || rep.Tool.Version != version || rep.Source != "pass.mp4" || rep.Standard != "wcag22" {
		t.Errorf("header %+v %q %q", rep.Tool, rep.Source, rep.Standard)
	}

	if rep.Results != nil {
		t.Error("per-frame results without -frames")
	}

	_, out, _ = runPSV(t, nil, "-progress=false", "-format", "json", "-standard", "trace24", "-area", "30", passing)

	if rep, err = psv.DecodeReport(strings.NewReader(out)); err != nil || rep.Standard != "custom" {
		t.Errorf("overridden thresholds: standard %q, %v; want custom", rep.Standard, err)
	}
}

// TestRenderShowsReportUI pipes the analysis of a failing clip, which has
// flashing masks to show, into psv render and checks the page is the report UI
// with the report inlined.
func TestRenderShowsReportUI(t *testing.T) {
	needFFmpeg(t)

	code, report, stderr := runPSV(t, nil, "-progress=false", "-format", "json", "-frames", failing)
	if code != exitFail {
		t.Fatalf("analysis exit %d: %s", code, stderr)
	}

	code, page, stderr := runPSV(t, []byte(report), "render")
	if code != exitPass {
		t.Fatalf("render exit %d: %s", code, stderr)
	}

	js, err := fs.ReadFile(ui.Files, "app.js")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(page, string(js)) || !strings.Contains(page, `id="psv-report"`) || !strings.Contains(page, `"fail.mp4"`) {
		t.Error("rendered page lacks the report UI script or the report")
	}

	// The same page from a file, written with -o.
	dir := t.TempDir()
	in, out := filepath.Join(dir, "report.json"), filepath.Join(dir, "report.html")

	if err := os.WriteFile(in, []byte(report), 0o600); err != nil {
		t.Fatal(err)
	}

	if code, _, stderr := runPSV(t, nil, "render", "-o", out, in); code != exitPass {
		t.Fatalf("render -o exit %d: %s", code, stderr)
	}

	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}

	if string(written) != page {
		t.Error("render -o wrote a different page than standard output")
	}
}

func TestRenderRejects(t *testing.T) {
	needFFmpeg(t)

	_, summary, _ := runPSV(t, nil, "-progress=false", "-format", "json", passing)

	for _, c := range []struct {
		name, stdin string
		args        []string
		error       string
	}{
		{"no per-frame results", summary, []string{"render"}, "-frames"},
		{"other JSON", `{"schemaVersion":"1.0"}`, []string{"render"}, "psv.report/v1"},
		{"not JSON", "hello", []string{"render", "-"}, "decode report"},
		{"missing file", "", []string{"render", "no-such-report.json"}, "no-such-report.json"},
		{"two files", "", []string{"render", "a.json", "b.json"}, "usage"},
	} {
		t.Run(c.name, func(t *testing.T) {
			code, out, stderr := runPSV(t, []byte(c.stdin), c.args...)
			if code != exitError || out != "" || !strings.Contains(stderr, c.error) {
				t.Errorf("exit %d, stdout %d bytes, stderr %q; want exit 2 naming %q", code, len(out), stderr, c.error)
			}
		})
	}
}

func TestServeRejectsArguments(t *testing.T) {
	if code, _, stderr := runPSV(t, nil, "serve", "extra"); code != exitError || !strings.Contains(stderr, "usage") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}
