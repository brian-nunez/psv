package psv

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
	"time"
)

// ReportSchema identifies the format of a JSON report.
const ReportSchema = "psv.report/v1"

// ErrReportSchema is returned when decoding a document that is not a
// ReportSchema report.
var ErrReportSchema = errors.New("psv: not a " + ReportSchema + " report")

// Tool names the program that produced a report.
type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// Report summarises an analysis for presentation. Its JSON encoding is the
// ReportSchema format shared by every PSV frontend.
type Report struct {
	// Schema is ReportSchema.
	Schema string `json:"schema"`
	// Tool is the program that produced the report.
	Tool Tool `json:"tool"`
	// Source names the analysed video, typically its file name.
	Source string `json:"source,omitempty"`
	// Standard names the threshold preset, such as wcag22, when one was
	// used.
	Standard string `json:"standard,omitempty"`

	Verdict         Verdict        `json:"verdict"`
	Passed          bool           `json:"passed"`
	FrameRate       int            `json:"frameRate"`
	Frames          int            `json:"frames"`
	DurationSeconds float64        `json:"durationSeconds"`
	Width           int            `json:"width"`
	Height          int            `json:"height"`
	Thresholds      Thresholds     `json:"thresholds"`
	Failures        Failures       `json:"failures"`
	Defects         []DefectReport `json:"defects"`
	Results         []FrameReport  `json:"results,omitempty"`
}

// DefectReport describes one defect.
type DefectReport struct {
	Defect

	Index        int     `json:"index"`
	StartSeconds float64 `json:"startSeconds"`
	EndSeconds   float64 `json:"endSeconds"`
}

// FrameReport describes one frame.
type FrameReport struct {
	FrameResult

	Frame   int     `json:"frame"`
	Seconds float64 `json:"seconds"`
}

// Report builds the report of the analysis; perFrame adds every frame's
// result.
func (a *Analysis) Report(perFrame bool) Report {
	verdict := a.Verdict()
	rep := Report{
		Schema:          ReportSchema,
		Tool:            Tool{Name: "psv"},
		Verdict:         verdict,
		Passed:          verdict.Passed(),
		FrameRate:       a.FrameRate,
		Frames:          len(a.Frames),
		DurationSeconds: a.Duration().Seconds(),
		Width:           a.Width,
		Height:          a.Height,
		Thresholds:      a.Thresholds,
		Failures:        a.Failures(),
		Defects:         []DefectReport{},
	}

	for i, d := range a.Defects() {
		rep.Defects = append(rep.Defects, DefectReport{
			Defect:       d,
			Index:        i + 1,
			StartSeconds: a.FrameTime(d.FirstFrame).Seconds(),
			EndSeconds:   a.FrameTime(d.LastFrame + 1).Seconds(),
		})
	}

	if perFrame {
		rep.Results = make([]FrameReport, len(a.Frames))
		for i, r := range a.Frames {
			rep.Results[i] = FrameReport{FrameResult: r, Frame: i + 1, Seconds: a.FrameTime(i + 1).Seconds()}
		}
	}

	return rep
}

// DecodeReport reads a JSON report and checks that it is a ReportSchema
// report.
func DecodeReport(r io.Reader) (*Report, error) {
	var rep Report
	if err := json.NewDecoder(r).Decode(&rep); err != nil {
		return nil, fmt.Errorf("psv: decode report: %w", err)
	}

	if rep.Schema != ReportSchema {
		return nil, fmt.Errorf("%w (schema %q)", ErrReportSchema, rep.Schema)
	}

	return &rep, nil
}

// WriteText writes the report as plain text.
func (r *Report) WriteText(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	length := time.Duration(r.DurationSeconds * float64(time.Second)).Round(time.Millisecond)

	var err error

	printf := func(format string, args ...any) {
		if err == nil {
			_, err = fmt.Fprintf(tw, format, args...)
		}
	}

	printf("Result:\t%s\n", r.Verdict)
	printf("Frame rate:\t%d fps\n", r.FrameRate)
	printf("Length:\t%s (%d %s)\n", length, r.Frames, plural(r.Frames, "frame"))
	printf("Video size:\t%dx%d\n", r.Width, r.Height)
	printf("Luminance flash failures:\t%d %s\n", r.Failures.Luminance, plural(r.Failures.Luminance, "frame"))
	printf("Red flash failures:\t%d %s\n", r.Failures.Red, plural(r.Failures.Red, "frame"))
	printf("\nDefects: %d\n", len(r.Defects))

	if len(r.Defects) > 0 {
		printf("  #\tFrames\tTime\tLuminance\tRed\n")

		for _, d := range r.Defects {
			printf("  %d\t%d-%d\t%.2fs-%.2fs\t%s\t%s\n",
				d.Index, d.FirstFrame, d.LastFrame, d.StartSeconds, d.EndSeconds, mark(d.Luminance), mark(d.Red))
		}
	}

	if err != nil {
		return err
	}

	return tw.Flush()
}

func mark(b bool) string {
	if b {
		return "X"
	}

	return ""
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}

	return word + "s"
}
