package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/brian-nunez/psv"
	"github.com/brian-nunez/psv/internal/ui"
)

// render writes a psv.report/v1 report as a self-contained page of the report
// UI, the same UI psv serve shows.
func render(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("psv render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printf(stderr, "%s\nRenders a psv.report/v1 JSON report, read from the file or standard input,\n"+
			"as an HTML page of the report UI. Create the report with psv -format json -frames.\n\nrender flags:\n", usage)
		fs.PrintDefaults()
	}

	out := fs.String("o", "", "write the page to this file instead of standard output")
	open := fs.Bool("open", false, "open the page in the browser; without -o it is written to a temporary file")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitPass
		}

		return exitError
	}

	if fs.NArg() > 1 {
		fs.Usage()

		return exitError
	}

	in := stdin

	if name := fs.Arg(0); name != "" && name != "-" {
		f, err := os.Open(name)
		if err != nil {
			printf(stderr, "%v\n", err)

			return exitError
		}
		defer func() { _ = f.Close() }()

		in = f
	}

	rep, err := psv.DecodeReport(in)
	if err != nil {
		printf(stderr, "%v\n", err)

		return exitError
	}

	var page bytes.Buffer
	if err := ui.Render(&page, rep); err != nil {
		printf(stderr, "%v\n", err)

		return exitError
	}

	path := *out
	if path == "" && *open {
		f, err := os.CreateTemp("", "psv-report-*.html")
		if err != nil {
			printf(stderr, "%v\n", err)

			return exitError
		}

		path = f.Name()
		_ = f.Close()
	}

	if path == "" {
		if _, err := stdout.Write(page.Bytes()); err != nil {
			printf(stderr, "%v\n", err)

			return exitError
		}

		return exitPass
	}

	if err := os.WriteFile(path, page.Bytes(), 0o644); err != nil {
		printf(stderr, "%v\n", err)

		return exitError
	}

	if *open {
		if err := openBrowser(path); err != nil {
			printf(stderr, "open %s: %v\n", path, err)

			return exitError
		}

		printf(stderr, "opened %s\n", path)
	}

	return exitPass
}

// openBrowser opens a file with the system's default handler.
func openBrowser(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}

	return cmd.Start()
}
