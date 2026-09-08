//go:build windows

package screen

import (
	"image"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

// Screen is the Windows platform.Screen.
type Screen struct{}

func New() *Screen { return &Screen{} }

func (*Screen) Monitors() ([]platform.Monitor, error) { return ListMonitors() }

func (*Screen) Capture(r geom.Rect) (*image.RGBA, error) {
	return win.CaptureRect(r.X, r.Y, r.W, r.H)
}

func (*Screen) CursorPos() (geom.Point, error) {
	p, err := win.GetCursorPos()
	return geom.Point{X: int(p.X), Y: int(p.Y)}, err
}

var _ platform.Screen = (*Screen)(nil)
