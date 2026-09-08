package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/input"
)

type TypeIn struct {
	Text             string    `json:"text" jsonschema:"text to type into the focused control; newlines press Enter, tabs press Tab"`
	Mode             string    `json:"mode,omitempty" jsonschema:"auto (default: unicode, paste when long), unicode, paste (clipboard + Ctrl+V), keys (slow per-character for apps that drop fast input)"`
	DelayMs          int       `json:"delay_ms,omitempty" jsonschema:"delay between characters in ms (default 0)"`
	ScreenshotRegion *RegionIn `json:"screenshot_region,omitempty" jsonschema:"after the action, return a zoomed screenshot of this rectangle (last-screenshot pixels) instead of the whole monitor; the coordinate space switches to that region"`
	Screenshot       *bool     `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func summarizeText(t string) string {
	t = strings.ReplaceAll(t, "\n", "⏎")
	r := []rune(t)
	if len(r) > 28 {
		return fmt.Sprintf("type|%q… (%d chars)", string(r[:28]), len(r))
	}
	return fmt.Sprintf("type|%q", t)
}

func (s *Session) toolType(ctx context.Context, req *mcp.CallToolRequest, in TypeIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	shot, err := s.shotFor(s.wantShot(in.Screenshot), in.ScreenshotRegion)
	if err != nil {
		return errResult("bad_args", err.Error()), nil, nil
	}
	if in.Text == "" {
		return errResult("bad_args", "text is empty"), nil, nil
	}
	if early := s.begin(ctx, "type", summarizeText(in.Text)); early != nil {
		return early, nil, nil
	}
	if err := s.actor.Type(in.Text, in.Mode, time.Duration(in.DelayMs)*time.Millisecond); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}
	return s.finish("type", t0, map[string]any{"chars": len([]rune(in.Text))}, shot, s.settleFor("type")), nil, nil
}

type KeyIn struct {
	Key              string    `json:"key,omitempty" jsonschema:"one chord such as ctrl+s, alt+f4, win+r, enter, f5"`
	Keys             []string  `json:"keys,omitempty" jsonschema:"a sequence of chords pressed one after another, e.g. [\"win+r\",\"enter\"]"`
	HoldMs           int       `json:"hold_ms,omitempty" jsonschema:"how long to hold each chord (default 10)"`
	ScreenshotRegion *RegionIn `json:"screenshot_region,omitempty" jsonschema:"after the action, return a zoomed screenshot of this rectangle (last-screenshot pixels) instead of the whole monitor; the coordinate space switches to that region"`
	Screenshot       *bool     `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func (s *Session) toolKey(ctx context.Context, req *mcp.CallToolRequest, in KeyIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	shot, err := s.shotFor(s.wantShot(in.Screenshot), in.ScreenshotRegion)
	if err != nil {
		return errResult("bad_args", err.Error()), nil, nil
	}
	names := in.Keys
	if in.Key != "" {
		names = append([]string{in.Key}, names...)
	}
	if len(names) == 0 {
		return errResult("bad_args", "give key (one chord) or keys (a sequence)"), nil, nil
	}
	chords := make([]input.Chord, 0, len(names))
	for _, n := range names {
		c, err := input.ParseChord(n)
		if err != nil {
			return errResult("bad_key", err.Error()), nil, nil
		}
		chords = append(chords, c)
	}
	if early := s.begin(ctx, "key", "key|"+strings.Join(names, " ")); early != nil {
		return early, nil, nil
	}
	for i, c := range chords {
		if err := s.actor.Chord(c, time.Duration(in.HoldMs)*time.Millisecond); err != nil {
			return errResult("input_failed", err.Error()), nil, nil
		}
		if i < len(chords)-1 {
			time.Sleep(40 * time.Millisecond)
		}
	}
	return s.finish("key", t0, map[string]any{"keys": names}, shot, s.settleFor("key")), nil, nil
}

type ClipboardIn struct {
	Action string `json:"action" jsonschema:"get or set"`
	Text   string `json:"text,omitempty" jsonschema:"text to place on the clipboard when action is set"`
}

func (s *Session) toolClipboard(ctx context.Context, req *mcp.CallToolRequest, in ClipboardIn) (*mcp.CallToolResult, any, error) {
	if s.d.Clip == nil {
		return errResult("unsupported", "clipboard is not available"), nil, nil
	}
	switch in.Action {
	case "get":
		t, err := s.d.Clip.GetText()
		if err != nil {
			return errResult("clipboard_failed", err.Error()), nil, nil
		}
		return okResult(map[string]any{"text": t}, nil), nil, nil
	case "set":
		if err := s.d.Clip.SetText(in.Text); err != nil {
			return errResult("clipboard_failed", err.Error()), nil, nil
		}
		return okResult(map[string]any{"chars": len([]rune(in.Text))}, nil), nil, nil
	}
	return errResult("bad_args", "action must be get or set"), nil, nil
}
