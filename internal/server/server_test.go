package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/brian-nunez/psv"
)

// The small generated clips in testdata/clips: these tests check the API's
// wiring, not the detection, which the conformance suite and the example
// videos cover.
const clips = "../../testdata/clips"

func needFFmpeg(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found on PATH")
	}
}

func newTestServer(t *testing.T, cfg Config) (*Server, *httptest.Server) {
	t.Helper()

	if cfg.TempDir == "" {
		cfg.TempDir = t.TempDir()
	}

	s := New(cfg)
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)

	return s, ts
}

func post(ctx context.Context, t *testing.T, url string, body io.Reader) *http.Response {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Set("Content-Type", "video/mp4")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// assertEmpty waits for the upload directory to be emptied.
func assertEmpty(t *testing.T, dir string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)

	for {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}

		if len(entries) == 0 {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("upload directory still holds %d files", len(entries))
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func TestAnalyzeStreamsReport(t *testing.T) {
	needFFmpeg(t)

	dir, video := t.TempDir(), filepath.Join(clips, "fail.mp4")
	_, ts := newTestServer(t, Config{Version: "test", TempDir: dir})

	res := post(context.Background(), t, ts.URL+"/v1/analyses?standard=wcag22&name=clip.mp4",
		bytes.NewReader(readFile(t, video)))
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/x-ndjson" {
		t.Fatalf("status %s, content type %q", res.Status, res.Header.Get("Content-Type"))
	}

	var events []Event

	sc := bufio.NewScanner(res.Body)
	sc.Buffer(nil, 64<<20)

	for sc.Scan() {
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("event %q: %v", sc.Bytes(), err)
		}

		events = append(events, e)
	}

	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}

	if len(events) < 3 || events[0].Type != "received" || events[1].Type != "progress" {
		t.Fatalf("events start %+v", events[:min(2, len(events))])
	}

	last := events[len(events)-1]
	if last.Type != "report" || last.Report == nil {
		t.Fatalf("last event %+v, want a report", last)
	}

	got := last.Report
	if got.Schema != psv.ReportSchema || got.Tool.Version != "test" || got.Source != "clip.mp4" || got.Standard != "wcag22" {
		t.Errorf("report header %q %+v %q %q", got.Schema, got.Tool, got.Source, got.Standard)
	}

	if len(got.Results) != got.Frames {
		t.Errorf("%d per-frame results for %d frames", len(got.Results), got.Frames)
	}

	// The API must report exactly what the library reports for the same file.
	analysis, err := psv.AnalyzeFile(context.Background(), video, psv.Options{Config: psv.Config{Thresholds: psv.WCAG22()}})
	if err != nil {
		t.Fatal(err)
	}

	want := analysis.Report(true)
	if got.Verdict != psv.Fail || got.Verdict != want.Verdict || got.Failures != want.Failures ||
		!reflect.DeepEqual(got.Defects, want.Defects) || !reflect.DeepEqual(got.Results, want.Results) {
		t.Errorf("API report differs from the library's: verdict %v/%v, failures %+v/%+v, defects %+v/%+v",
			got.Verdict, want.Verdict, got.Failures, want.Failures, got.Defects, want.Defects)
	}

	assertEmpty(t, dir)
}

func TestAnalyzeRejects(t *testing.T) {
	needFFmpeg(t)

	dir := t.TempDir()
	_, ts := newTestServer(t, Config{MaxUploadBytes: 1 << 20, TempDir: dir})
	big := bytes.Repeat([]byte{0}, 2<<20)

	for _, c := range []struct {
		name   string
		query  string
		body   io.Reader
		status int
		error  string
	}{
		{"unknown standard", "?standard=nope", strings.NewReader("x"), http.StatusBadRequest, "unknown standard"},
		{"empty body", "", strings.NewReader(""), http.StatusBadRequest, "empty"},
		{"too large", "", bytes.NewReader(big), http.StatusRequestEntityTooLarge, "limit"},
		// Without a length the limit is found while reading.
		{"too large, chunked", "", io.MultiReader(bytes.NewReader(big)), http.StatusRequestEntityTooLarge, "limit"},
		{"not a video", "?name=notes.txt", strings.NewReader("just some text"), http.StatusUnprocessableEntity, "not a readable video"},
	} {
		t.Run(c.name, func(t *testing.T) {
			res := post(context.Background(), t, ts.URL+"/v1/analyses"+c.query, c.body)
			defer func() { _ = res.Body.Close() }()

			var body struct{ Error string }
			if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}

			if res.StatusCode != c.status || !strings.Contains(body.Error, c.error) {
				t.Errorf("got %s %q, want %d containing %q", res.Status, body.Error, c.status, c.error)
			}

			if strings.Contains(body.Error, dir) {
				t.Errorf("error %q reveals the upload path", body.Error)
			}

			assertEmpty(t, dir)
		})
	}
}

func TestAnalyzeBusy(t *testing.T) {
	s, ts := newTestServer(t, Config{MaxJobs: 1})
	s.slots <- struct{}{}

	res := post(context.Background(), t, ts.URL+"/v1/analyses", strings.NewReader("x"))
	_ = res.Body.Close()

	if res.StatusCode != http.StatusTooManyRequests {
		t.Errorf("got %s, want 429", res.Status)
	}
}

func TestAnalyzeCancelledByClient(t *testing.T) {
	needFFmpeg(t)

	dir := t.TempDir()
	s, ts := newTestServer(t, Config{TempDir: dir, MaxJobs: 1})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	res := post(ctx, t, ts.URL+"/v1/analyses",
		bytes.NewReader(readFile(t, filepath.Join(clips, "long.mp4"))))

	line, err := bufio.NewReader(res.Body).ReadBytes('\n')
	if err != nil || !bytes.Contains(line, []byte(`"received"`)) {
		t.Fatalf("first event %q, %v", line, err)
	}

	cancel()

	_ = res.Body.Close()

	assertEmpty(t, dir)

	// The analysis slot is released once the handler returns.
	deadline := time.Now().Add(10 * time.Second)
	for len(s.slots) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("analysis slot was not released")
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func TestServesUI(t *testing.T) {
	_, ts := newTestServer(t, Config{})

	for path, want := range map[string]string{
		"/":        `<script src="app.js"></script>`,
		"/app.js":  `const SCHEMA = "psv.report/v1";`,
		"/app.css": "--accent",
		"/healthz": `"ok"`,
	} {
		res, err := http.Get(ts.URL + path) //nolint:noctx // test against a local server
		if err != nil {
			t.Fatal(err)
		}

		b, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()

		if res.StatusCode != http.StatusOK || !strings.Contains(string(b), want) {
			t.Errorf("GET %s: %s, body lacks %q", path, res.Status, want)
		}
	}
}

func TestSourceName(t *testing.T) {
	for in, want := range map[string]string{
		"clip.mp4":               "clip.mp4",
		"/etc/passwd":            "passwd",
		`C:\videos\clip.mov`:     "clip.mov",
		"":                       "video",
		"../":                    "..",
		string([]byte{0xff}):     "video",
		strings.Repeat("a", 300): strings.Repeat("a", 200),
	} {
		if got := sourceName(in); got != want {
			t.Errorf("sourceName(%q) = %q, want %q", in, got, want)
		}
	}

	for in, want := range map[string]string{"a.MP4": ".mp4", "a.tar.gz": ".gz", "a": "", "a.m p4": "", "a.verylongext": ""} {
		if got := extension(in); got != want {
			t.Errorf("extension(%q) = %q, want %q", in, got, want)
		}
	}
}
