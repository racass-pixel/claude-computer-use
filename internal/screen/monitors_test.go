package screen

import (
	"testing"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

var twoMons = []platform.Monitor{
	{ID: 1, Rect: geom.Rect{X: 0, Y: 0, W: 1920, H: 1080}, Primary: true},
	{ID: 2, Rect: geom.Rect{X: 1920, Y: -200, W: 2560, H: 1440}},
}

func TestActiveMonitorPrefersForegroundWindowCenter(t *testing.T) {
	fg := geom.Rect{X: 2000, Y: 100, W: 800, H: 600} // center on monitor 2
	if m := ActiveMonitor(twoMons, fg, geom.Point{X: 10, Y: 10}); m.ID != 2 {
		t.Fatalf("got monitor %d, want 2", m.ID)
	}
}

func TestActiveMonitorFallsBackToCursorThenPrimary(t *testing.T) {
	if m := ActiveMonitor(twoMons, geom.Rect{}, geom.Point{X: 3000, Y: 300}); m.ID != 2 {
		t.Fatalf("cursor fallback: got %d, want 2", m.ID)
	}
	if m := ActiveMonitor(twoMons, geom.Rect{}, geom.Point{X: -9999, Y: -9999}); m.ID != 1 {
		t.Fatalf("primary fallback: got %d, want 1", m.ID)
	}
}

func TestVirtualScreenIsUnion(t *testing.T) {
	if got := VirtualScreen(twoMons); got != (geom.Rect{X: 0, Y: -200, W: 4480, H: 1440}) {
		t.Fatalf("VirtualScreen = %v", got)
	}
}

func TestSortAndNumberPutsPrimaryFirst(t *testing.T) {
	mons := sortAndNumber([]platform.Monitor{
		{Name: "B", Rect: geom.Rect{X: -1920, Y: 0, W: 1920, H: 1080}},
		{Name: "A", Rect: geom.Rect{X: 0, Y: 0, W: 1920, H: 1080}, Primary: true},
	})
	if mons[0].Name != "A" || mons[0].ID != 1 || mons[1].ID != 2 {
		t.Fatalf("bad order: %+v", mons)
	}
}
