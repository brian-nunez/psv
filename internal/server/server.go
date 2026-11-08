// Package server is the PSV API server. It stores nothing: a client streams a
// video in the body of POST /v1/analyses and the response streams back
// newline-delimited JSON events ending with the psv.report/v1 report. The
// upload lives in a temporary file only while its request runs. The server
// also serves the report UI, which shows the reports it returns.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/brian-nunez/psv"
	"github.com/brian-nunez/psv/internal/standards"
	"github.com/brian-nunez/psv/internal/ui"
)

// Defaults for the zero Config fields.
const (
	DefaultMaxUploadBytes = 2 << 30
	DefaultMaxJobs        = 2
	DefaultTimeout        = 30 * time.Minute
)

// progressInterval is the shortest time between progress events.
const progressInterval = 100 * time.Millisecond

// Config configures a Server.
type Config struct {
	// Version is reported in the tool field of reports.
	Version string
	// MaxUploadBytes is the largest video accepted.
	MaxUploadBytes int64
	// MaxJobs is the most analyses run at once; further requests get 429.
	MaxJobs int
	// Timeout limits each analysis, upload included.
	Timeout time.Duration
	// TempDir holds uploads while they are analysed; empty uses the system
	// temporary directory.
	TempDir string
	// Tools locates FFmpeg.
	Tools psv.Tools
}

// Event is one line of the POST /v1/analyses response.
type Event struct {
	// Type is "received" once the upload is stored, "progress" while
	// frames are analysed, then "report" or "error".
	Type string `json:"type"`
	// Bytes is the size of the upload, for "received".
	Bytes int64 `json:"bytes,omitempty"`
	// Frame and Total are the frames read and the expected total, which is
	// zero when unknown, for "progress".
	Frame int `json:"frame,omitempty"`
	Total int `json:"total,omitempty"`
	// Report is the result, for "report".
	Report *psv.Report `json:"report,omitempty"`
	// Error describes the failure, for "error".
	Error string `json:"error,omitempty"`
}

// Server is the PSV API server.
type Server struct {
	cfg   Config
	slots chan struct{}
	mux   *http.ServeMux
}

// New returns a server.
func New(cfg Config) *Server {
	if cfg.MaxUploadBytes <= 0 {
		cfg.MaxUploadBytes = DefaultMaxUploadBytes
	}

	if cfg.MaxJobs <= 0 {
		cfg.MaxJobs = DefaultMaxJobs
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultTimeout
	}

	s := &Server{cfg: cfg, slots: make(chan struct{}, cfg.MaxJobs), mux: http.NewServeMux()}

	s.mux.Handle("GET /", ui.Handler())
	s.mux.HandleFunc("POST /v1/analyses", s.analyze)
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	s.mux.ServeHTTP(w, r)
}

// analyze handles POST /v1/analyses?standard=wcag22&name=clip.mp4 with the
// video as the request body. Problems found before the analysis starts are
// answered with an error status and a JSON {"error": ...} body; once it has
// started the status is 200 and a failure ends the stream with an "error"
// event.
func (s *Server) analyze(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	standard := q.Get("standard")
	if standard == "" {
		standard = standards.Default
	}

	th, err := standards.Lookup(standard)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())

		return
	}

	if r.ContentLength > s.cfg.MaxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, s.tooLarge())

		return
	}

	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		writeError(w, http.StatusTooManyRequests, "the server is busy with other analyses; try again later")

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.Timeout)
	defer cancel()

	name := sourceName(q.Get("name"))

	path, size, err := s.store(w, r, name)
	if path != "" {
		defer func() { _ = os.Remove(path) }()
	}

	if err != nil {
		var tooBig *http.MaxBytesError

		switch {
		case errors.As(err, &tooBig):
			writeError(w, http.StatusRequestEntityTooLarge, s.tooLarge())
		case errors.Is(err, errEmpty):
			writeError(w, http.StatusBadRequest, "the request body is empty; send the video as the body")
		default:
			writeError(w, http.StatusBadRequest, "reading the upload failed: "+err.Error())
		}

		return
	}

	if _, err := psv.Probe(ctx, path, s.cfg.Tools); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "not a readable video: "+clean(err, path, name))

		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	stream := newStream(w)
	stream.send(Event{Type: "received", Bytes: size})

	var last time.Time

	analysis, err := psv.AnalyzeFile(ctx, path, psv.Options{
		Config: psv.Config{Thresholds: th},
		Tools:  s.cfg.Tools,
		Progress: func(frame, total int) {
			if now := time.Now(); now.Sub(last) >= progressInterval || frame == total {
				last = now

				stream.send(Event{Type: "progress", Frame: frame, Total: total})
			}
		},
	})
	if err != nil {
		msg := clean(err, path, name)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			msg = fmt.Sprintf("the analysis took longer than %s", s.cfg.Timeout)
		}

		stream.send(Event{Type: "error", Error: msg})

		return
	}

	rep := analysis.Report(true)
	rep.Tool.Version = s.cfg.Version
	rep.Source = name
	rep.Standard = standard

	stream.send(Event{Type: "report", Report: &rep})
}

var errEmpty = errors.New("empty body")

// store copies the request body to a temporary file and returns its path,
// which is set whenever the file was created.
func (s *Server) store(w http.ResponseWriter, r *http.Request, name string) (string, int64, error) {
	f, err := os.CreateTemp(s.cfg.TempDir, "psv-upload-*"+extension(name))
	if err != nil {
		return "", 0, err
	}

	n, err := io.Copy(f, http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes))
	if cerr := f.Close(); err == nil {
		err = cerr
	}

	if err == nil && n == 0 {
		err = errEmpty
	}

	return f.Name(), n, err
}

func (s *Server) tooLarge() string {
	return fmt.Sprintf("the video is larger than the %d MB limit", s.cfg.MaxUploadBytes>>20)
}

// stream writes events and flushes each one. Write errors mean the client
// has gone; its request context then stops the analysis.
type stream struct {
	enc *json.Encoder
	rc  *http.ResponseController
}

func newStream(w http.ResponseWriter) *stream {
	return &stream{enc: json.NewEncoder(w), rc: http.NewResponseController(w)}
}

func (s *stream) send(e Event) {
	if err := s.enc.Encode(e); err == nil {
		_ = s.rc.Flush()
	}
}

// sourceName turns the client's file name into a display name.
func sourceName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	if name == "." || name == "/" || !utf8.ValidString(name) {
		return "video"
	}

	if r := []rune(name); len(r) > 200 {
		name = string(r[:200])
	}

	return name
}

// extension keeps a short alphanumeric extension of name, which helps FFmpeg
// recognise some formats.
func extension(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if len(ext) < 2 || len(ext) > 8 {
		return ""
	}

	for _, c := range ext[1:] {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return ""
		}
	}

	return ext
}

// clean replaces the temporary path in an error with the client's name.
func clean(err error, path, name string) string {
	return strings.ReplaceAll(err.Error(), path, name)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
