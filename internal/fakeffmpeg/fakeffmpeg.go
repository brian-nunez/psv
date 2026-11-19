// Package fakeffmpeg stands in for ffmpeg and ffprobe in unit tests.
package fakeffmpeg

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const (
	integrationEnv = "PSV_TEST_INTEGRATION"

	width  = 320
	height = 180
	rate   = 30
)

type clip struct {
	frames int
	render func(pix []byte, n int)
}

var clips = map[string]clip{
	"2ac450a05e8d6285d2a5aedb7dd19e7eac29a2128865b7f43d65d6da8fa9dd45": {frames: 90, render: bars},
	"c142314afa4da98dedfd659b1e3d2a363cd155a8ff9f8963255a4915713cae61": {frames: 90, render: flashes},
	"24c8774247c91ff213c054e23ab41f196b7d8d625d7b9df48dc1c579bd64e4ff": {frames: 1800, render: bars},
}

// Run runs the tests with a fake ffmpeg and ffprobe first on PATH, unless
// PSV_TEST_INTEGRATION is set; call it from TestMain.
func Run(m *testing.M) int {
	switch filepath.Base(os.Args[0]) {
	case "ffprobe":
		return probe(os.Args[1:], os.Stdout, os.Stderr)
	case "ffmpeg":
		return decode(os.Args[1:], os.Stdout, os.Stderr)
	}

	if os.Getenv(integrationEnv) != "" {
		return m.Run()
	}

	dir, err := install()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "fake ffmpeg:", err)

		return 1
	}

	defer func() { _ = os.RemoveAll(dir) }()

	return m.Run()
}

func install() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	dir, err := os.MkdirTemp("", "fakeffmpeg")
	if err != nil {
		return "", err
	}

	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if err := os.Symlink(exe, filepath.Join(dir, name)); err != nil {
			return dir, err
		}
	}

	return dir, os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func lookup(path string, stderr io.Writer) (clip, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%s: No such file or directory\n", path)

		return clip{}, false
	}

	sum := sha256.Sum256(data)

	c, ok := clips[hex.EncodeToString(sum[:])]
	if !ok {
		_, _ = fmt.Fprintf(stderr, "%s: Invalid data found when processing input\n", path)
	}

	return c, ok
}

func probe(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "fake ffprobe: no input")

		return 1
	}

	c, ok := lookup(args[len(args)-1], stderr)
	if !ok {
		return 1
	}

	duration := fmt.Sprintf("%.6f", float64(c.frames)/rate)

	doc := map[string]any{
		"streams": []map[string]any{{
			"codec_name":     "h264",
			"width":          width,
			"height":         height,
			"r_frame_rate":   fmt.Sprintf("%d/1", rate),
			"avg_frame_rate": fmt.Sprintf("%d/1", rate),
			"duration":       duration,
			"nb_frames":      fmt.Sprint(c.frames),
		}},
		"format": map[string]any{"duration": duration},
	}

	if err := json.NewEncoder(stdout).Encode(doc); err != nil {
		_, _ = fmt.Fprintln(stderr, "fake ffprobe:", err)

		return 1
	}

	return 0
}

func decode(args []string, stdout, stderr io.Writer) int {
	i := slices.Index(args, "-i")
	if i < 0 || i+1 >= len(args) {
		_, _ = fmt.Fprintln(stderr, "fake ffmpeg: no input")

		return 1
	}

	c, ok := lookup(args[i+1], stderr)
	if !ok {
		return 1
	}

	out := bufio.NewWriterSize(stdout, width*height*3)
	pix := make([]byte, width*height*3)

	for n := range c.frames {
		c.render(pix, n)

		if _, err := out.Write(pix); err != nil {
			return 1
		}
	}

	if err := out.Flush(); err != nil {
		return 1
	}

	return 0
}

func bars(pix []byte, _ int) {
	colors := [7][3]byte{
		{191, 191, 191},
		{191, 191, 0},
		{0, 191, 191},
		{0, 191, 0},
		{191, 0, 191},
		{191, 0, 0},
		{0, 0, 191},
	}

	for y := range height {
		for x := range width {
			copy(pix[(y*width+x)*3:], colors[x*len(colors)/width][:])
		}
	}
}

func flashes(pix []byte, n int) {
	var c [3]byte

	switch {
	case n%6 >= 3:
	case n < 45:
		c = [3]byte{255, 255, 255}
	default:
		c = [3]byte{255, 0, 0}
	}

	for i := 0; i < len(pix); i += 3 {
		copy(pix[i:], c[:])
	}
}
