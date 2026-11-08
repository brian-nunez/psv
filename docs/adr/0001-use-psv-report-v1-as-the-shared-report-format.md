---
status: accepted
date: 2026-11-07
decision-makers: Brian Nunez
---

# Use psv.report/v1 as the report format shared by every frontend

## Context and Problem Statement

PSV has three ways to produce or show an analysis: the `psv` command line, the API server (`psv serve`) and the report UI (served by the API and written by `psv render`). Before this decision each spoke its own JSON. The CLI printed the bare `psv.Report`, with no way to tell what produced it or which format it was in. The prototype report server wrapped the same report in an ad-hoc `{file, standard, report}` envelope. The earlier `epilepsy-detector` tool, whose reports sat in the psv-data repository, used a different `schemaVersion: "1.0"` document.

A report made by one frontend has to be readable by every other: a CLI report opened in the UI, an API response saved and rendered with `psv render`. Which single JSON document do all of them produce and accept?

## Decision Drivers

* One document flows between every frontend: CLI output into `psv render`, API responses into the UI, a saved report into the CLI.
* A reader can tell a PSV report from any other JSON and know its format version.
* The report records what made it: the tool and version, the source video and the threshold preset.
* No rewrite of the analysis data the UI already renders: per-frame results, masks, regions and defects.
* PSV is a proof of concept, so the format starts at version 1 rather than continuing another tool's numbering.

## Considered Options

* Keep the bare `psv.Report` JSON
* Evolve psv-data's `epilepsy-detector` `schemaVersion: "1.0"` document to 2.0
* Add a `psv.report/v1` header to the existing `psv.Report`

## Decision Outcome

Chosen option: "Add a `psv.report/v1` header to the existing `psv.Report`", because it is the only option that identifies the document and its producer while leaving the analysis fields, which the core already produces and the UI already renders, unchanged.

Every report now opens with four fields, followed by the existing report:

```json
{
  "schema": "psv.report/v1",
  "tool": { "name": "psv", "version": "v0.1.0" },
  "source": "video.mp4",
  "standard": "wcag22",
  "verdict": "Failed"
}
```

`standard` is the preset name, or `custom` when threshold flags change it. `psv.DecodeReport` reads a report and rejects any document whose `schema` is not `psv.report/v1`, and the UI's `app.js` applies the same check.

### Consequences

* Good, because the CLI, the API and the UI exchange one document: `psv -format json -frames video.mp4 | psv render` and a saved API response render the same page.
* Good, because a report names the release that made it (`tool.version`), the video (`source`) and the thresholds used (`standard` and `thresholds`).
* Good, because the analysis fields did not change, so the UI's rendering code and the example reports carried over.
* Bad, because the analysis fields keep Go's encoding of `image.Rectangle` (`{"Min":{"X":…},"Max":{…}}`) for `region` and `flashing`, which is clumsier for other consumers than `{x,y,width,height}`.
* Bad, because a reader sees a version 1 that is not the 1.0 of the earlier `epilepsy-detector` reports; the two are unrelated formats.
* Neutral, because any incompatible change to a field needs a new schema name (`psv.report/v2`) and a decoder that accepts both.

### Confirmation

* `TestReportSchemaRoundTrip` and `TestDecodeReportRejectsOtherDocuments` (`report_test.go`) check that reports carry the header, round-trip through JSON and that other documents, including the `epilepsy-detector` 1.0 shape, are rejected.
* `TestScriptAcceptsReportSchema` (`internal/ui/ui_test.go`) checks that `app.js` accepts exactly `psv.ReportSchema`.
* `TestAnalyzeStreamsReport` (`internal/server/server_test.go`) and `TestAnalyzeJSONReport` (`cmd/psv/main_test.go`) check that the API and the CLI emit the header.
* `TestExamples` decodes the committed example reports with `psv.DecodeReport`.

## Pros and Cons of the Options

### Keep the bare `psv.Report` JSON

The report the CLI printed with `-format json` before this decision, without any header.

* Good, because nothing changes.
* Bad, because nothing marks the document as a PSV report or says which format it is in, so readers cannot reject other JSON or detect an older format.
* Bad, because it does not record the tool version, the source video or the preset, which the UI shows and the prototype had to wrap around it.
* Bad, because the prototype's envelope and the CLI's output stay two different documents.

### Evolve psv-data's `epilepsy-detector` `schemaVersion: "1.0"` document to 2.0

The document the earlier tool wrote: `schemaVersion`, `tool`, `generatedAt`, `media` (codec, colour metadata), `standard` with a reference link, per-kind `luminance` and `red` summaries, per-kind `hazards`, and a columnar `frames` series.

* Good, because it already identifies itself and carries richer metadata (media codec and colour space, standard reference URL).
* Good, because rectangles are `{x,y,width,height}`.
* Bad, because PSV's analysis is a different model: merged defects instead of per-kind hazards, and per-frame regions, flashing boxes and masks that the 1.0 frames series has no place for. Adopting it meant rewriting the report builder and the UI.
* Bad, because it ties PSV's version numbering to another tool's history.

### Add a `psv.report/v1` header to the existing `psv.Report`

* Good, because the header identifies the format, the producer and the input, and gives the format a version to evolve.
* Good, because every analysis field stays as the core produces it, so no consumer is rewritten.
* Neutral, because the header is split between the core, which sets `schema` and `tool.name` in `Analysis.Report`, and the frontend that produced the report, which sets `tool.version`, `source` and `standard`.
* Bad, because it keeps the existing field shapes, including the Go rectangle encoding.

## More Information

* The format is defined by `psv.Report` and `psv.ReportSchema` in `report.go`; the README's "JSON report" section documents it with real output of the v0.1.0 release.
* [ADR-0003](0003-build-the-cli-api-and-ui-as-thin-frontends-over-the-core.md) describes how the CLI, the API and the UI produce and consume this format.
* Revisit when a consumer outside PSV needs a field shape the format does not offer; that change ships as `psv.report/v2`.
