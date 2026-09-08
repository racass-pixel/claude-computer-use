//go:build windows

package screen

import (
	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

func rectOf(r win.RECT) geom.Rect {
	return geom.Rect{X: int(r.Left), Y: int(r.Top), W: int(r.Width()), H: int(r.Height())}
}

// ListMonitors enumerates monitors, primary first, IDs 1..n.
func ListMonitors() ([]platform.Monitor, error) {
	raw, err := win.EnumMonitors()
	if err != nil {
		return nil, err
	}
	mons := make([]platform.Monitor, 0, len(raw))
	for _, m := range raw {
		mons = append(mons, platform.Monitor{
			Name: m.Device, Rect: rectOf(m.Rect), Work: rectOf(m.Work),
			ScaleFactor: float64(m.DPI) / 96, Primary: m.Primary,
		})
	}
	return sortAndNumber(mons), nil
}
