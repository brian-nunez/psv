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
            verdict ◀── defects ◀── frame results ◀── shared sequences ◀┘
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
4. **Sequences and area**. Pixels whose transitions within the last second fall
   on the same frames are grouped, and the group is measured against the area
   threshold.
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

The guidelines measure the area of flashes that happen *concurrently*. PSV
takes that literally: pixels flash together only when their transitions fall
on exactly the same frames.

For every frame in which pixels transition, PSV:

1. Groups those pixels by their transition bitmask. A group is a set of pixels
   flashing in lockstep.
2. For a group that has more transitions than the count allows, also adds every
   pixel whose transitions *include* the group's frames. A pixel that flashes
   with the group and also flashes on its own (where two flashing areas
   overlap) still flashes with the group.
3. Finds the region (341×256 pixels under WCAG 2.2) holding the most of those
   pixels, with a summed-area table so that every region position is checked.
4. Reports the group with the most transitions whose pixels cover at least the
   area threshold of one region.

A frame **fails** when that group has more transitions than allowed.

This rule keeps apart what the guidelines keep apart:

- Two areas that each flash three times a second, out of step with each other,
  do not add up to six flashes.
- Two halves of an area that flash in sync combine into one area.
- If one transition of one half is a single frame late, the halves no longer
  flash together and are measured separately.

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
- Groups are tried in order of transition count, and the search stops at the
  first one that covers the area threshold. The region search only covers the
  group's bounding box and runs in parallel.
- ffmpeg decodes the next frames while the current one is analysed.

On a 10-core laptop, a 61-second 1080p strobe clip takes about 22 seconds and a
20-second clip of ordinary moving footage about 13 seconds.

## Design choices and limits

- **Saturated red in linear light.** PSV computes R/(R+G+B) on linearised
  values. The test media's saturated-red patterns sit at 0.80 in linear light
  and 0.57–0.66 gamma-encoded, and its red tests only agree with the linear
  reading.
- **Exact synchrony.** Areas flashing even one frame apart are measured
  separately, as the test media requires. Real footage whose flashes spread
  over two frames, such as a rolling shutter, may be measured as smaller areas
  than a viewer perceives.
- **One display.** Luminance is relative (0 to 1). PSV does not model the
  display's brightness, HDR or viewing distance.
- **Frame-based timing.** Timestamps are frame numbers. Convert variable frame
  rate video with `-fps` for accurate timing.
- **Large frames.** Frames above full HD are reduced by averaging. The region is
  scaled to match, but very fine flashing detail can blend.
- **Spatial patterns.** Regular patterns such as stripes and checkerboards are a
  separate hazard in the guidelines and are not checked.
