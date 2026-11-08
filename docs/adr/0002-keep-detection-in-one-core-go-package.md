---
status: accepted
date: 2026-11-07
decision-makers: Brian Nunez
---

# Keep flash detection and reporting in one core Go package

## Context and Problem Statement

PSV decides whether a video contains hazardous flashing under WCAG 2.2 Success Criterion 2.3.1 and the related Trace 2024, broadcast and scene-change rules. That logic has to give the same answer wherever it runs: the command line, the API server, and any future frontend such as a browser build, a screen recorder or a mobile camera feed that already has decoded frames.

Where does the detection logic live, what does it take as input, and how does it relate to FFmpeg, which only some environments have?

## Decision Drivers

* One implementation of the detection, so every frontend reports the same verdict, defects and per-frame results.
* Benchmarked against the Trace RERC PSE test media frame by frame, without encoding video.
* Usable without FFmpeg by callers that already have decoded frames.
* No third-party dependencies in the core.
* Analysis of long videos in bounded memory, deciding frames as they stream.

## Considered Options

* One core package, `psv`, with a frame-level `Analyzer` and FFmpeg confined to optional file helpers
* Change the core to build for the browser (WebAssembly), with frame decoding in the browser
* Let each frontend compute its own derived results on top of a minimal core

## Decision Outcome

Chosen option: "One core package, `psv`, with a frame-level `Analyzer` and FFmpeg confined to optional file helpers", because it is the only option that keeps a single implementation of every result while letting callers with and without FFmpeg use it unchanged.

The root package `github.com/brian-nunez/psv` holds:

* **Detection**: `NewAnalyzer(fps, width, height, Config)`, `Analyzer.Analyze(*Frame)` and `Analyzer.Finish()`. A `Frame` is packed 8-bit sRGB. A frame is decided one second after it is analysed, because whether its pixels flash depends on what follows. Pictures larger than 1920×1080 are reduced by an integer factor, and each frame's work is split across goroutines (`workers`, `parallel` in `detector.go`).
* **Thresholds**: `Thresholds` and the presets `WCAG22`, `Trace24`, `Broadcast` and `SceneChange`.
* **Results**: `Analysis` with `Verdict`, `Failures`, `Defects` and per-frame `FrameResult`s, including flashing boxes and 64×36 masks.
* **Reports**: `Analysis.Report` and `DecodeReport`, the `psv.report/v1` format of [ADR-0001](0001-use-psv-report-v1-as-the-shared-report-format.md), and `Report.WriteText`.
* **File helpers**: `Probe`, `Decode` and `AnalyzeFile`, the only code that runs FFmpeg, through `os/exec`. Everything above works without it.

The package imports only the standard library.

### Consequences

* Good, because the CLI, the API and the conformance suite call the same functions, so their results cannot drift: the API test checks its report against `AnalyzeFile` frame by frame.
* Good, because the conformance suite renders the benchmark's patterns in memory and feeds them to `Analyzer.Analyze`, so all 378 checks run without encoding or decoding video.
* Good, because callers with their own frames need no FFmpeg, and the core builds for any Go target; a WebAssembly build of the unchanged package passed the short conformance suite under Node.
* Good, because a streaming analyzer keeps per-pixel state for about two seconds of video, not every frame's pixels, so memory does not grow with the length of the video beyond one small result per frame.
* Bad, because the derived figures the report UI shows (moments, flashes per second, footprints) are computed in the UI's JavaScript from the per-frame results, not in the core, so a new frontend that wants them has to compute them again.
* Bad, because `AnalyzeFile` depends on an external FFmpeg on `PATH` and on how that FFmpeg converts colour, which the core does not control.
* Neutral, because the frame-level API is the stable surface; FFmpeg handling can change without touching detection.

### Confirmation

* `TestConformance` (`media_test.go`) checks the core against every video of the Trace RERC PSE test media through `Analyzer.Analyze`.
* `TestExamples` (`examples_test.go`) checks `AnalyzeFile` on real footage against committed reports.
* `go list -deps .` shows only standard-library imports for the root package; review rejects a third-party import there.
* Review rejects detection or report logic in `cmd/` or `internal/` that duplicates the core.

## Pros and Cons of the Options

### One core package, `psv`, with a frame-level `Analyzer` and FFmpeg confined to optional file helpers

* Good, because there is one implementation of detection, results and the report format.
* Good, because the frame API serves any source of frames: FFmpeg, rendered benchmark patterns, a browser canvas, a camera.
* Good, because FFmpeg is isolated in `media.go` and `AnalyzeFile`, so the rest needs no external tools.
* Bad, because the package is broad: detection, presets, reporting and FFmpeg integration share one namespace.

### Change the core to build for the browser (WebAssembly), with frame decoding in the browser

Considered so that a public static page could analyse video with no server. The browser would decode with WebCodecs and pass frames to the Go analyzer compiled to WebAssembly.

* Good, because analysis would run entirely on the viewer's device and could be hosted as static files.
* Good, because the core already builds for `js/wasm` unchanged; only FFmpeg helpers would be excluded with a build tag.
* Bad, because measured analysis speed under WebAssembly was 6.6 fps at 1080p and 14.7 fps at 720p, against 127 and 268 fps natively on all cores: minutes for a one-minute video.
* Bad, because browser colour conversion differs from FFmpeg's, so verdicts could differ from the CLI's on borderline videos.
* Bad, because making it fast enough meant refactoring the detector to split frames across Web Workers.
* Dropped in favour of the CLI and API server.

### Let each frontend compute its own derived results on top of a minimal core

The prototype report UI did this: it computed moments, flash rates and footprints in JavaScript from the per-frame results.

* Good, because each frontend can present results in its own way without core changes.
* Bad, because the same figure can be computed differently by different frontends and drift.
* Bad, because every new frontend re-implements the same calculations.

## More Information

* `docs/how-it-works.md` describes the detection model, its design choices and its limits; `docs/testing.md` describes the conformance suite and its scoring.
* [ADR-0003](0003-build-the-cli-api-and-ui-as-thin-frontends-over-the-core.md) describes how the frontends use this package.
* Revisit if the derived figures the UI computes are needed by another frontend; moving them into the core keeps the single-implementation rule.
