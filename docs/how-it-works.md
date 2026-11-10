# How PSV works

PSV looks for flashing that the photosensitive epilepsy guidelines consider
hazardous. All of them agree that a flashing sequence is hazardous only when it
exceeds three thresholds at once:

- **Intensity**: each change is large enough.
- **Area**: enough of the field of view changes together.
- **Count**: it changes back and forth often enough within one second.

PSV measures the three for every frame. The thresholds come from a preset
(`psv.WCAG22()`, `psv.Trace24()`, `psv.Broadcast()`, `psv.SceneChange()`) or
a custom `psv.Thresholds`.

## Pipeline

```
video file ──ffmpeg──▶ RGB frames ──▶ per-pixel measurements ──▶ per-pixel transitions
                                                                       │
            verdict ◀── defects ◀── frame results ◀── region transitions ◀┘
```

1. **Decoding** (`media.go`). ffprobe reads the frame rate and size; ffmpeg
   decodes every frame once, in presentation order, to packed 8-bit RGB. The
   colour matrix and range come from the file's tags. Decoding runs ahead of
   analysis in a background goroutine.
2. **Measurement** (`frame.go`). Each pixel's sRGB values are linearised. PSV
   keeps its relative luminance and linear RGB, and marks it saturated red
   when R/(R+G+B) in linear light reaches the threshold. Frames larger than
   full HD are reduced by an integer factor, averaging blocks in linear light.
3. **Transitions** (`detector.go`). Each pixel is compared with the previous
   frame.
4. **Area and count**. Each region transitions when enough flashing pixels in it
   transition together; a region that transitions too often fails.
5. **Verdict** (`analysis.go`, `report.go`). Frames that complete a failing
   sequence become defects, and the video fails when any frame does.

## Transitions

A **luminance transition** is a frame-to-frame change in relative luminance of
at least 0.1 where the darker of the two frames is below 0.8.

A **red transition** is a frame-to-frame change in CIE 1976 u'v' chromaticity
of at least 0.2 where the pixel is saturated red in one of the two frames. A
change into saturated red goes "up", out of it "down".

Two rules make the count match how the guidelines count flashes:

- **Steps merge.** A ramp of steps in the same direction on consecutive frames
  is one transition, counted at its first step. A ramp of steps that are each
  below the threshold is no transition, even if the total change is larger.
- **Transitions alternate.** A transition only counts when it goes the opposite
  way to the pixel's last counted one. A flash is a pair of opposing
  transitions, so "more than three flashes" is seven or more alternating
  transitions.

Each pixel stores the frames of its counted transitions within the last second
as a bitmask.

## Area: flashing together

The guidelines measure the combined area of flashes that happen
*concurrently*. PSV applies that per frame and per region:

1. **Flashing pixels.** A pixel flashes when it makes more than the allowed
   alternating transitions (six under WCAG 2.2) within some one-second window.
   Only flashing pixels count toward the area.
2. **Region transitions.** For every frame, PSV counts, in every region
   position (341×256 pixels under WCAG 2.2), the flashing pixels that start a
   transition up and those that start one down in that frame, using summed-area
   tables so every position is checked. The region transitions in that
   direction when either count reaches the area threshold.
3. **Region count.** Each region position keeps its recent transitions. Only a
   transition opposite to the region's last one counts. A region with more
   than the allowed alternating transitions within one second fails, and so
   does the frame.

The two rules together keep apart what the guidelines keep apart:

- Two areas that each flash three times a second, out of step with each other,
  do not add up to six flashes: none of their pixels flash on their own.
- Two halves of an area that flash in sync combine into one area.
- If one transition of one half is a single frame late, the region misses that
  transition, because neither half covers the area alone in either frame.
- Moving patterns, where every pixel flashes fast but neighbours are a frame or
  two out of phase, still fail: in each frame enough of them transition
  together.

### Where it flashes

The region a frame reports is the area the threshold was measured in: a
fixed-size rectangle, placed where most of it transitioned. It says which
part of the screen failed, not the shape of what flashes. Failing frames
therefore also report every flashing pixel that starts a transition in that
frame, as a bounding box (`Flashes.Flashing`) and as its share of each cell of
a 64×36 grid over the frame (`Flashes.Mask`). They cover the whole frame, so a
flash larger than the region, or several flashes at once, are located in full.

### Deciding a frame takes a second

Whether a pixel flashes depends on the second around a frame, including the
second after it. PSV therefore decides each frame once one more second of video
has been seen. `Analyzer.Analyze` returns how many frames it decided, and
`Analyzer.Finish` decides the last second at the end of the video.

Only the absence of flashing needs that second. A pixel that has made more
than six transitions within the second ending at the current frame flashes,
whatever comes next: later frames can only add transitions.

### Watching live

`Analyzer.Watch` serves live streams, such as a screen guard that covers
flashing as it plays ([ADR-0004](adr/0004-watch-live-streams-with-psv-watch-v1.md)).
It runs the same detector as `Analyze` but keeps no decided frames, so memory
stays flat, and returns a `psv.watch/v1` update for every frame:

- `Building` measures, in the frame being watched, each pixel that starts a
  transition by its transitions within the last second: the highest count
  (`Peak`), the share of the frame that already flashes (`Flashing`) and its
  bounding box (`Bounds`), the share that transitions (`Area`), and the highest
  count in each cell of the 64×36 grid (`Mask`). It reads the per-pixel history
  the detector already keeps and changes nothing, so it costs a bit count per
  transitioning pixel.
- `Decided` carries the frames decided by this one, a second behind and
  identical to those of `Analyze`.

`Building` is provisional and pixel-level. Flashing pixels it reports are
final, and appear in the frame that completes a failing sequence, a second
before the failure is decided. It does not apply the area threshold, so it also
reports flashing too small to fail.

Frame numbers are stored as 32-bit integers. Every 2^30 frames, about 200 days
at 60 fps, the detector moves them back by a multiple of the frame rate, so a
watch can run indefinitely while the frame numbers it reports keep counting.

## Timing

The count window is one second of frames: `round(fps)` frames, so 29.97 fps
uses 30. Seven transitions fail when the first and the seventh are less than a
second apart, that is, at most `rate − 1` frames apart. Frame times are frame
numbers divided by the rate. Variable frame rate video can be converted to a
constant rate with `-fps`.

## Regions in pixels

WCAG 2.2 defines the area as 25% of a 10° visual field, which it equates to a
341×256 pixel rectangle. PSV measures that rectangle in source pixels at every
position, so the threshold does not change with the video's resolution. Frames
smaller than the region use the whole frame. The broadcast and scene presets
measure the whole screen.

## Performance

A 1080p frame has two million pixels, and strobe footage can make most of them
transition at once. PSV keeps this fast without approximating:

- Pixels that do not transition cost one comparison per frame; their
  histories are only updated when they transition.
- Chromaticity is only computed for pixels that are saturated red in one of the
  two frames.
- Each frame is split into row bands analysed in parallel.
- The region search only runs in frames where enough flashing pixels
  transition to reach the area threshold, and runs in parallel.
- ffmpeg decodes the next frames while the current one is analysed.

On a 10-core laptop, a 61-second 1080p strobe clip takes about 16 seconds.

## Design choices and limits

- **Saturated red in linear light.** PSV computes R/(R+G+B) on linearised
  values. The test media's saturated-red patterns sit at 0.80 in linear light
  and 0.57–0.66 gamma-encoded, and its red tests only agree with the linear
  reading.
- **Frame-exact concurrency.** A region transitions when enough of it
  transitions in the same frame, as the test media requires. A flash whose
  edge sweeps across a region over several frames, such as a rolling shutter,
  is measured frame by frame and may count as a smaller area than a viewer
  perceives.
- **One display.** Luminance is relative (0 to 1). PSV does not model the
  display's brightness, HDR or viewing distance.
- **Frame-based timing.** Timestamps are frame numbers. Convert variable frame
  rate video with `-fps` for accurate timing.
- **Large frames.** Frames above full HD are reduced by averaging. The region is
  scaled to match, but very fine flashing detail can blend.
- **Spatial patterns.** Regular patterns such as stripes and checkerboards are a
  separate hazard in the guidelines and are not checked.
