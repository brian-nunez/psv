// Package psv is the core of Photosafety Validator: it analyses video for
// flashing that can trigger photosensitive seizures, following WCAG 2.2
// Success Criterion 2.3.1 (Three Flashes or Below Threshold).
//
// Each frame is compared with the previous one pixel by pixel. A pixel makes
// a general (luminance) transition when its relative luminance changes by at
// least 0.1 and the darker side is below 0.8, and a red transition when its
// u'v' chromaticity changes by at least 0.2 with a saturated red on one side.
// A 341×256-pixel region transitions when at least a quarter of it starts a
// transition in the same direction in the same frame, and a region that makes
// more than three flashes, seven alternating transitions, within one second
// fails. The Trace24 preset measures area in 416×416-pixel regions instead.
//
// The package decodes video with FFmpeg (AnalyzeFile) or analyses frames
// supplied by the caller (Analyzer). Analyzer.Analyze keeps every decided
// frame for a report; Analyzer.Watch serves live streams, reporting flashing
// as it builds up in each frame and keeping nothing, in the psv.watch/v1
// format.
package psv
