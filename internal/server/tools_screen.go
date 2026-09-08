package server

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/screen"
)

type ScreenshotIn struct {
	Monitor string    `json:"monitor,omitempty" jsonschema:"\"active\" (default), \"all\" for every monitor at once, or a monitor id such as \"2\""`
	Region  *RegionIn `json:"region,omitempty" jsonschema:"zoom into a rectangle given in the coordinate space of the last screenshot; later x,y refer to this zoomed image"`
	Scale   float64   `json:"scale,omitempty" jsonschema:"override output scale (0.1-2.0); default fits the long edge to the configured size"`
	Format  string    `json:"format,omitempty" jsonschema:"png (default) or jpeg"`
}

func (s *Session) toolScreenshot(ctx context.Context, req *mcp.CallToolRequest, in ScreenshotIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	scale := in.Scale
	if scale != 0 {
		if scale < 0.1 {
			scale = 0.1
		}
		if scale > 2 {
			scale = 2
		}
	}
	shot, meta, err := s.capture(captureSpec{monitor: in.Monitor, region: in.Region, scale: scale, format: in.Format})
	if err != nil {
		return errResult("screenshot_failed", err.Error()), nil, nil
	}
	f := map[string]any{"ms": time.Since(t0).Milliseconds()}
	meta.into(f)
	s.logTiming("screenshot", t0)
	return okResult(f, shot), nil, nil
}

type monitorOut struct {
	platform.Monitor
	HasCursor     bool `json:"has_cursor"`
	HasForeground bool `json:"has_foreground"`
}

func (s *Session) toolMonitors(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
	mons, err := s.monitors()
	if err != nil {
		return errResult("monitors_failed", err.Error()), nil, nil
	}
	cur := s.cursor()
	fgc := s.foreground().Rect.Center()
	out := make([]monitorOut, 0, len(mons))
	for _, m := range mons {
		out = append(out, monitorOut{Monitor: m, HasCursor: m.Rect.Contains(cur), HasForeground: m.Rect.Contains(fgc)})
	}
	return okResult(map[string]any{"monitors": out, "virtual_screen": screen.VirtualScreen(mons), "cursor": [2]int{cur.X, cur.Y}}, nil), nil, nil
}
