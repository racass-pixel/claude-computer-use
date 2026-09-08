package server

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/screen"
)

type ForegroundInfo struct {
	ID      uintptr `json:"id"`
	Title   string  `json:"title"`
	Process string  `json:"process"`
}

// Meta describes the coordinate space of the returned/last screenshot.
type Meta struct {
	Monitor    int             `json:"monitor"`
	Image      [2]int          `json:"image"`
	Scale      float64         `json:"scale"`
	Screen     geom.Rect       `json:"screen"`
	Cursor     [2]int          `json:"cursor"`
	Foreground *ForegroundInfo `json:"foreground,omitempty"`
	Paused     bool            `json:"paused,omitempty"`
}

func (m Meta) into(f map[string]any) {
	f["monitor"] = m.Monitor
	f["image"] = m.Image
	f["scale"] = m.Scale
	f["screen"] = m.Screen
	f["cursor"] = m.Cursor
	if m.Foreground != nil {
		f["foreground"] = m.Foreground
	}
	if m.Paused {
		f["paused"] = true
	}
}

func jsonText(v any) *mcp.TextContent {
	b, err := json.Marshal(v)
	if err != nil {
		b = []byte(`{"error":"encode_failed"}`)
	}
	return &mcp.TextContent{Text: string(b)}
}

// okResult returns one JSON text block (+ image when shot != nil). "ok" defaults to true.
func okResult(fields map[string]any, shot *screen.Shot) *mcp.CallToolResult {
	if _, set := fields["ok"]; !set {
		fields["ok"] = true
	}
	res := &mcp.CallToolResult{Content: []mcp.Content{jsonText(fields)}}
	if shot != nil {
		res.Content = append(res.Content, &mcp.ImageContent{Data: shot.Data, MIMEType: shot.MIME})
	}
	return res
}

func errResult(code, msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{jsonText(map[string]any{"error": code, "message": msg})},
	}
}
