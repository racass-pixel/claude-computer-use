// Package geom holds the integer geometry types shared by every package.
// Values are physical pixels unless a function says otherwise.
package geom

type Point struct{ X, Y int }
type Size struct{ W, H int }
type Rect struct{ X, Y, W, H int }

func (p Point) Add(o Point) Point { return Point{p.X + o.X, p.Y + o.Y} }

func (r Rect) Right() int    { return r.X + r.W }
func (r Rect) Bottom() int   { return r.Y + r.H }
func (r Rect) Empty() bool   { return r.W <= 0 || r.H <= 0 }
func (r Rect) Center() Point { return Point{r.X + r.W/2, r.Y + r.H/2} }
func (r Rect) Size() Size    { return Size{r.W, r.H} }

func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.X < r.Right() && p.Y >= r.Y && p.Y < r.Bottom()
}

func (r Rect) Intersect(o Rect) Rect {
	x0, y0 := max(r.X, o.X), max(r.Y, o.Y)
	x1, y1 := min(r.Right(), o.Right()), min(r.Bottom(), o.Bottom())
	if x1 <= x0 || y1 <= y0 {
		return Rect{}
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

func (r Rect) Union(o Rect) Rect {
	if r.Empty() {
		return o
	}
	if o.Empty() {
		return r
	}
	x0, y0 := min(r.X, o.X), min(r.Y, o.Y)
	x1, y1 := max(r.Right(), o.Right()), max(r.Bottom(), o.Bottom())
	return Rect{x0, y0, x1 - x0, y1 - y0}
}
