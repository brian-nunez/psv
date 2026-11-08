// Package ui holds the PSV report UI. The same files are served by the API
// server and inlined, with a report, into the single HTML page psv render
// writes, so both show the same report UI.
package ui

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"

	"github.com/brian-nunez/psv"
)

//go:embed assets
var assets embed.FS

// Files is the report UI: index.html, app.css and app.js.
var Files = func() fs.FS {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}

	return sub
}()

// Placeholders in index.html that Render replaces.
const (
	styleTag  = `<link rel="stylesheet" href="app.css">`
	scriptTag = `<script src="app.js"></script>`
	reportTag = `<!-- psv:report -->`
)

// Handler serves the report UI.
func Handler() http.Handler {
	return http.FileServerFS(Files)
}

// Render writes the report UI as one self-contained HTML page showing rep.
// The report must include per-frame results.
func Render(w io.Writer, rep *psv.Report) error {
	if len(rep.Results) != rep.Frames {
		return errors.New("report has no per-frame results; create it with psv -format json -frames")
	}

	page, err := fs.ReadFile(Files, "index.html")
	if err != nil {
		return err
	}

	css, err := fs.ReadFile(Files, "app.css")
	if err != nil {
		return err
	}

	js, err := fs.ReadFile(Files, "app.js")
	if err != nil {
		return err
	}

	// json.Marshal escapes <, > and &, so the report cannot end the script
	// element it is embedded in.
	data, err := json.Marshal(rep)
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}

	for _, r := range []struct{ old, new string }{
		{styleTag, "<style>\n" + string(css) + "</style>"},
		{reportTag, `<script type="application/json" id="psv-report">` + string(data) + `</script>`},
		{scriptTag, "<script>\n" + string(js) + "</script>"},
	} {
		if bytes.Count(page, []byte(r.old)) != 1 {
			return fmt.Errorf("ui: index.html must contain %s once", r.old)
		}

		page = bytes.Replace(page, []byte(r.old), []byte(r.new), 1)
	}

	_, err = w.Write(page)

	return err
}
