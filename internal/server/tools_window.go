package server

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/window"
)

type WindowsIn struct {
	Filter  string `json:"filter,omitempty" jsonschema:"case-insensitive regexp on title or process name"`
	Monitor int    `json:"monitor,omitempty" jsonschema:"only windows whose center is on this monitor id"`
}

type windowOut struct {
	platform.WindowInfo
	Monitor int `json:"monitor"`
}

func (s *Session) monitorOf(r geom.Rect) int {
	mons, err := s.monitors()
	if err != nil {
		return 0
	}
	c := r.Center()
	for _, m := range mons {
		if m.Rect.Contains(c) {
			return m.ID
		}
	}
	return 0
}

func (s *Session) toolWindows(ctx context.Context, req *mcp.CallToolRequest, in WindowsIn) (*mcp.CallToolResult, any, error) {
	list, err := s.d.Wins.List()
	if err != nil {
		return errResult("windows_failed", err.Error()), nil, nil
	}
	var re *regexp.Regexp
	if in.Filter != "" {
		if re, err = regexp.Compile("(?i)" + in.Filter); err != nil {
			return errResult("bad_args", "filter: "+err.Error()), nil, nil
		}
	}
	out := make([]windowOut, 0, len(list))
	for _, w := range list {
		if re != nil && !re.MatchString(w.Title) && !re.MatchString(w.Process) {
			continue
		}
		mon := s.monitorOf(w.Rect)
		if in.Monitor != 0 && mon != in.Monitor {
			continue
		}
		out = append(out, windowOut{WindowInfo: w, Monitor: mon})
	}
	return okResult(map[string]any{"windows": out, "count": len(out)}, nil), nil, nil
}

type WindowIn struct {
	Action           string    `json:"action" jsonschema:"focus, minimize, maximize, restore, close, move or resize"`
	Target           string    `json:"target,omitempty" jsonschema:"\"foreground\" (default), a window id from windows, or a case-insensitive regexp on title/process"`
	Rect             *RegionIn `json:"rect,omitempty" jsonschema:"for move/resize: target frame rect in SCREEN pixels (use monitors for bounds)"`
	ScreenshotRegion *RegionIn `json:"screenshot_region,omitempty" jsonschema:"after the action, return a zoomed screenshot of this rectangle (last-screenshot pixels) instead of the whole monitor; the coordinate space switches to that region"`
	Screenshot       *bool     `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func (s *Session) toolWindow(ctx context.Context, req *mcp.CallToolRequest, in WindowIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	shot, err := s.shotFor(s.wantShot(in.Screenshot), in.ScreenshotRegion)
	if err != nil {
		return errResult("bad_args", err.Error()), nil, nil
	}
	list, err := s.d.Wins.List()
	if err != nil {
		return errResult("windows_failed", err.Error()), nil, nil
	}
	w, err := window.Match(list, in.Target)
	if err != nil {
		return errResult("no_window", err.Error()), nil, nil
	}
	if early := s.begin(ctx, "window", fmt.Sprintf("window %s %q", in.Action, w.Title)); early != nil {
		return early, nil, nil
	}
	switch in.Action {
	case "focus":
		err = s.d.Wins.Focus(w.ID)
	case "minimize":
		err = s.d.Wins.SetState(w.ID, platform.WindowMinimized)
	case "maximize":
		err = s.d.Wins.SetState(w.ID, platform.WindowMaximized)
	case "restore":
		err = s.d.Wins.SetState(w.ID, platform.WindowNormal)
	case "close":
		err = s.d.Wins.Close(w.ID)
	case "move", "resize":
		if in.Rect == nil {
			return errResult("bad_args", "move/resize need rect {x,y,w,h} in screen pixels"), nil, nil
		}
		r := geom.Rect{X: in.Rect.X, Y: in.Rect.Y, W: in.Rect.W, H: in.Rect.H}
		if in.Action == "resize" && (r.W == 0 || r.H == 0) {
			return errResult("bad_args", "resize needs w and h"), nil, nil
		}
		if r.W == 0 || r.H == 0 { // move keeps the size
			r.W, r.H = w.Rect.W, w.Rect.H
		}
		err = s.d.Wins.Move(w.ID, r)
	default:
		return errResult("bad_args", "unknown window action "+in.Action), nil, nil
	}
	if err != nil {
		return errResult("window_failed", err.Error()), nil, nil
	}
	s.mu.Lock()
	s.mons = nil // a moved window may change the active monitor
	s.mu.Unlock()
	return s.finish("window", t0, map[string]any{"window": w.ID, "title": w.Title, "did": in.Action}, shot, s.settleFor("window")), nil, nil
}
