package server

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/window"
)

type FindIn struct {
	Query        string `json:"query,omitempty" jsonschema:"case-insensitive regexp on the element name, e.g. \"^Save$\" or \"file name\""`
	Role         string `json:"role,omitempty" jsonschema:"Button, Edit, CheckBox, ComboBox, MenuItem, ListItem, TreeItem, TabItem, Hyperlink, Text, Document, Window, Pane..."`
	Window       string `json:"window,omitempty" jsonschema:"\"foreground\" (default), a window id from windows, \"all\" for the whole desktop, or a regexp on title/process"`
	AutomationID string `json:"automation_id,omitempty" jsonschema:"exact AutomationId (stable ids in native apps)"`
	Limit        int    `json:"limit,omitempty" jsonschema:"max results (default 25)"`
	Screenshot   *bool  `json:"screenshot,omitempty" jsonschema:"also return a screenshot (default false)"`
}

type elementOut struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	AutomationID string    `json:"automation_id,omitempty"`
	Value        string    `json:"value,omitempty"`
	Rect         geom.Rect `json:"rect"`
	Center       [2]int    `json:"center"`
	ScreenRect   geom.Rect `json:"screen_rect"`
	Enabled      bool      `json:"enabled"`
	Focused      bool      `json:"focused"`
	InView       bool      `json:"in_view"`
}

func (s *Session) storeElements(els []platform.Element) {
	s.mu.Lock()
	old := s.elemRefs
	s.elements = map[string]platform.Element{}
	s.elemRefs = nil
	for i := range els {
		els[i].ID = fmt.Sprintf("e%d", i+1)
		s.elements[els[i].ID] = els[i]
		if els[i].Ref != nil {
			s.elemRefs = append(s.elemRefs, els[i].Ref)
		}
	}
	s.mu.Unlock()
	if s.d.Access != nil && len(old) > 0 {
		s.d.Access.Release(old)
	}
}

func (s *Session) resolveFindWindow(target string) (uintptr, platform.WindowInfo, error) {
	switch target {
	case "all":
		return 0, platform.WindowInfo{}, nil
	case "", "foreground":
		w := s.foreground()
		if w.ID == 0 {
			return 0, w, fmt.Errorf("no foreground window; pass window:\"all\" or a window id")
		}
		return w.ID, w, nil
	}
	list, err := s.d.Wins.List()
	if err != nil {
		return 0, platform.WindowInfo{}, err
	}
	w, err := window.Match(list, target)
	return w.ID, w, err
}

func (s *Session) findElements(in FindIn) ([]elementOut, platform.WindowInfo, error) {
	if s.d.Access == nil {
		return nil, platform.WindowInfo{}, fmt.Errorf("UI Automation is not available; use screenshots")
	}
	hwnd, w, err := s.resolveFindWindow(in.Window)
	if err != nil {
		return nil, w, err
	}
	els, err := s.d.Access.Find(platform.FindQuery{Name: in.Query, Role: in.Role, AutomationID: in.AutomationID, Window: hwnd, Limit: in.Limit})
	if err != nil {
		return nil, w, err
	}
	sort.SliceStable(els, func(i, j int) bool {
		if els[i].Rect.Y != els[j].Rect.Y {
			return els[i].Rect.Y < els[j].Rect.Y
		}
		return els[i].Rect.X < els[j].Rect.X
	})
	s.storeElements(els)
	v := s.currentView()
	out := make([]elementOut, 0, len(els))
	for _, e := range els {
		r := v.RectToImage(e.Rect)
		c := v.ToImage(e.Rect.Center())
		out = append(out, elementOut{
			ID: e.ID, Name: e.Name, Role: e.Role, AutomationID: e.AutomationID, Value: e.Value,
			Rect: r, Center: [2]int{c.X, c.Y}, ScreenRect: e.Rect, Enabled: e.Enabled, Focused: e.Focused,
			InView: !e.Rect.Intersect(v.Screen).Empty() && !e.Offscreen,
		})
	}
	return out, w, nil
}

func (s *Session) toolFind(ctx context.Context, req *mcp.CallToolRequest, in FindIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	out, w, err := s.findElements(in)
	if err != nil {
		if s.d.Access == nil {
			return errResult("unsupported", err.Error()), nil, nil
		}
		return errResult("find_failed", err.Error()), nil, nil
	}
	f := map[string]any{"elements": out, "count": len(out), "note": "rect/center are in the coordinate space of the last screenshot; ids are valid until the next find"}
	if w.ID != 0 {
		f["window"] = map[string]any{"id": w.ID, "title": w.Title, "process": w.Process}
	}
	s.logTiming("find", t0)
	if in.Screenshot != nil && *in.Screenshot {
		return s.finish("find", t0, f, shotSpec{Want: true}, 0), nil, nil
	}
	return okResult(f, nil), nil, nil
}
