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
	Perim                    [4][]float64 // normalised perimeter position 0..1 for shimmer
	perimTotal               float64      // total perimeter length
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

	// Perimeter: top (left→right) + right (top→bottom) + bottom (right→left) + left (bottom→top).
	perimTotal := float64(2*(spec.W+spec.H) - 4*th)
	s.perimTotal = perimTotal

	// perimAt returns a normalised 0..1 perimeter position for a monitor-space pixel, using its
	// nearest edge point projected onto the clockwise perimeter rectangle.
	perimAt := func(gx, gy int) float64 {
		fx, fy := float64(gx), float64(gy)
		fw, fh := float64(spec.W), float64(spec.H)
		// Clamp to perimeter rectangle edges
		var pos float64
		// Determine nearest edge
		dTop, dBot, dLeft, dRight := fy, fh-1-fy, fx, fw-1-fx
		minD := dTop
		pos = fx // top edge
		if dRight < minD {
			minD = dRight
			pos = fw + fy // right edge
		}
		if dBot < minD {
			minD = dBot
			pos = fw + fh + (fw - fx) // bottom edge (reversed)
		}
		if dLeft < minD {
			pos = 2*fw + fh + (fh - fy) // left edge (reversed)
		}
		_ = minD
		return pos / (2*fw + 2*fh)
	}

	fill := func(idx int, img *image.RGBA, toGlobal func(x, y int) (int, int)) {
		b := img.Bounds()
		n := b.Dx() * b.Dy()
		s.Alpha[idx] = make([]float64, n)
		s.Perim[idx] = make([]float64, n)
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				gx, gy := toGlobal(x, y)
				a := alphaAt(gx, gy)
				off := y*b.Dx() + x
				s.Alpha[idx][off] = a
				s.Perim[idx][off] = perimAt(gx, gy)
				img.SetRGBA(x, y, color.RGBA{R: spec.Color.R, G: spec.Color.G, B: spec.Color.B, A: uint8(a * 255)})
			}
		}
	}

	// Top strip: local (x,y) -> monitor (x, y)
	fill(0, s.Top, func(x, y int) (int, int) { return x, y })
	// Bottom strip: local (x,y) -> monitor (x, H-Thick+y)
	fill(1, s.Bottom, func(x, y int) (int, int) { return x, spec.H - th + y })
	// Left strip: local (x,y) -> monitor (x, Thick+y)
	fill(2, s.Left, func(x, y int) (int, int) { return x, th + y })
	// Right strip: local (x,y) -> monitor (W-Thick+x, Thick+y)
	fill(3, s.Right, func(x, y int) (int, int) { return spec.W - th + x, th + y })
	return s
}

// apply rescales every strip's alpha by breath (0..1) for the breathing animation.
// shimmerPos < 0 disables shimmer; otherwise 0..1 is the perimeter position of the bright spot.
func (s *stripSet) apply(breath, shimmerPos float64) {
	shimmer := shimmerPos >= 0
	// sigma = 6% of the perimeter; precompute 1/(2*sigma^2)
	const sigma = 0.06
	inv2s2 := 1.0 / (2 * sigma * sigma)
	for i, img := range s.images() {
		base := s.Alpha[i]
		perim := s.Perim[i]
		for p := 0; p < len(base); p++ {
			a := base[p] * breath
			if shimmer && a > 0 {
				// Perimeter distance (wrapping around 0..1 circle)
				d := perim[p] - shimmerPos
				if d > 0.5 {
					d -= 1
				} else if d < -0.5 {
					d += 1
				}
				a *= 1 + 0.45*math.Exp(-d*d*inv2s2)
			}
			v := a * 255
			if v > 255 {
				v = 255
			}
			img.Pix[p*4+3] = uint8(v)
		}
	}
}
