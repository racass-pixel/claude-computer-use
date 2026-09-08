// Package overlay draws the take-over UI: glowing monitor border, HUD pill and click ripples.
package overlay

import (
	"image"
	"image/color"
	"math"
)

// premultiplyBGRA converts straight-alpha RGBA into the premultiplied BGRA layout UpdateLayeredWindow expects.
func premultiplyBGRA(dst []byte, src *image.RGBA) {
	p := src.Pix
	for i := 0; i+3 < len(p) && i+3 < len(dst); i += 4 {
		a := uint32(p[i+3])
		dst[i+0] = byte(uint32(p[i+2]) * a / 255)
		dst[i+1] = byte(uint32(p[i+1]) * a / 255)
		dst[i+2] = byte(uint32(p[i+0]) * a / 255)
		dst[i+3] = byte(a)
	}
}

// blendPixel draws c over img at (x,y) with extra coverage (0..1) for anti-aliasing.
func blendPixel(img *image.RGBA, x, y int, c color.RGBA, coverage float64) {
	if coverage <= 0 || !(image.Point{X: x, Y: y}.In(img.Bounds())) {
		return
	}
	a := float64(c.A) / 255 * min(1, coverage)
	d := img.RGBAAt(x, y)
	da := float64(d.A) / 255
	outA := a + da*(1-a)
	if outA <= 0 {
		return
	}
	mix := func(s, dst uint8) uint8 {
		return uint8((float64(s)*a + float64(dst)*da*(1-a)) / outA)
	}
	img.SetRGBA(x, y, color.RGBA{R: mix(c.R, d.R), G: mix(c.G, d.G), B: mix(c.B, d.B), A: uint8(outA * 255)})
}

// sdRoundedRect is the signed distance from (px,py) to a rounded box centered at (cx,cy) with half sizes hw,hh.
func sdRoundedRect(px, py, cx, cy, hw, hh, r float64) float64 {
	qx := math.Abs(px-cx) - hw + r
	qy := math.Abs(py-cy) - hh + r
	return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - r
}

func fillRoundedRect(img *image.RGBA, r image.Rectangle, radius float64, c color.RGBA) {
	cx := float64(r.Min.X+r.Max.X) / 2
	cy := float64(r.Min.Y+r.Max.Y) / 2
	hw, hh := float64(r.Dx())/2, float64(r.Dy())/2
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			d := sdRoundedRect(float64(x)+0.5, float64(y)+0.5, cx, cy, hw, hh, radius)
			blendPixel(img, x, y, c, 0.5-d) // 1px anti-aliased edge
		}
	}
}

func strokeRing(img *image.RGBA, cx, cy, radius, width float64, c color.RGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			d := math.Abs(math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)-radius) - width/2
			blendPixel(img, x, y, c, 0.5-d)
		}
	}
}
