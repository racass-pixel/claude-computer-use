package server

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

type ControlIn struct {
	Action string `json:"action" jsonschema:"status, acquire (show the overlay now), release (hide it), or hud (set the task title shown to the user)"`
	Task   string `json:"task,omitempty" jsonschema:"short task title for the HUD, e.g. \"Заполняю форму заказа\""`
	Note   string `json:"note,omitempty" jsonschema:"optional current step shown after the hotkey hint"`
}

func (s *Session) controlStatus() map[string]any {
	f := map[string]any{"controlling": false, "paused": false, "hotkey": s.cfg.Hotkey}
	if c := s.d.Controller; c != nil {
		st := c.Status()
		f["state"] = st.State
		f["controlling"] = st.State == "controlling"
		f["paused"] = st.State == "paused"
		f["hotkey"] = st.Hotkey
		f["idle_ms"] = st.IdleMs
	}
	if m := s.activeMonitor(); m.ID != 0 {
		f["active_monitor"] = m.ID
	}
	return f
}

func (s *Session) toolControl(ctx context.Context, req *mcp.CallToolRequest, in ControlIn) (*mcp.CallToolResult, any, error) {
	now := time.Now()
	switch in.Action {
	case "", "status":
	case "acquire":
		if c := s.d.Controller; c != nil {
			if c.IsPaused() {
				return errResult("user_took_control", "The user has control. Wait for the user to hand it back or to ask you to continue."), nil, nil
			}
			c.Acquire(now)
		}
		if in.Task != "" {
			s.d.Overlay.SetTitle(in.Task)
		}
		if m := s.activeMonitor(); m.ID != 0 {
			s.d.Overlay.Show(m, platform.OverlayControlling)
		}
	case "release":
		if c := s.d.Controller; c != nil {
			c.Release(now)
		}
		s.d.Overlay.Hide()
	case "hud":
		s.d.Overlay.SetTitle(in.Task)
		if in.Note != "" {
			s.d.Overlay.SetAction(in.Note)
		}
	default:
		return errResult("bad_args", "action must be status, acquire, release or hud"), nil, nil
	}
	return okResult(s.controlStatus(), nil), nil, nil
}
