package overlay

import (
	"image"
	"image/color"
	"testing"
)

func TestPremultiplyBGRA(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1, 1))
	src.Pix[0], src.Pix[1], src.Pix[2], src.Pix[3] = 255, 0, 0, 128
	dst := make([]byte, 4)
	premultiplyBGRA(dst, src)
	if dst[0] != 0 || dst[1] != 0 || dst[2] != 128 || dst[3] != 128 {
		t.Fatalf("got %v want [0 0 128 128] (B G R A premultiplied)", dst)
	}
}

func TestRoundedRectCornersAreTransparent(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	fillRoundedRect(img, img.Bounds(), 8, color.RGBA{R: 20, G: 20, B: 20, A: 255})
	if a := img.RGBAAt(0, 0).A; a != 0 {
		t.Fatalf("corner alpha = %d, want 0", a)
	}
	if a := img.RGBAAt(20, 10).A; a != 255 {
		t.Fatalf("center alpha = %d, want 255", a)
	}
	if a := img.RGBAAt(20, 0).A; a != 255 {
		t.Fatalf("top edge middle alpha = %d, want 255", a)
	}
}

func TestRingHasHoleAndEdge(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 60, 60))
	strokeRing(img, 30, 30, 20, 3, color.RGBA{R: 255, A: 255})
	if img.RGBAAt(30, 30).A != 0 {
		t.Fatal("ring center must be empty")
	}
	if img.RGBAAt(50, 30).A == 0 {
		t.Fatal("ring edge must be painted")
	}
}
