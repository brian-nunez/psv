# Examples

Real-world videos for testing PSV on real footage, decoded with FFmpeg.

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
```

The videos come from the
[psv-data](https://github.com/brian-nunez/psv-data) repository.
