package overlay

import (
	"image/color"
	"testing"
)

func TestStripsFormOneContinuousFrame(t *testing.T) {
	s := renderStrips(borderSpec{W: 400, H: 200, Thick: 40, Color: color.RGBA{R: 217, G: 119, B: 87, A: 255}, Peak: 0.8})
	s.apply(1)
	// rim is at full peak, interior fades
	if a := s.Top.RGBAAt(200, 0).A; a < 200 || a > 210 {
		t.Fatalf("rim alpha = %d, want ~204", a)
	}
	if s.Top.RGBAAt(200, 39).A > 10 {
		t.Fatalf("inner edge must be nearly transparent: %d", s.Top.RGBAAt(200, 39).A)
	}
	// continuity across the Top/Left seam: Top(x=5, y=39) is d=5 and Left(x=5, y=0) is d=5
	if s.Top.RGBAAt(5, 39).A != s.Left.RGBAAt(5, 0).A {
		t.Fatalf("seam mismatch top=%d left=%d", s.Top.RGBAAt(5, 39).A, s.Left.RGBAAt(5, 0).A)
	}
	// corner pixel is on the rim in both directions
	if s.Top.RGBAAt(0, 0).A != s.Top.RGBAAt(200, 0).A {
		t.Fatalf("corner must be as bright as the rim")
	}
	// Right/Bottom mirror Left/Top
	if s.Right.RGBAAt(39, 10).A != s.Left.RGBAAt(0, 10).A || s.Bottom.RGBAAt(100, 39).A != s.Top.RGBAAt(100, 0).A {
		t.Fatalf("strips must be mirror-symmetric")
	}
	s.apply(0.5)
	if half := s.Top.RGBAAt(200, 0).A; half < 98 || half > 106 {
		t.Fatalf("breath 0.5 must halve alpha: %d", half)
	}
}
