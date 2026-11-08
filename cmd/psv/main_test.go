package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brian-nunez/psv"
)

const examples = "../../testdata/examples"

var (
	passing = filepath.Join(examples, "video-ad-sample-2", "video.mp4")
	failing = filepath.Join(examples, "colors-epilepsy-warning", "video.mp4")
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
		{"missing file", []string{"-progress=false", "no-such-video.mp4"}, exitError},
		{"no video", []string{}, exitError},
		{"unknown flag", []string{"-nope", passing}, exitError},
		{"unknown standard", []string{"-standard", "nope", passing}, exitError},
		{"unknown format", []string{"-format", "xml", passing}, exitError},
	}

	if !testing.Short() {
		cases = append(cases, struct {
			name string
			args []string
			code int
		}{"fail", []string{"-progress=false", failing}, exitFail})
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

	if rep.Tool.Name != "psv" || rep.Tool.Version != version || rep.Source != "video.mp4" || rep.Standard != "wcag22" {
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
