package overlay

import (
	"image"
	"image/color"
	"math"
)

// renderSpark draws the Claude starburst mark: 12 irregular rays from the center.
// cx, cy = center; r = nominal radius; phase = rotation angle (advances 2π per 8 s);
// c = color (alpha is respected). The mark breathes via r.
func renderSpark(img *image.RGBA, cx, cy, r, phase float64, c color.RGBA) {
	if r <= 0 {
		return
	}
	const nRays = 12
	const angleStep = 2 * math.Pi / nRays
	for i := 0; i < nRays; i++ {
		angle := float64(i)*angleStep + phase
		rayLen := r * (0.78 + 0.22*math.Sin(3*angle+phase))
		drawRay(img, cx, cy, angle, rayLen, r, c)
	}
}

// drawRay draws one tapered ray from (cx,cy) outward at the given angle.
// The ray width tapers from 0.18*r at the center to 0.05*r at the tip.
func drawRay(img *image.RGBA, cx, cy, angle, rayLen, r float64, c color.RGBA) {
	if rayLen <= 0 {
		return
	}
	// Walk along the ray in small steps, drawing anti-aliased cross-sections.
	steps := int(math.Ceil(rayLen * 2)) // ~2 steps per pixel
	if steps < 4 {
		steps = 4
	}
	cosA, sinA := math.Cos(angle), math.Sin(angle)
	perpCos, perpSin := -sinA, cosA // perpendicular

	for s := 0; s <= steps; s++ {
		t := float64(s) / float64(steps) // 0 at center, 1 at tip
		// Position along the ray
		px := cx + cosA*rayLen*t
		py := cy + sinA*rayLen*t
		// Half-width at this point (lerp from 0.18r to 0.05r)
		hw := r * (0.18*(1-t) + 0.05*t)
		// Draw perpendicular cross-section
		crossSteps := int(math.Ceil(hw * 2))
		if crossSteps < 2 {
			crossSteps = 2
		}
		for cs := -crossSteps; cs <= crossSteps; cs++ {
			ct := float64(cs) / float64(crossSteps) // -1..1
			d := math.Abs(ct) * hw                  // distance from center of cross-section
			x := px + perpCos*hw*ct
			y := py + perpSin*hw*ct
			// Coverage: 1 inside, anti-alias at edges
			cov := 1.0 - math.Max(0, d-hw+0.7)/0.7
			if cov <= 0 {
				continue
			}
			// Fade at the tip
			tipFade := 1.0 - math.Max(0, t-0.85)/0.15
			blendPixel(img, int(x), int(y), c, cov*tipFade)
		}
	}
}
