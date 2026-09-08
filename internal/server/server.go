// Package server exposes the desktop as MCP tools.
package server

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/actions"
	"github.com/racass-pixel/claude-computer-use/internal/config"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/screen"
)

// Controller is the pause/acquire state machine (implemented by guard.Machine in Task 10). nil = never paused.
type Controller interface {
	IsPaused() bool
	Acquire(now time.Time)
	Touch(now time.Time)
	Release(now time.Time)
	WaitResume(ctx context.Context) bool // true when control returned to Claude before ctx ended
	Status() ControllerStatus
}

type ControllerStatus struct {
	State  string `json:"state"` // idle | controlling | paused
	Hotkey string `json:"hotkey"`
	IdleMs int64  `json:"idle_ms"`
}

type Deps struct {
	Screen     platform.Screen
	Input      platform.Input
	Clip       platform.Clipboard
	Wins       platform.Windows
	Access     platform.Accessibility // nil until Task 15 → find returns an error result
	Overlay    platform.Overlay       // nil → platform.NopOverlay{}
	Controller Controller             // nil → never paused
	Version    string
}

type Session struct {
	d     Deps
	cfg   config.Config
	log   *log.Logger
	actor *actions.Actor

	mu       sync.Mutex
	view     screen.View
	hasView  bool
	mons     []platform.Monitor
	monsAt   time.Time
	elements map[string]platform.Element
	elemRefs []any
}

func New(d Deps, cfg config.Config, logger *log.Logger) *Session {
	if d.Overlay == nil {
		d.Overlay = platform.NopOverlay{}
	}
	return &Session{
		d: d, cfg: cfg, log: logger,
		actor:    &actions.Actor{In: d.Input, Clip: d.Clip, PasteThreshold: cfg.PasteThreshold},
		elements: map[string]platform.Element{},
	}
}

// Register adds every tool. Descriptions are what the model reads — keep them precise.
func (s *Session) Register(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "screenshot", Description: "Capture the screen. Default: the active monitor (the one with the foreground window). All x,y you pass to other tools are pixels of the LAST screenshot; a region screenshot zooms in and switches the coordinate space to that region until the next screenshot. Returns the image plus JSON metadata (monitor, image size, cursor, foreground window)."}, s.toolScreenshot)
	mcp.AddTool(srv, &mcp.Tool{Name: "monitors", Description: "List monitors with ids, physical pixel rects, DPI scale, and which one holds the cursor and the foreground window."}, s.toolMonitors)
}

func Run(ctx context.Context, d Deps, cfg config.Config, logger *log.Logger) error {
	s := New(d, cfg, logger)
	srv := mcp.NewServer(&mcp.Implementation{Name: "desktop", Version: d.Version}, nil)
	s.Register(srv)
	logger.Printf("desktop MCP server %s ready", d.Version)
	return srv.Run(ctx, &mcp.StdioTransport{})
}
