<p align="center">
  <img src="docs/brand/icon.svg" width="128" height="128" alt="PSV icon: a white ring on a cobalt tile with a red dot at its upper right">
</p>

<h1 align="center">Photosafety Validator</h1>

<p align="center">
  <b>Photosensitive epilepsy checker; a Go library and CLI that finds hazardous flashing in video against WCAG 2.2 and broadcast guidelines.</b>
</p>

<p align="center">
  PSV finds general (luminance) flashes and red flashes, measures how much of the screen flashes together and how often,<br>
  and reports a pass or fail verdict with the frames and screen regions involved.<br>
  <i>By Brian Nunez.</i>
</p>

<p align="center">
  <img alt="Written in Go 1.25" src="https://img.shields.io/badge/written%20in-Go%201.25-2346ff?style=flat-square&logo=go&logoColor=white">
  <img alt="Requires FFmpeg" src="https://img.shields.io/badge/requires-FFmpeg-1b2fb3?style=flat-square&logo=ffmpeg&logoColor=white">
  <img alt="Standards: WCAG 2.2, Trace 2024, broadcast" src="https://img.shields.io/badge/standards-WCAG%202.2%20%C2%B7%20Trace%202024%20%C2%B7%20broadcast-2346ff?style=flat-square">
  <img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-1b2fb3?style=flat-square">
</p>

<p align="center">
  <a href="#quick-start"><img alt="Quick start" src="https://img.shields.io/badge/Quick%20start-2346FF?style=for-the-badge&logo=go&logoColor=white" height="40"></a>
</p>

<p align="center">
  <a href="#quick-start"><b>Quick start</b></a> ·
  <a href="docs/how-it-works.md">How it works</a> ·
  <a href="docs/testing.md">Testing</a> ·
  <a href="#library">Library</a>
</p>

<br>

<p align="center">
  <img src="docs/images/report.png" alt="PSV report prototype showing a failed verdict for a 20 second video, its length, frames, frame rate, size and standard, and a timeline of three flashing moments with transition and area charts against their limits" width="100%">
  <br><sub><b>Report prototype</b>: the throwaway web UI in <code>prototype/reportui</code>, running a real analysis of a generated test clip.</sub>
</p>

<p align="center">
  <img src="docs/images/cli.png" alt="Terminal running psv on the same test clip: result Failed, 600 frames at 30 fps, three defects with their frames, times and flash types, and exit status 1" width="80%">
  <br><sub><b>Command line</b>: the same clip through <code>psv</code>, with exit status 1 for a failed video.</sub>
</p>

> [!NOTE]
> **PSV checks flashing, not every visual hazard.** Regular spatial patterns such as stripes and checkerboards are a separate hazard and are not checked, and display brightness, HDR and viewing distance are not modelled. See the [design choices and limits](docs/how-it-works.md#design-choices-and-limits).

<p align="center">
  <a href="#why-psv">Why PSV</a> ·
  <a href="#quick-start">Quick start</a> ·
  <a href="#usage">Usage</a> ·
  <a href="#standards">Standards</a> ·
  <a href="#library">Library</a> ·
  <a href="#conformance">Conformance</a> ·
  <a href="#license">License</a>
</p>

## Why PSV

- **Standards-based.** Implements [WCAG 2.2 Success Criterion 2.3.1](https://www.w3.org/TR/WCAG22/#three-flashes-or-below-threshold) by default, with presets for the Trace R&D Center's 2024 guidelines, the broadcast guidelines (ITU-R BT.1702, Ofcom, ISO 9241-391) and an experimental scene-change rule.
- **Benchmarked.** Tested against every video of the [Trace RERC PSE test media](https://github.com/traceRERC/pse-test-media), a benchmark built to sit just above or just below each threshold of each standard. See [Conformance](#conformance).
- **Specific.** Failures name their frames, times and screen regions, and per-frame JSON locates the flashing pixels wherever they are in the frame.
- **Pipeline-ready.** The exit status is `0` when the video passes, `1` when it fails and `2` on an error, so `psv` can gate a build or upload pipeline.
- **Embeddable.** The repository is a Go module with one core package, `psv`, and thin apps on top of it. The command-line tool lives in `cmd/psv`; web, API and other frontends call the same package, and apps with their own decoded frames can stream them straight to an `Analyzer`.

## Quick start

```sh
make build                     # build bin/psv
./bin/psv video.mp4            # check a video against WCAG 2.2
make install                   # or install psv to $GOPATH/bin
```

Requirements:

- Go 1.25 or later
- `ffmpeg` and `ffprobe` on `PATH`, or passed with `-ffmpeg` and `-ffprobe`
- For development: [golangci-lint](https://golangci-lint.run) v2

## Usage

```bash
./bin/psv video.mp4
```

```text
Result:                    Failed
Frame rate:                30 fps
Length:                    20s (600 frames)
Video size:                1280x720
Luminance flash failures:  72 frames
Red flash failures:        24 frames

Defects: 3
  #  Frames   Time           Luminance  Red
  1  93-167   3.07s-5.57s    X
  2  273-331  9.07s-11.03s   X          X
  3  453-495  15.07s-16.50s  X
```

A defect is a run of frames covered by failing flash sequences, from the first transition of a sequence to the frame that completes it.

### Flags

| Flag | Default | Description |
| --- | --- | --- |
| `-standard` | `wcag22` | Threshold preset: `wcag22`, `trace24`, `broadcast` or `scene` |
| `-format` | `text` | `text` summary or `json` report |
| `-frames` | off | Include every frame's result in JSON output |
| `-fps` | `0` | Convert the video to this frame rate before analysis |
| `-deinterlace` | off | Deinterlace frames flagged as interlaced |
| `-progress` | on in a terminal | Show progress on stderr |
| `-luminance-change` | preset | Relative luminance change of a luminance transition |
| `-dark-limit` | preset | Relative luminance the darker side of a transition must be below |
| `-red-change` | preset | u'v' chromaticity change of a red transition |
| `-red-saturation` | preset | R/(R+G+B) at which a colour is saturated red |
| `-region` | preset | Area region as `WIDTHxHEIGHT` pixels, `0x0` for the whole frame |
| `-area` | preset | Share of a region, in percent, that must flash together |
| `-transitions` | preset | Most alternating transitions allowed in any one second |
| `-ffmpeg`, `-ffprobe` | `PATH` | Locations of the FFmpeg tools |
| `-version` | | Print the version |

Threshold flags override single values of the chosen preset.

## Standards

| Preset | Luminance | Red | Area | Count |
| --- | --- | --- | --- | --- |
| **`wcag22`** | ΔY ≥ 0.1, darker < 0.8 | Δu'v' ≥ 0.2, one side R/(R+G+B) ≥ 0.8 | ≥ 25% of any 341×256 px region | > 6 alternating transitions in 1 s |
| `trace24` | as WCAG | as WCAG | ≥ 25% of any 416×416 px region | as WCAG |
| `broadcast` | as WCAG | as WCAG | ≥ 25% of the screen | as WCAG |
| `scene` | as WCAG | as WCAG | ≥ 80% of the screen | > 3 alternating transitions in 1 s |

ΔY is the change in relative luminance (0 to 1). On the 200 cd/m² display the broadcast guidelines assume, 0.1 is their 20 cd/m² threshold.

## JSON report

`-format json` prints the full report in the `psv.report/v1` format:

```json
{
  "schema": "psv.report/v1",
  "tool": { "name": "psv", "version": "v0.1.0" },
  "source": "video.mp4",
  "standard": "wcag22",
  "verdict": "Failed",
  "passed": false,
  "frameRate": 30,
  "frames": 600,
  "durationSeconds": 20,
  "width": 1280,
  "height": 720,
  "thresholds": {
    "luminanceChange": 0.1,
    "darkLimit": 0.8,
    "redChange": 0.2,
    "redSaturation": 0.8,
    "regionWidth": 341,
    "regionHeight": 256,
    "areaPercent": 25,
    "maxTransitions": 6
  },
  "failures": { "luminance": 72, "red": 24 },
  "defects": [
    {
      "firstFrame": 93,
      "lastFrame": 167,
      "luminance": true,
      "red": false,
      "index": 1,
      "startSeconds": 3.066666666,
      "endSeconds": 5.566666666
    }
  ]
}
```

With `-frames`, `results` adds one entry per frame. Each entry has `luminance` and `red` measurements for the region that transitioned in that frame with the most alternating transitions: how many fall within the last second (`transitions`), the frame the sequence started (`since`), the region (`region`, in source pixels), the share of it that transitioned together (`area`, percent) and whether the frame completes a failing sequence (`failed`).

Failing frames also locate what flashes, wherever it is in the frame: `flashing` is the bounding box, in source pixels, of the flashing pixels that start a transition in that frame, and `mask` (base64) gives their share, 0 to 100, of each cell of a 64×36 grid over the frame, row by row from the top left. Unlike `region`, which is the fixed-size area the threshold is measured in, these follow the shape of the flashing.

`standard` is the preset name, or `custom` when threshold flags change it.

## Report prototype

`make prototype` starts a throwaway web UI at `http://127.0.0.1:8099` for judging report designs against real analyses. It is not production code and does not ship.

<table>
<tr>
<td width="50%" valign="top"><img src="docs/images/report-moments.png" alt="Flashing moments list with three moments, and a detail panel for the first showing what flashes, when, where, how many flashes and how fast"></td>
<td width="50%" valign="top"><img src="docs/images/report-screen.png" alt="Where on screen maps showing the footprint of each flashing moment, overall and split into luminance and red"></td>
</tr>
</table>

## Library

```go
analysis, err := psv.AnalyzeFile(ctx, "video.mp4", psv.Options{})
if err != nil {
	return err
}

report := analysis.Report(false)
fmt.Println(report.Verdict, report.Failures.Luminance, len(report.Defects))
```

Apps that already have decoded frames, such as a screen recorder or a mobile camera feed, use an `Analyzer` directly. A frame is decided one second after it is analysed, because whether its pixels flash depends on what follows:

```go
a, err := psv.NewAnalyzer(30, 1920, 1080, psv.Config{Thresholds: psv.WCAG22()})
if err != nil {
	return err
}

for frame := range frames { // *psv.Frame: packed 8-bit sRGB, top row first
	n, err := a.Analyze(frame)
	if err != nil {
		return err
	}

	decided := a.Analysis().Frames
	for _, r := range decided[len(decided)-n:] {
		if r.Failed() {
			// warn: a failing sequence completed
		}
	}
}

verdict := a.Finish().Verdict()
```

## Conformance

PSV runs against the 346 videos of the [Trace RERC PSE test media](https://github.com/traceRERC/pse-test-media). A set built for several standards runs once per preset, which makes 378 checks: 369 pass, 0 fail and 9 are skipped because their listing entries contradict the thresholds they test. Each skip is explained in the [testing guide](docs/testing.md#upstream-errors).

```bash
make test                                    # run every test
go test -run TestConformance .               # the conformance suite alone
go test -run 'TestConformance/wcag22/' .     # one standard
go test -run TestExamples .                  # the real-world example videos
```

The test media is vendored in `testdata/pse-test-media`, and real-world example videos with their reports are in [`testdata/examples`](testdata/examples).

## Documentation

- [How it works](docs/how-it-works.md): the detection model, its design choices and its limits.
- [Testing](docs/testing.md): the conformance suite and how it is scored.

## Development

```sh
make all                       # format check, lint, build
make test                      # run the race detector on a subset, then every test
make examples                  # regenerate the example reports
make lint                      # run golangci-lint
make fmt                       # format and apply lint fixes
```

## License

PSV is licensed under the [MIT License](LICENSE). Copyright (c) 2026 Brian Nunez.
