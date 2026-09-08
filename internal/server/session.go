package server

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/input"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/screen"
)

type RegionIn struct {
	X int `json:"x,omitempty"`
	Y int `json:"y,omitempty"`
	W int `json:"w,omitempty"`
	H int `json:"h,omitempty"`
}

type captureSpec struct {
	monitor string
	region  *RegionIn
	scale   float64
	format  string
}

func (s *Session) monitors() ([]platform.Monitor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mons != nil && time.Since(s.monsAt) < 2*time.Second {
		return s.mons, nil
	}
	mons, err := s.d.Screen.Monitors()
	if err != nil {
		return nil, err
	}
	if len(mons) == 0 {
		return nil, fmt.Errorf("no monitors found")
	}
	s.mons, s.monsAt = mons, time.Now()
	return mons, nil
}

func (s *Session) foreground() platform.WindowInfo {
	if s.d.Wins == nil {
		return platform.WindowInfo{}
	}
	w, err := s.d.Wins.Foreground()
	if err != nil {
		return platform.WindowInfo{}
	}
	return w
}

func (s *Session) cursor() geom.Point {
	p, _ := s.d.Screen.CursorPos()
	return p
}

func (s *Session) activeMonitor() platform.Monitor {
	mons, err := s.monitors()
	if err != nil {
		return platform.Monitor{}
	}
	return screen.ActiveMonitor(mons, s.foreground().Rect, s.cursor())
}

// currentView is the coordinate space tool inputs are in right now.
func (s *Session) currentView() screen.View {
	s.mu.Lock()
	if s.hasView {
		v := s.view
		s.mu.Unlock()
		return v
	}
	s.mu.Unlock()
	m := s.activeMonitor()
	return screen.NewView(m.ID, m.Rect, screen.AutoScale(m.Rect, s.cfg.ScreenshotLongEdge))
}

func (s *Session) setView(v screen.View) {
	s.mu.Lock()
	s.view, s.hasView = v, true
	s.mu.Unlock()
}

func (s *Session) metaFor(v screen.View) Meta {
	m := Meta{Monitor: v.Monitor, Image: [2]int{v.Image.W, v.Image.H}, Scale: v.Scale, Screen: v.Screen}
	c := v.ToImage(s.cursor())
	m.Cursor = [2]int{c.X, c.Y}
	if fg := s.foreground(); fg.ID != 0 {
		m.Foreground = &ForegroundInfo{ID: fg.ID, Title: fg.Title, Process: fg.Process}
	}
	if s.d.Controller != nil && s.d.Controller.IsPaused() {
		m.Paused = true
	}
	return m
}

// capture takes a screenshot per spec and makes it the current view.
func (s *Session) capture(spec captureSpec) (*screen.Shot, Meta, error) {
	mons, err := s.monitors()
	if err != nil {
		return nil, Meta{}, err
	}
	var rect geom.Rect
	monID := 0
	scale := spec.scale
	switch {
	case spec.region != nil:
		v := s.currentView()
		r := geom.Rect{X: spec.region.X, Y: spec.region.Y, W: spec.region.W, H: spec.region.H}
		rect = v.RectToScreen(r).Intersect(v.Screen)
		if rect.Empty() {
			return nil, Meta{}, fmt.Errorf("region %+v is outside the current view (%dx%d)", r, v.Image.W, v.Image.H)
		}
		monID = v.Monitor
		if scale <= 0 {
			scale = screen.ZoomScale(rect, s.cfg.ScreenshotLongEdge, 2)
		}
	case spec.monitor == "all":
		rect = screen.VirtualScreen(mons)
		if scale <= 0 {
			scale = screen.AutoScale(rect, s.cfg.ScreenshotLongEdge*3/2)
		}
	case spec.monitor == "" || spec.monitor == "active":
		m := screen.ActiveMonitor(mons, s.foreground().Rect, s.cursor())
		rect, monID = m.Rect, m.ID
		if scale <= 0 {
			scale = screen.AutoScale(rect, s.cfg.ScreenshotLongEdge)
		}
	default:
		id, perr := strconv.Atoi(spec.monitor)
		m, ok := screen.MonitorByID(mons, id)
		if perr != nil || !ok {
			return nil, Meta{}, fmt.Errorf("unknown monitor %q (use active, all, or an id from monitors)", spec.monitor)
		}
		rect, monID = m.Rect, m.ID
		if scale <= 0 {
			scale = screen.AutoScale(rect, s.cfg.ScreenshotLongEdge)
		}
	}
	format := spec.format
	if format == "" {
		format = s.cfg.ScreenshotFormat
	}
	shot, err := screen.Grab(s.d.Screen, rect, scale, format, s.cfg.JPEGQuality)
	if err != nil {
		return nil, Meta{}, err
	}
	v := screen.NewView(monID, rect, scale)
	s.setView(v)
	return shot, s.metaFor(v), nil
}

func (s *Session) logTiming(tool string, t0 time.Time) {
	s.log.Printf("tool=%s ms=%d", tool, time.Since(t0).Milliseconds())
}

// begin gates an action on the Controller (pause semantics, spec §7) and updates the overlay.
func (s *Session) begin(ctx context.Context, action, summary string) *mcp.CallToolResult {
	now := time.Now()
	if c := s.d.Controller; c != nil {
		if c.IsPaused() {
			wctx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.PauseWaitMs)*time.Millisecond)
			resumed := c.WaitResume(wctx)
			cancel()
			if !resumed {
				return errResult("user_took_control", "The user took control of the computer (hotkey or physical input). Stop now, report what was done and what remains, and wait for the user to ask you to continue.")
			}
			shot, meta, err := s.capture(captureSpec{})
			f := map[string]any{"ok": false, "resumed": true, "note": "The user handed control back. This action was NOT performed; look at the fresh screenshot and continue from the current state."}
			if err == nil {
				meta.into(f)
			}
			return okResult(f, shot)
		}
		c.Acquire(now)
		c.Touch(now)
	}
	s.d.Overlay.SetAction(summary)
	if m := s.activeMonitor(); m.ID != 0 {
		s.d.Overlay.Show(m, platform.OverlayControlling)
	}
	return nil
}

func (s *Session) settleFor(action string) time.Duration {
	switch action {
	case "click", "key":
		return 100 * time.Millisecond
	case "type":
		return 60 * time.Millisecond
	case "scroll":
		return 120 * time.Millisecond
	case "drag":
		return 150 * time.Millisecond
	case "window":
		return 200 * time.Millisecond
	}
	return 50 * time.Millisecond
}

// finish waits for the UI to settle, captures if wanted, and builds the standard result.
func (s *Session) finish(action string, t0 time.Time, extra map[string]any, withShot bool, settle time.Duration) *mcp.CallToolResult {
	f := map[string]any{"action": action}
	for k, v := range extra {
		f[k] = v
	}
	var shot *screen.Shot
	if withShot {
		time.Sleep(settle)
		var err error
		var meta Meta
		shot, meta, err = s.capture(captureSpec{monitor: s.sameMonitorSpec()})
		if err == nil {
			meta.into(f)
		} else {
			f["screenshot_error"] = err.Error()
		}
	} else {
		s.metaFor(s.currentView()).into(f)
	}
	f["ms"] = time.Since(t0).Milliseconds()
	s.logTiming(action, t0)
	return okResult(f, shot)
}

// sameMonitorSpec keeps follow-up screenshots on the monitor of the current view (region views reset to full monitor).
func (s *Session) sameMonitorSpec() string {
	v := s.currentView()
	if v.Monitor == 0 {
		return "all"
	}
	return strconv.Itoa(v.Monitor)
}

func (s *Session) wantShot(p *bool) bool { return p == nil || *p }

// resolvePoint turns image-space x,y or an element id into a screen point.
func (s *Session) resolvePoint(x, y *int, element string) (geom.Point, error) {
	if element != "" {
		s.mu.Lock()
		el, ok := s.elements[element]
		s.mu.Unlock()
		if !ok {
			return geom.Point{}, fmt.Errorf("unknown element %q (ids are valid only until the next find)", element)
		}
		r := el.Rect
		if s.d.Access != nil {
			if fresh, err := s.d.Access.Rect(el.Ref); err == nil && !fresh.Empty() {
				r = fresh
			}
		}
		return r.Center(), nil
	}
	if x == nil || y == nil {
		return geom.Point{}, fmt.Errorf("give x and y (pixels of the last screenshot) or an element id from find")
	}
	v := s.currentView()
	return v.ToScreen(v.ClampImage(geom.Point{X: *x, Y: *y})), nil
}

func parseModifiers(names []string) ([]uint16, error) {
	var out []uint16
	for _, n := range names {
		vk, ok := input.ModifierVK(n)
		if !ok {
			return nil, fmt.Errorf("unknown modifier %q (ctrl, alt, shift, win)", n)
		}
		out = append(out, vk)
	}
	return out, nil
}
