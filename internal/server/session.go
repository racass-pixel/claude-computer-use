package server

import (
	"fmt"
	"strconv"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
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
