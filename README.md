# Photosafety Validator (PSV)

PSV checks video for flashing that can trigger seizures in people with
photosensitive epilepsy. It finds general (luminance) flashes and red flashes,
measures how much of the screen flashes together and how often, and reports a
pass or fail verdict with the frames and screen regions involved.

It implements [WCAG 2.2 Success Criterion 2.3.1](https://www.w3.org/TR/WCAG22/#three-flashes-or-below-threshold)
by default, with presets for the Trace R&D Center's 2024 guidelines, the
broadcast guidelines (ITU-R BT.1702, Ofcom, ISO 9241-391) and an experimental
scene-change rule. It is tested against every video of the
[Trace RERC PSE test media](https://github.com/traceRERC/pse-test-media).

The repository is a Go module with one core package, `psv`, and thin apps on
top of it. The command-line tool lives in `cmd/psv`; web, API and other
frontends call the same package.

## Requirements

- Go 1.25 or later
- `ffmpeg` and `ffprobe` on `PATH`, or passed with `-ffmpeg` and `-ffprobe`
- For development: [golangci-lint](https://golangci-lint.run) v2

## Build

```bash
make build
```

The binary is written to `bin/psv`. `make install` puts it in `$GOPATH/bin`.

## Usage

```bash
./bin/psv video.mp4
```

```text
Result:                    Failed
Frame rate:                30 fps
Length:                    1m1.433s (1843 frames)
Video size:                1920x1080
Luminance flash failures:  335 frames
Red flash failures:        384 frames

Defects: 8
  #  Frames     Time           Luminance  Red
  1  72-115     2.37s-3.83s    X
  2  281-332    9.33s-11.07s   X          X
  ...
```

A defect is a run of frames covered by failing flash sequences, from the first
transition of a sequence to the frame that completes it.

The exit status is `0` when the video passes, `1` when it fails and `2` on an
error, so `psv` can gate a build or upload pipeline.

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

### Standards

| Preset | Luminance | Red | Area | Count |
| --- | --- | --- | --- | --- |
| `wcag22` | ΔY ≥ 0.1, darker < 0.8 | Δu'v' ≥ 0.2, one side R/(R+G+B) ≥ 0.8 | ≥ 25% of any 341×256 px region | > 6 alternating transitions in 1 s |
| `trace24` | as WCAG | as WCAG | ≥ 25% of any 416×416 px region | as WCAG |
| `broadcast` | as WCAG | as WCAG | ≥ 25% of the screen | as WCAG |
| `scene` | as WCAG | as WCAG | ≥ 80% of the screen | > 3 alternating transitions in 1 s |

ΔY is the change in relative luminance (0 to 1). On the 200 cd/m² display the
broadcast guidelines assume, 0.1 is their 20 cd/m² threshold.

### JSON report

`-format json` prints the full report:

```json
{
  "verdict": "Failed",
  "passed": false,
  "frameRate": 30,
  "frames": 1843,
  "durationSeconds": 61.433333333,
  "width": 1920,
  "height": 1080,
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
  "failures": { "luminance": 335, "red": 384 },
  "defects": [
    {
      "firstFrame": 72,
      "lastFrame": 115,
      "luminance": true,
      "red": false,
      "index": 1,
      "startSeconds": 2.366666666,
      "endSeconds": 3.833333333
    }
  ]
}
```

With `-frames`, `results` adds one entry per frame. Each entry has
`luminance` and `red` measurements: the alternating transitions within the last
second (`transitions`), the frame the sequence started (`since`), the region
(`region`, in source pixels), the share of it that flashed together (`area`,
percent) and whether the frame completes a failing sequence (`failed`).

## Library

```go
analysis, err := psv.AnalyzeFile(ctx, "video.mp4", psv.Options{})
if err != nil {
	return err
}

report := analysis.Report(false)
fmt.Println(report.Verdict, report.Failures.Luminance, len(report.Defects))
```

Apps that already have decoded frames, such as a screen recorder or a mobile
camera feed, use an `Analyzer` directly:

```go
a, err := psv.NewAnalyzer(30, 1920, 1080, psv.Config{Thresholds: psv.WCAG22()})
if err != nil {
	return err
}

for frame := range frames { // *psv.Frame: packed 8-bit sRGB, top row first
	result, err := a.Analyze(frame)
	if err != nil {
		return err
	}

	if result.Failed() {
		// warn as soon as a failing sequence completes
	}
}

verdict := a.Analysis().Verdict()
```

## Documentation

- [How it works](docs/how-it-works.md): the detection model, its design choices
  and its limits.
- [Testing](docs/testing.md): the conformance suite and how it is scored.

## Development

```bash
make all      # format check, lint, build
make test     # fetch the test media, run the race detector on a subset, then every test
make lint
make fmt
```

The test media is downloaded to `testdata/pse-test-media` on the first
`make test`, pinned to a fixed commit.
