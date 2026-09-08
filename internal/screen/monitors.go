// Package screen implements platform.Screen and the screenshot view transform.
package screen

import (
	"sort"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

// sortAndNumber orders monitors primary-first, then by X then Y, and assigns IDs.
func sortAndNumber(mons []platform.Monitor) []platform.Monitor {
	sort.SliceStable(mons, func(i, j int) bool {
		if mons[i].Primary != mons[j].Primary {
			return mons[i].Primary
		}
		if mons[i].Rect.X != mons[j].Rect.X {
			return mons[i].Rect.X < mons[j].Rect.X
		}
		return mons[i].Rect.Y < mons[j].Rect.Y
	})
	for i := range mons {
		mons[i].ID = i + 1
	}
	return mons
}

func MonitorByID(mons []platform.Monitor, id int) (platform.Monitor, bool) {
	for _, m := range mons {
		if m.ID == id {
			return m, true
		}
	}
	return platform.Monitor{}, false
}

// ActiveMonitor picks the monitor containing the foreground window center, else the cursor, else primary.
func ActiveMonitor(mons []platform.Monitor, fg geom.Rect, cursor geom.Point) platform.Monitor {
	if len(mons) == 0 {
		return platform.Monitor{}
	}
	if !fg.Empty() {
		c := fg.Center()
		for _, m := range mons {
			if m.Rect.Contains(c) {
				return m
			}
		}
	}
	for _, m := range mons {
		if m.Rect.Contains(cursor) {
			return m
		}
	}
	for _, m := range mons {
		if m.Primary {
			return m
		}
	}
	return mons[0]
}

func VirtualScreen(mons []platform.Monitor) geom.Rect {
	var u geom.Rect
	for _, m := range mons {
		u = u.Union(m.Rect)
	}
	return u
}
