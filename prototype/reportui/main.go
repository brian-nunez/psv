// Command reportui is a PROTOTYPE of the PSV report web UI. It is throwaway
// code for judging the users' and admin report designs against real
// analyses; it is not production code and must not ship.
//
// Run it with `make prototype` and open the printed address.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	_ "embed"

	"github.com/brian-nunez/psv"
)

//go:embed index.html
var page []byte

var presets = map[string]func() psv.Thresholds{
	"wcag22":    psv.WCAG22,
	"trace24":   psv.Trace24,
	"broadcast": psv.Broadcast,
	"scene":     psv.SceneChange,
}

type progress struct {
	Frame     int    `json:"frame"`
	Total     int    `json:"total"`
	Decided   int    `json:"decided"`
	Luminance int    `json:"luminance"`
	Red       int    `json:"red"`
	Defects   int    `json:"defects"`
	Done      bool   `json:"done"`
	Error     string `json:"error,omitempty"`
}

type job struct {
	mu       sync.Mutex
	path     string
	file     string
	standard string
	state    progress
	report   *psv.Report
}

type server struct {
	dir  string
	mu   sync.Mutex
	jobs map[string]*job
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8099", "listen address")

	flag.Parse()

	if err := serve(*addr); err != nil {
		log.Fatal(err)
	}
}

// serve runs the server until interrupted, then removes the uploaded videos.
func serve(addr string) error {
	dir, err := os.MkdirTemp("", "psv-prototype-*")
	if err != nil {
		return err
	}

	defer func() { _ = os.RemoveAll(dir) }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	s := &server{dir: dir, jobs: map[string]*job{}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	mux.HandleFunc("POST /api/jobs", s.create)
	mux.HandleFunc("GET /api/jobs/{id}/progress", s.progress)
	mux.HandleFunc("GET /api/jobs/{id}/report", s.reportJSON)
	mux.HandleFunc("GET /api/jobs/{id}/frame", s.frame)

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	go func() {
		<-ctx.Done()

		_ = srv.Close()
	}()

	log.Printf("PSV report prototype: http://%s", addr)

	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *server) get(w http.ResponseWriter, r *http.Request) *job {
	s.mu.Lock()
	j := s.jobs[r.PathValue("id")]
	s.mu.Unlock()

	if j == nil {
		http.NotFound(w, r)
	}

	return j
}

func (s *server) create(w http.ResponseWriter, r *http.Request) {
	standard := r.FormValue("standard")
	if presets[standard] == nil {
		standard = "wcag22"
	}

	src, hdr, err := r.FormFile("video")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}
	defer func() { _ = src.Close() }()

	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)

	path := filepath.Join(s.dir, id+filepath.Ext(hdr.Filename))

	dst, err := os.Create(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	_, err = io.Copy(dst, src)
	if cerr := dst.Close(); err == nil {
		err = cerr
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	j := &job{path: path, file: hdr.Filename, standard: standard}

	s.mu.Lock()
	s.jobs[id] = j
	s.mu.Unlock()

	go j.run()

	writeJSON(w, map[string]string{"id": id})
}

// run analyses the video with the streaming Analyzer so that live counts can
// be reported while frames are decided.
func (j *job) run() {
	err := j.analyze(context.Background())

	j.mu.Lock()
	defer j.mu.Unlock()

	j.state.Done = true
	if err != nil {
		j.state.Error = err.Error()
	}
}

func (j *job) analyze(ctx context.Context) error {
	info, err := psv.Probe(ctx, j.path, psv.Tools{})
	if err != nil {
		return err
	}

	a, err := psv.NewAnalyzer(info.FrameRate.Float64(), info.Width, info.Height,
		psv.Config{Thresholds: presets[j.standard]()})
	if err != nil {
		return err
	}

	dec, err := psv.Decode(ctx, j.path, info, psv.Tools{}, psv.DecodeOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = dec.Close() }()

	var lum, red int

	for n := 1; ; n++ {
		f, err := dec.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return err
		}

		decided, err := a.Analyze(f)
		if err != nil {
			return err
		}

		frames := a.Analysis().Frames
		for _, r := range frames[len(frames)-decided:] {
			if r.Luminance.Failed {
				lum++
			}

			if r.Red.Failed {
				red++
			}
		}

		defects := -1
		if n%15 == 0 {
			defects = len(a.Analysis().Defects())
		}

		j.mu.Lock()
		j.state.Frame, j.state.Total, j.state.Decided = n, info.Frames, len(frames)
		j.state.Luminance, j.state.Red = lum, red

		if defects >= 0 {
			j.state.Defects = defects
		}
		j.mu.Unlock()
	}

	rep := a.Finish().Report(true)

	j.mu.Lock()
	j.report = &rep
	j.state.Decided = rep.Frames
	j.state.Luminance, j.state.Red = rep.Failures.Luminance, rep.Failures.Red
	j.state.Defects = len(rep.Defects)
	j.mu.Unlock()

	return nil
}

func (s *server) progress(w http.ResponseWriter, r *http.Request) {
	j := s.get(w, r)
	if j == nil {
		return
	}

	j.mu.Lock()
	st := j.state
	j.mu.Unlock()

	writeJSON(w, st)
}

func (s *server) reportJSON(w http.ResponseWriter, r *http.Request) {
	j := s.get(w, r)
	if j == nil {
		return
	}

	j.mu.Lock()
	rep := j.report
	j.mu.Unlock()

	if rep == nil {
		http.Error(w, "analysis not finished", http.StatusConflict)

		return
	}

	writeJSON(w, map[string]any{"file": j.file, "standard": j.standard, "report": rep})
}

// frame extracts frame n (from 1) as a JPEG.
func (s *server) frame(w http.ResponseWriter, r *http.Request) {
	j := s.get(w, r)
	if j == nil {
		return
	}

	n, _ := strconv.Atoi(r.URL.Query().Get("n"))

	j.mu.Lock()
	rep := j.report
	j.mu.Unlock()

	if rep == nil || n < 1 || n > rep.Frames {
		http.Error(w, "no such frame", http.StatusNotFound)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-i", j.path,
		"-vf", fmt.Sprintf("select=eq(n\\,%d),scale=960:-2", n-1),
		"-frames:v", "1", "-f", "image2pipe", "-vcodec", "mjpeg", "pipe:1")

	out, err := cmd.Output()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "max-age=3600")
	_, _ = w.Write(out)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Print(err)
	}
}
