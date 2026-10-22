package psv

import (
	"image"
	"math"
	"math/bits"
	"runtime"
	"sync"
)

// Flash detection follows WCAG 2.2 Success Criterion 2.3.1.
//
// A pixel makes a luminance transition between two frames when its relative
// luminance changes by at least Thresholds.LuminanceChange and the darker of
// the two is below Thresholds.DarkLimit. It makes a red transition when its
// u'v' chromaticity changes by at least Thresholds.RedChange and it is
// saturated red in one of the two frames. Steps in the same direction on
// consecutive frames are one transition, and only transitions opposite to
// the pixel's last one count.
//
// A pixel flashes when it makes more than Thresholds.MaxTransitions
// transitions within one second. A region of
// Thresholds.RegionWidth×RegionHeight pixels, or the whole frame, transitions
// in a frame when flashing pixels covering at least Thresholds.AreaPercent of
// it transition in the same direction in that frame. A region that makes more
// than Thresholds.MaxTransitions alternating transitions within one second
// fails; under WCAG 2.2 that is seven transitions, more than three flashes.
//
// Whether a pixel flashes depends on its transitions up to a second after
// the frame, so a frame is decided one second after it is seen.

// detector runs flash detection over a sequence of frames.
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
	// transitions each band found.
	bands []band
}

type band struct {
	y0, y1    int
	luma, red []event
}

// event is a pixel starting an alternating transition.
type event struct {
	pixel int32
	dir   int8
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

	n := workers(height)
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
		luma:   newChannel(width, height, rw, rh, rate, th.MaxTransitions),
		red:    newChannel(width, height, rw, rh, rate, th.MaxTransitions),
		bands:  bands,
	}
}

// delay is the number of frames between seeing a frame and deciding it.
func (d *detector) delay() int {
	return d.rate - 1
}

// process takes frame f and returns the results of the frames it decides.
func (d *detector) process(f *Frame) []FrameResult {
	t := d.frame
	d.frame++

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

	d.cur, d.prev = d.prev, d.cur

	luma, red := d.luma.pending(t), d.red.pending(t)
	for b := range d.bands {
		luma = append(luma, d.bands[b].luma...)
		red = append(red, d.bands[b].red...)
	}

	d.luma.setPending(t, luma)
	d.red.setPending(t, red)

	if decided := t - d.delay(); decided >= 0 {
		return []FrameResult{d.decide(decided, t)}
	}

	return nil
}

// flush decides the frames still waiting for later frames.
func (d *detector) flush() []FrameResult {
	last := d.frame - 1

	var out []FrameResult

	for f := max(last-d.delay()+1, 0); f <= last; f++ {
		out = append(out, d.decide(f, last))
	}

	return out
}

// decide measures frame f, knowing the frames up to now.
func (d *detector) decide(f, now int) FrameResult {
	res := FrameResult{
		Luminance: d.luma.regions(f, now, d.th.AreaPercent),
		Red:       d.red.regions(f, now, d.th.AreaPercent),
	}

	d.finish(&res.Luminance)
	d.finish(&res.Red)

	return res
}

// steps advances the transition state of the pixels of one band in frame t.
func (d *detector) steps(b *band, t int) {
	b.luma, b.red = b.luma[:0], b.red[:0]

	for i := b.y0 * d.width; i < b.y1*d.width; i++ {
		if dir := d.lumaStep(i); d.luma.step(i, t, dir) {
			b.luma = append(b.luma, event{int32(i), dir})
		}

		if dir := d.redStep(i); d.red.step(i, t, dir) {
			b.red = append(b.red, event{int32(i), dir})
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

// lumaStep returns the direction of pixel i's luminance step from the
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

// redStep returns the direction of pixel i's red step from the previous
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
	// rw and rh are the region size; cols and rows the number of region
	// positions along each axis.
	rw, rh     int
	cols, rows int

	rate, maxTransitions int

	// Per pixel: the step direction in the previous frame, the direction of
	// the last counted transition, and a bitmask of the frames of its counted
	// transitions, where bit k is k frames before the frame in updated.
	stepDir []int8
	lastDir []int8
	hist    []uint64
	updated []int32
	words   int

	// queue holds, per frame waiting to be decided, the pixels that started
	// a transition in it; slot t%rate is frame t.
	queue [][]event

	up, down       []int32
	sumUp, sumDown []int32

	// Per region position: the direction of its last counted transition and
	// the frames of its most recent alternating transitions, kept in a ring
	// of keep entries.
	posDir []int8
	times  []int32
	head   []uint8
	keep   int
}

func newChannel(width, height, rw, rh, rate, maxTransitions int) *channel {
	n := width * height
	cols, rows := width-rw+1, height-rh+1
	positions := cols * rows
	// The history spans the second before and the second after a frame.
	words := (2*rate + 63) / 64
	keep := maxTransitions + 1

	c := &channel{
		width: width, height: height,
		rw: rw, rh: rh,
		cols: cols, rows: rows,
		rate: rate, maxTransitions: maxTransitions,
		stepDir: make([]int8, n),
		lastDir: make([]int8, n),
		hist:    make([]uint64, n*words),
		updated: make([]int32, n),
		words:   words,
		queue:   make([][]event, rate),
		up:      make([]int32, n),
		down:    make([]int32, n),
		sumUp:   make([]int32, (width+1)*(height+1)),
		sumDown: make([]int32, (width+1)*(height+1)),
		posDir:  make([]int8, positions),
		times:   make([]int32, positions*keep),
		head:    make([]uint8, positions),
		keep:    keep,
	}

	for i := range c.times {
		c.times[i] = math.MinInt32
	}

	return c
}

func (c *channel) pending(t int) []event {
	return c.queue[t%c.rate][:0]
}

func (c *channel) setPending(t int, events []event) {
	c.queue[t%c.rate] = events
}

// step records pixel i's step direction in frame t and reports whether it
// starts an alternating transition: a step that starts a transition opposite
// to the pixel's last counted one.
func (c *channel) step(i, t int, dir int8) bool {
	starts := dir != 0 && dir != c.stepDir[i]
	c.stepDir[i] = dir

	if !starts || dir == c.lastDir[i] {
		return false
	}

	c.lastDir[i] = dir

	h := c.hist[i*c.words : (i+1)*c.words]
	age(h, t-int(c.updated[i]))
	c.updated[i] = int32(t)
	h[0] |= 1

	return true
}

// age moves a history n frames into the past, dropping what falls out of it.
func age(h []uint64, n int) {
	words, shift := n/64, uint(n%64)

	for w := len(h) - 1; w >= 0; w-- {
		var v uint64

		if src := w - words; src >= 0 {
			v = h[src] << shift
			if shift > 0 && src > 0 {
				v |= h[src-1] >> (64 - shift)
			}
		}

		h[w] = v
	}
}

// flashing reports whether pixel i makes more than maxTransitions
// transitions within some one-second window that contains frame f, knowing
// the frames up to now.
func (c *channel) flashing(i, f, now int) bool {
	h := c.hist[i*c.words : (i+1)*c.words]
	upd := int(c.updated[i])
	span := c.rate - 1

	// Counts the transitions in frames lo..hi.
	count := func(lo, hi int) int {
		n := 0

		for k := max(upd-hi, 0); k <= upd-lo && k < c.words*64; k++ {
			if h[k/64]>>(k%64)&1 != 0 {
				n++
			}
		}

		return n
	}

	if c.words == 1 {
		count = func(lo, hi int) int {
			kLo, kHi := upd-hi, upd-lo
			if kHi < 0 {
				return 0
			}

			kLo = max(kLo, 0)
			if kLo > 63 {
				return 0
			}

			width := min(kHi, 63) - kLo + 1

			return bits.OnesCount64(h[0] >> kLo & (1<<width - 1))
		}
	}

	for start := f - span; start <= f; start++ {
		if count(start, min(start+span, now)) > c.maxTransitions {
			return true
		}
	}

	return false
}

// regions decides frame f, knowing the frames up to now: it finds the
// regions that transition in f and returns the measurement of the one with
// the most alternating transitions within the second ending at f.
func (c *channel) regions(f, now int, areaPercent float64) Flashes {
	events := c.queue[f%c.rate]

	area := c.rw * c.rh
	need := areaPercent * float64(area) / 100

	// Keep the transitions of flashing pixels.
	parts := make([][]event, workers(len(events)))

	parallel(len(events), len(parts), func(part, lo, hi int) {
		var out []event

		for _, e := range events[lo:hi] {
			if c.flashing(int(e.pixel), f, now) {
				out = append(out, e)
			}
		}

		parts[part] = out
	})

	ups, downs := 0, 0

	for _, p := range parts {
		for _, e := range p {
			if e.dir > 0 {
				c.up[e.pixel] = 1
				ups++
			} else {
				c.down[e.pixel] = 1
				downs++
			}
		}
	}

	defer func() {
		for _, p := range parts {
			for _, e := range p {
				c.up[e.pixel], c.down[e.pixel] = 0, 0
			}
		}
	}()

	if float64(ups) < need && float64(downs) < need {
		return Flashes{}
	}

	integral(c.up, c.sumUp, c.width, c.height)
	integral(c.down, c.sumDown, c.width, c.height)

	type peak struct {
		count, moved, since, x, y int
	}

	peaks := make([]peak, workers(c.rows))
	stride := c.width + 1
	box := func(s []int32, x, y int) int {
		return int(s[(y+c.rh)*stride+x+c.rw] - s[y*stride+x+c.rw] - s[(y+c.rh)*stride+x] + s[y*stride+x])
	}

	parallel(c.rows, len(peaks), func(part, lo, hi int) {
		var p peak

		for y := lo; y < hi; y++ {
			for x := range c.cols {
				up, down := box(c.sumUp, x, y), box(c.sumDown, x, y)

				var dir int8

				switch {
				case float64(up) >= need && up >= down:
					dir = 1
				case float64(down) >= need:
					dir = -1
				default:
					continue
				}

				pos := y*c.cols + x
				if dir == c.posDir[pos] {
					continue
				}

				count, since := c.record(pos, dir, f)
				moved := max(up, down)

				if count > p.count || (count == p.count && moved > p.moved) {
					p = peak{count, moved, since, x, y}
				}
			}
		}

		peaks[part] = p
	})

	var best peak
	for _, p := range peaks {
		if p.count > best.count || (p.count == best.count && p.moved > best.moved) {
			best = p
		}
	}

	if best.count == 0 {
		return Flashes{}
	}

	return Flashes{
		Transitions: best.count,
		Since:       best.since,
		Region:      image.Rect(best.x, best.y, best.x+c.rw, best.y+c.rh),
		Area:        float64(best.moved) * 100 / float64(area),
	}
}

// record adds a transition of region position pos in frame f and returns the
// number of alternating transitions within the second ending at f and the
// frame of the first of them.
func (c *channel) record(pos int, dir int8, f int) (count, since int) {
	c.posDir[pos] = dir
	ring := c.times[pos*c.keep : (pos+1)*c.keep]
	ring[c.head[pos]] = int32(f)
	c.head[pos] = uint8((int(c.head[pos]) + 1) % c.keep)

	since = f

	for _, at := range ring {
		if int(at) > f-c.rate {
			count++
			since = min(since, int(at))
		}
	}

	return count, since
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
