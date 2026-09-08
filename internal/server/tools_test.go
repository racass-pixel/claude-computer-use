package server

import (
	"context"
	"strings"
	"testing"
)

func TestClickMapsImageToScreenAndReturnsScreenshot(t *testing.T) {
	h := newHarness(t)
	h.s.toolScreenshot(context.Background(), nil, ScreenshotIn{}) // view = monitor 1 at 1366/1920
	x, y := 683, 384
	res, _, _ := h.s.toolClick(context.Background(), nil, ClickIn{X: &x, Y: &y})
	fields, img := decode(t, res)
	if res.IsError || fields["ok"] != true || img == nil {
		t.Fatalf("res=%+v", fields)
	}
	if got := strings.Join(h.in.Calls, "|"); got != "move 960,540|down left|up left" {
		t.Fatalf("calls = %q", got)
	}
	if !strings.Contains(strings.Join(h.ov.Calls, "|"), "action click 683,384") {
		t.Fatalf("overlay must show the action: %v", h.ov.Calls)
	}
	if !strings.Contains(strings.Join(h.ov.Calls, "|"), "ripple 960,540") {
		t.Fatalf("overlay must ripple at the screen point: %v", h.ov.Calls)
	}
}

func TestClickRequiresTarget(t *testing.T) {
	h := newHarness(t)
	res, _, _ := h.s.toolClick(context.Background(), nil, ClickIn{})
	if !res.IsError {
		t.Fatalf("click without x,y or element must be an error result")
	}
}

func TestKeyAcceptsSingleAndSequence(t *testing.T) {
	h := newHarness(t)
	off := false
	h.s.toolKey(context.Background(), nil, KeyIn{Key: "ctrl+s", Screenshot: &off})
	h.s.toolKey(context.Background(), nil, KeyIn{Keys: []string{"win+r", "enter"}, Screenshot: &off})
	got := strings.Join(h.in.Calls, "|")
	want := "key_down 17|key_down 83|key_up 83|key_up 17|key_down 91|key_down 82|key_up 82|key_up 91|key_down 13|key_up 13"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestWindowFocusByRegex(t *testing.T) {
	h := newHarness(t)
	off := false
	res, _, _ := h.s.toolWindow(context.Background(), nil, WindowIn{Action: "focus", Target: "notepad", Screenshot: &off})
	if res.IsError || h.wins.Calls[0] != "focus 42" {
		t.Fatalf("focus: %+v %v", res, h.wins.Calls)
	}
}
