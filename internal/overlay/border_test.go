package overlay

import (
	"image/color"
	"testing"
)

func TestStripsSizesAndFalloff(t *testing.T) {
	s := renderStrips(borderSpec{W: 200, H: 100, Thick: 10, Color: color.RGBA{R: 217, G: 119, B: 87, A: 255}, Peak: 0.6})
	if s.Top.Bounds().Dx() != 200 || s.Top.Bounds().Dy() != 10 || s.Left.Bounds().Dx() != 10 || s.Left.Bounds().Dy() != 80 {
		t.Fatalf("strip sizes wrong: top %v left %v", s.Top.Bounds(), s.Left.Bounds())
	}
	s.apply(1)
	edge, inner := s.Top.RGBAAt(100, 0).A, s.Top.RGBAAt(100, 9).A
	if edge < 140 || edge > 160 || inner >= edge/4 {
		t.Fatalf("alpha must fall off from ~153 at the edge: edge=%d inner=%d", edge, inner)
	}
	s.apply(0.5)
	if half := s.Top.RGBAAt(100, 0).A; half < 70 || half > 80 {
		t.Fatalf("breath 0.5 must halve alpha: %d", half)
	}
}
