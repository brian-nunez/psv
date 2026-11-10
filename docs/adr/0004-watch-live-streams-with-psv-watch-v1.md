---
status: accepted
date: 2026-11-09
decision-makers: Brian Nunez
---

# Watch live streams through Analyzer.Watch and the psv.watch/v1 update

## Context and Problem Statement

`Analyzer` already streams: an app feeds decoded frames one at a time. A screen guard, such as a macOS menu bar app that captures every display at about 60 fps and covers flashing as it happens, uses it differently from the CLI and the API server:

* **Latency.** `Analyze` only reports a frame once it is decided, `round(fps) − 1` frames after it is seen, because whether its pixels flash depends on the second that follows. Until then a live consumer has no signal at all.
* **Length.** Every decided frame is appended to `Analysis().Frames` and kept, so a session of hours or days grows without bound. Trimming that slice from outside relies on undocumented internals.
* **Location.** `Flashes.Flashing` and `Flashes.Mask` are only set on frames that fail, so where flashing builds up is unknown until it has already failed.
* **Frame numbers.** The detector stores frame numbers as int32, which overflow after about 414 days at 60 fps and 103 days at 240 fps.

How does the core serve live consumers without changing a single detection result, the `psv.report/v1` report or the existing API?

## Decision Drivers

* Detection results stay identical: every test, Trace benchmark video and example video.
* `psv.report/v1`, the CLI, the API server and the report UI do not change.
* Existing callers of `NewAnalyzer`, `Analyze`, `Analysis` and `Finish` keep working; the new behaviour is opt-in.
* Live information is as early as the detection model allows, with its latency documented.
* Flat memory for sessions of any length.
* A small cost per frame: one analyser per display at 60 fps on a laptop.
* Detection stays in the core ([ADR-0002](0002-keep-detection-in-one-core-go-package.md)), in pure Go with no new dependencies.
* The live format can be swapped or A/B tested later.

## Considered Options

* A new `Analyzer.Watch` method returning a `psv.watch/v1` `Update` with a pixel-level live measurement
* A `Config` option that stops `Analyze` from retaining frames, with the live measurement on `Analysis`
* Provisional region results: re-run the region search on recent frames each frame
* Document that callers may trim `Analysis().Frames`

## Decision Outcome

Chosen option: "A new `Analyzer.Watch` method returning a `psv.watch/v1` `Update` with a pixel-level live measurement", because it is the only option that delivers live information in the frame it happens at almost no cost, keeps memory flat, and leaves `Analyze` and every result it produces untouched.

* `Analyze` and the new `Watch` share the same detector. An analyser is used one way or the other: mixing them returns `ErrMode`.
* `Watch(*Frame)` keeps no history and returns an `Update`:
  * `Schema` is `psv.watch/v1` (`WatchSchema`). An incompatible change to `Update` is a new schema name with its own ADR, the rule ADR-0001 sets for reports.
  * `Method` names the algorithm that measured `Building`. The first is `pixel` (`WatchPixel`), selected by `Config.WatchMethod`. A different method filling the same shape is a new value, not a new schema, so two methods can be compared on the same stream.
  * `Frame` is the frame number; `Building` the live measurement of luminance and red flashing; `Decided` the frames this one decides, which are the same `FrameResult`s `Analyze` appends; `DecidedFrom` the number of the first of them.
* `Stop` returns the frames still waiting to be decided when the stream ends. `Finish` after `Watch` returns an analysis without frames.
* The `pixel` method measures, for each pixel that starts a transition in the frame being watched, how many transitions it made in the second ending at that frame. That count is already in the detector's per-pixel history, so it costs a popcount per transitioning pixel, and it only reads detector state. `Activity` reports the highest count (`Peak`), the share of the frame that already flashes (`Flashing`) and its bounding box (`Bounds`), the share that transitions (`Area`), and the highest count in each cell of the 64×36 grid (`Mask`).
* Stored frame numbers move back by a multiple of the frame rate every 2^30 frames, keeping the last two seconds intact, so an analyser can run indefinitely. Reported frame numbers stay absolute.

What each piece of information can be known by, at frame rate `r`:

| Information | Earliest | Why |
| --- | --- | --- |
| A pixel's transitions within the last second, and where | the frame they happen | the history is complete up to the current frame |
| A pixel flashes | the frame of its seventh transition within a second | more frames can only add transitions, so a pixel that flashes in the second ending at frame `f` flashes in every decision that covers `f` |
| A pixel does not flash | `r − 1` frames later | a window that contains the frame may still gain transitions |
| A region fails, its region, defects | `r − 1` frames later | a region transition counts only flashing pixels, which is the previous row |

### Consequences

* Good, because a live consumer sees flashing pixels in the frame that completes a failing sequence, a full second before that failure is decided: on all 118 failing Trace benchmark videos, the live measurement came 29 to 38 frames ahead of the decided failure at 30 fps.
* Good, because `Peak` and `Mask` show flashing building up before it flashes, so a consumer can act at its own, lower, threshold.
* Good, because the decided frames of `Watch` are identical to those of `Analyze` on every clip, benchmark video and example video, which the tests check frame by frame.
* Good, because memory is flat: a simulated three-hour session at 60 fps ends with the same live heap it had after a minute.
* Good, because the cost is small: at 640×360 and 60 fps, a frame of full-screen flashing takes about 1.5 ms with `Watch`, the same as with `Analyze`, on an Apple M3 Pro.
* Bad, because `Building` is pixel-level: it does not apply the area threshold, so it also reports flashing too small to fail. 91 of the 251 passing benchmark videos, built to sit just under a threshold, show flashing pixels live. Only `Decided` is the verdict.
* Bad, because a watched stream has no `Analysis` frames, so no report or summary; a consumer keeps what it needs.
* Neutral, because `Update.Decided` uses `FrameResult`, so a future `psv.report/v2` that changes it also needs a `psv.watch/v2`.
* Neutral, because the conformance suite now runs every video through both `Analyze` and `Watch`, which roughly doubles its run time.

### Confirmation

* `TestConformance` (`media_test.go`) runs every benchmark video through `Analyze` and `Watch` in lockstep: identical decided frames, and every failing video shows flashing pixels live no later than the frame that completes its first failing sequence.
* `TestExamples` (`examples_test.go`) watches every example video and checks the report rebuilt from `Update.Decided` against the committed one.
* `TestWatchClips` (`watch_test.go`) does the same on `testdata/clips` and checks the live measurement on `fail.mp4` comes a full decision delay ahead.
* `TestWatchLongSession` checks flat memory over three simulated hours at 60 fps; `TestRebaseKeepsResults` (`watch_internal_test.go`) forces frame numbers to move back every few seconds and checks the results against an `Analyze` run.
* `TestWatchMeasuresFullFrameFlash`, `TestWatchUpdateJSON` and `TestWatchModes` cover the measurement, the `psv.watch/v1` JSON and the mode rules; `BenchmarkWatch` measures the cost.

## Pros and Cons of the Options

### A new `Analyzer.Watch` method returning a `psv.watch/v1` `Update` with a pixel-level live measurement

* Good, because `Analyze` keeps its code path and results; `Watch` only adds a read-only pass.
* Good, because the live measurement is exact for what it claims (transition counts, and flashing pixels that cannot be withdrawn) and available in the frame.
* Good, because the schema and method names allow later methods and formats side by side.
* Bad, because it adds API surface: `Watch`, `Stop`, `Update`, `Building`, `Activity`, `ErrMode` and `Config.WatchMethod`.

### A `Config` option that stops `Analyze` from retaining frames, with the live measurement on `Analysis`

* Good, because there is no new method.
* Bad, because `Analysis()` would mean different things depending on configuration, and its `Verdict`, `Failures` and `Defects` would silently describe nothing.
* Bad, because the live measurement has no natural place on `Analysis`, which holds decided results.

### Provisional region results: re-run the region search on recent frames each frame

Each frame, decide the last second of frames again with what is known so far, giving provisional region counts.

* Good, because it applies the area threshold, so it would not report flashing too small to fail.
* Bad, because the region search is the expensive part of a frame and keeps state per region position, so a provisional pass needs a copy of that state and a search over up to a second of frames per frame: far more than doubling the cost.
* Bad, because its counts would still only be lower bounds until the frame is decided.

### Document that callers may trim `Analysis().Frames`

* Good, because it needs no code.
* Bad, because it solves neither latency nor location, and freezes an implementation detail into the API.

## More Information

* `docs/how-it-works.md` ("Deciding a frame takes a second", "Watching live") describes the model; the README's Library section has a live example.
* Revisit when a consumer needs the area threshold applied live; a region-aware method would be a new `Method` value under `psv.watch/v1`.
