package geom

import "testing"

func TestRectContainsAndIntersect(t *testing.T) {
	r := Rect{X: 10, Y: 20, W: 100, H: 50}
	if !r.Contains(Point{10, 20}) || r.Contains(Point{110, 20}) || r.Contains(Point{50, 70}) {
		t.Fatalf("Contains is wrong: half-open [X, X+W) x [Y, Y+H) expected")
	}
	if got := r.Center(); got != (Point{60, 45}) {
		t.Fatalf("Center = %v, want {60 45}", got)
	}
	got := r.Intersect(Rect{X: 50, Y: 0, W: 100, H: 30})
	if got != (Rect{X: 50, Y: 20, W: 60, H: 10}) {
		t.Fatalf("Intersect = %v", got)
	}
	if !r.Intersect(Rect{X: 500, Y: 500, W: 10, H: 10}).Empty() {
		t.Fatalf("disjoint rects must intersect to an empty rect")
	}
	u := r.Union(Rect{X: -5, Y: 30, W: 10, H: 100})
	if u != (Rect{X: -5, Y: 20, W: 115, H: 110}) {
		t.Fatalf("Union = %v", u)
	}
}
