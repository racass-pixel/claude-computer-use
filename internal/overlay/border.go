package overlay

import (
	"image"
	"image/color"
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

// falloff is 1 at the monitor edge and 0 at Thick pixels inward (quadratic).
func falloff(d, thick int) float64 {
	t := 1 - float64(d)/float64(thick)
	return t * t
}

func renderStrips(spec borderSpec) *stripSet {
	th := spec.Thick
	s := &stripSet{
		Top:    image.NewRGBA(image.Rect(0, 0, spec.W, th)),
		Bottom: image.NewRGBA(image.Rect(0, 0, spec.W, th)),
		Left:   image.NewRGBA(image.Rect(0, 0, th, spec.H-2*th)),
		Right:  image.NewRGBA(image.Rect(0, 0, th, spec.H-2*th)),
	}
	fill := func(idx int, img *image.RGBA, alphaAt func(x, y int) float64) {
		b := img.Bounds()
		s.Alpha[idx] = make([]float64, b.Dx()*b.Dy())
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				a := alphaAt(x, y) * spec.Peak
				s.Alpha[idx][y*b.Dx()+x] = a
				img.SetRGBA(x, y, color.RGBA{R: spec.Color.R, G: spec.Color.G, B: spec.Color.B, A: uint8(a * 255)})
			}
		}
	}
	corner := func(x, w int) float64 { // soften the strip ends so corners don't double up
		if x < th {
			return falloff(th-1-x, th)*0.5 + 0.5
		}
		if x >= w-th {
			return falloff(x-(w-th), th)*0.5 + 0.5
		}
		return 1
	}
	fill(0, s.Top, func(x, y int) float64 { return falloff(y, th) * corner(x, spec.W) })
	fill(1, s.Bottom, func(x, y int) float64 { return falloff(th-1-y, th) * corner(x, spec.W) })
	fill(2, s.Left, func(x, y int) float64 { return falloff(x, th) })
	fill(3, s.Right, func(x, y int) float64 { return falloff(th-1-x, th) })
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
