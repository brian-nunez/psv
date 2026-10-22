package psv_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/brian-nunez/psv"
)

// The conformance suite renders the benchmark videos of the Trace RERC PSE
// test media (https://github.com/traceRERC/pse-test-media) frame by frame,
// exactly as its generator composes them, and checks every verdict. Each set
// declares the standards it targets in its README and lists every video's
// result for them in its listing CSV, whose dimension column names the
// threshold a passing video stays under, or the failure (FAIL, fail_red,
// fail_luminance, fail_both). With -short only shortSets run. Fetch the
// media with `make testdata`.

const mediaDir = "testdata/pse-test-media"

// padding is the number of copies of the first frame the generator puts in
// front of every video (video_config.json).
const padding = 10

// standards maps the standard names used by the media to PSV presets. The
// broadcast guidelines share one preset; "none" marks the set for the
// experimental scene change hazard its README defines.
var standards = map[string]struct {
	preset string
	th     func() psv.Thresholds
}{
	"wcag2":     {"wcag22", psv.WCAG22},
	"trace24":   {"trace24", psv.Trace24},
	"iso":       {"broadcast", psv.Broadcast},
	"itu_r1702": {"broadcast", psv.Broadcast},
	"itu-r1702": {"broadcast", psv.Broadcast},
	"ofcom":     {"broadcast", psv.Broadcast},
	"none":      {"scene", psv.SceneChange},
}

// upstreamErrors lists listing entries that contradict the thresholds they
// claim to test.
var upstreamErrors = map[string]string{
	"wcagc_30fps_area01/f011f014":              errF011,
	"wcagc_30fps_area03/f011f005":              errF011,
	"wcagc_30fps_area02/f012fr014":             errF012,
	"wcagc_30fps_area03/f012fr013":             errF012,
	"trace24_30fps_inf01/f003cr013a_a004fr013": errCR013Alt,
	"trace24_30fps_inf01/f011cr013a_f007cr013": errCR013Alt,
	"broadcast_30fps_inf02/f003tf01_f005ico01": "listed as failing, but its 01 patterns show grey 90 on a grey 64 " +
		"background, a change of 0.05 relative luminance; the folder's README states every flash exceeds 0.1, " +
		"which holds only on the black background a003tf01_f008ico01 uses",
	"trace24_30fps_sync01/05m90f033_05n10f033": "listed as passing, but the set's README defines m90 with n10 as slightly " +
		"more than the failing area and both areas flash the same failing sequence in sync; the video's own JSON expects a failure",
	"trace24_30fps_sync01/05m70f007_05n30ff007": "listed as failing, but m70 with n30 only exceeds the area together and the ff " +
		"pattern moves one transition a frame out of sync; the video's own JSON expects a pass",
}

const (
	errF011 = "area pattern f011 flashes 21282 pixels, under 25% of 341×256 (21824) in any placement, " +
		"the same as the passing a011"
	errF012 = "area pattern f012 flashes 21402 pixels, under 25% of 341×256 (21824) in any placement, " +
		"the same as the passing a012"
	errCR013Alt = "listed as passing on count, but its cr013alt area alternates by 0.10 relative luminance " +
		"at frames 4, 5, 11, 12, 21, 22 and 31 of the pattern, seven transitions within one second"
)

// shortSets cover luminance, red, area and synchronisation with -short.
var shortSets = map[string]bool{
	"30fps_alternating_01": true,
	"wcagc_30fps_area03":   true,
	"trace24_30fps_sync01": true,
}

type videoSpec struct {
	FrameRate float64 `json:"framerate"`
	BGColor   string  `json:"bgcolor"`
	Pattern   []struct {
		Spatial       string `json:"spatial"`
		TemporalColor string `json:"temporal_color"`
	} `json:"pattern"`
}

func TestConformance(t *testing.T) {
	sets, err := filepath.Glob(filepath.Join(mediaDir, "video_creation", "*", "README.md"))
	if err != nil {
		t.Fatal(err)
	}

	if len(sets) == 0 {
		t.Skipf("benchmark media not found in %s; run `make testdata`", mediaDir)
	}

	masks := &maskCache{masks: map[string]*mask{}}

	for _, readme := range sets {
		dir := filepath.Dir(readme)
		set := filepath.Base(dir)

		if testing.Short() && !shortSets[set] {
			continue
		}

		targets := setStandards(t, readme)
		listing := readListing(t, filepath.Join(dir, set+".csv"))

		for _, std := range targets {
			preset := standards[std]

			for _, video := range listing {
				name := set + "/" + video.name

				t.Run(preset.preset+"/"+name, func(t *testing.T) {
					t.Parallel()

					if reason, ok := upstreamErrors[name]; ok {
						t.Skip("upstream listing error: " + reason)
					}

					spec := readSpec(t, filepath.Join(dir, video.name+".json"))
					analysis := analyze(t, dir, spec, preset.th(), masks)

					if got := analysis.Verdict().Passed(); got != video.pass {
						t.Errorf("verdict %s, want %s (%s)", analysis.Verdict(), verdictName(video.pass), peak(analysis))
					}
				})
			}
		}
	}
}

func verdictName(pass bool) string {
	if pass {
		return "pass"
	}

	return "fail"
}

// setStandards returns the standards a set's README declares, one per PSV
// preset.
func setStandards(t *testing.T, readme string) []string {
	t.Helper()

	b, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}

	var out []string

	for _, line := range strings.Split(string(b), "\n") {
		if !strings.Contains(line, "**Standard") {
			continue
		}

		seen := map[string]bool{}

		for _, field := range strings.FieldsFunc(line[strings.Index(line, ":**")+3:], func(r rune) bool {
			return r == ',' || r == ' '
		}) {
			if s, ok := standards[field]; ok && !seen[s.preset] {
				seen[s.preset] = true

				out = append(out, field)
			}
		}

		break
	}

	return out
}

type listedVideo struct {
	name string
	pass bool
}

// readListing reads a set's listing: a video passes unless its dimension
// names a failure.
func readListing(t *testing.T, path string) []listedVideo {
	t.Helper()

	rows := readCSV(t, path)
	dim := -1

	for i, name := range rows[0] {
		if strings.TrimSpace(name) == "dimension" {
			dim = i
		}
	}

	if dim < 0 {
		t.Fatalf("%s: no dimension column", path)
	}

	videos := make([]listedVideo, 0, len(rows)-1)
	for _, row := range rows[1:] {
		videos = append(videos, listedVideo{
			name: strings.TrimSpace(row[0]),
			pass: !strings.HasPrefix(strings.ToLower(strings.TrimSpace(row[dim])), "fail"),
		})
	}

	return videos
}

// peak describes the most transitions and largest area any frame reached.
func peak(a *psv.Analysis) string {
	var lum, red psv.Flashes

	for _, r := range a.Frames {
		lum.Transitions = max(lum.Transitions, r.Luminance.Transitions)
		lum.Area = max(lum.Area, r.Luminance.Area)
		red.Transitions = max(red.Transitions, r.Red.Transitions)
		red.Area = max(red.Area, r.Red.Area)
	}

	return fmt.Sprintf("luminance: %d transitions, %.1f%% area; red: %d transitions, %.1f%% area",
		lum.Transitions, lum.Area, red.Transitions, red.Area)
}

func readSpec(t *testing.T, path string) videoSpec {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var spec videoSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatalf("%s: %v", path, err)
	}

	return spec
}

// layer is one spatial pattern recoloured frame by frame.
type layer struct {
	mask   *mask
	colors []rgba
}

type rgba struct{ r, g, b, a uint8 }

func analyze(t *testing.T, dir string, spec videoSpec, th psv.Thresholds, masks *maskCache) *psv.Analysis {
	t.Helper()

	bg := parseColor(t, spec.BGColor)
	layers := make([]layer, len(spec.Pattern))
	frames := 0

	for i, p := range spec.Pattern {
		layers[i] = layer{
			mask:   masks.load(t, filepath.Join(dir, p.Spatial)),
			colors: readColors(t, filepath.Join(dir, p.TemporalColor)),
		}
		frames = max(frames, len(layers[i].colors))
	}

	w, h := layers[0].mask.width, layers[0].mask.height

	a, err := psv.NewAnalyzer(spec.FrameRate, w, h, psv.Config{Thresholds: th})
	if err != nil {
		t.Fatal(err)
	}

	frame := &psv.Frame{Width: w, Height: h, Pix: make([]byte, w*h*3)}

	for n := -padding; n < frames; n++ {
		render(frame, layers, max(n, 0), bg)

		if _, err := a.Analyze(frame); err != nil {
			t.Fatal(err)
		}
	}

	return a.Finish()
}

// render composes frame n: the background, then every layer that has a
// colour for the frame, later layers on top. Colours are fully opaque or
// fully transparent.
func render(f *psv.Frame, layers []layer, n int, bg rgba) {
	for i := 0; i < len(f.Pix); i += 3 {
		f.Pix[i], f.Pix[i+1], f.Pix[i+2] = bg.r, bg.g, bg.b
	}

	for _, l := range layers {
		if n >= len(l.colors) || l.colors[n].a == 0 {
			continue
		}

		c := l.colors[n]
		for _, i := range l.mask.pixels {
			f.Pix[i*3], f.Pix[i*3+1], f.Pix[i*3+2] = c.r, c.g, c.b
		}
	}
}

func parseColor(t *testing.T, s string) rgba {
	t.Helper()

	parts := strings.Split(strings.Trim(s, "()"), ",")
	if len(parts) != 4 {
		t.Fatalf("bad colour %q", s)
	}

	var v [4]uint8

	for i, p := range parts {
		n, err := strconv.ParseUint(strings.TrimSpace(p), 10, 8)
		if err != nil {
			t.Fatalf("bad colour %q: %v", s, err)
		}

		v[i] = uint8(n)
	}

	return rgba{v[0], v[1], v[2], v[3]}
}

func readColors(t *testing.T, path string) []rgba {
	t.Helper()

	rows := readCSV(t, path)

	if len(rows) < 2 {
		t.Fatalf("%s: no frames", path)
	}

	col := map[string]int{}
	for i, name := range rows[0] {
		col[strings.TrimSpace(name)] = i
	}

	colors := make([]rgba, 0, len(rows)-1)

	for _, row := range rows[1:] {
		var v [4]uint8

		for i, name := range []string{"r", "g", "b", "a"} {
			n, err := strconv.ParseUint(strings.TrimSpace(row[col[name]]), 10, 8)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}

			v[i] = uint8(n)
		}

		colors = append(colors, rgba{v[0], v[1], v[2], v[3]})
	}

	return colors
}

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	rows, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}

	return rows
}

// mask lists the opaque pixels of a spatial pattern.
type mask struct {
	width, height int
	pixels        []int
}

type maskCache struct {
	mu    sync.Mutex
	masks map[string]*mask
}

func (c *maskCache) load(t *testing.T, path string) *mask {
	t.Helper()

	c.mu.Lock()
	defer c.mu.Unlock()

	if m, ok := c.masks[path]; ok {
		return m
	}

	m, err := readMask(path)
	if err != nil {
		t.Fatal(err)
	}

	c.masks[path] = m

	return m
}

func readMask(path string) (*mask, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	b := img.Bounds()
	m := &mask{width: b.Dx(), height: b.Dy()}

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
				m.pixels = append(m.pixels, (y-b.Min.Y)*m.width+(x-b.Min.X))
			}
		}
	}

	return m, nil
}
