package psv

import (
	"encoding/binary"
	"image"
	"math"
	"math/bits"
	"runtime"
	"sort"
	"sync"
)

// Flash detection follows WCAG 2.2 Success Criterion 2.3.1.
//
// A pixel makes a luminance transition between two frames when its relative
// luminance changes by at least Thresholds.LuminanceChange and the darker of
// the two is below Thresholds.DarkLimit. It makes a red transition when its
// u'v' chromaticity changes by at least Thresholds.RedChange and it is
// saturated red in one of the two frames. Steps in the same direction on
// consecutive frames are one transition.
//
// Each pixel keeps the frames of its alternating transitions within the last
// second. A sequence of transitions is shared by every pixel that made a
// transition on each of its frames. A region of
// Thresholds.RegionWidth×RegionHeight pixels, or the whole frame, fails when
// a sequence of more than Thresholds.MaxTransitions transitions is shared by
// pixels covering at least Thresholds.AreaPercent of it; under WCAG 2.2 that
// is seven transitions, more than three flashes. Areas that flash out of
// step, even by one frame, are measured separately.

// Thresholds are the detection parameters.
type Thresholds struct {
	// LuminanceChange is the change in relative luminance, 0 to 1, that
	// makes a luminance transition.
	LuminanceChange float64 `json:"luminanceChange"`
	// DarkLimit is the relative luminance the darker side of a luminance
	// transition must be below.
	DarkLimit float64 `json:"darkLimit"`
	// RedChange is the change in u'v' chromaticity that makes a red
	// transition.
	RedChange float64 `json:"redChange"`
	// RedSaturation is the share of red, R/(R+G+B) in linear light, at which
	// a colour is saturated red.
	RedSaturation float64 `json:"redSaturation"`
	// RegionWidth and RegionHeight are the size, in pixels, of the regions in
	// which the flashing area is measured. Zero measures it over the whole
	// frame.
	RegionWidth  int `json:"regionWidth"`
	RegionHeight int `json:"regionHeight"`
	// AreaPercent is the share of a region, in percent, that must transition
	// together for the region to transition.
	AreaPercent float64 `json:"areaPercent"`
	// MaxTransitions is the most alternating transitions a region may make
	// within one second. A flash is a pair of opposing transitions, so six
	// allows three flashes.
	MaxTransitions int `json:"maxTransitions"`
}

// WCAG22 returns the thresholds of WCAG 2.2 Success Criterion 2.3.1: the
// general flash and red flash thresholds, with the flashing area measured in
// any 341×256-pixel region, the reference for a quarter of a 10° visual field.
func WCAG22() Thresholds {
	return Thresholds{
		LuminanceChange: 0.1,
		DarkLimit:       0.8,
		RedChange:       0.2,
		RedSaturation:   0.8,
		RegionWidth:     341,
		RegionHeight:    256,
		AreaPercent:     25,
		MaxTransitions:  6,
	}
}

// Trace24 returns the thresholds of the photosensitive epilepsy guidelines
// proposed by the Trace Research & Development Center in 2024. They match
// WCAG 2.2 but measure the flashing area in any 416×416-pixel region.
func Trace24() Thresholds {
	th := WCAG22()
	th.RegionWidth, th.RegionHeight = 416, 416

	return th
}

// Broadcast returns the thresholds of the broadcast guidelines: ITU-R
// BT.1702, the Ofcom guidance note on flashing images and ISO 9241-391. A
// flash of 20 cd/m² on a 200 cd/m² display is a change of 0.1 in relative
// luminance, and the flashing area is a quarter of the whole screen.
func Broadcast() Thresholds {
	th := WCAG22()
	th.RegionWidth, th.RegionHeight = 0, 0

	return th
}

// SceneChange returns the thresholds of the experimental scene change hazard
// defined by the Trace RERC PSE test media: luminance changes of at least 0.1
// over at least 80% of the screen, more than three alternating transitions in
// one second.
func SceneChange() Thresholds {
	th := Broadcast()
	th.AreaPercent = 80
	th.MaxTransitions = 3

	return th
}

// DefaultThresholds returns the WCAG 2.2 thresholds.
func DefaultThresholds() Thresholds {
	return WCAG22()
}

// Flashes is the flash measurement of one kind in one frame.
type Flashes struct {
	// Transitions is the number of alternating transitions within the last
	// second of the pixels that flash together over at least the area
	// threshold of a region, for the group with the most of them, among the
	// pixels that transitioned in this frame.
	Transitions int `json:"transitions"`
	// Since is the frame number of the first of those transitions.
	Since int `json:"since,omitempty"`
	// Region is that region, in source pixels.
	Region image.Rectangle `json:"region"`
	// Area is the share of that region, in percent, covered by those
	// pixels; zero when no pixels transitioning together cover the area
	// threshold of a region.
	Area float64 `json:"area"`
	// Failed reports that the region flashed too often.
	Failed bool `json:"failed"`
}

// FrameResult is the analysis of one frame.
type FrameResult struct {
	Luminance Flashes `json:"luminance"`
	Red       Flashes `json:"red"`
}

// Failed reports whether the frame completes a failing flash sequence.
func (r *FrameResult) Failed() bool {
	return r.Luminance.Failed || r.Red.Failed
}

// detector runs flash detection over a sequence of pictures.
type detector struct {
	th     Thresholds
	rate   int
	width  int
	height int
	// factor is the number of source pixels per picture pixel along each
	// axis.
	factor int

	sample    *sampler
	frame     int
	cur, prev *picture

	luma, red *channel

	// bands split each frame into row ranges processed in parallel, with the
	// pixels each band found starting a transition.
	bands []band
}

type band struct {
	y0, y1    int
	luma, red []int32
}

func newDetector(rate int, s *sampler, th Thresholds) *detector {
	width, height, factor := s.width, s.height, s.factor

	region := func(size, frame int) int {
		if size <= 0 {
			return frame
		}

		return min(max(int(math.Round(float64(size)/float64(factor))), 1), frame)
	}
	rw, rh := region(th.RegionWidth, width), region(th.RegionHeight, height)

	n := min(runtime.GOMAXPROCS(0), height)
	bands := make([]band, n)

	for b := range bands {
		bands[b] = band{y0: height * b / n, y1: height * (b + 1) / n}
	}

	return &detector{
		th:     th,
		rate:   rate,
		width:  width,
		height: height,
		factor: factor,
		sample: s,
		cur:    newPicture(width * height),
		prev:   newPicture(width * height),
		luma:   newChannel(width, height, rw, rh, rate),
		red:    newChannel(width, height, rw, rh, rate),
		bands:  bands,
	}
}

// process analyses frame f against the previous one.
func (d *detector) process(f *Frame) FrameResult {
	t := d.frame
	d.frame++

	defer func() { d.cur, d.prev = d.prev, d.cur }()

	var wg sync.WaitGroup

	for b := range d.bands {
		wg.Add(1)

		go func(b *band) {
			defer wg.Done()

			d.sample.sample(f, d.cur, b.y0, b.y1)

			if t > 0 {
				d.steps(b, t)
			}
		}(&d.bands[b])
	}

	wg.Wait()

	if t == 0 {
		return FrameResult{}
	}

	for b := range d.bands {
		d.luma.started = append(d.luma.started, d.bands[b].luma...)
		d.red.started = append(d.red.started, d.bands[b].red...)
	}

	var res FrameResult

	if len(d.luma.started) > 0 {
		res.Luminance = d.luma.regions(t, d.th.MaxTransitions, d.th.AreaPercent)
	}

	if len(d.red.started) > 0 {
		res.Red = d.red.regions(t, d.th.MaxTransitions, d.th.AreaPercent)
	}

	d.finish(&res.Luminance)
	d.finish(&res.Red)

	return res
}

// steps advances the transition state of the pixels of one band.
func (d *detector) steps(b *band, t int) {
	b.luma, b.red = b.luma[:0], b.red[:0]

	for i := b.y0 * d.width; i < b.y1*d.width; i++ {
		if d.luma.step(i, t, d.lumaStep(i)) {
			b.luma = append(b.luma, int32(i))
		}

		if d.red.step(i, t, d.redStep(i)) {
			b.red = append(b.red, int32(i))
		}
	}
}

// finish marks failure and converts a measurement to frame numbers from 1
// and source pixels.
func (d *detector) finish(f *Flashes) {
	f.Failed = f.Transitions > d.th.MaxTransitions

	if f.Transitions > 0 {
		f.Since++
	}

	f.Region.Min = f.Region.Min.Mul(d.factor)
	f.Region.Max = f.Region.Max.Mul(d.factor)
}

// lumaStep returns the direction of pixel i's luminance transition from the
// previous picture, or 0.
func (d *detector) lumaStep(i int) int8 {
	a, b := float64(d.prev.lum[i]), float64(d.cur.lum[i])

	if math.Abs(b-a) < d.th.LuminanceChange || min(a, b) >= d.th.DarkLimit {
		return 0
	}

	if b > a {
		return 1
	}

	return -1
}

// redStep returns the direction of pixel i's red transition from the previous
// picture: +1 toward saturated red, -1 away from it, or 0.
func (d *detector) redStep(i int) int8 {
	cur, prev := d.cur.red[i], d.prev.red[i]
	if !cur && !prev {
		return 0
	}

	cu, cv := d.cur.chromaticity(i)
	pu, pv := d.prev.chromaticity(i)
	du, dv := cu-pu, cv-pv

	if math.Hypot(du, dv) < d.th.RedChange {
		return 0
	}

	switch {
	case cur && !prev:
		return 1
	case prev && !cur:
		return -1
	case du >= 0:
		// Between two saturated reds, toward the redder one.
		return 1
	default:
		return -1
	}
}

// channel tracks the transitions of one kind of flash.
type channel struct {
	width, height int
	// rw and rh are the region size.
	rw, rh int

	// Per pixel: the step direction in the previous frame and the direction
	// of the last counted transition.
	stepDir []int8
	lastDir []int8
	// hist holds, per pixel, words bits marking the frames of the last
	// second in which it made an alternating transition; bit k is k frames
	// before the frame in updated. window masks the bits of one second.
	hist    []uint64
	updated []int32
	words   int
	window  []uint64

	// started lists the pixels that make an alternating transition in this
	// frame; shared is scratch space for sharing.
	started []int32
	shared  []int32

	mask, sum []int32
}

func newChannel(width, height, rw, rh, rate int) *channel {
	n := width * height
	words := (rate + 63) / 64

	c := &channel{
		width: width, height: height,
		rw: rw, rh: rh,
		stepDir: make([]int8, n),
		lastDir: make([]int8, n),
		hist:    make([]uint64, n*words),
		updated: make([]int32, n),
		words:   words,
		window:  make([]uint64, words),
		mask:    make([]int32, n),
		sum:     make([]int32, (width+1)*(height+1)),
	}

	for k := range rate {
		c.window[k/64] |= 1 << (k % 64)
	}

	return c
}

// step records pixel i's step direction in frame t and reports whether it
// makes an alternating transition: a step that starts a transition opposite
// to the pixel's last one.
func (c *channel) step(i, t int, dir int8) bool {
	starts := dir != 0 && dir != c.stepDir[i]
	c.stepDir[i] = dir

	if !starts || dir == c.lastDir[i] {
		return false
	}

	c.lastDir[i] = dir

	h := c.hist[i*c.words : (i+1)*c.words]
	c.age(h, t-int(c.updated[i]))
	c.updated[i] = int32(t)
	h[0] |= 1

	return true
}

// age moves a history n frames into the past, dropping what falls out of the
// window.
func (c *channel) age(h []uint64, n int) {
	words, shift := n/64, uint(n%64)

	for w := len(h) - 1; w >= 0; w-- {
		var v uint64

		if src := w - words; src >= 0 {
			v = h[src] << shift
			if shift > 0 && src > 0 {
				v |= h[src-1] >> (64 - shift)
			}
		}

		h[w] = v & c.window[w]
	}
}

// sequence is a set of transition frames, as a history bitmask, with the
// pixels whose transitions within the last second are exactly those.
type sequence struct {
	bits        []uint64
	transitions int
	own         []int32
}

// first returns how many frames ago the oldest transition was.
func (q *sequence) first() int {
	for w := len(q.bits) - 1; w >= 0; w-- {
		if q.bits[w] != 0 {
			return w*64 + 63 - bits.LeadingZeros64(q.bits[w])
		}
	}

	return 0
}

// maxShared bounds the failing sequences whose sharing pixels are gathered in
// one frame.
const maxShared = 16

// regions finds the transition sequences of the pixels that transitioned in
// frame t and returns the measurement of the one with the most transitions
// that covers the area threshold of a region.
func (c *channel) regions(t, maxTransitions int, areaPercent float64) Flashes {
	defer func() { c.started = c.started[:0] }()

	area := c.rw * c.rh
	need := areaPercent * float64(area) / 100

	index := map[string]*sequence{}
	key := make([]byte, 0, c.words*8)

	for _, i := range c.started {
		h := c.hist[int(i)*c.words : (int(i)+1)*c.words]

		key = key[:0]
		for _, w := range h {
			key = binary.LittleEndian.AppendUint64(key, w)
		}

		q, ok := index[string(key)]
		if !ok {
			n := 0
			for _, w := range h {
				n += bits.OnesCount64(w)
			}

			q = &sequence{bits: h, transitions: n}
			index[string(key)] = q
		}

		q.own = append(q.own, i)
	}

	seqs := make([]*sequence, 0, len(index))
	for _, q := range index {
		seqs = append(seqs, q)
	}

	// Most transitions first, so the first sequence that covers the area
	// threshold is the one to report.
	sort.Slice(seqs, func(a, b int) bool {
		if seqs[a].transitions != seqs[b].transitions {
			return seqs[a].transitions > seqs[b].transitions
		}

		return len(seqs[a].own) > len(seqs[b].own)
	})

	// A sequence that fails the count is also shared by pixels with further
	// transitions. Gathering those costs a pass over every transitioning
	// pixel, so it is done for the largest failing sequences only.
	var failing []*sequence

	for _, q := range seqs {
		if q.transitions > maxTransitions {
			failing = append(failing, q)
		}
	}

	sort.SliceStable(failing, func(a, b int) bool { return len(failing[a].own) > len(failing[b].own) })

	shares := map[*sequence]bool{}
	for _, q := range failing[:min(len(failing), maxShared)] {
		shares[q] = true
	}

	for _, q := range seqs {
		pixels := q.own
		if shares[q] {
			pixels = c.sharing(q)
		}

		if float64(len(pixels)) < need {
			continue
		}

		moved, x, y := c.densest(pixels)
		if float64(moved) < need {
			continue
		}

		return Flashes{
			Transitions: q.transitions,
			Since:       t - q.first(),
			Region:      image.Rect(x, y, x+c.rw, y+c.rh),
			Area:        float64(moved) * 100 / float64(area),
		}
	}

	return Flashes{}
}

// sharing returns the pixels that transitioned in this frame and made a
// transition on every frame of q. The slice is reused by the next call.
func (c *channel) sharing(q *sequence) []int32 {
	parts := make([][]int32, workers(len(c.started)))

	parallel(len(c.started), len(parts), func(part, lo, hi int) {
		var out []int32

		for _, i := range c.started[lo:hi] {
			h := c.hist[int(i)*c.words : (int(i)+1)*c.words]
			shares := true

			for w, b := range q.bits {
				if h[w]&b != b {
					shares = false

					break
				}
			}

			if shares {
				out = append(out, i)
			}
		}

		parts[part] = out
	})

	out := c.shared[:0]
	for _, p := range parts {
		out = append(out, p...)
	}

	c.shared = out

	return out
}

// densest returns the most pixels of the set that any region holds, and that
// region's position; ties go to the topmost, then leftmost. Only regions that
// overlap the set's bounding box are searched.
func (c *channel) densest(pixels []int32) (count, x, y int) {
	if len(pixels) == 0 {
		return 0, 0, 0
	}

	minX, minY, maxX, maxY := c.width, c.height, 0, 0

	for _, i := range pixels {
		px, py := int(i)%c.width, int(i)/c.width
		minX, maxX = min(minX, px), max(maxX, px)
		minY, maxY = min(minY, py), max(maxY, py)
	}

	// Region positions that overlap the box, and the area they span.
	x0, x1 := max(minX-c.rw+1, 0), min(maxX, c.width-c.rw)
	y0, y1 := max(minY-c.rh+1, 0), min(maxY, c.height-c.rh)
	w, h := x1-x0+c.rw, y1-y0+c.rh

	mask, sum := c.mask[:w*h], c.sum[:(w+1)*(h+1)]
	clear(mask)

	for _, i := range pixels {
		mask[(int(i)/c.width-y0)*w+int(i)%c.width-x0] = 1
	}

	integral(mask, sum, w, h)

	stride := w + 1
	rows := y1 - y0 + 1

	type peak struct{ count, x, y int }

	peaks := make([]peak, workers(rows))

	parallel(rows, len(peaks), func(part, lo, hi int) {
		var p peak

		for ry := lo; ry < hi; ry++ {
			for rx := 0; rx <= x1-x0; rx++ {
				n := int(sum[(ry+c.rh)*stride+rx+c.rw] - sum[ry*stride+rx+c.rw] -
					sum[(ry+c.rh)*stride+rx] + sum[ry*stride+rx])
				if n > p.count {
					p = peak{n, rx, ry}
				}
			}
		}

		peaks[part] = p
	})

	var best peak
	for _, p := range peaks {
		if p.count > best.count {
			best = p
		}
	}

	return best.count, best.x + x0, best.y + y0
}

// workers returns how many goroutines to split n items across.
func workers(n int) int {
	return max(min(runtime.GOMAXPROCS(0), n), 1)
}

// parallel splits [0, n) into parts contiguous ranges and runs fn on each
// concurrently.
func parallel(n, parts int, fn func(part, lo, hi int)) {
	var wg sync.WaitGroup

	for p := range parts {
		wg.Add(1)

		go func() {
			defer wg.Done()

			fn(p, n*p/parts, n*(p+1)/parts)
		}()
	}

	wg.Wait()
}

// integral fills t with the summed-area table of the w×h plane v, with one
// extra row and column of zeros at the top and left.
func integral(v, t []int32, w, h int) {
	stride := w + 1

	clear(t[:stride])

	// Row sums, then column sums, each split across goroutines.
	parallel(h, workers(h), func(_, lo, hi int) {
		for y := lo + 1; y <= hi; y++ {
			var row int32

			t[y*stride] = 0

			for x := 1; x <= w; x++ {
				row += v[(y-1)*w+x-1]
				t[y*stride+x] = row
			}
		}
	})

	parallel(w, workers(w), func(_, lo, hi int) {
		for y := 2; y <= h; y++ {
			above, cur := t[(y-1)*stride+lo+1:(y-1)*stride+hi+1], t[y*stride+lo+1:y*stride+hi+1]
			for x := range cur {
				cur[x] += above[x]
			}
		}
	})
}
