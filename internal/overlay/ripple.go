package overlay

import (
	"image"
	"image/color"
)

// renderRipple draws an expanding, fading ring; t runs 0..1 over the animation.
func renderRipple(size int, t float64, accent color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	if t >= 1 {
		return img
	}
	r := float64(size) * (0.18 + 0.30*t)
	c := accent
	c.A = uint8(230 * (1 - t))
	strokeRing(img, float64(size)/2, float64(size)/2, r, 3, c)
	return img
}
