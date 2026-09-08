package screen

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform/fake"
)

func TestScaleHalvesDimensions(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	out := Scale(img, 0.5)
	if out.Bounds().Dx() != 100 || out.Bounds().Dy() != 50 {
		t.Fatalf("scaled bounds = %v", out.Bounds())
	}
	if Scale(img, 1) != img {
		t.Fatalf("scale 1 must return the same image")
	}
}

func TestEncodeFormats(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	pngData, mime, err := Encode(img, "png", 85)
	if err != nil || mime != "image/png" || !bytes.HasPrefix(pngData, []byte("\x89PNG")) {
		t.Fatalf("png: %v %s", err, mime)
	}
	jpgData, mime, err := Encode(img, "jpeg", 85)
	if err != nil || mime != "image/jpeg" || !bytes.HasPrefix(jpgData, []byte{0xFF, 0xD8}) {
		t.Fatalf("jpeg: %v %s", err, mime)
	}
	if _, _, err := Encode(img, "gif", 85); err == nil {
		t.Fatalf("unknown format must error")
	}
}

func TestGrabUsesScaleAndReportsSize(t *testing.T) {
	s := &fake.Screen{Fill: color.RGBA{R: 10, G: 20, B: 30, A: 255}}
	shot, err := Grab(s, geom.Rect{X: 0, Y: 0, W: 400, H: 200}, 0.5, "png", 85)
	if err != nil {
		t.Fatal(err)
	}
	if shot.Size != (geom.Size{W: 200, H: 100}) || shot.MIME != "image/png" || shot.Scale != 0.5 {
		t.Fatalf("shot = %+v", shot)
	}
}
