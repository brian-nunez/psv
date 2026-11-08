package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brian-nunez/psv"
	"github.com/brian-nunez/psv/internal/server"
)

// serve runs the API server, which also serves the report UI, until
// interrupted.
func serve(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("psv serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printf(stderr, "%s\nRuns the PSV API (POST /v1/analyses) and the report UI. Nothing is stored:\n"+
			"each upload is deleted when its analysis ends.\n\nserve flags:\n", usage)
		fs.PrintDefaults()
	}

	addr := fs.String("addr", "127.0.0.1:8080", "listen address")
	maxUpload := fs.Int64("max-upload", server.DefaultMaxUploadBytes>>20, "largest video accepted, in MB")
	maxJobs := fs.Int("max-jobs", server.DefaultMaxJobs, "most analyses run at once")
	timeout := fs.Duration("timeout", server.DefaultTimeout, "longest time an analysis may take, upload included")
	ffmpeg := fs.String("ffmpeg", "", "path to ffmpeg (default: found on PATH)")
	ffprobe := fs.String("ffprobe", "", "path to ffprobe (default: found on PATH)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitPass
		}

		return exitError
	}

	if fs.NArg() != 0 {
		fs.Usage()

		return exitError
	}

	// Uploads go to a directory of this run, removed on the way out even if
	// a request is still running.
	dir, err := os.MkdirTemp("", "psv-serve-*")
	if err != nil {
		printf(stderr, "%v\n", err)

		return exitError
	}
	defer func() { _ = os.RemoveAll(dir) }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Handler: server.New(server.Config{
			Version:        version,
			MaxUploadBytes: *maxUpload << 20,
			MaxJobs:        *maxJobs,
			Timeout:        *timeout,
			TempDir:        dir,
			Tools:          psv.Tools{FFmpeg: *ffmpeg, FFprobe: *ffprobe},
		}),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		printf(stderr, "%v\n", err)

		return exitError
	}

	printf(stderr, "psv %s serving the report UI at http://%s and the API at http://%s/v1/analyses\n", version, ln.Addr(), ln.Addr())

	errc := make(chan error, 1)

	go func() { errc <- srv.Serve(ln) }()

	select {
	case err := <-errc:
		printf(stderr, "%v\n", err)

		return exitError
	case <-ctx.Done():
	}

	// Running analyses are cancelled through BaseContext; give their
	// handlers a moment to finish their streams.
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdown); err != nil {
		_ = srv.Close()
	}

	return exitPass
}
