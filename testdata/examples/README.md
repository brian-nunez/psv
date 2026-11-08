# Examples

Real-world videos with the `psv.report/v1` report of their last accepted
analysis (WCAG 2.2, with per-frame results). `TestExamples` decodes every video
with FFmpeg and checks that the verdict, frame count, failures and defects still
match the report; the API and CLI tests use them too.

> [!WARNING]
> Most of these videos contain flashing that may trigger seizures in people
> with photosensitive epilepsy. Do not watch them unless you know you are not
> at risk.

| Example | Video | Verdict | Defects |
| --- | --- | --- | --- |
| [`colors-epilepsy-warning`](colors-epilepsy-warning) | 0:20, 1920×1080, 30 fps | Failed | 1 (luminance and red, whole video) |
| [`electric-soldier-porygon-strobes`](electric-soldier-porygon-strobes) | 1:01, 1920×1080, 30 fps | Failed | 8 |
| [`psychedelic-flash-short`](psychedelic-flash-short) | 0:10, 608×1080, 30 fps | Failed | 1 (luminance, whole video) |
| [`video-ad-sample-2`](video-ad-sample-2) | 0:05, 1920×1080, 24 fps | Passed | 0 |

Each folder holds:

```
video.mp4     the video
source.url    where it came from
report.json   psv -format json -frames video.mp4
```

The videos come from the
[psv-data](https://github.com/brian-nunez/psv-data) repository.

## Regenerating

After a change that is meant to alter results, regenerate the reports with the
current build and review the difference:

```sh
make examples
git diff --stat testdata/examples
```

To view one in the report UI:

```sh
./bin/psv render -open testdata/examples/electric-soldier-porygon-strobes/report.json
```
