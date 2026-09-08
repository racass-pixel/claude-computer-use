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
	X          *int     `json:"x,omitempty" jsonschema:"x in pixels of the last screenshot"`
	Y          *int     `json:"y,omitempty" jsonschema:"y in pixels of the last screenshot"`
	Element    string   `json:"element,omitempty" jsonschema:"element id from find, used instead of x,y"`
	Button     string   `json:"button,omitempty" jsonschema:"left (default), right or middle"`
	Count      int      `json:"count,omitempty" jsonschema:"1 (default), 2 for double click, 3 for triple"`
	Modifiers  []string `json:"modifiers,omitempty" jsonschema:"keys held during the click: ctrl, alt, shift, win"`
	Screenshot *bool    `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func (s *Session) toolClick(ctx context.Context, req *mcp.CallToolRequest, in ClickIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
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
	if early := s.begin(ctx, "click", "click "+target); early != nil {
		return early, nil, nil
	}
	if err := s.actor.Click(p, platform.MouseButton(in.Button), in.Count, mods); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}
	s.d.Overlay.Ripple(p)
	return s.finish("click", t0, map[string]any{"screen_point": [2]int{p.X, p.Y}}, s.wantShot(in.Screenshot), s.settleFor("click")), nil, nil
}

type MoveIn struct {
	X          *int   `json:"x,omitempty" jsonschema:"x in pixels of the last screenshot"`
	Y          *int   `json:"y,omitempty" jsonschema:"y in pixels of the last screenshot"`
	Element    string `json:"element,omitempty" jsonschema:"element id from find, used instead of x,y"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default false for move)"`
}

func (s *Session) toolMove(ctx context.Context, req *mcp.CallToolRequest, in MoveIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	p, err := s.resolvePoint(in.X, in.Y, in.Element)
	if err != nil {
		return errResult("bad_target", err.Error()), nil, nil
	}
	if early := s.begin(ctx, "move", fmt.Sprintf("move %d,%d", p.X, p.Y)); early != nil {
		return early, nil, nil
	}
	if err := s.actor.MoveTo(p); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}
	return s.finish("move", t0, map[string]any{"screen_point": [2]int{p.X, p.Y}}, in.Screenshot != nil && *in.Screenshot, s.settleFor("move")), nil, nil
}

type PointIn struct {
	X       *int   `json:"x,omitempty" jsonschema:"x in pixels of the last screenshot"`
	Y       *int   `json:"y,omitempty" jsonschema:"y in pixels of the last screenshot"`
	Element string `json:"element,omitempty" jsonschema:"element id from find, used instead of x,y"`
}

type DragIn struct {
	From       PointIn `json:"from" jsonschema:"where to press"`
	To         PointIn `json:"to" jsonschema:"where to release"`
	Button     string  `json:"button,omitempty" jsonschema:"left (default), right or middle"`
	DurationMs int     `json:"duration_ms,omitempty" jsonschema:"drag duration in ms (default 250); use 600+ for drag-and-drop into other apps"`
	Screenshot *bool   `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func (s *Session) toolDrag(ctx context.Context, req *mcp.CallToolRequest, in DragIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	from, err := s.resolvePoint(in.From.X, in.From.Y, in.From.Element)
	if err != nil {
		return errResult("bad_target", "from: "+err.Error()), nil, nil
	}
	to, err := s.resolvePoint(in.To.X, in.To.Y, in.To.Element)
	if err != nil {
		return errResult("bad_target", "to: "+err.Error()), nil, nil
	}
	if early := s.begin(ctx, "drag", fmt.Sprintf("drag → %d,%d", to.X, to.Y)); early != nil {
		return early, nil, nil
	}
	if err := s.actor.Drag(from, to, platform.MouseButton(in.Button), time.Duration(in.DurationMs)*time.Millisecond); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}
	s.d.Overlay.Ripple(to)
	return s.finish("drag", t0, map[string]any{"from": [2]int{from.X, from.Y}, "to": [2]int{to.X, to.Y}}, s.wantShot(in.Screenshot), s.settleFor("drag")), nil, nil
}

type ScrollIn struct {
	X          *int  `json:"x,omitempty" jsonschema:"optional x (last-screenshot pixels) to scroll at"`
	Y          *int  `json:"y,omitempty" jsonschema:"optional y (last-screenshot pixels) to scroll at"`
	Dx         int   `json:"dx,omitempty" jsonschema:"horizontal wheel ticks; positive scrolls right"`
	Dy         int   `json:"dy,omitempty" jsonschema:"vertical wheel ticks; positive scrolls DOWN (3 ≈ one small step, 10 ≈ a page)"`
	Screenshot *bool `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func (s *Session) toolScroll(ctx context.Context, req *mcp.CallToolRequest, in ScrollIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
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
	if early := s.begin(ctx, "scroll", fmt.Sprintf("scroll %d,%d", in.Dx, in.Dy)); early != nil {
		return early, nil, nil
	}
	if err := s.actor.Scroll(at, in.Dx, in.Dy); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}
	return s.finish("scroll", t0, nil, s.wantShot(in.Screenshot), s.settleFor("scroll")), nil, nil
}
