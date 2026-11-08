# Testing

PSV is tested against the
[Trace RERC PSE test media](https://github.com/traceRERC/pse-test-media), a
benchmark of 346 videos built to sit just above or just below each threshold of
each standard, and against real-world example videos.

Both are in the repository:

- `testdata/pse-test-media`: the benchmark at commit `ef78542`, under its BSD
  3-Clause license.
- `testdata/examples`: real-world videos with the `psv.report/v1` report of
  their last accepted analysis. `TestExamples` decodes them with FFmpeg and
  checks that the verdict, frame count, failures and defects still match, and
  the CLI tests run them end to end. See its
  [README](../testdata/examples/README.md).

## Running

```bash
make test
```

This runs two passes:

1. The race detector over a representative subset (`go test -race -short`):
   luminance, red, area and synchronisation sets, plus the CLI tests on the
   shortest example video.
2. Every test (`go test`), including every example video. Tests that decode
   video skip when `ffmpeg` is not on `PATH`.

The conformance suite alone, without the race detector:

```bash
go test -run TestConformance .
```

A single set, or one standard:

```bash
go test -run 'TestConformance/wcag22/' .
```

```bash
go test -run 'TestConformance/trace24/trace24_30fps_sync01' -v .
```

## How the videos are made

The benchmark ships patterns, not videos: PNG masks of where things flash, CSV
files of the colour of each frame, and a JSON file per video that combines
them. `media_test.go` renders each frame in Go exactly as the benchmark's
generator does. Every opaque mask pixel takes the frame's colour, later
patterns are drawn over earlier ones, the background fills the rest, and ten
copies of the first frame are added in front. Rendering in memory keeps the
frames lossless and needs neither Python nor OpenCV.

## How the videos are scored

Each set's README names the standards it was built for, and each set's listing
CSV gives every video's verdict for them. The listing's `dimension` column names
the threshold a passing video stays under, or the failure (`FAIL`, `fail_red`,
`fail_luminance`, `fail_both`).

| Standards in the README | PSV preset |
| --- | --- |
| `wcag2` | `wcag22` |
| `trace24` | `trace24` |
| `iso`, `itu_r1702`, `ofcom` | `broadcast` |
| `none` (scene set) | `scene` |

A set built for several standards runs once per preset, so the 346 videos make
378 checks. Test names are `TestConformance/<preset>/<set>/<video>`.

The listings are used rather than each video's `expected_result` JSON because
the JSON is copied or defaulted in several sets. For example, every red video
in the broadcast red sets is labelled `pass` in its JSON and `FAIL` in its
listing, and in the trace24 sets the `wcag2_2` label always equals the
`trace24` one.

## Result

369 pass, 0 fail, 9 skipped.

## Upstream errors

Nine listing entries contradict the thresholds they test. They are skipped with
the reason, in `upstreamErrors` in `media_test.go`.

| Video | Listed | Why it cannot be right |
| --- | --- | --- |
| `wcagc_30fps_area01/f011f014`, `wcagc_30fps_area03/f011f005` | fail | Area pattern `f011` flashes on 21,282 pixels, under the 21,824 (25% of 341×256) that any placement needs, and the same size as the passing `a011` |
| `wcagc_30fps_area02/f012fr014`, `wcagc_30fps_area03/f012fr013` | fail | Area pattern `f012` flashes on 21,402 pixels, likewise the same size as the passing `a012` |
| `trace24_30fps_inf01/f003cr013a_a004fr013`, `trace24_30fps_inf01/f011cr013a_f007cr013` | pass | Their count-passing area uses `cr013alt`, which alternates by 0.10 at pattern frames 4, 5, 11, 12, 21, 22 and 31: seven transitions within one second |
| `trace24_30fps_sync01/05m90f033_05n10f033` | pass | The set's README defines m90 with n10 as just over the failing area, both flash the same failing sequence in sync, and the video's JSON expects a failure |
| `trace24_30fps_sync01/05m70f007_05n30ff007` | fail | m70 and n30 only exceed the area together, the `ff` pattern puts one transition a frame out of sync, and the video's JSON expects a pass |
| `broadcast_30fps_inf02/f003tf01_f005ico01` | fail | Its `01` patterns are grey 90 on a grey 64 background, a 0.05 change; the folder's README says every flash exceeds 0.1, which only holds on the black background `a003tf01_f008ico01` uses |

The first four are also the known issues of the earlier `epilepsy-detector`,
which only scores the 52 WCAG videos. Scored on all 346, the earlier tool
disagrees with the listings on 96.
