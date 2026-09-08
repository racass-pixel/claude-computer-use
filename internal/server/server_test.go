package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image/color"
	"log"
	"os"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/config"
	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/platform/fake"
)

type harness struct {
	s    *Session
	in   *fake.Input
	scr  *fake.Screen
	wins *fake.Windows
	clip *fake.Clipboard
	ov   *fake.Overlay
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{
		in:   &fake.Input{},
		clip: &fake.Clipboard{},
		ov:   &fake.Overlay{},
		scr: &fake.Screen{
			Fill:   color.RGBA{R: 1, G: 2, B: 3, A: 255},
			Cursor: geom.Point{X: 960, Y: 540},
			Mons: []platform.Monitor{
				{ID: 1, Rect: geom.Rect{W: 1920, H: 1080}, Primary: true, ScaleFactor: 1},
				{ID: 2, Rect: geom.Rect{X: 1920, Y: 0, W: 1920, H: 1080}, ScaleFactor: 1},
			},
		},
		wins: &fake.Windows{Wins: []platform.WindowInfo{
			{ID: 42, Title: "Untitled - Notepad", Process: "notepad.exe", Rect: geom.Rect{X: 100, Y: 100, W: 800, H: 600}, Foreground: true},
		}},
	}
	cfg := config.Default()
	h.s = New(Deps{Screen: h.scr, Input: h.in, Clip: h.clip, Wins: h.wins, Overlay: h.ov, Version: "test"}, cfg, log.New(os.Stderr, "", 0))
	return h
}

// decode splits a tool result into its JSON fields and the image bytes (nil if none).
func decode(t *testing.T, res *mcp.CallToolResult) (map[string]any, []byte) {
	t.Helper()
	var fields map[string]any
	var img []byte
	for _, c := range res.Content {
		switch v := c.(type) {
		case *mcp.TextContent:
			if err := json.Unmarshal([]byte(v.Text), &fields); err != nil {
				t.Fatalf("bad JSON %q: %v", v.Text, err)
			}
		case *mcp.ImageContent:
			img = v.Data
		}
	}
	return fields, img
}

func TestScreenshotActiveMonitorSetsView(t *testing.T) {
	h := newHarness(t)
	res, _, err := h.s.toolScreenshot(context.Background(), nil, ScreenshotIn{})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	fields, img := decode(t, res)
	if !bytes.HasPrefix(img, []byte("\x89PNG")) {
		t.Fatalf("expected PNG image content")
	}
	if fields["monitor"] != float64(1) {
		t.Fatalf("monitor = %v", fields["monitor"])
	}
	size := fields["image"].([]any)
	if size[0] != float64(1366) || size[1] != float64(768) {
		t.Fatalf("image = %v", size)
	}
	if h.s.currentView().Monitor != 1 || h.s.currentView().Image.W != 1366 {
		t.Fatalf("view not stored: %+v", h.s.currentView())
	}
	fg := fields["foreground"].(map[string]any)
	if fg["process"] != "notepad.exe" {
		t.Fatalf("foreground = %v", fg)
	}
}

func TestScreenshotRegionZoomsAndSecondMonitorOffsets(t *testing.T) {
	h := newHarness(t)
	res, _, _ := h.s.toolScreenshot(context.Background(), nil, ScreenshotIn{Monitor: "2"})
	fields, _ := decode(t, res)
	if fields["monitor"] != float64(2) || h.s.currentView().Offset.X != 1920 {
		t.Fatalf("monitor 2 view wrong: %+v", h.s.currentView())
	}
	res, _, _ = h.s.toolScreenshot(context.Background(), nil, ScreenshotIn{Region: &RegionIn{X: 100, Y: 100, W: 200, H: 100}})
	fields, _ = decode(t, res)
	size := fields["image"].([]any)
	if size[0] != float64(562) || size[1] != float64(282) { // region 200x100 image px = 281x141 screen px, zoomed 2x
		t.Fatalf("zoomed image = %v", size)
	}
	v := h.s.currentView()
	if v.Monitor != 2 || v.Scale != 2 {
		t.Fatalf("zoom view = %+v", v)
	}
}

func TestMonitorsToolListsFlags(t *testing.T) {
	h := newHarness(t)
	res, _, _ := h.s.toolMonitors(context.Background(), nil, struct{}{})
	fields, _ := decode(t, res)
	mons := fields["monitors"].([]any)
	if len(mons) != 2 || mons[0].(map[string]any)["has_cursor"] != true || mons[0].(map[string]any)["has_foreground"] != true {
		t.Fatalf("monitors = %v", mons)
	}
}

func TestInputSchemasMarkOnlyRequiredFields(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	newHarness(t).s.Register(srv)
	// The go-sdk infers schemas from struct tags: every field of ScreenshotIn is optional.
	// If this test fails, add `omitempty` to the field or make it a pointer.
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	go srv.Run(ctx, serverTransport)
	client := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil)
	sess, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	tools, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	// allowedRequired lists the only fields each tool's schema may mark required
	// (the go-sdk infers "required" from struct tags: a field without `omitempty`,
	// or a non-pointer struct field, is required).
	allowedRequired := map[string]map[string]bool{
		"screenshot":  {},
		"monitors":    {},
		"mouse_down":  {},
		"mouse_up":    {},
		"click":       {},
		"move":        {},
		"drag":        {"from": true, "to": true},
		"scroll":      {},
		"type":        {"text": true},
		"key":         {},
		"clipboard":   {"action": true},
		"windows":     {},
		"window":      {"action": true},
		"find":        {},
		"wait":        {},
		"pixel":       {},
		"click_until": {"probe": true, "color": true},
		"batch":       {"actions": true},
		"control":     {"action": true},
		"recipe":      {"action": true},
	}
	if len(tools.Tools) != len(allowedRequired) {
		t.Fatalf("got %d tools, want %d: %v", len(tools.Tools), len(allowedRequired), tools.Tools)
	}
	for _, tool := range tools.Tools {
		allowed, known := allowedRequired[tool.Name]
		if !known {
			t.Fatalf("unexpected tool %q; add it to allowedRequired", tool.Name)
		}
		b, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("%s: marshal schema: %v", tool.Name, err)
		}
		var schema struct {
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(b, &schema); err != nil {
			t.Fatalf("%s: unmarshal schema: %v", tool.Name, err)
		}
		for _, f := range schema.Required {
			if !allowed[f] {
				t.Fatalf("%s: field %q must not be required (schema: %s)", tool.Name, f, b)
			}
		}
		for f := range allowed {
			found := false
			for _, r := range schema.Required {
				if r == f {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s: field %q must be required (schema: %s)", tool.Name, f, b)
			}
		}
	}
}
