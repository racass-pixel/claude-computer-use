package server

import (
	"context"
	"image/color"
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

// (a) click with screenshot_region returns a zoomed image and switches the view.
func TestClickWithScreenshotRegion(t *testing.T) {
	h := newHarness(t)
	// Establish a view: monitor 1, 1920x1080, scale ~0.7114, image ~1366x768.
	h.s.toolScreenshot(context.Background(), nil, ScreenshotIn{})
	x, y := 683, 384
	region := &RegionIn{X: 100, Y: 100, W: 200, H: 100}
	res, _, _ := h.s.toolClick(context.Background(), nil, ClickIn{X: &x, Y: &y, ScreenshotRegion: region})
	fields, img := decode(t, res)
	if res.IsError || img == nil {
		t.Fatalf("res=%+v", fields)
	}
	size := fields["image"].([]any)
	// region 200x100 image px ≈ 281x141 screen px, ZoomScale capped at 2 → 562×282.
	if size[0] != float64(562) || size[1] != float64(282) {
		t.Fatalf("image = %v, want [562 282]", size)
	}
	v := h.s.currentView()
	if v.Scale != 2 {
		t.Fatalf("view scale = %v, want 2", v.Scale)
	}
}

// (b) pixel returns hex color and screen coords from the fake fill.
func TestPixelReturnsColor(t *testing.T) {
	h := newHarness(t)
	h.scr.Fill = color.RGBA{R: 10, G: 20, B: 30, A: 255}
	x, y := 10, 10
	res, _, _ := h.s.toolPixel(context.Background(), nil, PixelIn{X: &x, Y: &y})
	fields, _ := decode(t, res)
	if fields["ok"] != true {
		t.Fatalf("res=%+v", fields)
	}
	pixels := fields["pixels"].([]any)
	p := pixels[0].(map[string]any)
	if p["color"] != "#0A141E" {
		t.Fatalf("color = %v, want #0A141E", p["color"])
	}
	screen := p["screen"].([]any)
	if screen[0] != float64(14) || screen[1] != float64(14) {
		t.Fatalf("screen = %v, want [14 14]", screen)
	}
}

// (c) click_until stops on color match after the expected number of clicks.
func TestClickUntilStopsOnMatch(t *testing.T) {
	h := newHarness(t)
	red := []byte{0xFF, 0x00, 0x00, 0xFF}
	yellow := []byte{0xFF, 0xFF, 0x00, 0xFF}
	h.scr.Frames = [][]byte{red, red, yellow}
	x, y := 683, 384
	px, py := 10, 10
	off := false
	res, _, _ := h.s.toolClickUntil(context.Background(), nil, ClickUntilIn{
		X: &x, Y: &y,
		Probe:      PointIn{X: &px, Y: &py},
		Color:      "#FFFF00",
		IntervalMs: 50,
		Screenshot: &off,
	})
	fields, _ := decode(t, res)
	if fields["clicks"] != float64(2) {
		t.Fatalf("clicks = %v, want 2", fields["clicks"])
	}
	if fields["stopped_by"] != "match" {
		t.Fatalf("stopped_by = %v, want match", fields["stopped_by"])
	}
	got := strings.Join(h.in.Calls, "|")
	want := "move 960,540|down left|up left|down left|up left"
	if got != want {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}

// (d) click_until stops on max when the probe never matches.
func TestClickUntilStopsOnMax(t *testing.T) {
	h := newHarness(t)
	red := []byte{0xFF, 0x00, 0x00, 0xFF}
	h.scr.Frames = [][]byte{red, red, red}
	x, y := 683, 384
	px, py := 10, 10
	off := false
	res, _, _ := h.s.toolClickUntil(context.Background(), nil, ClickUntilIn{
		X: &x, Y: &y,
		Probe:      PointIn{X: &px, Y: &py},
		Color:      "#FFFF00",
		Max:        3,
		IntervalMs: 50,
		Screenshot: &off,
	})
	fields, _ := decode(t, res)
	if fields["clicks"] != float64(3) {
		t.Fatalf("clicks = %v, want 3", fields["clicks"])
	}
	if fields["stopped_by"] != "max" {
		t.Fatalf("stopped_by = %v, want max", fields["stopped_by"])
	}
}

// (e) click_until works as a batch step.
func TestBatchClickUntil(t *testing.T) {
	h := newHarness(t)
	yellow := []byte{0xFF, 0xFF, 0x00, 0xFF}
	h.scr.Frames = [][]byte{yellow} // already matches → 0 clicks
	off := false
	res, _, _ := h.s.toolBatch(context.Background(), nil, BatchIn{
		Actions: []BatchAction{{
			Tool: "click_until",
			Args: map[string]any{
				"x": 683, "y": 384,
				"probe": map[string]any{"x": 10, "y": 10},
				"color": "#FFFF00",
			},
		}},
		Screenshot: &off,
	})
	fields, _ := decode(t, res)
	steps := fields["steps"].([]any)
	if len(steps) != 1 || steps[0].(map[string]any)["ok"] != true {
		t.Fatalf("batch steps = %v", steps)
	}
}

// screenshot_region outside the view returns an error and no input calls.
func TestClickWithBadScreenshotRegionReturnsError(t *testing.T) {
	h := newHarness(t)
	h.s.toolScreenshot(context.Background(), nil, ScreenshotIn{})
	x, y := 683, 384
	region := &RegionIn{X: 9000, Y: 9000, W: 200, H: 100} // far outside 1366x768 view
	res, _, _ := h.s.toolClick(context.Background(), nil, ClickIn{X: &x, Y: &y, ScreenshotRegion: region})
	if !res.IsError {
		t.Fatalf("expected error for out-of-range screenshot_region")
	}
	if len(h.in.Calls) != 0 {
		t.Fatalf("no input calls expected, got %v", h.in.Calls)
	}
}

// click_until with stop:"mismatch" stops when the probe stops matching.
func TestClickUntilStopsMismatch(t *testing.T) {
	h := newHarness(t)
	yellow := []byte{0xFF, 0xFF, 0x00, 0xFF}
	red := []byte{0xFF, 0x00, 0x00, 0xFF}
	h.scr.Frames = [][]byte{yellow, yellow, red}
	x, y := 683, 384
	px, py := 10, 10
	off := false
	res, _, _ := h.s.toolClickUntil(context.Background(), nil, ClickUntilIn{
		X: &x, Y: &y,
		Probe:      PointIn{X: &px, Y: &py},
		Color:      "#FFFF00",
		Stop:       "mismatch",
		IntervalMs: 50,
		Screenshot: &off,
	})
	fields, _ := decode(t, res)
	if fields["clicks"] != float64(2) {
		t.Fatalf("clicks = %v, want 2", fields["clicks"])
	}
	if fields["stopped_by"] != "match" {
		t.Fatalf("stopped_by = %v, want match", fields["stopped_by"])
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
