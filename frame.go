package psv

import (
	"fmt"
	"math"
)

// Frame is a decoded picture: packed 8-bit sRGB triplets (R, G, B),
// row-major from the top row.
type Frame struct {
	Width, Height int
	Pix           []byte
}

func (f *Frame) validate(width, height int) error {
	if f.Width != width || f.Height != height {
		return fmt.Errorf("psv: frame is %dx%d, want %dx%d", f.Width, f.Height, width, height)
	}

	if len(f.Pix) < width*height*3 {
		return fmt.Errorf("psv: frame buffer holds %d bytes, want %d", len(f.Pix), width*height*3)
	}

	return nil
}

// linear maps an 8-bit sRGB value to linear light between 0 and 1.
var linear = func() (lut [256]float64) {
	for i := range lut {
		v := float64(i) / 255
		if v <= 0.04045 {
			lut[i] = v / 12.92
		} else {
			lut[i] = math.Pow((v+0.055)/1.055, 2.4)
		}
	}

	return lut
}()

// D65 white point chromaticity, used for black, whose chromaticity is
// undefined.
const (
	whiteU = 0.19783982482140777
	whiteV = 0.46833630293240974
)

// picture holds the measurements of one frame that flash detection needs,
// one entry per analysed pixel.
type picture struct {
	// lum is the relative luminance, 0 to 1.
	lum []float32
	// rgb holds the linear red, green and blue of each pixel.
	rgb []float32
	// red marks saturated red pixels: R/(R+G+B) of at least the saturation
	// threshold in linear light.
	red []bool
}

func newPicture(n int) *picture {
	return &picture{
		lum: make([]float32, n),
		rgb: make([]float32, 3*n),
		red: make([]bool, n),
	}
}

// chromaticity returns the CIE 1976 u'v' coordinates of pixel i.
func (p *picture) chromaticity(i int) (u, v float64) {
	r, g, b := float64(p.rgb[3*i]), float64(p.rgb[3*i+1]), float64(p.rgb[3*i+2])
	X := 0.4124*r + 0.3576*g + 0.1805*b
	Y := 0.2126*r + 0.7152*g + 0.0722*b
	Z := 0.0193*r + 0.1192*g + 0.9505*b

	d := X + 15*Y + 3*Z
	if d <= 0 {
		return whiteU, whiteV
	}

	return 4 * X / d, 9 * Y / d
}

// sampler converts source frames to pictures. Sources larger than full HD are
// reduced by an integer factor, averaging each block of pixels in linear
// light, so that every picture pixel stands for factor×factor source pixels.
type sampler struct {
	srcW, srcH int
	factor     int
	width      int
	height     int
	saturation float64
}

// maxPixels is the largest picture analysed at full resolution.
const maxPixels = 1920 * 1080

func newSampler(srcW, srcH int, saturation float64) *sampler {
	factor := 1
	for (srcW/factor)*(srcH/factor) > maxPixels {
		factor++
	}

	return &sampler{
		srcW: srcW, srcH: srcH,
		factor:     factor,
		width:      srcW / factor,
		height:     srcH / factor,
		saturation: saturation,
	}
}

// sample converts picture rows y0 to y1 of f into p.
func (s *sampler) sample(f *Frame, p *picture, y0, y1 int) {
	k := s.factor
	n := float64(k * k)
	// The tolerance keeps ratios exactly at the threshold from failing on
	// rounding.
	saturation := s.saturation - 1e-9

	for y := y0; y < y1; y++ {
		for x := range s.width {
			var r, g, b float64

			if k == 1 {
				px := f.Pix[(y*s.srcW+x)*3:]
				r, g, b = linear[px[0]], linear[px[1]], linear[px[2]]
			} else {
				for dy := range k {
					row := f.Pix[((y*k+dy)*s.srcW+x*k)*3:]
					for dx := range k {
						px := row[dx*3:]
						r += linear[px[0]]
						g += linear[px[1]]
						b += linear[px[2]]
					}
				}

				r, g, b = r/n, g/n, b/n
			}

			i := y*s.width + x
			p.lum[i] = float32(0.2126*r + 0.7152*g + 0.0722*b)
			p.rgb[3*i], p.rgb[3*i+1], p.rgb[3*i+2] = float32(r), float32(g), float32(b)

			sum := r + g + b
			p.red[i] = sum > 0 && r >= saturation*sum
		}
	}
}
