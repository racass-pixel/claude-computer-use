package server

import (
	"image"
	"testing"
)

func TestFrameDiff(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, 10, 10))
	b := image.NewRGBA(image.Rect(0, 0, 10, 10))
	if d := frameDiff(a, b); d != 0 {
		t.Fatalf("identical frames diff = %v", d)
	}
	for i := 0; i < len(b.Pix); i += 4 {
		b.Pix[i] = 255
	}
	if d := frameDiff(a, b); d < 60 {
		t.Fatalf("changed frames diff = %v, want >= 60", d)
	}
}
