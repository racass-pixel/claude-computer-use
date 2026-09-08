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
	"github.com/racass-pixel/claude-computer-use/internal/recipes"
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
	Recipes    *recipes.Store         // nil → recipe tool returns "unsupported"
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

	// trace is a ring buffer of the last 200 actions (newest at the end).
	trace     [200]traceEntry
	traceN    int            // total entries ever written
	traceSum  string         // summary set by begin(), consumed by finish()
	traceArgs map[string]any // sanitized args set by begin(), consumed by finish()

	// Job state for recipe suggestions and auto-recording.
	taskCaption    string // set by acquire with a task
	taskApp        string // foreground process at acquire time
	recipeRanInJob bool   // true if recipe run was called in this job

	// ReleaseHook is called by the server when control is released (both explicit and idle).
	ReleaseHook func()
}

type traceEntry struct {
	Tool    string         `json:"tool"`
	Summary string         `json:"summary"`
	Args    map[string]any `json:"args,omitempty"`
	OK      bool           `json:"ok"`
	Ms      int64          `json:"ms"`
	At      time.Time      `json:"at"`
}

func New(d Deps, cfg config.Config, logger *log.Logger) *Session {
	if d.Overlay == nil {
		d.Overlay = platform.NopOverlay{}
	}
	return &Session{
		d: d, cfg: cfg, log: logger,
		actor: &actions.Actor{
			In: d.Input, Clip: d.Clip, PasteThreshold: cfg.PasteThreshold,
			GlideMs: cfg.MouseGlideMs, Pos: d.Screen.CursorPos,
		},
		elements: map[string]platform.Element{},
	}
}

// Register adds every tool. Descriptions are what the model reads — keep them precise.
func (s *Session) Register(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "screenshot", Description: "Capture the screen. Default: the active monitor (the one with the foreground window). All x,y you pass to other tools are pixels of the LAST screenshot; a region screenshot zooms in and switches the coordinate space to that region until the next screenshot. Returns the image plus JSON metadata (monitor, image size, cursor, foreground window)."}, s.toolScreenshot)
	mcp.AddTool(srv, &mcp.Tool{Name: "monitors", Description: "List monitors with ids, physical pixel rects, DPI scale, and which one holds the cursor and the foreground window."}, s.toolMonitors)
	mcp.AddTool(srv, &mcp.Tool{Name: "click", Description: "Click at x,y (pixels of the last screenshot) or on an element id from find. Supports right/middle button, double/triple click and held modifiers. Returns a screenshot after the click by default."}, s.toolClick)
	mcp.AddTool(srv, &mcp.Tool{Name: "move", Description: "Move the mouse (hover) to x,y of the last screenshot or to an element. No screenshot by default."}, s.toolMove)
	mcp.AddTool(srv, &mcp.Tool{Name: "drag", Description: "Press at from, move smoothly, release at to (drag-and-drop, selections, sliders, window moves). Coordinates are pixels of the last screenshot or element ids."}, s.toolDrag)
	mcp.AddTool(srv, &mcp.Tool{Name: "scroll", Description: "Scroll the mouse wheel at an optional x,y. dy>0 scrolls down, dx>0 scrolls right, in ticks."}, s.toolScroll)
	mcp.AddTool(srv, &mcp.Tool{Name: "type", Description: "Type text into the focused control (Unicode, any language). Long texts are pasted via the clipboard. Newlines press Enter."}, s.toolType)
	mcp.AddTool(srv, &mcp.Tool{Name: "key", Description: "Press one chord (key: \"ctrl+s\") or a sequence (keys: [\"win+r\",\"enter\"]). Names: ctrl, alt, shift, win, enter, esc, tab, space, backspace, delete, home, end, pageup, pagedown, arrows, f1-f24, letters, digits."}, s.toolKey)
	mcp.AddTool(srv, &mcp.Tool{Name: "clipboard", Description: "Read (get) or write (set) the text clipboard."}, s.toolClipboard)
	mcp.AddTool(srv, &mcp.Tool{Name: "windows", Description: "List open top-level windows: id, title, process, rect (screen px), monitor, state, is_foreground. Optional regexp filter."}, s.toolWindows)
	mcp.AddTool(srv, &mcp.Tool{Name: "window", Description: "Act on a window: focus, minimize, maximize, restore, close, move, resize. Target by id, \"foreground\", or a regexp on title/process. Use this to switch apps instead of clicking the taskbar."}, s.toolWindow)
	mcp.AddTool(srv, &mcp.Tool{Name: "find", Description: "Find UI elements by name/role via Windows UI Automation (buttons, fields, menu items, list rows) in the foreground window by default. Returns ids you can pass to click/drag/move as element:\"e3\" plus rects in the last screenshot's coordinates. Faster and more precise than guessing pixels in native apps; use vision for web pages and canvases."}, s.toolFind)
	mcp.AddTool(srv, &mcp.Tool{Name: "wait", Description: "Wait for something instead of polling with screenshots: ms (sleep), window (regexp appears), or stable (screen stops changing). Returns a screenshot when done; ok:false with timeout:true if it did not happen."}, s.toolWait)
	mcp.AddTool(srv, &mcp.Tool{Name: "pixel", Description: "Read the color of one or more pixels (last-screenshot coordinates). Use it to learn the color of a visual cue (a filled star, a badge, a status dot) before click_until."}, s.toolPixel)
	mcp.AddTool(srv, &mcp.Tool{Name: "click_until", Description: "Grind through a list without screenshots: click a point repeatedly (e.g. Deny on the top row) until a probe pixel turns (or stops being) a color, e.g. until the 5th star of the top row is yellow. Runs server-side at interval_ms per click; returns the click count and why it stopped. Then handle the matching item yourself."}, s.toolClickUntil)
	mcp.AddTool(srv, &mcp.Tool{Name: "batch", Description: "Run several actions in one call when you are confident of the sequence (e.g. click a field, type, press Enter). Steps run without screenshots; one screenshot is returned at the end. Stops at the first failure."}, s.toolBatch)
	mcp.AddTool(srv, &mcp.Tool{Name: "control", Description: "Session control: status (are you controlling / did the user pause), acquire (show the take-over overlay now; returns suggested_recipes when task is given — if one scores >= 0.5, run it with recipe run before doing the job by hand), release (hide it when the task is done; auto-records a recipe draft when the trace has enough actions), hud (set the task title the user sees)."}, s.toolControl)
	mcp.AddTool(srv, &mcp.Tool{Name: "recipe", Description: "Procedural memory. search: find a saved recipe. run: replay a recipe by slug with values for its {{params}} — fast, no screenshots between steps; reports failed steps so you can finish by hand. draft: build a recipe from the current trace (keeps action tools, parametrises long texts, adds wait steps). trace: the actions performed so far. save: store a recipe (name in the user's language, description with synonyms, params for variable text/paths, wait steps between app transitions); saving on an existing slug replaces it and resets counters. get/list/delete manage them."}, s.toolRecipe)
}

func Run(ctx context.Context, d Deps, cfg config.Config, logger *log.Logger) error {
	s := New(d, cfg, logger)
	srv := mcp.NewServer(&mcp.Implementation{Name: "desktop", Version: d.Version}, nil)
	s.Register(srv)
	logger.Printf("desktop MCP server %s ready", d.Version)
	return srv.Run(ctx, &mcp.StdioTransport{})
}
