//go:build windows

package window

import (
	"strings"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

type Windows struct{}

func New() *Windows { return &Windows{} }

func toInfo(w win.RawWindow, fg uintptr) platform.WindowInfo {
	st := platform.WindowNormal
	if w.Minimized {
		st = platform.WindowMinimized
	} else if w.Maximized {
		st = platform.WindowMaximized
	}
	return platform.WindowInfo{
		ID: w.HWND, Title: w.Title, Process: win.ProcessImageName(w.PID), PID: w.PID,
		Rect:  geom.Rect{X: int(w.Rect.Left), Y: int(w.Rect.Top), W: int(w.Rect.Width()), H: int(w.Rect.Height())},
		State: st, Foreground: w.HWND == fg,
	}
}

func (*Windows) List() ([]platform.WindowInfo, error) {
	raw, err := win.EnumTopLevelWindows()
	if err != nil {
		return nil, err
	}
	fg := win.ForegroundWindow()
	out := make([]platform.WindowInfo, 0, len(raw))
	for _, w := range raw {
		if strings.HasPrefix(w.Class, ExcludeClassPrefix) {
			continue
		}
		out = append(out, toInfo(w, fg))
	}
	return out, nil
}

func (ws *Windows) Foreground() (platform.WindowInfo, error) {
	list, err := ws.List()
	if err != nil {
		return platform.WindowInfo{}, err
	}
	return Match(list, "foreground")
}

func (*Windows) Focus(id uintptr) error { return win.FocusWindow(id) }

func (*Windows) SetState(id uintptr, s platform.WindowState) error {
	switch s {
	case platform.WindowMinimized:
		win.ShowWindowCmd(id, win.SW_MINIMIZE)
	case platform.WindowMaximized:
		win.ShowWindowCmd(id, win.SW_MAXIMIZE)
	default:
		win.ShowWindowCmd(id, win.SW_RESTORE)
	}
	return nil
}

func (*Windows) Close(id uintptr) error             { return win.CloseWindow(id) }
func (*Windows) Move(id uintptr, r geom.Rect) error { return win.SetWindowRect(id, r.X, r.Y, r.W, r.H) }

var _ platform.Windows = (*Windows)(nil)
