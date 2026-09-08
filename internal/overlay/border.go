package overlay

import (
	"image"
	"image/color"
	"math"
)

type borderSpec struct {
	W, H, Thick int
	Color       color.RGBA
	Peak        float64 // alpha at the very edge, 0..1
}

// stripSet holds the four edge strips and their base alpha so frames only rescale alpha.
type stripSet struct {
	Top, Bottom, Left, Right *image.RGBA
	Alpha                    [4][]float64
}

func (s *stripSet) images() [4]*image.RGBA { return [4]*image.RGBA{s.Top, s.Bottom, s.Left, s.Right} }

func renderStrips(spec borderSpec) *stripSet {
	th := spec.Thick
	s := &stripSet{
		Top:    image.NewRGBA(image.Rect(0, 0, spec.W, th)),
		Bottom: image.NewRGBA(image.Rect(0, 0, spec.W, th)),
		Left:   image.NewRGBA(image.Rect(0, 0, th, spec.H-2*th)),
		Right:  image.NewRGBA(image.Rect(0, 0, th, spec.H-2*th)),
	}

	// Global alpha function: for a pixel at monitor coordinates (gx, gy),
	// d = min distance to the nearest monitor edge; t = d/Thick clamped [0,1];
	// alpha = Peak * (1-t)^1.7, with a thin rim at full peak.
	rim := 2.0 * math.Max(1, float64(th)/40)
	alphaAt := func(gx, gy int) float64 {
		d := gx
		if gy < d {
			d = gy
		}
		if spec.W-1-gx < d {
			d = spec.W - 1 - gx
		}
		if spec.H-1-gy < d {
			d = spec.H - 1 - gy
		}
		t := float64(d) / float64(th)
		if t > 1 {
			t = 1
		}
		a := spec.Peak * math.Pow(1-t, 1.7)
		if float64(d) < rim {
			if spec.Peak > a {
				a = spec.Peak
			}
		}
		return a
	}

	fill := func(idx int, img *image.RGBA, toGlobal func(x, y int) (int, int)) {
		b := img.Bounds()
		s.Alpha[idx] = make([]float64, b.Dx()*b.Dy())
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				gx, gy := toGlobal(x, y)
				a := alphaAt(gx, gy)
				s.Alpha[idx][y*b.Dx()+x] = a
				img.SetRGBA(x, y, color.RGBA{R: spec.Color.R, G: spec.Color.G, B: spec.Color.B, A: uint8(a * 255)})
			}
		}
	}

	// Top strip: local (x,y) → monitor (x, y)
	fill(0, s.Top, func(x, y int) (int, int) { return x, y })
	// Bottom strip: local (x,y) → monitor (x, H-Thick+y)
	fill(1, s.Bottom, func(x, y int) (int, int) { return x, spec.H - th + y })
	// Left strip: local (x,y) → monitor (x, Thick+y)
	fill(2, s.Left, func(x, y int) (int, int) { return x, th + y })
	// Right strip: local (x,y) → monitor (W-Thick+x, Thick+y)
	fill(3, s.Right, func(x, y int) (int, int) { return spec.W - th + x, th + y })
	return s
}

// apply rescales every strip's alpha by breath (0..1) for the breathing animation.
func (s *stripSet) apply(breath float64) {
	for i, img := range s.images() {
		base := s.Alpha[i]
		for p := 0; p < len(base); p++ {
			img.Pix[p*4+3] = uint8(base[p] * breath * 255)
		}
	}
}
