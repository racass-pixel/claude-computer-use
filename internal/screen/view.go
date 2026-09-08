package screen

import (
	"math"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
)

// View is the coordinate space of the last screenshot: image px = (screen px - Offset) * Scale.
type View struct {
	Monitor int        `json:"monitor"` // 0 = whole virtual screen
	Offset  geom.Point `json:"-"`
	Scale   float64    `json:"scale"`
	Image   geom.Size  `json:"-"`
	Screen  geom.Rect  `json:"-"`
}

func NewView(monitor int, screenRect geom.Rect, scale float64) View {
	if scale <= 0 {
		scale = 1
	}
	return View{
		Monitor: monitor,
		Offset:  geom.Point{X: screenRect.X, Y: screenRect.Y},
		Scale:   scale,
		Image:   geom.Size{W: roundInt(float64(screenRect.W) * scale), H: roundInt(float64(screenRect.H) * scale)},
		Screen:  screenRect,
	}
}

func roundInt(f float64) int { return int(math.Round(f)) }

func (v View) ToScreen(p geom.Point) geom.Point {
	x := v.Offset.X + roundInt(float64(p.X)/v.Scale)
	y := v.Offset.Y + roundInt(float64(p.Y)/v.Scale)
	// never map past the last screen pixel of the view
	x = min(x, v.Screen.Right()-1)
	y = min(y, v.Screen.Bottom()-1)
	return geom.Point{X: x, Y: y}
}

func (v View) ToImage(p geom.Point) geom.Point {
	return geom.Point{
		X: roundInt(float64(p.X-v.Offset.X) * v.Scale),
		Y: roundInt(float64(p.Y-v.Offset.Y) * v.Scale),
	}
}

func (v View) RectToImage(r geom.Rect) geom.Rect {
	p := v.ToImage(geom.Point{X: r.X, Y: r.Y})
	return geom.Rect{X: p.X, Y: p.Y, W: roundInt(float64(r.W) * v.Scale), H: roundInt(float64(r.H) * v.Scale)}
}

func (v View) RectToScreen(r geom.Rect) geom.Rect {
	return geom.Rect{
		X: v.Offset.X + roundInt(float64(r.X)/v.Scale),
		Y: v.Offset.Y + roundInt(float64(r.Y)/v.Scale),
		W: roundInt(float64(r.W) / v.Scale),
		H: roundInt(float64(r.H) / v.Scale),
	}
}

func (v View) ClampImage(p geom.Point) geom.Point {
	return geom.Point{X: max(0, min(p.X, v.Image.W-1)), Y: max(0, min(p.Y, v.Image.H-1))}
}

// AutoScale shrinks r so its long edge is at most longEdge; never upscales.
func AutoScale(r geom.Rect, longEdge int) float64 {
	long := max(r.W, r.H)
	if longEdge <= 0 || long <= longEdge || long == 0 {
		return 1
	}
	return float64(longEdge) / float64(long)
}

// ZoomScale is AutoScale for regions: it may upscale up to maxZoom so small targets get big.
func ZoomScale(r geom.Rect, longEdge int, maxZoom float64) float64 {
	long := max(r.W, r.H)
	if long == 0 || longEdge <= 0 {
		return 1
	}
	return min(maxZoom, float64(longEdge)/float64(long))
}
