package ui

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/brian-nunez/psv"
)

func testReport() *psv.Report {
	a := &psv.Analysis{FrameRate: 10, Width: 64, Height: 36, Thresholds: psv.WCAG22(), Frames: make([]psv.FrameResult, 12)}
	rep := a.Report(true)
	rep.Source = "clip.mp4"
	rep.Standard = "wcag22"

	return &rep
}

func asset(t *testing.T, name string) []byte {
	t.Helper()

	b, err := fs.ReadFile(Files, name)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// TestRenderMatchesServedUI checks that the page psv render writes carries
// the very files the server serves, so both show the same report UI.
func TestRenderMatchesServedUI(t *testing.T) {
	var page bytes.Buffer
	if err := Render(&page, testReport()); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(Handler())
	defer srv.Close()

	for _, name := range []string{"app.css", "app.js"} {
		served := get(t, srv.URL+"/"+name)
		if !bytes.Equal(served, asset(t, name)) {
			t.Errorf("served %s differs from the embedded file", name)
		}

		if !bytes.Contains(page.Bytes(), served) {
			t.Errorf("rendered page does not inline the served %s", name)
		}
	}

	index := get(t, srv.URL+"/")
	for _, tag := range []string{styleTag, scriptTag, reportTag} {
		if !bytes.Contains(index, []byte(tag)) {
			t.Errorf("served index.html lacks %s", tag)
		}

		if bytes.Contains(page.Bytes(), []byte(tag)) {
			t.Errorf("rendered page still contains %s", tag)
		}
	}
}

func TestRenderEmbedsReport(t *testing.T) {
	rep := testReport()
	rep.Source = `evil</script><script>alert(1)</script>.mp4`

	var page bytes.Buffer
	if err := Render(&page, rep); err != nil {
		t.Fatal(err)
	}

	html := page.String()
	if strings.Contains(html, "alert(1)</script>") {
		t.Fatal("report data can close its script element")
	}

	const open = `<script type="application/json" id="psv-report">`

	start := strings.Index(html, open)
	if start < 0 {
		t.Fatal("rendered page has no report element")
	}

	data, _, ok := strings.Cut(html[start+len(open):], "</script>")
	if !ok {
		t.Fatal("report element is not closed")
	}

	got, err := psv.DecodeReport(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	if got.Source != rep.Source || got.Frames != rep.Frames || len(got.Results) != rep.Frames {
		t.Errorf("embedded report %+v differs from %+v", got, rep)
	}
}

func TestRenderNeedsFrameResults(t *testing.T) {
	rep := testReport()
	rep.Results = nil

	if err := Render(io.Discard, rep); err == nil || !strings.Contains(err.Error(), "-frames") {
		t.Errorf("got %v, want an error naming -frames", err)
	}
}

func TestScriptCanBeInlined(t *testing.T) {
	// app.js is inlined in a script element, which its text must not end.
	if bytes.Contains(bytes.ToLower(asset(t, "app.js")), []byte("</script")) {
		t.Error("app.js contains </script")
	}
}

func TestScriptAcceptsReportSchema(t *testing.T) {
	want := `const SCHEMA = "` + psv.ReportSchema + `";`
	if !bytes.Contains(asset(t, "app.js"), []byte(want)) {
		t.Errorf("app.js does not declare %s", want)
	}
}

func get(t *testing.T, url string) []byte {
	t.Helper()

	res, err := http.Get(url) //nolint:noctx // test against a local server
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %s", url, res.Status)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	return b
}
