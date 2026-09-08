package overlay

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestRenderSparkPaintsCenter(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 60, 60))
	renderSpark(img, 30, 30, 12, 0, color.RGBA{R: 217, G: 119, B: 87, A: 255})
	// Center must be painted (rays emanate from the center).
	c := img.RGBAAt(30, 30)
	if c.A == 0 {
		t.Fatal("center must be painted")
	}
}

func TestRenderSparkRayTipPainted(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 60, 60))
	renderSpark(img, 30, 30, 12, 0, color.RGBA{R: 217, G: 119, B: 87, A: 255})
	// At least one pixel near the nominal radius should be painted (a ray tip).
	found := false
	for y := 0; y < 60; y++ {
		for x := 0; x < 60; x++ {
			d := math.Hypot(float64(x)-30, float64(y)-30)
			if d > 8 && d < 14 && img.RGBAAt(x, y).A > 0 {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("no painted pixel near the ray tips (radius 8..14)")
	}
}

func TestRenderSparkCornerTransparent(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 60, 60))
	renderSpark(img, 30, 30, 12, 0, color.RGBA{R: 217, G: 119, B: 87, A: 255})
	if img.RGBAAt(0, 0).A != 0 {
		t.Fatal("corner (0,0) must be transparent")
	}
	if img.RGBAAt(59, 59).A != 0 {
		t.Fatal("corner (59,59) must be transparent")
	}
}

func TestRenderSparkTwoPhasesDiffer(t *testing.T) {
	img1 := image.NewRGBA(image.Rect(0, 0, 60, 60))
	img2 := image.NewRGBA(image.Rect(0, 0, 60, 60))
	renderSpark(img1, 30, 30, 12, 0, color.RGBA{R: 217, G: 119, B: 87, A: 255})
	renderSpark(img2, 30, 30, 12, math.Pi/2, color.RGBA{R: 217, G: 119, B: 87, A: 255})
	diffs := 0
	for i := 0; i < len(img1.Pix); i += 4 {
		if img1.Pix[i+3] != img2.Pix[i+3] {
			diffs++
		}
	}
	if diffs == 0 {
		t.Fatal("two different phases must produce different images")
	}
}

func TestRenderSparkZeroRadius(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	renderSpark(img, 5, 5, 0, 0, color.RGBA{R: 255, A: 255})
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0 {
			t.Fatal("zero radius must paint nothing")
		}
	}
}
