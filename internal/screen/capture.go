package screen

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"time"

	xdraw "golang.org/x/image/draw"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

// Shot is an encoded screenshot plus the numbers needed to build a View from it.
type Shot struct {
	Data                         []byte
	MIME                         string
	Size                         geom.Size
	Rect                         geom.Rect // screen rect captured
	Scale                        float64
	CaptureMs, ScaleMs, EncodeMs int64
}

// Scale resizes with a bilinear filter; scale 1 returns img unchanged.
func Scale(img *image.RGBA, scale float64) *image.RGBA {
	if scale > 0.999 && scale < 1.001 {
		return img
	}
	w := max(1, roundInt(float64(img.Bounds().Dx())*scale))
	h := max(1, roundInt(float64(img.Bounds().Dy())*scale))
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Src, nil)
	return dst
}

func Encode(img image.Image, format string, jpegQuality int) ([]byte, string, error) {
	var buf bytes.Buffer
	switch format {
	case "", "png":
		enc := png.Encoder{CompressionLevel: png.BestSpeed}
		if err := enc.Encode(&buf, img); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/png", nil
	case "jpeg", "jpg":
		if jpegQuality <= 0 || jpegQuality > 100 {
			jpegQuality = 85
		}
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/jpeg", nil
	}
	return nil, "", fmt.Errorf("unknown image format %q (use png or jpeg)", format)
}

// BeforeCapture/AfterCapture let the overlay hide itself around a capture when the OS cannot exclude it.
var BeforeCapture, AfterCapture func()

// Grab captures rect from s, scales it and encodes it.
func Grab(s platform.Screen, rect geom.Rect, scale float64, format string, jpegQuality int) (*Shot, error) {
	if rect.Empty() {
		return nil, fmt.Errorf("empty capture rect")
	}
	if BeforeCapture != nil {
		BeforeCapture()
	}
	t0 := time.Now()
	img, err := s.Capture(rect)
	if err != nil {
		if AfterCapture != nil {
			AfterCapture()
		}
		return nil, err
	}
	if AfterCapture != nil {
		AfterCapture()
	}
	t1 := time.Now()
	scaled := Scale(img, scale)
	t2 := time.Now()
	data, mime, err := Encode(scaled, format, jpegQuality)
	if err != nil {
		return nil, err
	}
	t3 := time.Now()
	return &Shot{
		Data: data, MIME: mime,
		Size: geom.Size{W: scaled.Bounds().Dx(), H: scaled.Bounds().Dy()},
		Rect: rect, Scale: scale,
		CaptureMs: t1.Sub(t0).Milliseconds(), ScaleMs: t2.Sub(t1).Milliseconds(), EncodeMs: t3.Sub(t2).Milliseconds(),
	}, nil
}
