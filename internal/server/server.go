// Package server exposes the desktop as MCP tools.
package server

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
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
	Acquire(now time.Time) bool // returns false if userHold is latched
	Touch(now time.Time)
	Release(now time.Time)
	WaitResume(ctx context.Context) bool // true when control returned to Claude before ctx ended (Paused→Controlling only)
	Status() ControllerStatus
}

type ControllerStatus struct {
	State    string `json:"state"` // idle | controlling | paused
	Hotkey   string `json:"hotkey"`
	IdleMs   int64  `json:"idle_ms"`
	UserHold bool   `json:"user_hold"`
}

type Deps struct {
	Screen         platform.Screen
	Input          platform.Input
	Clip           platform.Clipboard
	Wins           platform.Windows
	Access         platform.Accessibility // nil until Task 15 → find returns an error result
	Overlay        platform.Overlay       // nil → platform.NopOverlay{}
	Controller     Controller             // nil → never paused
	Recipes        *recipes.Store         // nil → recipe tool returns "unsupported"
	Version        string
	OnSessionReady func(*Session) // called after New, before serving; lets callers wire hooks
}

type Session struct {
	d     Deps
	cfg   config.Config
	log   *log.Logger
	actor *actions.Actor
	Now   func() time.Time // injectable clock; nil = time.Now

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

	// Held mouse button state (cross-call drags).
	heldButton platform.MouseButton // "" = no button held
	heldSince  time.Time

	// Job state for recipe suggestions and auto-recording.
	taskCaption    string // set by acquire with a task
	taskApp        string // foreground process at acquire time
	recipeRanInJob bool   // true if recipe run was called in this job
	jobRecorded    bool   // true once autoRecord saved a recipe for this job

	// Auto-release tracking (T24-2).
	autoReleased   bool                 // set by begin() when drag timeout releases a button
	releasedButton platform.MouseButton // the button that was released
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

// safeHandler wraps a typed tool handler with panic recovery (I10).
func safeHandler[In any](logger *log.Logger, h func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, any, error)) func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (res *mcp.CallToolResult, extra any, err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				logger.Printf("PANIC in tool handler: %v\n%s", r, stack)
				res = errResult("internal_error", fmt.Sprintf("internal error: %v", r))
				extra = nil
				err = nil
			}
		}()
		return h(ctx, req, in)
	}
}

// Register adds every tool. Descriptions are what the model reads — keep them precise.
func (s *Session) Register(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "screenshot", Description: "Capture the screen. Default: the active monitor (the one with the foreground window). All x,y you pass to other tools are pixels of the LAST screenshot; a region screenshot zooms in and switches the coordinate space to that region until the next screenshot. Returns the image plus JSON metadata (monitor, image size, cursor, foreground window)."}, safeHandler(s.log, s.toolScreenshot))
	mcp.AddTool(srv, &mcp.Tool{Name: "monitors", Description: "List monitors with ids, physical pixel rects, DPI scale, and which one holds the cursor and the foreground window."}, safeHandler(s.log, s.toolMonitors))
	mcp.AddTool(srv, &mcp.Tool{Name: "click", Description: "Click at x,y (pixels of the last screenshot) or on an element id from find. Supports right/middle button, double/triple click and held modifiers. Returns a screenshot after the click by default."}, safeHandler(s.log, s.toolClick))
	mcp.AddTool(srv, &mcp.Tool{Name: "move", Description: "Move the mouse (hover) to x,y of the last screenshot or to an element. No screenshot by default."}, safeHandler(s.log, s.toolMove))
	mcp.AddTool(srv, &mcp.Tool{Name: "mouse_down", Description: "Press a mouse button at x,y (or an element) and HOLD it across calls. Use for cross-window drag-and-drop: mouse_down on the file → move to the target window's taskbar button → wait{ms:1200} (Windows activates it) → move to the drop zone → mouse_up. Or Alt+Tab mid-drag. The button is auto-released on pause, idle, or timeout."}, safeHandler(s.log, s.toolMouseDown))
	mcp.AddTool(srv, &mcp.Tool{Name: "mouse_up", Description: "Release a held mouse button, optionally at x,y (omit to release at the current cursor position). Completes a cross-window drag started with mouse_down."}, safeHandler(s.log, s.toolMouseUp))
	mcp.AddTool(srv, &mcp.Tool{Name: "drag", Description: "Press at from, move smoothly, release at to (drag-and-drop, selections, sliders, window moves). Coordinates are pixels of the last screenshot or element ids. For cross-window drags, add via:[{x,y,wait_ms:1200}] to hover taskbar buttons mid-drag."}, safeHandler(s.log, s.toolDrag))
	mcp.AddTool(srv, &mcp.Tool{Name: "scroll", Description: "Scroll the mouse wheel at an optional x,y. dy>0 scrolls down, dx>0 scrolls right, in ticks."}, safeHandler(s.log, s.toolScroll))
	mcp.AddTool(srv, &mcp.Tool{Name: "type", Description: "Type text into the focused control (Unicode, any language). Long texts are pasted via the clipboard. Newlines press Enter."}, safeHandler(s.log, s.toolType))
	mcp.AddTool(srv, &mcp.Tool{Name: "key", Description: "Press one chord (key: \"ctrl+s\") or a sequence (keys: [\"win+r\",\"enter\"]). Names: ctrl, alt, shift, win, enter, esc, tab, space, backspace, delete, home, end, pageup, pagedown, arrows, f1-f24, letters, digits."}, safeHandler(s.log, s.toolKey))
	mcp.AddTool(srv, &mcp.Tool{Name: "clipboard", Description: "Read (get) or write (set) the text clipboard."}, safeHandler(s.log, s.toolClipboard))
	mcp.AddTool(srv, &mcp.Tool{Name: "windows", Description: "List open top-level windows: id, title, process, rect (screen px), monitor, state, is_foreground. Optional regexp filter."}, safeHandler(s.log, s.toolWindows))
	mcp.AddTool(srv, &mcp.Tool{Name: "window", Description: "Act on a window: focus, minimize, maximize, restore, close, move, resize. Target by id, \"foreground\", or a regexp on title/process. Use this to switch apps instead of clicking the taskbar."}, safeHandler(s.log, s.toolWindow))
	mcp.AddTool(srv, &mcp.Tool{Name: "find", Description: "Find UI elements by name/role via Windows UI Automation (buttons, fields, menu items, list rows) in the foreground window by default. Returns ids you can pass to click/drag/move as element:\"e3\" plus rects in the last screenshot's coordinates. Faster and more precise than guessing pixels in native apps; use vision for web pages and canvases."}, safeHandler(s.log, s.toolFind))
	mcp.AddTool(srv, &mcp.Tool{Name: "wait", Description: "Wait for something instead of polling with screenshots: ms (sleep), window (regexp appears), or stable (screen stops changing). Returns a screenshot when done; ok:false with timeout:true if it did not happen."}, safeHandler(s.log, s.toolWait))
	mcp.AddTool(srv, &mcp.Tool{Name: "pixel", Description: "Read the color of one or more pixels (last-screenshot coordinates). Use it to learn the color of a visual cue (a filled star, a badge, a status dot) before click_until."}, safeHandler(s.log, s.toolPixel))
	mcp.AddTool(srv, &mcp.Tool{Name: "click_until", Description: "Grind through a list without screenshots: click a point repeatedly (e.g. Deny on the top row) until a probe pixel turns (or stops being) a color, e.g. until the 5th star of the top row is yellow. Runs server-side at interval_ms per click; returns the click count and why it stopped. Then handle the matching item yourself."}, safeHandler(s.log, s.toolClickUntil))
	mcp.AddTool(srv, &mcp.Tool{Name: "batch", Description: "Run several actions in one call when you are confident of the sequence (e.g. click a field, type, press Enter). Steps run without screenshots; one screenshot is returned at the end. Stops at the first failure."}, safeHandler(s.log, s.toolBatch))
	mcp.AddTool(srv, &mcp.Tool{Name: "control", Description: "Session control: status (are you controlling / did the user pause), acquire (show the take-over overlay now; returns suggested_recipes when task is given — if one scores >= 0.5, run it with recipe run before doing the job by hand), release (hide it when the task is done; auto-records a recipe draft when the trace has enough actions), hud (set the task title the user sees)."}, safeHandler(s.log, s.toolControl))
	mcp.AddTool(srv, &mcp.Tool{Name: "recipe", Description: "Procedural memory. search: find a saved recipe. run: replay a recipe by slug with values for its {{params}} — fast, no screenshots between steps; reports failed steps so you can finish by hand. draft: build a recipe from the current trace (keeps action tools, parametrises long texts, adds wait steps). trace: the actions performed so far. save: store a recipe (name in the user's language, description with synonyms, params for variable text/paths, wait steps between app transitions); saving on an existing slug replaces it and resets counters. get/list/delete manage them."}, safeHandler(s.log, s.toolRecipe))
}

func Run(ctx context.Context, d Deps, cfg config.Config, logger *log.Logger) error {
	s := New(d, cfg, logger)
	if d.OnSessionReady != nil {
		d.OnSessionReady(s)
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "desktop", Version: d.Version}, nil)
	s.Register(srv)
	logger.Printf("desktop MCP server %s ready", d.Version)
	return srv.Run(ctx, &mcp.StdioTransport{})
}
