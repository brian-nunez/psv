package psv

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func testAnalysis() *Analysis {
	a := &Analysis{FrameRate: 10, Width: 64, Height: 36, Thresholds: WCAG22(), Frames: make([]FrameResult, 20)}
	for f := 4; f < 9; f++ {
		a.Frames[f].Luminance = Flashes{Transitions: 7, Since: 2, Area: 50, Failed: true, Mask: []byte{1, 2, 3}}
	}

	return a
}

func TestReportSchemaRoundTrip(t *testing.T) {
	rep := testAnalysis().Report(true)

	if rep.Schema != ReportSchema || rep.Tool.Name != "psv" {
		t.Fatalf("schema %q, tool %+v", rep.Schema, rep.Tool)
	}

	rep.Source, rep.Standard, rep.Tool.Version = "clip.mp4", "wcag22", "v1"

	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}

	got, err := DecodeReport(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(*got, rep) {
		t.Errorf("round trip changed the report:\ngot  %+v\nwant %+v", *got, rep)
	}

	if got.Verdict != Fail || len(got.Defects) != 1 || len(got.Results) != 20 {
		t.Errorf("verdict %v, %d defects, %d results", got.Verdict, len(got.Defects), len(got.Results))
	}
}

func TestDecodeReportRejectsOtherDocuments(t *testing.T) {
	for _, doc := range []string{
		`{"verdict":"Failed"}`,
		`{"schema":"psv.report/v2","verdict":"Failed"}`,
		`{"schemaVersion":"1.0","verdict":"fail"}`,
	} {
		if _, err := DecodeReport(strings.NewReader(doc)); err == nil {
			t.Errorf("%s: decoded without error", doc)
		} else if !strings.Contains(doc, `"verdict":"fail"`) && !errors.Is(err, ErrReportSchema) {
			t.Errorf("%s: error %v, want ErrReportSchema", doc, err)
		}
	}

	if _, err := DecodeReport(strings.NewReader(`{`)); err == nil {
		t.Error("truncated JSON decoded without error")
	}
}

func TestVerdictText(t *testing.T) {
	for _, v := range []Verdict{Pass, Fail} {
		b, err := v.MarshalText()
		if err != nil {
			t.Fatal(err)
		}

		var got Verdict
		if err := got.UnmarshalText(b); err != nil || got != v {
			t.Errorf("%s: got %v, %v", b, got, err)
		}
	}

	var v Verdict
	if err := v.UnmarshalText([]byte("Maybe")); err == nil {
		t.Error("unknown verdict decoded without error")
	}
}
