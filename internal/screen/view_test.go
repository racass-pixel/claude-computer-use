package screen

import (
	"math"
	"testing"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
)

func TestAutoScaleFitsLongEdge(t *testing.T) {
	s := AutoScale(geom.Rect{W: 1920, H: 1080}, 1366)
	if math.Abs(s-1366.0/1920.0) > 1e-9 {
		t.Fatalf("scale = %v", s)
	}
	if AutoScale(geom.Rect{W: 800, H: 600}, 1366) != 1 {
		t.Fatalf("small screens must not be upscaled")
	}
	if AutoScale(geom.Rect{W: 800, H: 600}, 0) != 1 {
		t.Fatalf("longEdge<=0 means no scaling")
	}
}

func TestZoomScaleCapsAtMaxZoom(t *testing.T) {
	if z := ZoomScale(geom.Rect{W: 200, H: 100}, 1366, 2); z != 2 {
		t.Fatalf("zoom = %v, want 2", z)
	}
	if z := ZoomScale(geom.Rect{W: 1000, H: 500}, 1366, 2); math.Abs(z-1.366) > 1e-9 {
		t.Fatalf("zoom = %v, want 1.366", z)
	}
}

func TestViewRoundTripOnPrimary(t *testing.T) {
	v := NewView(1, geom.Rect{W: 1920, H: 1080}, AutoScale(geom.Rect{W: 1920, H: 1080}, 1366))
	if v.Image != (geom.Size{W: 1366, H: 768}) {
		t.Fatalf("image size = %v", v.Image)
	}
	if got := v.ToScreen(geom.Point{X: 683, Y: 384}); got != (geom.Point{X: 960, Y: 540}) {
		t.Fatalf("ToScreen = %v", got)
	}
	if got := v.ToImage(geom.Point{X: 960, Y: 540}); got != (geom.Point{X: 683, Y: 384}) {
		t.Fatalf("ToImage = %v", got)
	}
	if got := v.ToScreen(geom.Point{X: 1365, Y: 767}); got.X > 1919 || got.Y > 1079 {
		t.Fatalf("last image pixel must stay on screen: %v", got)
	}
}

func TestViewOffsetOnSecondMonitorAndRegionZoom(t *testing.T) {
	v := NewView(2, geom.Rect{X: 1920, Y: -200, W: 2560, H: 1440}, 0.5)
	if got := v.ToScreen(geom.Point{X: 0, Y: 0}); got != (geom.Point{X: 1920, Y: -200}) {
		t.Fatalf("origin = %v", got)
	}
	z := NewView(1, geom.Rect{X: 100, Y: 100, W: 200, H: 100}, 2)
	if z.Image != (geom.Size{W: 400, H: 200}) {
		t.Fatalf("zoom image = %v", z.Image)
	}
	if got := z.ToScreen(geom.Point{X: 400, Y: 200}); got != (geom.Point{X: 299, Y: 199}) { // clamped to the last screen pixel of the view
		t.Fatalf("zoom ToScreen = %v", got)
	}
	if got := z.RectToImage(geom.Rect{X: 150, Y: 120, W: 10, H: 5}); got != (geom.Rect{X: 100, Y: 40, W: 20, H: 10}) {
		t.Fatalf("RectToImage = %v", got)
	}
	if got := z.ClampImage(geom.Point{X: 999, Y: -5}); got != (geom.Point{X: 399, Y: 0}) {
		t.Fatalf("ClampImage = %v", got)
	}
}
