# Clips

Small generated clips the CLI and API tests decode. They test the wiring (flags, exit statuses, the
`psv.report/v1` JSON, `psv render` and the API stream), not the detection, so they are kept tiny:
320×180 at 30 fps, H.264, under 100 KB together. CI runs on these; the full-resolution
real-world videos in [`testdata/examples`](../examples) run on demand.

| Clip | Content | Verdict |
| --- | --- | --- |
| `pass.mp4` | 3 seconds of still colour bars | Passed |
| `long.mp4` | 60 seconds of still colour bars, long enough for the API test to cancel an analysis that is still running | Passed |
| `fail.mp4` | 3 seconds of the whole frame flashing white on black, then red on black, five times a second | Failed: luminance and red |

> [!WARNING]
> `fail.mp4` is full-screen flashing. Do not watch it unless you know you are not at risk.

`make clips` regenerates them with FFmpeg; the commands are in the [Makefile](../../Makefile).
