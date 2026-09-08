package server

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

type ClickIn struct {
	X                *int      `json:"x,omitempty" jsonschema:"x in pixels of the last screenshot"`
	Y                *int      `json:"y,omitempty" jsonschema:"y in pixels of the last screenshot"`
	Element          string    `json:"element,omitempty" jsonschema:"element id from find, used instead of x,y"`
	Button           string    `json:"button,omitempty" jsonschema:"left (default), right or middle"`
	Count            int       `json:"count,omitempty" jsonschema:"1 (default), 2 for double click, 3 for triple"`
	Modifiers        []string  `json:"modifiers,omitempty" jsonschema:"keys held during the click: ctrl, alt, shift, win"`
	ScreenshotRegion *RegionIn `json:"screenshot_region,omitempty" jsonschema:"after the action, return a zoomed screenshot of this rectangle (last-screenshot pixels) instead of the whole monitor; the coordinate space switches to that region"`
	Screenshot       *bool     `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func (s *Session) toolClick(ctx context.Context, req *mcp.CallToolRequest, in ClickIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	shot, err := s.shotFor(s.wantShot(in.Screenshot), in.ScreenshotRegion)
	if err != nil {
		return errResult("bad_args", err.Error()), nil, nil
	}
	target := in.Element
	if in.X != nil && in.Y != nil {
		target = fmt.Sprintf("%d,%d", *in.X, *in.Y)
	}
	p, err := s.resolvePoint(in.X, in.Y, in.Element)
	if err != nil {
		return errResult("bad_target", err.Error()), nil, nil
	}
	mods, err := parseModifiers(in.Modifiers)
	if err != nil {
		return errResult("bad_modifier", err.Error()), nil, nil
	}
	if early := s.begin(ctx, "click", "click|"+target, toArgsMap(in)); early != nil {
		return early, nil, nil
	}
	if err := s.actor.Click(p, platform.MouseButton(in.Button), in.Count, mods); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}
	s.d.Overlay.Ripple(p)
	return s.finish("click", t0, map[string]any{"screen_point": [2]int{p.X, p.Y}}, shot, s.settleFor("click")), nil, nil
}

type MoveIn struct {
	X                *int      `json:"x,omitempty" jsonschema:"x in pixels of the last screenshot"`
	Y                *int      `json:"y,omitempty" jsonschema:"y in pixels of the last screenshot"`
	Element          string    `json:"element,omitempty" jsonschema:"element id from find, used instead of x,y"`
	ScreenshotRegion *RegionIn `json:"screenshot_region,omitempty" jsonschema:"after the action, return a zoomed screenshot of this rectangle (last-screenshot pixels) instead of the whole monitor; the coordinate space switches to that region"`
	Screenshot       *bool     `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default false for move)"`
}

func (s *Session) toolMove(ctx context.Context, req *mcp.CallToolRequest, in MoveIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	shot, err := s.shotFor(in.Screenshot != nil && *in.Screenshot, in.ScreenshotRegion)
	if err != nil {
		return errResult("bad_args", err.Error()), nil, nil
	}
	p, err := s.resolvePoint(in.X, in.Y, in.Element)
	if err != nil {
		return errResult("bad_target", err.Error()), nil, nil
	}
	if early := s.begin(ctx, "move", fmt.Sprintf("move|%d,%d", p.X, p.Y), toArgsMap(in)); early != nil {
		return early, nil, nil
	}
	if err := s.actor.MoveTo(p); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}
	return s.finish("move", t0, map[string]any{"screen_point": [2]int{p.X, p.Y}}, shot, s.settleFor("move")), nil, nil
}

type PointIn struct {
	X       *int   `json:"x,omitempty" jsonschema:"x in pixels of the last screenshot"`
	Y       *int   `json:"y,omitempty" jsonschema:"y in pixels of the last screenshot"`
	Element string `json:"element,omitempty" jsonschema:"element id from find, used instead of x,y"`
}

type DragIn struct {
	From             PointIn   `json:"from" jsonschema:"where to press"`
	To               PointIn   `json:"to" jsonschema:"where to release"`
	Button           string    `json:"button,omitempty" jsonschema:"left (default), right or middle"`
	DurationMs       int       `json:"duration_ms,omitempty" jsonschema:"drag duration in ms (default 250); use 600+ for drag-and-drop into other apps"`
	ScreenshotRegion *RegionIn `json:"screenshot_region,omitempty" jsonschema:"after the action, return a zoomed screenshot of this rectangle (last-screenshot pixels) instead of the whole monitor; the coordinate space switches to that region"`
	Screenshot       *bool     `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func (s *Session) toolDrag(ctx context.Context, req *mcp.CallToolRequest, in DragIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	shot, err := s.shotFor(s.wantShot(in.Screenshot), in.ScreenshotRegion)
	if err != nil {
		return errResult("bad_args", err.Error()), nil, nil
	}
	from, err := s.resolvePoint(in.From.X, in.From.Y, in.From.Element)
	if err != nil {
		return errResult("bad_target", "from: "+err.Error()), nil, nil
	}
	to, err := s.resolvePoint(in.To.X, in.To.Y, in.To.Element)
	if err != nil {
		return errResult("bad_target", "to: "+err.Error()), nil, nil
	}
	if early := s.begin(ctx, "drag", fmt.Sprintf("drag|→ %d,%d", to.X, to.Y), toArgsMap(in)); early != nil {
		return early, nil, nil
	}
	if err := s.actor.Drag(from, to, platform.MouseButton(in.Button), time.Duration(in.DurationMs)*time.Millisecond); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}
	s.d.Overlay.Ripple(to)
	return s.finish("drag", t0, map[string]any{"from": [2]int{from.X, from.Y}, "to": [2]int{to.X, to.Y}}, shot, s.settleFor("drag")), nil, nil
}

type ScrollIn struct {
	X                *int      `json:"x,omitempty" jsonschema:"optional x (last-screenshot pixels) to scroll at"`
	Y                *int      `json:"y,omitempty" jsonschema:"optional y (last-screenshot pixels) to scroll at"`
	Dx               int       `json:"dx,omitempty" jsonschema:"horizontal wheel ticks; positive scrolls right"`
	Dy               int       `json:"dy,omitempty" jsonschema:"vertical wheel ticks; positive scrolls DOWN (3 ≈ one small step, 10 ≈ a page)"`
	ScreenshotRegion *RegionIn `json:"screenshot_region,omitempty" jsonschema:"after the action, return a zoomed screenshot of this rectangle (last-screenshot pixels) instead of the whole monitor; the coordinate space switches to that region"`
	Screenshot       *bool     `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func (s *Session) toolScroll(ctx context.Context, req *mcp.CallToolRequest, in ScrollIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	shot, err := s.shotFor(s.wantShot(in.Screenshot), in.ScreenshotRegion)
	if err != nil {
		return errResult("bad_args", err.Error()), nil, nil
	}
	var at *geom.Point
	if in.X != nil && in.Y != nil {
		p, err := s.resolvePoint(in.X, in.Y, "")
		if err != nil {
			return errResult("bad_target", err.Error()), nil, nil
		}
		at = &p
	}
	if in.Dx == 0 && in.Dy == 0 {
		return errResult("bad_args", "give dx and/or dy in wheel ticks"), nil, nil
	}
	if early := s.begin(ctx, "scroll", fmt.Sprintf("scroll|%d,%d", in.Dx, in.Dy), toArgsMap(in)); early != nil {
		return early, nil, nil
	}
	if err := s.actor.Scroll(at, in.Dx, in.Dy); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}
	return s.finish("scroll", t0, nil, shot, s.settleFor("scroll")), nil, nil
}

// --- pixel tool ---

type PixelIn struct {
	X      *int      `json:"x,omitempty" jsonschema:"x in pixels of the last screenshot (single point)"`
	Y      *int      `json:"y,omitempty" jsonschema:"y in pixels of the last screenshot (single point)"`
	Points []PointIn `json:"points,omitempty" jsonschema:"multiple points to sample"`
}

func (s *Session) toolPixel(ctx context.Context, req *mcp.CallToolRequest, in PixelIn) (*mcp.CallToolResult, any, error) {
	var points []PointIn
	if in.X != nil && in.Y != nil {
		points = append(points, PointIn{X: in.X, Y: in.Y})
	}
	points = append(points, in.Points...)
	if len(points) == 0 {
		return errResult("bad_args", "give x,y or points"), nil, nil
	}

	type pixResult struct {
		X      int    `json:"x"`
		Y      int    `json:"y"`
		Screen [2]int `json:"screen"`
		Color  string `json:"color"`
		RGB    [3]int `json:"rgb"`
	}

	results := make([]pixResult, 0, len(points))
	for _, pt := range points {
		sp, err := s.resolvePoint(pt.X, pt.Y, pt.Element)
		if err != nil {
			return errResult("bad_target", err.Error()), nil, nil
		}
		img, err := s.d.Screen.Capture(geom.Rect{X: sp.X, Y: sp.Y, W: 1, H: 1})
		if err != nil {
			return errResult("capture_failed", err.Error()), nil, nil
		}
		r, g, b := img.Pix[0], img.Pix[1], img.Pix[2]
		ix, iy := 0, 0
		if pt.X != nil {
			ix = *pt.X
		}
		if pt.Y != nil {
			iy = *pt.Y
		}
		results = append(results, pixResult{
			X:      ix,
			Y:      iy,
			Screen: [2]int{sp.X, sp.Y},
			Color:  fmt.Sprintf("#%02X%02X%02X", r, g, b),
			RGB:    [3]int{int(r), int(g), int(b)},
		})
	}
	return okResult(map[string]any{"pixels": results}, nil), nil, nil
}

// --- click_until tool ---

type ClickUntilIn struct {
	X                *int      `json:"x,omitempty" jsonschema:"x in pixels of the last screenshot"`
	Y                *int      `json:"y,omitempty" jsonschema:"y in pixels of the last screenshot"`
	Element          string    `json:"element,omitempty" jsonschema:"element id from find, used instead of x,y"`
	Button           string    `json:"button,omitempty" jsonschema:"left (default), right or middle"`
	Probe            PointIn   `json:"probe" jsonschema:"pixel to watch (last-screenshot coordinates)"`
	Color            string    `json:"color" jsonschema:"#RRGGBB the probe is compared with"`
	Tolerance        int       `json:"tolerance,omitempty" jsonschema:"max per-channel difference to count as a match (default 40)"`
	Stop             string    `json:"stop,omitempty" jsonschema:"match (default): stop when the probe matches the color; mismatch: stop when it stops matching"`
	Max              int       `json:"max,omitempty" jsonschema:"max clicks (default 30, cap 500)"`
	IntervalMs       int       `json:"interval_ms,omitempty" jsonschema:"pause after each click for the UI to update (default 300, min 50)"`
	ScreenshotRegion *RegionIn `json:"screenshot_region,omitempty" jsonschema:"after the action, return a zoomed screenshot of this rectangle (last-screenshot pixels) instead of the whole monitor; the coordinate space switches to that region"`
	Screenshot       *bool     `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func (s *Session) toolClickUntil(ctx context.Context, req *mcp.CallToolRequest, in ClickUntilIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	// Resolve screenshot region from current view before anything changes.
	shot, err := s.shotFor(s.wantShot(in.Screenshot), in.ScreenshotRegion)
	if err != nil {
		return errResult("bad_args", err.Error()), nil, nil
	}

	// Resolve click point.
	clickPt, err := s.resolvePoint(in.X, in.Y, in.Element)
	if err != nil {
		return errResult("bad_target", err.Error()), nil, nil
	}
	// Resolve probe point.
	probePt, err := s.resolvePoint(in.Probe.X, in.Probe.Y, in.Probe.Element)
	if err != nil {
		return errResult("bad_target", "probe: "+err.Error()), nil, nil
	}
	// Parse target color.
	cr, cg, cb, err := parseHexColor(in.Color)
	if err != nil {
		return errResult("bad_args", err.Error()), nil, nil
	}

	// Defaults and caps.
	maxClicks := in.Max
	if maxClicks <= 0 {
		maxClicks = 30
	}
	if maxClicks > 500 {
		maxClicks = 500
	}
	interval := in.IntervalMs
	if interval <= 0 {
		interval = 300
	}
	if interval < 50 {
		interval = 50
	}
	tolerance := in.Tolerance
	if tolerance <= 0 {
		tolerance = 40
	}
	stopMode := in.Stop
	if stopMode == "" {
		stopMode = "match"
	}
	btn := platform.MouseButton(in.Button)
	if btn == "" {
		btn = platform.ButtonLeft
	}

	if early := s.begin(ctx, "click_until", fmt.Sprintf("click_until|%d,%d probe %d,%d", clickPt.X, clickPt.Y, probePt.X, probePt.Y), toArgsMap(in)); early != nil {
		return early, nil, nil
	}

	// Move to click point once (no glide between clicks).
	if err := s.actor.MoveTo(clickPt); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}

	clicks := 0
	stoppedBy := "max"
	var finalColor string

	for i := 0; i < maxClicks; i++ {
		// Check pause between iterations.
		if c := s.d.Controller; c != nil && c.IsPaused() {
			stoppedBy = "paused"
			break
		}
		// Probe color via a 1x1 capture at the screen-space probe point.
		img, err := s.d.Screen.Capture(geom.Rect{X: probePt.X, Y: probePt.Y, W: 1, H: 1})
		if err != nil {
			return errResult("capture_failed", err.Error()), nil, nil
		}
		pr, pg, pb := img.Pix[0], img.Pix[1], img.Pix[2]
		finalColor = fmt.Sprintf("#%02X%02X%02X", pr, pg, pb)

		matches := colorMatch(pr, pg, pb, cr, cg, cb, tolerance)
		if matches == (stopMode == "match") {
			stoppedBy = "match"
			break
		}
		// Click: MouseDown + MouseUp at the point (no glide, no settle).
		if err := s.actor.In.MouseDown(btn); err != nil {
			return errResult("input_failed", err.Error()), nil, nil
		}
		if err := s.actor.In.MouseUp(btn); err != nil {
			return errResult("input_failed", err.Error()), nil, nil
		}
		clicks++
		// Sleep interval to let the UI update.
		time.Sleep(time.Duration(interval) * time.Millisecond)
	}

	ok := stoppedBy != "paused"
	probeImg := [2]int{0, 0}
	if in.Probe.X != nil {
		probeImg[0] = *in.Probe.X
	}
	if in.Probe.Y != nil {
		probeImg[1] = *in.Probe.Y
	}
	extra := map[string]any{
		"ok":           ok,
		"clicks":       clicks,
		"stopped_by":   stoppedBy,
		"probe":        probeImg,
		"probe_screen": [2]int{probePt.X, probePt.Y},
		"color_final":  finalColor,
	}
	return s.finish("click_until", t0, extra, shot, 0), nil, nil
}
