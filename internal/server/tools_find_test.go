package server

import (
	"context"
	"strings"
	"testing"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/platform/fake"
)

func TestFindReturnsViewRectsAndClickByElement(t *testing.T) {
	h := newHarness(t)
	h.s.d.Access = &fake.Accessibility{Elems: []platform.Element{
		{Name: "Save", Role: "Button", Rect: geom.Rect{X: 900, Y: 500, W: 120, H: 40}, Enabled: true, Ref: geom.Rect{X: 900, Y: 500, W: 120, H: 40}},
		{Name: "Cancel", Role: "Button", Rect: geom.Rect{X: 1040, Y: 500, W: 120, H: 40}, Enabled: true, Ref: geom.Rect{X: 1040, Y: 500, W: 120, H: 40}},
		{Name: "File name", Role: "Edit", Rect: geom.Rect{X: 300, Y: 400, W: 500, H: 30}, Enabled: true, Ref: geom.Rect{X: 300, Y: 400, W: 500, H: 30}},
	}}
	h.s.toolScreenshot(context.Background(), nil, ScreenshotIn{}) // view 1366/1920
	res, _, _ := h.s.toolFind(context.Background(), nil, FindIn{Query: "^save$", Role: "Button"})
	fields, _ := decode(t, res)
	els := fields["elements"].([]any)
	if len(els) != 1 {
		t.Fatalf("elements = %v", els)
	}
	el := els[0].(map[string]any)
	if el["id"] != "e1" || el["role"] != "Button" {
		t.Fatalf("element = %v", el)
	}
	c := el["center"].([]any)
	if c[0] != float64(683) || c[1] != float64(370) { // (960,520) screen → image
		t.Fatalf("center = %v", c)
	}
	off := false
	res, _, _ = h.s.toolClick(context.Background(), nil, ClickIn{Element: "e1", Screenshot: &off})
	if res.IsError || !strings.Contains(strings.Join(h.in.Calls, "|"), "move 960,520") {
		t.Fatalf("click by element: %v", h.in.Calls)
	}
	res, _, _ = h.s.toolClick(context.Background(), nil, ClickIn{Element: "e9", Screenshot: &off})
	if !res.IsError {
		t.Fatal("unknown element id must be an error")
	}
}

func TestFindWithoutAccessibilityIsAnError(t *testing.T) {
	h := newHarness(t)
	res, _, _ := h.s.toolFind(context.Background(), nil, FindIn{Query: "x"})
	if !res.IsError {
		t.Fatal("expected unsupported error")
	}
}
