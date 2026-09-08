# Claude Computer Use Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Claude Code plugin (`computer-use`) whose Go MCP server `cu.exe` gives Claude fast, precise control of a Windows desktop with a glowing take-over overlay and an instant user interrupt.

**Architecture:** One pure-Go binary (`cmd/cu`) runs as a stdio MCP server per Claude Code session. Platform code (Win32 via `x/sys/windows`) sits behind `internal/platform` interfaces; pure logic (coordinate view transform, key chord parser, guard state machine, batch runner, config) is unit-tested with fakes. A dedicated OS thread owns the overlay windows and low-level input hooks. The plugin adds an `operator` agent (Sonnet), an orchestrator skill, and hooks that resume/release control.

**Tech Stack:** Go 1.26 (no cgo), `github.com/modelcontextprotocol/go-sdk/mcp` v1.2.x, `golang.org/x/sys/windows`, `golang.org/x/image` (draw, font/opentype, font/gofont), `github.com/go-ole/go-ole`, `github.com/Microsoft/go-winio`. Tests: stdlib `testing` only.

**Spec:** `docs/superpowers/specs/2026-09-08-computer-use-design.md` — read it first; every task below implements a section of it.

## Global Constraints

- Module path: `github.com/racass-pixel/claude-computer-use`. Go directive: `go 1.26`. `CGO_ENABLED=0` always; nothing in the repo may require gcc.
- Git identity for commits in this repo: `racass-pixel <racass57@gmail.com>` (already set in `.git/config`).
- Windows only for now: every file that touches Win32 has a `//go:build windows` line; every package still compiles on other OSes (interfaces, fakes and pure logic have no build tag). Windows-only commands get an `_other.go` twin with `//go:build !windows` that returns an error.
- Plugin name `computer-use`, MCP server name `desktop`. Tool names in Claude Code are `mcp__plugin_computer-use_desktop__<tool>`.
- Binary name `cu.exe`, built to `bin/cu.exe` (git-ignored) with `-trimpath -ldflags "-s -w -X main.version=<plugin.json version>"`.
- All internal coordinates are physical pixels of the virtual screen. Tool inputs/outputs use the coordinate space of the last screenshot (spec §4).
- Stdout of `cu serve` is the MCP transport. Never print to stdout in server code; all logging goes to stderr via a `*log.Logger`.
- Every tool result: one `TextContent` holding one-line JSON, optionally followed by one `ImageContent`. Expected failures are `CallToolResult{IsError:true}` with JSON `{"error":"<code>","message":"..."}`; never return a Go `error` from a tool handler for an expected failure.
- Commit after every task with a conventional message (`feat:`, `test:`, `docs:`, `chore:`) and these trailer lines:
  `Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>`
  `Claude-Session: https://claude.ai/code/session_01Sxm6N7JAYL37cRNwwAkPh4`
- Run `go build ./... && go vet ./... && go test ./...` before every commit. All three must be clean.
- Speed budget: any action tool must finish its own work (excluding waits the caller asked for) in under 100 ms on a 1080p monitor; log timing on stderr as `tool=<name> ms=<n>`.

## File Structure

```
go.mod / go.sum
cmd/cu/main.go                      subcommand dispatch, version
cmd/cu/cmd_serve.go                 `cu serve` — builds Deps and runs the MCP server
cmd/cu/cmd_ctl.go                   `cu ctl status|resume|release|pause [--quiet]`
cmd/cu/cmd_doctor.go                `cu doctor` — environment report
cmd/cu/cmd_demo.go                  `cu demo` — show overlay for N seconds
cmd/cu/cmd_screenshot.go            `cu screenshot [-m N] [-o file]` dev helper
cmd/cu/cmd_input.go                 `cu input move|click|type|key ...` dev helper
internal/geom/geom.go               Point, Size, Rect + tests
internal/platform/platform.go       interfaces + value types (no Win32 here)
internal/platform/fake/fake.go      in-memory fakes recording calls (tests)
internal/win/dll.go                 lazy DLL procs, RECT/POINT, error helpers
internal/win/dpi.go                 SetPerMonitorDPIAwareV2, MonitorDPI
internal/win/monitors.go            EnumMonitors, GetCursorPos, VirtualScreenRect
internal/win/capture.go             CaptureRect (BitBlt → *image.RGBA)
internal/win/input.go               SendInput structs, mouse/keyboard senders
internal/win/clipboard.go           Get/SetClipboardText
internal/win/window.go              EnumTopLevelWindows, foreground, focus, state, move, close
internal/win/hooks.go               WH_KEYBOARD_LL / WH_MOUSE_LL
internal/win/layered.go             layered windows, UpdateLayeredWindow, display affinity
internal/win/process.go             parent PID, wait for parent exit, UI language
internal/screen/monitors.go         sortAndNumber, ActiveMonitor, VirtualScreen, MonitorByID
internal/screen/monitors_windows.go ListMonitors
internal/screen/view.go             View transform (+ tests)
internal/screen/capture.go          Scale, Encode, Grab (+ tests)
internal/screen/screen_windows.go   platform.Screen impl
internal/input/keys.go              VK table, ParseChord (+ tests)
internal/input/input_windows.go     platform.Input impl over win.SendInput
internal/input/clipboard_windows.go platform.Clipboard impl
internal/actions/actions.go         Click/Drag/Scroll/Type/Chord orchestration (+ tests with fakes)
internal/window/window_windows.go   platform.Windows impl
internal/config/config.go           Config, Default, Load (+ tests)
internal/guard/hotkey.go            Hotkey parse/format, TapMatcher (+ tests)
internal/guard/machine.go           state machine (+ tests)
internal/guard/runner_windows.go    wires win hooks → machine
internal/overlay/overlay.go         public API, state, animation loop
internal/overlay/uithread_windows.go locked OS thread + dispatcher
internal/overlay/border.go          strip rendering (pure)
internal/overlay/hud.go             pill rendering, text, localization (pure)
internal/overlay/ripple.go          click ripple rendering (pure)
internal/overlay/draw.go            SDF rounded rect, premultiply helpers (+ tests)
internal/overlay/overlay_windows.go window plumbing
internal/uia/uia_windows.go         Accessibility impl (COM, dedicated thread)
internal/uia/com_windows.go         raw vtable calls, constants
internal/uia/roles.go               role name ↔ control type id (+ tests)
internal/ipc/pipe_windows.go        named-pipe server
internal/ipc/client_windows.go      client: broadcast command to all servers
internal/server/server.go           Deps, Session, New, Run, registration
internal/server/respond.go          result/err helpers, Meta
internal/server/session.go          view state, begin/finish action, pause semantics
internal/server/tools_screen.go     screenshot, monitors
internal/server/tools_mouse.go      click, move, drag, scroll
internal/server/tools_keyboard.go   type, key, clipboard
internal/server/tools_window.go     windows, window
internal/server/tools_find.go       find (UIA)
internal/server/tools_wait.go       wait
internal/server/tools_batch.go      batch (+ tests)
internal/server/tools_control.go    control
.claude-plugin/plugin.json, .claude-plugin/marketplace.json, .mcp.json
agents/operator.md
skills/computer-use/SKILL.md, skills/doctor/SKILL.md, skills/demo/SKILL.md
hooks/hooks.json
bin/cu.cmd, scripts/build.ps1, scripts/install.ps1
.github/workflows/ci.yml, .github/workflows/release.yml, .goreleaser.yaml
README.md, README.ru.md, LICENSE, CHANGELOG.md, .gitignore
docs/superpowers/specs/2026-09-08-computer-use-design.md (exists)
```

---

### Task 1: Repository scaffold, plugin manifest, launcher, CI

**Files:**
- Create: `go.mod`, `.gitignore`, `LICENSE`, `CHANGELOG.md`, `README.md` (stub)
- Create: `cmd/cu/main.go`, `cmd/cu/stubs.go`
- Create: `internal/geom/geom.go`, `internal/geom/geom_test.go`
- Create: `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `.mcp.json`
- Create: `bin/cu.cmd`, `scripts/build.ps1`, `scripts/install.ps1`, `.github/workflows/ci.yml`

**Interfaces:**
- Produces: package `geom` with `Point{X,Y int}`, `Size{W,H int}`, `Rect{X,Y,W,H int}` and methods `Right()`, `Bottom()`, `Center() Point`, `Contains(Point) bool`, `Intersect(Rect) Rect`, `Empty() bool`, `Union(Rect) Rect`, `Size() Size`, `Point.Add(Point) Point`. `cmd/cu` dispatches `version|serve|ctl|doctor|demo|screenshot|input` to `runServe(args []string) error` etc.

- [ ] **Step 1: Write the failing geom test**

`internal/geom/geom_test.go`:
```go
package geom

import "testing"

func TestRectContainsAndIntersect(t *testing.T) {
	r := Rect{X: 10, Y: 20, W: 100, H: 50}
	if !r.Contains(Point{10, 20}) || r.Contains(Point{110, 20}) || r.Contains(Point{50, 70}) {
		t.Fatalf("Contains is wrong: half-open [X, X+W) x [Y, Y+H) expected")
	}
	if got := r.Center(); got != (Point{60, 45}) {
		t.Fatalf("Center = %v, want {60 45}", got)
	}
	got := r.Intersect(Rect{X: 50, Y: 0, W: 100, H: 30})
	if got != (Rect{X: 50, Y: 20, W: 60, H: 10}) {
		t.Fatalf("Intersect = %v", got)
	}
	if !r.Intersect(Rect{X: 500, Y: 500, W: 10, H: 10}).Empty() {
		t.Fatalf("disjoint rects must intersect to an empty rect")
	}
	u := r.Union(Rect{X: -5, Y: 30, W: 10, H: 100})
	if u != (Rect{X: -5, Y: 20, W: 115, H: 110}) {
		t.Fatalf("Union = %v", u)
	}
}
```

- [ ] **Step 2: Create go.mod and run the test to see it fail**

`go.mod`:
```
module github.com/racass-pixel/claude-computer-use

go 1.26
```
Run: `go test ./internal/geom/`
Expected: FAIL (undefined: Rect).

- [ ] **Step 3: Implement geom**

`internal/geom/geom.go`:
```go
// Package geom holds the integer geometry types shared by every package.
// Values are physical pixels unless a function says otherwise.
package geom

type Point struct{ X, Y int }
type Size struct{ W, H int }
type Rect struct{ X, Y, W, H int }

func (p Point) Add(o Point) Point { return Point{p.X + o.X, p.Y + o.Y} }

func (r Rect) Right() int    { return r.X + r.W }
func (r Rect) Bottom() int   { return r.Y + r.H }
func (r Rect) Empty() bool   { return r.W <= 0 || r.H <= 0 }
func (r Rect) Center() Point { return Point{r.X + r.W/2, r.Y + r.H/2} }
func (r Rect) Size() Size    { return Size{r.W, r.H} }

func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.X < r.Right() && p.Y >= r.Y && p.Y < r.Bottom()
}

func (r Rect) Intersect(o Rect) Rect {
	x0, y0 := max(r.X, o.X), max(r.Y, o.Y)
	x1, y1 := min(r.Right(), o.Right()), min(r.Bottom(), o.Bottom())
	if x1 <= x0 || y1 <= y0 {
		return Rect{}
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

func (r Rect) Union(o Rect) Rect {
	if r.Empty() {
		return o
	}
	if o.Empty() {
		return r
	}
	x0, y0 := min(r.X, o.X), min(r.Y, o.Y)
	x1, y1 := max(r.Right(), o.Right()), max(r.Bottom(), o.Bottom())
	return Rect{x0, y0, x1 - x0, y1 - y0}
}
```
Run: `go test ./internal/geom/` → PASS.

- [ ] **Step 4: Create cmd/cu with subcommand dispatch and stubs**

`cmd/cu/main.go`:
```go
// cu is the Claude Computer Use binary: MCP server, control CLI and dev helpers.
package main

import (
	"fmt"
	"os"
)

// version is injected at build time: -ldflags "-X main.version=0.1.0".
var version = "dev"

func usage() {
	fmt.Fprintln(os.Stderr, `usage: cu <command> [args]

commands:
  serve                      run the MCP server over stdio (used by Claude Code)
  ctl status|resume|release|pause [--quiet]
  doctor                     report monitors, DPI, privileges, capture speed
  demo [-seconds N]          show the overlay for N seconds (default 5)
  screenshot [-m N] [-o file.png]
  input move X Y | click X Y [button] | type TEXT | key CHORD
  version`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	args := os.Args[2:]
	var err error
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println(version)
	case "serve":
		err = runServe(args)
	case "ctl":
		err = runCtl(args)
	case "doctor":
		err = runDoctor(args)
	case "demo":
		err = runDemo(args)
	case "screenshot":
		err = runScreenshot(args)
	case "input":
		err = runInput(args)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "cu:", err)
		os.Exit(1)
	}
}
```

`cmd/cu/stubs.go` (later tasks delete one stub each when they add the real `cmd_<name>.go`):
```go
package main

import "errors"

var errNotImplemented = errors.New("not implemented yet")

func runServe(args []string) error      { return errNotImplemented }
func runCtl(args []string) error        { return errNotImplemented }
func runDoctor(args []string) error     { return errNotImplemented }
func runDemo(args []string) error       { return errNotImplemented }
func runScreenshot(args []string) error { return errNotImplemented }
func runInput(args []string) error      { return errNotImplemented }
```
Run: `go build ./... && go run ./cmd/cu version` → prints `dev`.

- [ ] **Step 5: Plugin manifest, marketplace, .mcp.json**

`.claude-plugin/plugin.json`:
```json
{
  "name": "computer-use",
  "displayName": "Computer Use",
  "version": "0.1.0",
  "description": "Full desktop control for Claude Code on Windows: mouse, keyboard, windows, multi-monitor, UI Automation, a glowing take-over overlay and an instant user interrupt (Esc Esc).",
  "author": { "name": "racass-pixel", "url": "https://github.com/racass-pixel" },
  "homepage": "https://github.com/racass-pixel/claude-computer-use",
  "repository": "https://github.com/racass-pixel/claude-computer-use",
  "license": "MIT",
  "keywords": ["computer-use", "desktop", "automation", "windows", "mcp", "gui", "operator"]
}
```

`.claude-plugin/marketplace.json`:
```json
{
  "name": "claude-computer-use",
  "owner": { "name": "racass-pixel", "url": "https://github.com/racass-pixel" },
  "plugins": [
    {
      "name": "computer-use",
      "source": "./",
      "description": "Full desktop control for Claude Code on Windows with a glowing take-over overlay and instant user interrupt."
    }
  ]
}
```

`.mcp.json`:
```json
{
  "mcpServers": {
    "desktop": {
      "type": "stdio",
      "command": "cmd",
      "args": ["/c", "${CLAUDE_PLUGIN_ROOT}/bin/cu.cmd", "serve"]
    }
  }
}
```

- [ ] **Step 6: Launcher, build script, CI, misc files**

`bin/cu.cmd`:
```bat
@echo off
setlocal
set "EXE=%~dp0cu.exe"
if not exist "%EXE%" (
  powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0..\scripts\install.ps1" 1>&2
)
if not exist "%EXE%" (
  echo cu: bin\cu.exe is missing and install failed. Run scripts\build.ps1 with Go installed. 1>&2
  exit /b 1
)
"%EXE%" %*
```

`scripts/build.ps1`:
```powershell
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$manifest = Get-Content (Join-Path $root ".claude-plugin\plugin.json") -Raw | ConvertFrom-Json
$version = $manifest.version
$env:CGO_ENABLED = "0"
$out = Join-Path $root "bin\cu.exe"
& go build -trimpath -ldflags "-s -w -X main.version=$version" -o $out "$root/cmd/cu"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "built $out ($version)"
```

`scripts/install.ps1` (build-only until Task 17 adds the release download):
```powershell
$ErrorActionPreference = "Stop"
if (Get-Command go -ErrorAction SilentlyContinue) {
  & (Join-Path $PSScriptRoot "build.ps1")
} else {
  Write-Error "Go is not installed and no release download is configured yet."
}
```

`.gitignore`:
```
bin/cu.exe
bin/*.zip
dist/
*.log
.DS_Store
```

`.github/workflows/ci.yml`:
```yaml
name: ci
on: [push, pull_request]
jobs:
  test:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26"
      - run: go vet ./...
      - run: go test ./...
      - run: go build -trimpath -ldflags "-s -w" -o bin/cu.exe ./cmd/cu
        env:
          CGO_ENABLED: "0"
```

`LICENSE`: the MIT license text with `Copyright (c) 2026 racass-pixel`. `CHANGELOG.md`: a `# Changelog` title and a `## Unreleased` heading. `README.md`: `# Claude Computer Use` and one line `Work in progress — see docs/superpowers/specs/2026-09-08-computer-use-design.md.`

- [ ] **Step 7: Validate the plugin shape and commit**

Run: `powershell -NoProfile -File scripts/build.ps1` → `bin/cu.exe` exists; `bin\cu.cmd version` prints `0.1.0`.
Run: `claude plugin validate .` → no errors (warnings about missing skills are fine).
Run: `go build ./... && go vet ./... && go test ./...` → clean.
```bash
git add -A
git commit -m "chore: scaffold cu binary, plugin manifest, launcher and CI"
```

---

### Task 2: Platform interfaces and fakes

**Files:**
- Create: `internal/platform/platform.go`, `internal/platform/fake/fake.go`

**Interfaces:**
- Produces (used by every later task; copy names exactly):

```go
package platform

import (
	"image"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
)

type Monitor struct {
	ID          int       `json:"id"`           // 1-based; primary first, then left-to-right, top-to-bottom
	Name        string    `json:"name"`         // e.g. `\\.\DISPLAY1`
	Rect        geom.Rect `json:"rect"`         // physical pixels, virtual-screen coordinates
	Work        geom.Rect `json:"work"`         // minus taskbar
	ScaleFactor float64   `json:"scale_factor"` // DPI / 96
	Primary     bool      `json:"primary"`
}

type WindowState string

const (
	WindowNormal    WindowState = "normal"
	WindowMinimized WindowState = "minimized"
	WindowMaximized WindowState = "maximized"
)

type WindowInfo struct {
	ID         uintptr     `json:"id"` // HWND
	Title      string      `json:"title"`
	Process    string      `json:"process"` // "notepad.exe"
	PID        uint32      `json:"pid"`
	Rect       geom.Rect   `json:"rect"` // visible frame bounds
	State      WindowState `json:"state"`
	Foreground bool        `json:"is_foreground"`
}

type MouseButton string

const (
	ButtonLeft   MouseButton = "left"
	ButtonRight  MouseButton = "right"
	ButtonMiddle MouseButton = "middle"
)

// Screen reads monitors and pixels.
type Screen interface {
	Monitors() ([]Monitor, error)
	Capture(r geom.Rect) (*image.RGBA, error) // r in screen coordinates
	CursorPos() (geom.Point, error)
}

// Input injects mouse and keyboard events. vk are Windows virtual-key codes.
type Input interface {
	MouseMove(p geom.Point) error
	MouseDown(b MouseButton) error
	MouseUp(b MouseButton) error
	Scroll(dx, dy int) error // wheel ticks; dy>0 scrolls down, dx>0 scrolls right
	KeyDown(vk uint16) error
	KeyUp(vk uint16) error
	TypeUnicode(s string) error // types s as unicode key events; \n → Enter, \t → Tab
}

type Clipboard interface {
	GetText() (string, error)
	SetText(s string) error
}

type Windows interface {
	List() ([]WindowInfo, error) // visible, titled, non-tool top-level windows
	Foreground() (WindowInfo, error)
	Focus(id uintptr) error
	SetState(id uintptr, s WindowState) error
	Close(id uintptr) error
	Move(id uintptr, r geom.Rect) error
}

// Element is a UI Automation element snapshot. Ref is an opaque handle valid until Release.
type Element struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Role         string    `json:"role"` // "Button", "Edit", "MenuItem", ...
	AutomationID string    `json:"automation_id,omitempty"`
	Value        string    `json:"value,omitempty"`
	Rect         geom.Rect `json:"-"` // screen coordinates
	Enabled      bool      `json:"enabled"`
	Focused      bool      `json:"focused"`
	Offscreen    bool      `json:"offscreen,omitempty"`
	Ref          any       `json:"-"`
}

type FindQuery struct {
	Name         string  // case-insensitive regexp on Name; empty = any
	Role         string  // control type name; empty = any
	AutomationID string  // exact; empty = any
	Window       uintptr // HWND to search under; 0 = whole desktop
	Limit        int     // max results (default 25)
}

type Accessibility interface {
	Find(q FindQuery) ([]Element, error)
	Rect(ref any) (geom.Rect, error) // fresh bounding rect for a Ref from Find
	Release(refs []any)
}

type OverlayState int

const (
	OverlayHidden OverlayState = iota
	OverlayControlling
	OverlayPaused
)

type Overlay interface {
	Show(m Monitor, s OverlayState)
	Hide()
	SetTitle(title string)   // task title line; empty = default localized title
	SetAction(action string) // "click 640,412"; shown after the hotkey hint
	Ripple(p geom.Point)     // screen coordinates
	Close()
}

// NopOverlay is used when the overlay is disabled or not yet implemented.
type NopOverlay struct{}

func (NopOverlay) Show(Monitor, OverlayState) {}
func (NopOverlay) Hide()                      {}
func (NopOverlay) SetTitle(string)            {}
func (NopOverlay) SetAction(string)           {}
func (NopOverlay) Ripple(geom.Point)          {}
func (NopOverlay) Close()                     {}
```

- [ ] **Step 1: Write platform.go exactly as above**

- [ ] **Step 2: Write the fakes**

`internal/platform/fake/fake.go`:
```go
// Package fake provides in-memory platform implementations for tests.
package fake

import (
	"fmt"
	"image"
	"image/color"
	"regexp"
	"sync"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

// Input records every call as a string like "move 10,20", "down left", "key_down 17", "type hello".
type Input struct {
	mu     sync.Mutex
	Calls  []string
	Cursor geom.Point
}

func (f *Input) record(s string) { f.mu.Lock(); f.Calls = append(f.Calls, s); f.mu.Unlock() }
func (f *Input) MouseMove(p geom.Point) error {
	f.Cursor = p
	f.record(fmt.Sprintf("move %d,%d", p.X, p.Y))
	return nil
}
func (f *Input) MouseDown(b platform.MouseButton) error { f.record("down " + string(b)); return nil }
func (f *Input) MouseUp(b platform.MouseButton) error   { f.record("up " + string(b)); return nil }
func (f *Input) Scroll(dx, dy int) error                { f.record(fmt.Sprintf("scroll %d,%d", dx, dy)); return nil }
func (f *Input) KeyDown(vk uint16) error                { f.record(fmt.Sprintf("key_down %d", vk)); return nil }
func (f *Input) KeyUp(vk uint16) error                  { f.record(fmt.Sprintf("key_up %d", vk)); return nil }
func (f *Input) TypeUnicode(s string) error             { f.record("type " + s); return nil }

// Screen serves a fixed monitor list and a solid-color image for any capture.
type Screen struct {
	Mons   []platform.Monitor
	Cursor geom.Point
	Fill   color.RGBA
	Frames [][]byte // optional: successive Capture calls return these raw RGBA pix if set
	n      int
}

func (s *Screen) Monitors() ([]platform.Monitor, error) { return s.Mons, nil }
func (s *Screen) CursorPos() (geom.Point, error)         { return s.Cursor, nil }
func (s *Screen) Capture(r geom.Rect) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, r.W, r.H))
	if s.n < len(s.Frames) {
		copy(img.Pix, s.Frames[s.n])
		s.n++
		return img, nil
	}
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = s.Fill.R, s.Fill.G, s.Fill.B, 255
	}
	return img, nil
}

type Clipboard struct{ Text string }

func (c *Clipboard) GetText() (string, error) { return c.Text, nil }
func (c *Clipboard) SetText(s string) error   { c.Text = s; return nil }

type Windows struct {
	Wins  []platform.WindowInfo
	Calls []string
}

func (w *Windows) List() ([]platform.WindowInfo, error) { return w.Wins, nil }
func (w *Windows) Foreground() (platform.WindowInfo, error) {
	for _, x := range w.Wins {
		if x.Foreground {
			return x, nil
		}
	}
	return platform.WindowInfo{}, fmt.Errorf("no foreground window")
}
func (w *Windows) Focus(id uintptr) error {
	for i := range w.Wins {
		w.Wins[i].Foreground = w.Wins[i].ID == id
	}
	w.Calls = append(w.Calls, fmt.Sprintf("focus %d", id))
	return nil
}
func (w *Windows) SetState(id uintptr, s platform.WindowState) error {
	w.Calls = append(w.Calls, fmt.Sprintf("state %d %s", id, s))
	return nil
}
func (w *Windows) Close(id uintptr) error { w.Calls = append(w.Calls, fmt.Sprintf("close %d", id)); return nil }
func (w *Windows) Move(id uintptr, r geom.Rect) error {
	w.Calls = append(w.Calls, fmt.Sprintf("move %d %v", id, r))
	return nil
}

type Accessibility struct{ Elems []platform.Element }

func (a *Accessibility) Find(q platform.FindQuery) ([]platform.Element, error) {
	var re *regexp.Regexp
	if q.Name != "" {
		re = regexp.MustCompile("(?i)" + q.Name)
	}
	var out []platform.Element
	for _, e := range a.Elems {
		if re != nil && !re.MatchString(e.Name) {
			continue
		}
		if q.Role != "" && q.Role != e.Role {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}
func (a *Accessibility) Rect(ref any) (geom.Rect, error) { return ref.(geom.Rect), nil }
func (a *Accessibility) Release([]any)                    {}

// Overlay records calls.
type Overlay struct{ Calls []string }

func (o *Overlay) Show(m platform.Monitor, s platform.OverlayState) {
	o.Calls = append(o.Calls, fmt.Sprintf("show %d %d", m.ID, s))
}
func (o *Overlay) Hide()               { o.Calls = append(o.Calls, "hide") }
func (o *Overlay) SetTitle(t string)   { o.Calls = append(o.Calls, "title "+t) }
func (o *Overlay) SetAction(a string)  { o.Calls = append(o.Calls, "action "+a) }
func (o *Overlay) Ripple(p geom.Point) { o.Calls = append(o.Calls, fmt.Sprintf("ripple %d,%d", p.X, p.Y)) }
func (o *Overlay) Close()              { o.Calls = append(o.Calls, "close") }

var (
	_ platform.Input         = (*Input)(nil)
	_ platform.Screen        = (*Screen)(nil)
	_ platform.Clipboard     = (*Clipboard)(nil)
	_ platform.Windows       = (*Windows)(nil)
	_ platform.Accessibility = (*Accessibility)(nil)
	_ platform.Overlay       = (*Overlay)(nil)
)
```

- [ ] **Step 3: Build and commit**

Run: `go build ./... && go vet ./...` → clean.
```bash
git add -A && git commit -m "feat: platform interfaces and test fakes"
```

---

### Task 3: Win32 base, DPI awareness, monitors, cursor, first `cu doctor`

**Files:**
- Create: `internal/win/dll.go`, `internal/win/dpi.go`, `internal/win/monitors.go`
- Create: `internal/screen/monitors.go`, `internal/screen/monitors_windows.go`, `internal/screen/monitors_test.go`
- Create: `cmd/cu/cmd_doctor.go`, `cmd/cu/cmd_doctor_other.go`; Modify: `cmd/cu/stubs.go` (remove `runDoctor`)

**Interfaces:**
- Produces: `win.SetPerMonitorDPIAwareV2() error`, `win.MonitorDPI(h uintptr) uint32`, `win.EnumMonitors() ([]win.MonitorInfo, error)` with `MonitorInfo{Handle uintptr; Rect, Work RECT; Primary bool; Device string; DPI uint32}`, `win.GetCursorPos() (POINT, error)`, `win.VirtualScreenRect() RECT`, `win.RECT{Left,Top,Right,Bottom int32}` with `Width()`, `Height()`, `win.POINT{X,Y int32}`, `win.callErr(name string, r uintptr, e error) error`; `screen.ListMonitors() ([]platform.Monitor, error)`, `screen.ActiveMonitor(mons []platform.Monitor, fg geom.Rect, cursor geom.Point) platform.Monitor`, `screen.VirtualScreen(mons []platform.Monitor) geom.Rect`, `screen.MonitorByID(mons []platform.Monitor, id int) (platform.Monitor, bool)`, `screen.rectOf(win.RECT) geom.Rect` (windows only).

- [ ] **Step 1: Write the failing test for ActiveMonitor / VirtualScreen**

`internal/screen/monitors_test.go`:
```go
package screen

import (
	"testing"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

var twoMons = []platform.Monitor{
	{ID: 1, Rect: geom.Rect{X: 0, Y: 0, W: 1920, H: 1080}, Primary: true},
	{ID: 2, Rect: geom.Rect{X: 1920, Y: -200, W: 2560, H: 1440}},
}

func TestActiveMonitorPrefersForegroundWindowCenter(t *testing.T) {
	fg := geom.Rect{X: 2000, Y: 100, W: 800, H: 600} // center on monitor 2
	if m := ActiveMonitor(twoMons, fg, geom.Point{X: 10, Y: 10}); m.ID != 2 {
		t.Fatalf("got monitor %d, want 2", m.ID)
	}
}

func TestActiveMonitorFallsBackToCursorThenPrimary(t *testing.T) {
	if m := ActiveMonitor(twoMons, geom.Rect{}, geom.Point{X: 3000, Y: 300}); m.ID != 2 {
		t.Fatalf("cursor fallback: got %d, want 2", m.ID)
	}
	if m := ActiveMonitor(twoMons, geom.Rect{}, geom.Point{X: -9999, Y: -9999}); m.ID != 1 {
		t.Fatalf("primary fallback: got %d, want 1", m.ID)
	}
}

func TestVirtualScreenIsUnion(t *testing.T) {
	if got := VirtualScreen(twoMons); got != (geom.Rect{X: 0, Y: -200, W: 4480, H: 1640}) {
		t.Fatalf("VirtualScreen = %v", got)
	}
}

func TestSortAndNumberPutsPrimaryFirst(t *testing.T) {
	mons := sortAndNumber([]platform.Monitor{
		{Name: "B", Rect: geom.Rect{X: -1920, Y: 0, W: 1920, H: 1080}},
		{Name: "A", Rect: geom.Rect{X: 0, Y: 0, W: 1920, H: 1080}, Primary: true},
	})
	if mons[0].Name != "A" || mons[0].ID != 1 || mons[1].ID != 2 {
		t.Fatalf("bad order: %+v", mons)
	}
}
```
Run: `go test ./internal/screen/` → FAIL (undefined).

- [ ] **Step 2: Win32 plumbing**

Run `go get golang.org/x/sys/windows@latest`.

`internal/win/dll.go`:
```go
//go:build windows

// Package win contains thin, dependency-free bindings to the Win32 APIs used by cu.
package win

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	shcore   = windows.NewLazySystemDLL("shcore.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
)

type RECT struct{ Left, Top, Right, Bottom int32 }
type POINT struct{ X, Y int32 }

func (r RECT) Width() int32  { return r.Right - r.Left }
func (r RECT) Height() int32 { return r.Bottom - r.Top }

// callErr converts a zero return + errno into a Go error carrying the API name.
func callErr(name string, r uintptr, e error) error {
	if r != 0 {
		return nil
	}
	if en, ok := e.(syscall.Errno); ok && en == 0 {
		return fmt.Errorf("%s failed", name)
	}
	return fmt.Errorf("%s: %w", name, e)
}
```

`internal/win/dpi.go`:
```go
//go:build windows

package win

import (
	"syscall"
	"unsafe"
)

var (
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procGetDpiForMonitor              = shcore.NewProc("GetDpiForMonitor")
)

// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 == (DPI_AWARENESS_CONTEXT)-4
const dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)

// SetPerMonitorDPIAwareV2 must run before any window or monitor API. Safe to call twice.
func SetPerMonitorDPIAwareV2() error {
	r, _, e := procSetProcessDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
	if r == 0 {
		if en, ok := e.(syscall.Errno); ok && en == syscall.ERROR_ACCESS_DENIED {
			return nil // already set for this process
		}
		return callErr("SetProcessDpiAwarenessContext", r, e)
	}
	return nil
}

// MonitorDPI returns the effective DPI of a monitor (96 = 100%).
func MonitorDPI(hMonitor uintptr) uint32 {
	var x, y uint32
	procGetDpiForMonitor.Call(hMonitor, 0 /* MDT_EFFECTIVE_DPI */, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)))
	if x == 0 {
		return 96
	}
	return x
}
```

`internal/win/monitors.go`:
```go
//go:build windows

package win

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
)

type monitorInfoEx struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
	SzDevice  [32]uint16
}

type MonitorInfo struct {
	Handle  uintptr
	Rect    RECT
	Work    RECT
	Primary bool
	Device  string
	DPI     uint32
}

// One callback for the process lifetime: windows.NewCallback must not be called repeatedly.
var (
	enumMu       sync.Mutex
	enumOut      []MonitorInfo
	enumCallback = windows.NewCallback(func(hMonitor, hdc uintptr, rc *RECT, lparam uintptr) uintptr {
		var mi monitorInfoEx
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		if r, _, _ := procGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&mi))); r == 0 {
			return 1
		}
		enumOut = append(enumOut, MonitorInfo{
			Handle:  hMonitor,
			Rect:    mi.RcMonitor,
			Work:    mi.RcWork,
			Primary: mi.DwFlags&1 != 0, // MONITORINFOF_PRIMARY
			Device:  windows.UTF16ToString(mi.SzDevice[:]),
			DPI:     MonitorDPI(hMonitor),
		})
		return 1
	})
)

// EnumMonitors lists all display monitors in physical pixels.
func EnumMonitors() ([]MonitorInfo, error) {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumOut = nil
	r, _, e := procEnumDisplayMonitors.Call(0, 0, enumCallback, 0)
	if err := callErr("EnumDisplayMonitors", r, e); err != nil {
		return nil, err
	}
	out := make([]MonitorInfo, len(enumOut))
	copy(out, enumOut)
	return out, nil
}

func GetCursorPos() (POINT, error) {
	var p POINT
	r, _, e := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p, callErr("GetCursorPos", r, e)
}

// VirtualScreenRect returns the bounding box of all monitors (SM_*VIRTUALSCREEN).
func VirtualScreenRect() RECT {
	x, _, _ := procGetSystemMetrics.Call(76)
	y, _, _ := procGetSystemMetrics.Call(77)
	w, _, _ := procGetSystemMetrics.Call(78)
	h, _, _ := procGetSystemMetrics.Call(79)
	return RECT{int32(x), int32(y), int32(x) + int32(w), int32(y) + int32(h)}
}
```

- [ ] **Step 3: screen.ListMonitors and pure helpers**

`internal/screen/monitors.go` (no build tag):
```go
// Package screen implements platform.Screen and the screenshot view transform.
package screen

import (
	"sort"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

// sortAndNumber orders monitors primary-first, then by X then Y, and assigns IDs.
func sortAndNumber(mons []platform.Monitor) []platform.Monitor {
	sort.SliceStable(mons, func(i, j int) bool {
		if mons[i].Primary != mons[j].Primary {
			return mons[i].Primary
		}
		if mons[i].Rect.X != mons[j].Rect.X {
			return mons[i].Rect.X < mons[j].Rect.X
		}
		return mons[i].Rect.Y < mons[j].Rect.Y
	})
	for i := range mons {
		mons[i].ID = i + 1
	}
	return mons
}

func MonitorByID(mons []platform.Monitor, id int) (platform.Monitor, bool) {
	for _, m := range mons {
		if m.ID == id {
			return m, true
		}
	}
	return platform.Monitor{}, false
}

// ActiveMonitor picks the monitor containing the foreground window center, else the cursor, else primary.
func ActiveMonitor(mons []platform.Monitor, fg geom.Rect, cursor geom.Point) platform.Monitor {
	if len(mons) == 0 {
		return platform.Monitor{}
	}
	if !fg.Empty() {
		c := fg.Center()
		for _, m := range mons {
			if m.Rect.Contains(c) {
				return m
			}
		}
	}
	for _, m := range mons {
		if m.Rect.Contains(cursor) {
			return m
		}
	}
	for _, m := range mons {
		if m.Primary {
			return m
		}
	}
	return mons[0]
}

func VirtualScreen(mons []platform.Monitor) geom.Rect {
	var u geom.Rect
	for _, m := range mons {
		u = u.Union(m.Rect)
	}
	return u
}
```

`internal/screen/monitors_windows.go`:
```go
//go:build windows

package screen

import (
	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

func rectOf(r win.RECT) geom.Rect {
	return geom.Rect{X: int(r.Left), Y: int(r.Top), W: int(r.Width()), H: int(r.Height())}
}

// ListMonitors enumerates monitors, primary first, IDs 1..n.
func ListMonitors() ([]platform.Monitor, error) {
	raw, err := win.EnumMonitors()
	if err != nil {
		return nil, err
	}
	mons := make([]platform.Monitor, 0, len(raw))
	for _, m := range raw {
		mons = append(mons, platform.Monitor{
			Name: m.Device, Rect: rectOf(m.Rect), Work: rectOf(m.Work),
			ScaleFactor: float64(m.DPI) / 96, Primary: m.Primary,
		})
	}
	return sortAndNumber(mons), nil
}
```
Run: `go test ./internal/screen/` → PASS.

- [ ] **Step 4: First `cu doctor`**

`cmd/cu/cmd_doctor.go` (delete `runDoctor` from `stubs.go`):
```go
//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/racass-pixel/claude-computer-use/internal/screen"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

func runDoctor(args []string) error {
	fmt.Printf("cu %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
	if err := win.SetPerMonitorDPIAwareV2(); err != nil {
		fmt.Println("dpi awareness: FAIL", err)
	} else {
		fmt.Println("dpi awareness: per-monitor v2")
	}
	mons, err := screen.ListMonitors()
	if err != nil {
		return err
	}
	fmt.Printf("monitors: %d\n", len(mons))
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(mons); err != nil {
		return err
	}
	if p, err := win.GetCursorPos(); err == nil {
		fmt.Printf("cursor: %d,%d\n", p.X, p.Y)
	}
	return nil
}
```
`cmd/cu/cmd_doctor_other.go`:
```go
//go:build !windows

package main

import "errors"

func runDoctor(args []string) error { return errors.New("cu doctor is Windows-only") }
```
Run: `go run ./cmd/cu doctor` → lists your monitors with physical sizes and scale factors that match Settings → Display; the primary monitor has ID 1.

- [ ] **Step 5: Commit**

```bash
go build ./... && go vet ./... && go test ./...
git add -A && git commit -m "feat: win32 base, DPI awareness, monitor enumeration, cu doctor"
```

---

### Task 4: Screen capture, scaling, encoding, View transform, `cu screenshot`

**Files:**
- Create: `internal/win/capture.go`
- Create: `internal/screen/view.go`, `internal/screen/view_test.go`, `internal/screen/capture.go`, `internal/screen/capture_test.go`, `internal/screen/screen_windows.go`
- Create: `cmd/cu/cmd_screenshot.go`, `cmd/cu/cmd_screenshot_other.go`; Modify: `cmd/cu/stubs.go` (remove `runScreenshot`)

**Interfaces:**
- Consumes: `win.RECT`, `win.callErr`, `screen.ListMonitors`, `platform.Screen`.
- Produces:
  - `win.CaptureRect(x, y, w, h int) (*image.RGBA, error)`; package var `win.IncludeLayeredWindows = true` (adds `CAPTUREBLT`; Task 12 may flip it).
  - `screen.View{Monitor int; Offset geom.Point; Scale float64; Image geom.Size; Screen geom.Rect}` with `NewView(monitor int, screenRect geom.Rect, scale float64) View`, `(View) ToScreen(geom.Point) geom.Point`, `(View) ToImage(geom.Point) geom.Point`, `(View) RectToImage(geom.Rect) geom.Rect`, `(View) RectToScreen(geom.Rect) geom.Rect`, `(View) ClampImage(geom.Point) geom.Point`, `screen.AutoScale(r geom.Rect, longEdge int) float64`, `screen.ZoomScale(r geom.Rect, longEdge int, maxZoom float64) float64`.
  - `screen.Scale(img *image.RGBA, scale float64) *image.RGBA`, `screen.Encode(img image.Image, format string, jpegQuality int) (data []byte, mime string, err error)`, `screen.Grab(s platform.Screen, rect geom.Rect, scale float64, format string, jpegQuality int) (*Shot, error)` with `Shot{Data []byte; MIME string; Size geom.Size; Rect geom.Rect; Scale float64; CaptureMs, ScaleMs, EncodeMs int64}`.
  - `screen.New() *Screen` implementing `platform.Screen` (windows).

- [ ] **Step 1: Write the failing View tests**

`internal/screen/view_test.go`:
```go
package screen

import (
	"math"
	"testing"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
)

func TestAutoScaleFitsLongEdge(t *testing.T) {
	s := AutoScale(geom.Rect{W: 1920, H: 1080}, 1366)
	if math.Abs(s-1366.0/1920.0) > 1e-9 {
		t.Fatalf("scale = %v", s)
	}
	if AutoScale(geom.Rect{W: 800, H: 600}, 1366) != 1 {
		t.Fatalf("small screens must not be upscaled")
	}
	if AutoScale(geom.Rect{W: 800, H: 600}, 0) != 1 {
		t.Fatalf("longEdge<=0 means no scaling")
	}
}

func TestZoomScaleCapsAtMaxZoom(t *testing.T) {
	if z := ZoomScale(geom.Rect{W: 200, H: 100}, 1366, 2); z != 2 {
		t.Fatalf("zoom = %v, want 2", z)
	}
	if z := ZoomScale(geom.Rect{W: 1000, H: 500}, 1366, 2); math.Abs(z-1.366) > 1e-9 {
		t.Fatalf("zoom = %v, want 1.366", z)
	}
}

func TestViewRoundTripOnPrimary(t *testing.T) {
	v := NewView(1, geom.Rect{W: 1920, H: 1080}, AutoScale(geom.Rect{W: 1920, H: 1080}, 1366))
	if v.Image != (geom.Size{W: 1366, H: 768}) {
		t.Fatalf("image size = %v", v.Image)
	}
	if got := v.ToScreen(geom.Point{X: 683, Y: 384}); got != (geom.Point{X: 960, Y: 540}) {
		t.Fatalf("ToScreen = %v", got)
	}
	if got := v.ToImage(geom.Point{X: 960, Y: 540}); got != (geom.Point{X: 683, Y: 384}) {
		t.Fatalf("ToImage = %v", got)
	}
	if got := v.ToScreen(geom.Point{X: 1365, Y: 767}); got.X > 1919 || got.Y > 1079 {
		t.Fatalf("last image pixel must stay on screen: %v", got)
	}
}

func TestViewOffsetOnSecondMonitorAndRegionZoom(t *testing.T) {
	v := NewView(2, geom.Rect{X: 1920, Y: -200, W: 2560, H: 1440}, 0.5)
	if got := v.ToScreen(geom.Point{X: 0, Y: 0}); got != (geom.Point{X: 1920, Y: -200}) {
		t.Fatalf("origin = %v", got)
	}
	z := NewView(1, geom.Rect{X: 100, Y: 100, W: 200, H: 100}, 2)
	if z.Image != (geom.Size{W: 400, H: 200}) {
		t.Fatalf("zoom image = %v", z.Image)
	}
	if got := z.ToScreen(geom.Point{X: 400, Y: 200}); got != (geom.Point{X: 299, Y: 199}) { // clamped to the last screen pixel of the view
		t.Fatalf("zoom ToScreen = %v", got)
	}
	if got := z.RectToImage(geom.Rect{X: 150, Y: 120, W: 10, H: 5}); got != (geom.Rect{X: 100, Y: 40, W: 20, H: 10}) {
		t.Fatalf("RectToImage = %v", got)
	}
	if got := z.ClampImage(geom.Point{X: 999, Y: -5}); got != (geom.Point{X: 399, Y: 0}) {
		t.Fatalf("ClampImage = %v", got)
	}
}
```
Run: `go test ./internal/screen/ -run View` → FAIL.

- [ ] **Step 2: Implement view.go**

`internal/screen/view.go`:
```go
package screen

import (
	"math"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
)

// View is the coordinate space of the last screenshot: image px = (screen px - Offset) * Scale.
type View struct {
	Monitor int        `json:"monitor"` // 0 = whole virtual screen
	Offset  geom.Point `json:"-"`
	Scale   float64    `json:"scale"`
	Image   geom.Size  `json:"-"`
	Screen  geom.Rect  `json:"-"`
}

func NewView(monitor int, screenRect geom.Rect, scale float64) View {
	if scale <= 0 {
		scale = 1
	}
	return View{
		Monitor: monitor,
		Offset:  geom.Point{X: screenRect.X, Y: screenRect.Y},
		Scale:   scale,
		Image:   geom.Size{W: roundInt(float64(screenRect.W) * scale), H: roundInt(float64(screenRect.H) * scale)},
		Screen:  screenRect,
	}
}

func roundInt(f float64) int { return int(math.Round(f)) }

func (v View) ToScreen(p geom.Point) geom.Point {
	x := v.Offset.X + roundInt(float64(p.X)/v.Scale)
	y := v.Offset.Y + roundInt(float64(p.Y)/v.Scale)
	// never map past the last screen pixel of the view
	x = min(x, v.Screen.Right()-1)
	y = min(y, v.Screen.Bottom()-1)
	return geom.Point{X: x, Y: y}
}

func (v View) ToImage(p geom.Point) geom.Point {
	return geom.Point{
		X: roundInt(float64(p.X-v.Offset.X) * v.Scale),
		Y: roundInt(float64(p.Y-v.Offset.Y) * v.Scale),
	}
}

func (v View) RectToImage(r geom.Rect) geom.Rect {
	p := v.ToImage(geom.Point{X: r.X, Y: r.Y})
	return geom.Rect{X: p.X, Y: p.Y, W: roundInt(float64(r.W) * v.Scale), H: roundInt(float64(r.H) * v.Scale)}
}

func (v View) RectToScreen(r geom.Rect) geom.Rect {
	return geom.Rect{
		X: v.Offset.X + roundInt(float64(r.X)/v.Scale),
		Y: v.Offset.Y + roundInt(float64(r.Y)/v.Scale),
		W: roundInt(float64(r.W) / v.Scale),
		H: roundInt(float64(r.H) / v.Scale),
	}
}

func (v View) ClampImage(p geom.Point) geom.Point {
	return geom.Point{X: max(0, min(p.X, v.Image.W-1)), Y: max(0, min(p.Y, v.Image.H-1))}
}

// AutoScale shrinks r so its long edge is at most longEdge; never upscales.
func AutoScale(r geom.Rect, longEdge int) float64 {
	long := max(r.W, r.H)
	if longEdge <= 0 || long <= longEdge || long == 0 {
		return 1
	}
	return float64(longEdge) / float64(long)
}

// ZoomScale is AutoScale for regions: it may upscale up to maxZoom so small targets get big.
func ZoomScale(r geom.Rect, longEdge int, maxZoom float64) float64 {
	long := max(r.W, r.H)
	if long == 0 || longEdge <= 0 {
		return 1
	}
	return min(maxZoom, float64(longEdge)/float64(long))
}
```
Run: `go test ./internal/screen/` → PASS.

- [ ] **Step 3: Write failing Scale/Encode tests**

`internal/screen/capture_test.go`:
```go
package screen

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform/fake"
)

func TestScaleHalvesDimensions(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	out := Scale(img, 0.5)
	if out.Bounds().Dx() != 100 || out.Bounds().Dy() != 50 {
		t.Fatalf("scaled bounds = %v", out.Bounds())
	}
	if Scale(img, 1) != img {
		t.Fatalf("scale 1 must return the same image")
	}
}

func TestEncodeFormats(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	pngData, mime, err := Encode(img, "png", 85)
	if err != nil || mime != "image/png" || !bytes.HasPrefix(pngData, []byte("\x89PNG")) {
		t.Fatalf("png: %v %s", err, mime)
	}
	jpgData, mime, err := Encode(img, "jpeg", 85)
	if err != nil || mime != "image/jpeg" || !bytes.HasPrefix(jpgData, []byte{0xFF, 0xD8}) {
		t.Fatalf("jpeg: %v %s", err, mime)
	}
	if _, _, err := Encode(img, "gif", 85); err == nil {
		t.Fatalf("unknown format must error")
	}
}

func TestGrabUsesScaleAndReportsSize(t *testing.T) {
	s := &fake.Screen{Fill: color.RGBA{R: 10, G: 20, B: 30, A: 255}}
	shot, err := Grab(s, geom.Rect{X: 0, Y: 0, W: 400, H: 200}, 0.5, "png", 85)
	if err != nil {
		t.Fatal(err)
	}
	if shot.Size != (geom.Size{W: 200, H: 100}) || shot.MIME != "image/png" || shot.Scale != 0.5 {
		t.Fatalf("shot = %+v", shot)
	}
}
```
Run: `go test ./internal/screen/` → FAIL.

- [ ] **Step 4: Implement capture.go (pure) and screen_windows.go**

Run `go get golang.org/x/image@latest`.

`internal/screen/capture.go`:
```go
package screen

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"time"

	xdraw "golang.org/x/image/draw"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

// Shot is an encoded screenshot plus the numbers needed to build a View from it.
type Shot struct {
	Data                          []byte
	MIME                          string
	Size                          geom.Size
	Rect                          geom.Rect // screen rect captured
	Scale                         float64
	CaptureMs, ScaleMs, EncodeMs int64
}

// Scale resizes with a bilinear filter; scale 1 returns img unchanged.
func Scale(img *image.RGBA, scale float64) *image.RGBA {
	if scale > 0.999 && scale < 1.001 {
		return img
	}
	w := max(1, roundInt(float64(img.Bounds().Dx())*scale))
	h := max(1, roundInt(float64(img.Bounds().Dy())*scale))
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Src, nil)
	return dst
}

func Encode(img image.Image, format string, jpegQuality int) ([]byte, string, error) {
	var buf bytes.Buffer
	switch format {
	case "", "png":
		enc := png.Encoder{CompressionLevel: png.BestSpeed}
		if err := enc.Encode(&buf, img); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/png", nil
	case "jpeg", "jpg":
		if jpegQuality <= 0 || jpegQuality > 100 {
			jpegQuality = 85
		}
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/jpeg", nil
	}
	return nil, "", fmt.Errorf("unknown image format %q (use png or jpeg)", format)
}

// Grab captures rect from s, scales it and encodes it.
func Grab(s platform.Screen, rect geom.Rect, scale float64, format string, jpegQuality int) (*Shot, error) {
	if rect.Empty() {
		return nil, fmt.Errorf("empty capture rect")
	}
	t0 := time.Now()
	img, err := s.Capture(rect)
	if err != nil {
		return nil, err
	}
	t1 := time.Now()
	scaled := Scale(img, scale)
	t2 := time.Now()
	data, mime, err := Encode(scaled, format, jpegQuality)
	if err != nil {
		return nil, err
	}
	t3 := time.Now()
	return &Shot{
		Data: data, MIME: mime,
		Size:  geom.Size{W: scaled.Bounds().Dx(), H: scaled.Bounds().Dy()},
		Rect:  rect, Scale: scale,
		CaptureMs: t1.Sub(t0).Milliseconds(), ScaleMs: t2.Sub(t1).Milliseconds(), EncodeMs: t3.Sub(t2).Milliseconds(),
	}, nil
}
```

`internal/win/capture.go`:
```go
//go:build windows

package win

import (
	"fmt"
	"image"
	"unsafe"
)

var (
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procBitBlt             = gdi32.NewProc("BitBlt")
)

type bitmapInfoHeader struct {
	Size          uint32
	Width, Height int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

const (
	srcCopy    = 0x00CC0020
	captureBlt = 0x40000000
)

// IncludeLayeredWindows adds CAPTUREBLT so layered windows (tooltips, some app UIs) are captured.
// The overlay excludes itself via display affinity (see layered.go).
var IncludeLayeredWindows = true

// CaptureRect copies a virtual-screen rectangle into an RGBA image (alpha forced to 255).
func CaptureRect(x, y, w, h int) (*image.RGBA, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("empty capture rect")
	}
	hdcScreen, _, _ := procGetDC.Call(0)
	if hdcScreen == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}
	defer procReleaseDC.Call(0, hdcScreen)
	hdcMem, _, _ := procCreateCompatibleDC.Call(hdcScreen)
	if hdcMem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(hdcMem)

	bi := bitmapInfo{Header: bitmapInfoHeader{Size: 40, Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}}
	var bits unsafe.Pointer
	hbm, _, e := procCreateDIBSection.Call(hdcScreen, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbm == 0 || bits == nil {
		return nil, callErr("CreateDIBSection", hbm, e)
	}
	defer procDeleteObject.Call(hbm)
	old, _, _ := procSelectObject.Call(hdcMem, hbm)
	defer procSelectObject.Call(hdcMem, old)

	rop := uintptr(srcCopy)
	if IncludeLayeredWindows {
		rop |= captureBlt
	}
	r, _, e := procBitBlt.Call(hdcMem, 0, 0, uintptr(w), uintptr(h), hdcScreen, uintptr(x), uintptr(y), rop)
	if err := callErr("BitBlt", r, e); err != nil {
		return nil, err
	}
	src := unsafe.Slice((*byte)(bits), w*h*4)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i+3 < len(src); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = src[i+2], src[i+1], src[i], 255
	}
	return img, nil
}
```

`internal/screen/screen_windows.go`:
```go
//go:build windows

package screen

import (
	"image"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

// Screen is the Windows platform.Screen.
type Screen struct{}

func New() *Screen { return &Screen{} }

func (*Screen) Monitors() ([]platform.Monitor, error) { return ListMonitors() }

func (*Screen) Capture(r geom.Rect) (*image.RGBA, error) {
	return win.CaptureRect(r.X, r.Y, r.W, r.H)
}

func (*Screen) CursorPos() (geom.Point, error) {
	p, err := win.GetCursorPos()
	return geom.Point{X: int(p.X), Y: int(p.Y)}, err
}

var _ platform.Screen = (*Screen)(nil)
```
Run: `go test ./internal/screen/` → PASS.

- [ ] **Step 5: `cu screenshot` dev command**

`cmd/cu/cmd_screenshot.go` (delete `runScreenshot` from stubs):
```go
//go:build windows

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/screen"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

func runScreenshot(args []string) error {
	fs := flag.NewFlagSet("screenshot", flag.ContinueOnError)
	mon := fs.Int("m", 1, "monitor id (0 = whole virtual screen)")
	out := fs.String("o", "screenshot.png", "output file (.png or .jpg)")
	longEdge := fs.Int("long-edge", 1366, "scale so the long edge is at most this many px")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := win.SetPerMonitorDPIAwareV2(); err != nil {
		return err
	}
	s := screen.New()
	mons, err := s.Monitors()
	if err != nil {
		return err
	}
	var rect geom.Rect
	if *mon == 0 {
		rect = screen.VirtualScreen(mons)
	} else {
		m, ok := screen.MonitorByID(mons, *mon)
		if !ok {
			return fmt.Errorf("no monitor %d", *mon)
		}
		rect = m.Rect
	}
	format := "png"
	if len(*out) > 4 && (*out)[len(*out)-4:] == ".jpg" {
		format = "jpeg"
	}
	shot, err := screen.Grab(s, rect, screen.AutoScale(rect, *longEdge), format, 85)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, shot.Data, 0o644); err != nil {
		return err
	}
	fmt.Printf("%s: %dx%d (scale %.3f) capture=%dms scale=%dms encode=%dms bytes=%d\n",
		*out, shot.Size.W, shot.Size.H, shot.Scale, shot.CaptureMs, shot.ScaleMs, shot.EncodeMs, len(shot.Data))
	return nil
}
```
Plus `cmd/cu/cmd_screenshot_other.go` (`//go:build !windows`, returns an error).

Run: `go run ./cmd/cu screenshot -o C:\Users\test\AppData\Local\Temp\shot.png` then open the file (the Read tool can display PNGs) — it must show the primary monitor, correctly scaled, no black areas. Total time on 1080p should print well under 100 ms; if capture > 40 ms, note it in the commit message (DXGI is a later optimization).

- [ ] **Step 6: Commit**

```bash
go build ./... && go vet ./... && go test ./...
git add -A && git commit -m "feat: screen capture, scaling, encoding and screenshot view transform"
```

---

### Task 5: Keyboard/mouse input, chord parser, clipboard, action orchestration, `cu input`

**Files:**
- Create: `internal/input/keys.go`, `internal/input/keys_test.go`, `internal/input/input_windows.go`, `internal/input/clipboard_windows.go`
- Create: `internal/win/input.go`, `internal/win/input_test.go`, `internal/win/clipboard.go`
- Create: `internal/actions/actions.go`, `internal/actions/actions_test.go`
- Create: `cmd/cu/cmd_input.go`, `cmd/cu/cmd_input_other.go`; Modify: `cmd/cu/stubs.go` (remove `runInput`)

**Interfaces:**
- Consumes: `platform.Input`, `platform.Clipboard`, `fake.Input`, `fake.Clipboard`, `win.VirtualScreenRect`, `win.GetCursorPos`.
- Produces:
  - `input.Chord{Mods []uint16; Key uint16}`, `input.ParseChord(s string) (Chord, error)`, `(Chord) String() string`, `input.KeyName(vk uint16) string`, `input.NormalizeModifier(vk uint16) uint16`, `input.IsModifier(vk uint16) bool`, `input.ExtendedKeys map[uint16]bool`, VK constants `input.VK_SHIFT=0x10, VK_CONTROL=0x11, VK_MENU=0x12, VK_LWIN=0x5B, VK_ESCAPE=0x1B, VK_RETURN=0x0D, VK_TAB=0x09, VK_V=0x56` (and the rest listed below).
  - `input.New() *Input` (platform.Input), `input.NewClipboard() *Clipboard` (platform.Clipboard).
  - `win.MouseMoveAbs(x, y int) error`, `win.MouseButtonEvent(flags uint32) error`, `win.MouseWheel(delta int32, horizontal bool) error`, `win.KeyEvent(vk, scan uint16, flags uint32) error`, `win.UnicodeText(units []uint16) error`, `win.MapVirtualKeyToScan(vk uint16) uint16`, constants `win.MouseEventfLeftDown` … and `win.InputTag = 0x0C1A0DE5`; `win.GetClipboardText() (string, error)`, `win.SetClipboardText(s string) error`.
  - `actions.Actor{In platform.Input; Clip platform.Clipboard; Sleep func(time.Duration); PasteThreshold int}` with `Click(p geom.Point, btn platform.MouseButton, count int, mods []uint16) error`, `Drag(from, to geom.Point, btn platform.MouseButton, dur time.Duration) error`, `Scroll(p *geom.Point, dx, dy int) error`, `Chord(c input.Chord, hold time.Duration) error`, `Type(text, mode string, delay time.Duration) error`.

- [ ] **Step 1: Failing tests for the chord parser**

`internal/input/keys_test.go`:
```go
package input

import "testing"

func TestParseChord(t *testing.T) {
	cases := map[string]Chord{
		"ctrl+shift+t":  {Mods: []uint16{VK_CONTROL, VK_SHIFT}, Key: 0x54},
		"Enter":         {Key: VK_RETURN},
		"win+r":         {Mods: []uint16{VK_LWIN}, Key: 0x52},
		"alt+f4":        {Mods: []uint16{VK_MENU}, Key: 0x73},
		"f12":           {Key: 0x7B},
		"ctrl+1":        {Mods: []uint16{VK_CONTROL}, Key: 0x31},
		"escape":        {Key: VK_ESCAPE},
		"esc":           {Key: VK_ESCAPE},
		"pgdn":          {Key: 0x22},
		"cmd+space":     {Mods: []uint16{VK_LWIN}, Key: 0x20},
		"shift+ctrl+a":  {Mods: []uint16{VK_CONTROL, VK_SHIFT}, Key: 0x41}, // canonical order ctrl, alt, shift, win
		"ctrl+plus":     {Mods: []uint16{VK_CONTROL}, Key: 0xBB},
		"arrowleft":     {Key: 0x25},
		"printscreen":   {Key: 0x2C},
	}
	for in, want := range cases {
		got, err := ParseChord(in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got.Key != want.Key || len(got.Mods) != len(want.Mods) {
			t.Fatalf("%q: got %+v want %+v", in, got, want)
		}
		for i := range want.Mods {
			if got.Mods[i] != want.Mods[i] {
				t.Fatalf("%q: mods %v want %v", in, got.Mods, want.Mods)
			}
		}
	}
	for _, bad := range []string{"", "ctrl+", "bogus", "ctrl+ctrl+a", "+", "ctrl+ü", "shift"} {
		if _, err := ParseChord(bad); err == nil {
			t.Fatalf("%q must fail", bad)
		}
	}
}

func TestChordString(t *testing.T) {
	c, _ := ParseChord("shift+ctrl+esc")
	if c.String() != "Ctrl+Shift+Esc" {
		t.Fatalf("String = %q", c.String())
	}
	if KeyName(0x0D) != "Enter" || KeyName(0x41) != "A" {
		t.Fatalf("KeyName wrong")
	}
	if NormalizeModifier(0xA2) != VK_CONTROL || NormalizeModifier(0x5C) != VK_LWIN || !IsModifier(0xA5) {
		t.Fatalf("modifier normalisation wrong")
	}
}
```
Run: `go test ./internal/input/` → FAIL.

- [ ] **Step 2: Implement keys.go**

`internal/input/keys.go`:
```go
// Package input implements platform.Input and the key chord grammar ("ctrl+shift+t").
package input

import (
	"fmt"
	"strings"
)

const (
	VK_BACK    = 0x08
	VK_TAB     = 0x09
	VK_RETURN  = 0x0D
	VK_SHIFT   = 0x10
	VK_CONTROL = 0x11
	VK_MENU    = 0x12
	VK_PAUSE   = 0x13
	VK_CAPITAL = 0x14
	VK_ESCAPE  = 0x1B
	VK_SPACE   = 0x20
	VK_PRIOR   = 0x21
	VK_NEXT    = 0x22
	VK_END     = 0x23
	VK_HOME    = 0x24
	VK_LEFT    = 0x25
	VK_UP      = 0x26
	VK_RIGHT   = 0x27
	VK_DOWN    = 0x28
	VK_SNAPSHOT = 0x2C
	VK_INSERT  = 0x2D
	VK_DELETE  = 0x2E
	VK_LWIN    = 0x5B
	VK_RWIN    = 0x5C
	VK_APPS    = 0x5D
	VK_V       = 0x56
	VK_NUMLOCK = 0x90
	VK_SCROLL  = 0x91
	VK_LSHIFT  = 0xA0
	VK_RSHIFT  = 0xA1
	VK_LCONTROL = 0xA2
	VK_RCONTROL = 0xA3
	VK_LMENU   = 0xA4
	VK_RMENU   = 0xA5
)

// Chord is one key with modifiers, e.g. Ctrl+Shift+T. Mods are in canonical order: ctrl, alt, shift, win.
type Chord struct {
	Mods []uint16
	Key  uint16
}

var modifierOrder = []uint16{VK_CONTROL, VK_MENU, VK_SHIFT, VK_LWIN}

var modifierNames = map[string]uint16{
	"ctrl": VK_CONTROL, "control": VK_CONTROL,
	"alt": VK_MENU, "option": VK_MENU,
	"shift": VK_SHIFT,
	"win": VK_LWIN, "cmd": VK_LWIN, "super": VK_LWIN, "meta": VK_LWIN, "windows": VK_LWIN,
}

var keyNames = map[string]uint16{
	"enter": VK_RETURN, "return": VK_RETURN,
	"esc": VK_ESCAPE, "escape": VK_ESCAPE,
	"tab": VK_TAB, "space": VK_SPACE, "backspace": VK_BACK,
	"delete": VK_DELETE, "del": VK_DELETE, "insert": VK_INSERT, "ins": VK_INSERT,
	"home": VK_HOME, "end": VK_END,
	"pageup": VK_PRIOR, "pgup": VK_PRIOR, "pagedown": VK_NEXT, "pgdn": VK_NEXT,
	"left": VK_LEFT, "arrowleft": VK_LEFT, "up": VK_UP, "arrowup": VK_UP,
	"right": VK_RIGHT, "arrowright": VK_RIGHT, "down": VK_DOWN, "arrowdown": VK_DOWN,
	"printscreen": VK_SNAPSHOT, "prtsc": VK_SNAPSHOT, "pause": VK_PAUSE,
	"capslock": VK_CAPITAL, "numlock": VK_NUMLOCK, "scrolllock": VK_SCROLL,
	"apps": VK_APPS, "menu": VK_APPS, "contextmenu": VK_APPS,
	"plus": 0xBB, "equal": 0xBB, "minus": 0xBD, "comma": 0xBC, "period": 0xBE, "dot": 0xBE,
	"slash": 0xBF, "backslash": 0xDC, "semicolon": 0xBA, "quote": 0xDE,
	"lbracket": 0xDB, "rbracket": 0xDD, "grave": 0xC0, "backquote": 0xC0, "tilde": 0xC0,
	"numpad0": 0x60, "numpad1": 0x61, "numpad2": 0x62, "numpad3": 0x63, "numpad4": 0x64,
	"numpad5": 0x65, "numpad6": 0x66, "numpad7": 0x67, "numpad8": 0x68, "numpad9": 0x69,
	"multiply": 0x6A, "add": 0x6B, "subtract": 0x6D, "decimal": 0x6E, "divide": 0x6F,
	"volumeup": 0xAF, "volumedown": 0xAE, "volumemute": 0xAD,
	"mediaplaypause": 0xB3, "mediastop": 0xB2, "medianext": 0xB0, "mediaprev": 0xB1,
	"browserback": 0xA6, "browserforward": 0xA7,
}

// ExtendedKeys need KEYEVENTF_EXTENDEDKEY so apps see the "real" key, not the numpad twin.
var ExtendedKeys = map[uint16]bool{
	VK_INSERT: true, VK_DELETE: true, VK_HOME: true, VK_END: true, VK_PRIOR: true, VK_NEXT: true,
	VK_LEFT: true, VK_UP: true, VK_RIGHT: true, VK_DOWN: true, VK_LWIN: true, VK_RWIN: true,
	VK_APPS: true, VK_NUMLOCK: true, VK_SNAPSHOT: true, 0x6F: true, VK_RCONTROL: true, VK_RMENU: true,
}

func init() {
	for c := 'a'; c <= 'z'; c++ {
		keyNames[string(c)] = uint16(0x41 + c - 'a')
	}
	for c := '0'; c <= '9'; c++ {
		keyNames[string(c)] = uint16(0x30 + c - '0')
	}
	for i := 1; i <= 24; i++ {
		keyNames[fmt.Sprintf("f%d", i)] = uint16(0x70 + i - 1)
	}
}

// ParseChord parses "ctrl+shift+t", "Enter", "win+r". Case-insensitive; spaces around '+' allowed.
func ParseChord(s string) (Chord, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return Chord{}, fmt.Errorf("empty key chord")
	}
	parts := strings.Split(s, "+")
	// a trailing "+" (e.g. "ctrl++") means the plus key
	var tokens []string
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			if i == len(parts)-1 && len(parts) > 1 && tokens != nil && parts[i-1] == "" {
				tokens = append(tokens, "plus")
			}
			continue
		}
		tokens = append(tokens, p)
	}
	if len(tokens) == 0 {
		return Chord{}, fmt.Errorf("invalid key chord %q", s)
	}
	seen := map[uint16]bool{}
	var c Chord
	for i, tok := range tokens {
		last := i == len(tokens)-1
		if vk, ok := modifierNames[tok]; ok && !last {
			if seen[vk] {
				return Chord{}, fmt.Errorf("duplicate modifier %q in %q", tok, s)
			}
			seen[vk] = true
			continue
		}
		if !last {
			return Chord{}, fmt.Errorf("%q is not a modifier in %q", tok, s)
		}
		vk, ok := keyNames[tok]
		if !ok {
			return Chord{}, fmt.Errorf("unknown key %q in %q (use `type` for text)", tok, s)
		}
		c.Key = vk
	}
	for _, m := range modifierOrder {
		if seen[m] {
			c.Mods = append(c.Mods, m)
		}
	}
	if c.Key == 0 {
		return Chord{}, fmt.Errorf("chord %q has no key", s)
	}
	return c, nil
}

var displayNames = map[uint16]string{
	VK_CONTROL: "Ctrl", VK_MENU: "Alt", VK_SHIFT: "Shift", VK_LWIN: "Win",
	VK_RETURN: "Enter", VK_ESCAPE: "Esc", VK_TAB: "Tab", VK_SPACE: "Space", VK_BACK: "Backspace",
	VK_DELETE: "Delete", VK_INSERT: "Insert", VK_HOME: "Home", VK_END: "End",
	VK_PRIOR: "PageUp", VK_NEXT: "PageDown", VK_LEFT: "Left", VK_UP: "Up", VK_RIGHT: "Right", VK_DOWN: "Down",
	VK_SNAPSHOT: "PrintScreen", VK_APPS: "Menu", 0xBB: "+", 0xBD: "-",
}

// KeyName is the display name of a virtual key ("Ctrl", "Enter", "A", "F5").
func KeyName(vk uint16) string {
	if n, ok := displayNames[vk]; ok {
		return n
	}
	switch {
	case vk >= 0x41 && vk <= 0x5A, vk >= 0x30 && vk <= 0x39:
		return string(rune(vk))
	case vk >= 0x70 && vk <= 0x87:
		return fmt.Sprintf("F%d", vk-0x70+1)
	}
	return fmt.Sprintf("VK%02X", vk)
}

func (c Chord) String() string {
	parts := make([]string, 0, len(c.Mods)+1)
	for _, m := range c.Mods {
		parts = append(parts, KeyName(m))
	}
	parts = append(parts, KeyName(c.Key))
	return strings.Join(parts, "+")
}

// NormalizeModifier maps left/right variants to the generic modifier code.
func NormalizeModifier(vk uint16) uint16 {
	switch vk {
	case VK_LSHIFT, VK_RSHIFT:
		return VK_SHIFT
	case VK_LCONTROL, VK_RCONTROL:
		return VK_CONTROL
	case VK_LMENU, VK_RMENU:
		return VK_MENU
	case VK_RWIN:
		return VK_LWIN
	}
	return vk
}

func IsModifier(vk uint16) bool {
	switch NormalizeModifier(vk) {
	case VK_SHIFT, VK_CONTROL, VK_MENU, VK_LWIN:
		return true
	}
	return false
}
```
Run: `go test ./internal/input/` → PASS (fix the `"+"`/`"ctrl+plus"` edge cases until green; `"+"` alone must fail, `"ctrl+plus"` must map to 0xBB).

- [ ] **Step 3: SendInput bindings with a size test**

`internal/win/input_test.go`:
```go
//go:build windows

package win

import (
	"testing"
	"unsafe"
)

func TestInputRecordSizesMatchWin32(t *testing.T) {
	if s := unsafe.Sizeof(inputMouseRec{}); s != 40 {
		t.Fatalf("INPUT(mouse) size = %d, want 40", s)
	}
	if s := unsafe.Sizeof(inputKeybdRec{}); s != 40 {
		t.Fatalf("INPUT(keyboard) size = %d, want 40", s)
	}
}
```

`internal/win/input.go`:
```go
//go:build windows

package win

import (
	"fmt"
	"math"
	"unsafe"
)

var (
	procSendInput      = user32.NewProc("SendInput")
	procSetCursorPos   = user32.NewProc("SetCursorPos")
	procMapVirtualKeyW = user32.NewProc("MapVirtualKeyW")
)

const (
	inputTypeMouse    = 0
	inputTypeKeyboard = 1

	MouseEventfMove       = 0x0001
	MouseEventfLeftDown   = 0x0002
	MouseEventfLeftUp     = 0x0004
	MouseEventfRightDown  = 0x0008
	MouseEventfRightUp    = 0x0010
	MouseEventfMiddleDown = 0x0020
	MouseEventfMiddleUp   = 0x0040
	MouseEventfWheel      = 0x0800
	MouseEventfHWheel     = 0x1000
	MouseEventfVirtualDesk = 0x4000
	MouseEventfAbsolute   = 0x8000

	KeyEventfExtendedKey = 0x0001
	KeyEventfKeyUp       = 0x0002
	KeyEventfUnicode     = 0x0004

	// InputTag marks input injected by cu (dwExtraInfo) so our hooks can recognise it.
	InputTag = 0x0C1A0DE5
)

type mouseInput struct {
	Dx, Dy    int32
	MouseData uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

type keybdInput struct {
	Vk, Scan  uint16
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
	_         [8]byte // pad the union to MOUSEINPUT's 32 bytes
}

type inputMouseRec struct {
	Type uint32
	_    uint32
	Mi   mouseInput
}

type inputKeybdRec struct {
	Type uint32
	_    uint32
	Ki   keybdInput
}

func sendInput(ptr unsafe.Pointer, n int, size uintptr) error {
	r, _, e := procSendInput.Call(uintptr(n), uintptr(ptr), size)
	if int(r) != n {
		return fmt.Errorf("SendInput sent %d of %d events: %v", r, n, e)
	}
	return nil
}

// MouseMoveAbs moves the cursor to virtual-screen coordinates and emits a real mouse-move event.
func MouseMoveAbs(x, y int) error {
	vs := VirtualScreenRect()
	nx := int32(math.Round(float64(x-int(vs.Left)) * 65535 / float64(max(1, int(vs.Width())-1))))
	ny := int32(math.Round(float64(y-int(vs.Top)) * 65535 / float64(max(1, int(vs.Height())-1))))
	rec := inputMouseRec{Type: inputTypeMouse, Mi: mouseInput{
		Dx: nx, Dy: ny, Flags: MouseEventfMove | MouseEventfAbsolute | MouseEventfVirtualDesk, ExtraInfo: InputTag,
	}}
	if err := sendInput(unsafe.Pointer(&rec), 1, unsafe.Sizeof(rec)); err != nil {
		return err
	}
	// normalisation may land one pixel off; snap exactly.
	if p, err := GetCursorPos(); err == nil && (int(p.X) != x || int(p.Y) != y) {
		procSetCursorPos.Call(uintptr(x), uintptr(y))
	}
	return nil
}

// MouseButtonEvent sends one button event (a MouseEventf* down/up flag) at the current position.
func MouseButtonEvent(flags uint32) error {
	rec := inputMouseRec{Type: inputTypeMouse, Mi: mouseInput{Flags: flags, ExtraInfo: InputTag}}
	return sendInput(unsafe.Pointer(&rec), 1, unsafe.Sizeof(rec))
}

// MouseWheel sends a wheel event; delta is in WHEEL_DELTA units (120 per tick), positive = up / right.
func MouseWheel(delta int32, horizontal bool) error {
	flags := uint32(MouseEventfWheel)
	if horizontal {
		flags = MouseEventfHWheel
	}
	rec := inputMouseRec{Type: inputTypeMouse, Mi: mouseInput{MouseData: uint32(delta), Flags: flags, ExtraInfo: InputTag}}
	return sendInput(unsafe.Pointer(&rec), 1, unsafe.Sizeof(rec))
}

func MapVirtualKeyToScan(vk uint16) uint16 {
	r, _, _ := procMapVirtualKeyW.Call(uintptr(vk), 0 /* MAPVK_VK_TO_VSC */)
	return uint16(r)
}

// KeyEvent sends one virtual-key event.
func KeyEvent(vk, scan uint16, flags uint32) error {
	rec := inputKeybdRec{Type: inputTypeKeyboard, Ki: keybdInput{Vk: vk, Scan: scan, Flags: flags, ExtraInfo: InputTag}}
	return sendInput(unsafe.Pointer(&rec), 1, unsafe.Sizeof(rec))
}

// UnicodeText types UTF-16 code units as KEYEVENTF_UNICODE down/up pairs, in one SendInput call per chunk.
func UnicodeText(units []uint16) error {
	const chunk = 48 // 96 events per call keeps SendInput well below its practical limits
	for start := 0; start < len(units); start += chunk {
		end := min(start+chunk, len(units))
		recs := make([]inputKeybdRec, 0, (end-start)*2)
		for _, u := range units[start:end] {
			recs = append(recs,
				inputKeybdRec{Type: inputTypeKeyboard, Ki: keybdInput{Scan: u, Flags: KeyEventfUnicode, ExtraInfo: InputTag}},
				inputKeybdRec{Type: inputTypeKeyboard, Ki: keybdInput{Scan: u, Flags: KeyEventfUnicode | KeyEventfKeyUp, ExtraInfo: InputTag}},
			)
		}
		if err := sendInput(unsafe.Pointer(&recs[0]), len(recs), unsafe.Sizeof(recs[0])); err != nil {
			return err
		}
	}
	return nil
}
```
Run: `go test ./internal/win/` → PASS.

- [ ] **Step 4: Clipboard bindings**

`internal/win/clipboard.go`:
```go
//go:build windows

package win

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procOpenClipboard            = user32.NewProc("OpenClipboard")
	procCloseClipboard           = user32.NewProc("CloseClipboard")
	procEmptyClipboard           = user32.NewProc("EmptyClipboard")
	procGetClipboardData         = user32.NewProc("GetClipboardData")
	procSetClipboardData         = user32.NewProc("SetClipboardData")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procGlobalAlloc              = kernel32.NewProc("GlobalAlloc")
	procGlobalLock               = kernel32.NewProc("GlobalLock")
	procGlobalUnlock             = kernel32.NewProc("GlobalUnlock")
	procGlobalFree               = kernel32.NewProc("GlobalFree")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

func openClipboard() error {
	for i := 0; i < 10; i++ {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("clipboard is busy")
}

func GetClipboardText() (string, error) {
	if err := openClipboard(); err != nil {
		return "", err
	}
	defer procCloseClipboard.Call()
	if r, _, _ := procIsClipboardFormatAvailable.Call(cfUnicodeText); r == 0 {
		return "", nil
	}
	h, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return "", nil
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		return "", fmt.Errorf("GlobalLock failed")
	}
	defer procGlobalUnlock.Call(h)
	var units []uint16
	for i := 0; ; i++ {
		u := *(*uint16)(unsafe.Pointer(p + uintptr(i)*2))
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	return windows.UTF16ToString(units), nil
}

func SetClipboardText(s string) error {
	units, err := windows.UTF16FromString(s)
	if err != nil {
		return err
	}
	size := uintptr(len(units) * 2)
	h, _, e := procGlobalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return callErr("GlobalAlloc", h, e)
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return fmt.Errorf("GlobalLock failed")
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(p)), len(units)), units)
	procGlobalUnlock.Call(h)
	if err := openClipboard(); err != nil {
		procGlobalFree.Call(h)
		return err
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()
	if r, _, e := procSetClipboardData.Call(cfUnicodeText, h); r == 0 {
		procGlobalFree.Call(h)
		return callErr("SetClipboardData", r, e)
	}
	return nil // ownership of h moved to the system
}
```

- [ ] **Step 5: platform.Input and platform.Clipboard implementations**

`internal/input/input_windows.go`:
```go
//go:build windows

package input

import (
	"fmt"
	"unicode/utf16"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

// Input is the Windows platform.Input built on SendInput.
type Input struct{}

func New() *Input { return &Input{} }

func (*Input) MouseMove(p geom.Point) error { return win.MouseMoveAbs(p.X, p.Y) }

func buttonFlags(b platform.MouseButton) (down, up uint32, err error) {
	switch b {
	case platform.ButtonLeft, "":
		return win.MouseEventfLeftDown, win.MouseEventfLeftUp, nil
	case platform.ButtonRight:
		return win.MouseEventfRightDown, win.MouseEventfRightUp, nil
	case platform.ButtonMiddle:
		return win.MouseEventfMiddleDown, win.MouseEventfMiddleUp, nil
	}
	return 0, 0, fmt.Errorf("unknown mouse button %q", b)
}

func (*Input) MouseDown(b platform.MouseButton) error {
	d, _, err := buttonFlags(b)
	if err != nil {
		return err
	}
	return win.MouseButtonEvent(d)
}

func (*Input) MouseUp(b platform.MouseButton) error {
	_, u, err := buttonFlags(b)
	if err != nil {
		return err
	}
	return win.MouseButtonEvent(u)
}

func (*Input) Scroll(dx, dy int) error {
	if dy != 0 {
		if err := win.MouseWheel(int32(-dy*120), false); err != nil {
			return err
		}
	}
	if dx != 0 {
		if err := win.MouseWheel(int32(dx*120), true); err != nil {
			return err
		}
	}
	return nil
}

func keyFlags(vk uint16) uint32 {
	if ExtendedKeys[vk] {
		return win.KeyEventfExtendedKey
	}
	return 0
}

func (*Input) KeyDown(vk uint16) error {
	return win.KeyEvent(vk, win.MapVirtualKeyToScan(vk), keyFlags(vk))
}

func (*Input) KeyUp(vk uint16) error {
	return win.KeyEvent(vk, win.MapVirtualKeyToScan(vk), keyFlags(vk)|win.KeyEventfKeyUp)
}

// TypeUnicode types text; newlines become Enter and tabs become Tab so editors behave naturally.
func (in *Input) TypeUnicode(s string) error {
	var pending []uint16
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		err := win.UnicodeText(pending)
		pending = pending[:0]
		return err
	}
	for _, r := range s {
		switch r {
		case '\r':
			continue
		case '\n', '\t':
			if err := flush(); err != nil {
				return err
			}
			vk := uint16(VK_RETURN)
			if r == '\t' {
				vk = VK_TAB
			}
			if err := in.KeyDown(vk); err != nil {
				return err
			}
			if err := in.KeyUp(vk); err != nil {
				return err
			}
		default:
			pending = append(pending, utf16.Encode([]rune{r})...)
		}
	}
	return flush()
}

var _ platform.Input = (*Input)(nil)
```

`internal/input/clipboard_windows.go`:
```go
//go:build windows

package input

import (
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

type Clipboard struct{}

func NewClipboard() *Clipboard                 { return &Clipboard{} }
func (*Clipboard) GetText() (string, error)    { return win.GetClipboardText() }
func (*Clipboard) SetText(s string) error      { return win.SetClipboardText(s) }

var _ platform.Clipboard = (*Clipboard)(nil)
```

- [ ] **Step 6: Failing tests for the action orchestration**

`internal/actions/actions_test.go`:
```go
package actions

import (
	"strings"
	"testing"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/input"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/platform/fake"
)

func newActor() (*Actor, *fake.Input, *fake.Clipboard) {
	in := &fake.Input{}
	clip := &fake.Clipboard{Text: "old"}
	return &Actor{In: in, Clip: clip, Sleep: func(time.Duration) {}, PasteThreshold: 200}, in, clip
}

func TestClickMovesThenPressesNTimes(t *testing.T) {
	a, in, _ := newActor()
	if err := a.Click(geom.Point{X: 10, Y: 20}, platform.ButtonLeft, 2, nil); err != nil {
		t.Fatal(err)
	}
	want := "move 10,20|down left|up left|down left|up left"
	if got := strings.Join(in.Calls, "|"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestClickWithModifiersWrapsPress(t *testing.T) {
	a, in, _ := newActor()
	_ = a.Click(geom.Point{X: 1, Y: 1}, platform.ButtonRight, 1, []uint16{input.VK_CONTROL})
	want := "key_down 17|move 1,1|down right|up right|key_up 17"
	if got := strings.Join(in.Calls, "|"); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestDragInterpolatesAndReleasesAtTarget(t *testing.T) {
	a, in, _ := newActor()
	_ = a.Drag(geom.Point{X: 0, Y: 0}, geom.Point{X: 100, Y: 50}, platform.ButtonLeft, 250*time.Millisecond)
	calls := in.Calls
	if calls[0] != "move 0,0" || calls[1] != "down left" || calls[len(calls)-1] != "up left" || calls[len(calls)-2] != "move 100,50" {
		t.Fatalf("bad drag sequence: %v", calls)
	}
	if len(calls) < 2+8+1 {
		t.Fatalf("drag must have at least 8 intermediate moves, got %d calls", len(calls))
	}
}

func TestChordOrder(t *testing.T) {
	a, in, _ := newActor()
	c, _ := input.ParseChord("ctrl+shift+t")
	_ = a.Chord(c, 0)
	want := "key_down 17|key_down 16|key_down 84|key_up 84|key_up 16|key_up 17"
	if got := strings.Join(in.Calls, "|"); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestTypeAutoUsesUnicodeForShortAndPasteForLong(t *testing.T) {
	a, in, clip := newActor()
	_ = a.Type("hello", "auto", 0)
	if strings.Join(in.Calls, "|") != "type hello" {
		t.Fatalf("short text: %v", in.Calls)
	}
	long := strings.Repeat("x", 300)
	in.Calls = nil
	_ = a.Type(long, "auto", 0)
	joined := strings.Join(in.Calls, "|")
	if !strings.Contains(joined, "key_down 17|key_down 86") {
		t.Fatalf("long text must paste with ctrl+v: %v", in.Calls)
	}
	if clip.Text != "old" {
		t.Fatalf("clipboard must be restored, got %q", clip.Text)
	}
}

func TestScrollMovesFirstWhenPointGiven(t *testing.T) {
	a, in, _ := newActor()
	p := geom.Point{X: 5, Y: 6}
	_ = a.Scroll(&p, 0, 3)
	if strings.Join(in.Calls, "|") != "move 5,6|scroll 0,3" {
		t.Fatalf("got %v", in.Calls)
	}
}
```
Run: `go test ./internal/actions/` → FAIL.

- [ ] **Step 7: Implement actions.go**

`internal/actions/actions.go`:
```go
// Package actions composes primitive platform.Input calls into human-like actions.
package actions

import (
	"fmt"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/input"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

type Actor struct {
	In             platform.Input
	Clip           platform.Clipboard // may be nil: paste mode then falls back to unicode
	Sleep          func(time.Duration)
	PasteThreshold int // "auto" typing pastes when the text is longer than this (runes)
}

func (a *Actor) sleep(d time.Duration) {
	if a.Sleep != nil {
		a.Sleep(d)
	} else {
		time.Sleep(d)
	}
}

func (a *Actor) holdMods(mods []uint16) error {
	for _, m := range mods {
		if err := a.In.KeyDown(m); err != nil {
			return err
		}
	}
	return nil
}

func (a *Actor) releaseMods(mods []uint16) error {
	for i := len(mods) - 1; i >= 0; i-- {
		if err := a.In.KeyUp(mods[i]); err != nil {
			return err
		}
	}
	return nil
}

// Click moves to p and presses btn count times (2 = double click) while holding mods.
func (a *Actor) Click(p geom.Point, btn platform.MouseButton, count int, mods []uint16) error {
	if count < 1 {
		count = 1
	}
	if btn == "" {
		btn = platform.ButtonLeft
	}
	if err := a.holdMods(mods); err != nil {
		return err
	}
	defer a.releaseMods(mods)
	if err := a.In.MouseMove(p); err != nil {
		return err
	}
	a.sleep(15 * time.Millisecond)
	for i := 0; i < count; i++ {
		if err := a.In.MouseDown(btn); err != nil {
			return err
		}
		a.sleep(12 * time.Millisecond)
		if err := a.In.MouseUp(btn); err != nil {
			return err
		}
		if i < count-1 {
			a.sleep(60 * time.Millisecond) // well inside the system double-click time
		}
	}
	return nil
}

// Drag presses at from, moves in steps over dur, pauses, and releases at to.
func (a *Actor) Drag(from, to geom.Point, btn platform.MouseButton, dur time.Duration) error {
	if btn == "" {
		btn = platform.ButtonLeft
	}
	if dur <= 0 {
		dur = 250 * time.Millisecond
	}
	if err := a.In.MouseMove(from); err != nil {
		return err
	}
	a.sleep(30 * time.Millisecond)
	if err := a.In.MouseDown(btn); err != nil {
		return err
	}
	a.sleep(80 * time.Millisecond) // Explorer and browsers start a drag only after the button is held
	steps := max(8, int(dur/(12*time.Millisecond)))
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		p := geom.Point{
			X: from.X + int(float64(to.X-from.X)*t+0.5),
			Y: from.Y + int(float64(to.Y-from.Y)*t+0.5),
		}
		if i == steps {
			p = to
		}
		if err := a.In.MouseMove(p); err != nil {
			return err
		}
		a.sleep(dur / time.Duration(steps))
	}
	a.sleep(80 * time.Millisecond) // let drop targets highlight before releasing
	return a.In.MouseUp(btn)
}

// Scroll optionally moves to p, then scrolls by ticks (dy>0 down, dx>0 right).
func (a *Actor) Scroll(p *geom.Point, dx, dy int) error {
	if p != nil {
		if err := a.In.MouseMove(*p); err != nil {
			return err
		}
		a.sleep(10 * time.Millisecond)
	}
	return a.In.Scroll(dx, dy)
}

// Chord presses modifiers, the key, holds, and releases in reverse order.
func (a *Actor) Chord(c input.Chord, hold time.Duration) error {
	if err := a.holdMods(c.Mods); err != nil {
		return err
	}
	if err := a.In.KeyDown(c.Key); err != nil {
		return err
	}
	if hold <= 0 {
		hold = 10 * time.Millisecond
	}
	a.sleep(hold)
	if err := a.In.KeyUp(c.Key); err != nil {
		return err
	}
	return a.releaseMods(c.Mods)
}

// Type enters text. mode: auto (unicode, paste when long), unicode, paste, keys (per-char with delay).
func (a *Actor) Type(text, mode string, delay time.Duration) error {
	runes := []rune(text)
	switch mode {
	case "", "auto":
		if a.Clip != nil && a.PasteThreshold > 0 && len(runes) > a.PasteThreshold {
			return a.paste(text)
		}
		return a.typeUnicode(runes, delay)
	case "unicode":
		return a.typeUnicode(runes, delay)
	case "paste":
		if a.Clip == nil {
			return a.typeUnicode(runes, delay)
		}
		return a.paste(text)
	case "keys":
		if delay < 15*time.Millisecond {
			delay = 15 * time.Millisecond
		}
		return a.typeUnicode(runes, delay)
	}
	return fmt.Errorf("unknown type mode %q (auto|unicode|paste|keys)", mode)
}

func (a *Actor) typeUnicode(runes []rune, delay time.Duration) error {
	if delay <= 0 {
		return a.In.TypeUnicode(string(runes))
	}
	for _, r := range runes {
		if err := a.In.TypeUnicode(string(r)); err != nil {
			return err
		}
		a.sleep(delay)
	}
	return nil
}

func (a *Actor) paste(text string) error {
	old, _ := a.Clip.GetText()
	if err := a.Clip.SetText(text); err != nil {
		return err
	}
	if err := a.Chord(input.Chord{Mods: []uint16{input.VK_CONTROL}, Key: input.VK_V}, 0); err != nil {
		return err
	}
	a.sleep(300 * time.Millisecond) // most apps read the clipboard synchronously on Ctrl+V; give slow ones time
	return a.Clip.SetText(old)
}
```
Run: `go test ./internal/actions/` → PASS.

- [ ] **Step 8: `cu input` dev command and manual check**

`cmd/cu/cmd_input.go` (delete `runInput` from stubs):
```go
//go:build windows

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/actions"
	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/input"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

func runInput(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cu input move X Y | click X Y [left|right|middle] | drag X1 Y1 X2 Y2 | type TEXT | key CHORD")
	}
	if err := win.SetPerMonitorDPIAwareV2(); err != nil {
		return err
	}
	a := &actions.Actor{In: input.New(), Clip: input.NewClipboard(), PasteThreshold: 200}
	atoi := func(s string) int { n, _ := strconv.Atoi(s); return n }
	switch args[0] {
	case "move":
		return a.In.MouseMove(geom.Point{X: atoi(args[1]), Y: atoi(args[2])})
	case "click":
		btn := platform.ButtonLeft
		if len(args) > 3 {
			btn = platform.MouseButton(args[3])
		}
		return a.Click(geom.Point{X: atoi(args[1]), Y: atoi(args[2])}, btn, 1, nil)
	case "drag":
		return a.Drag(geom.Point{X: atoi(args[1]), Y: atoi(args[2])}, geom.Point{X: atoi(args[3]), Y: atoi(args[4])}, platform.ButtonLeft, 300*time.Millisecond)
	case "type":
		time.Sleep(2 * time.Second) // time to focus a text field
		return a.Type(strings.Join(args[1:], " "), "auto", 0)
	case "key":
		c, err := input.ParseChord(args[1])
		if err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
		return a.Chord(c, 0)
	}
	return fmt.Errorf("unknown input subcommand %q", args[0])
}
```
Plus `cmd/cu/cmd_input_other.go` (`//go:build !windows`).

Manual check (do these for real):
1. `go run ./cmd/cu input move 500 500` → cursor jumps to 500,500 (check with `cu doctor`, cursor line).
2. Open Notepad, run `go run ./cmd/cu input type "Привет, мир! 🙂"` and click into Notepad within 2 s → text appears including Cyrillic and the emoji.
3. `go run ./cmd/cu input key ctrl+a` (focus Notepad) → text selected.
4. On a two-monitor setup, `cu input move` to a coordinate on the second monitor works (negative or > primary width).

- [ ] **Step 9: Commit**

```bash
go build ./... && go vet ./... && go test ./...
git add -A && git commit -m "feat: SendInput mouse/keyboard, chord parser, clipboard, action orchestration"
```

---

### Task 6: Window enumeration, focus, state, move, close

**Files:**
- Create: `internal/win/window.go`, `internal/window/window_windows.go`, `internal/window/filter.go`, `internal/window/filter_test.go`

**Interfaces:**
- Consumes: `win.RECT`, `win.callErr`, `win.KeyEvent`, `input.VK_MENU`.
- Produces:
  - `win.RawWindow{HWND uintptr; Title, Class string; PID, TID uint32; Rect RECT; Minimized, Maximized bool}`, `win.EnumTopLevelWindows() ([]RawWindow, error)` (visible, titled, not cloaked, not `WS_EX_TOOLWINDOW`), `win.ForegroundWindow() uintptr`, `win.ProcessImageName(pid uint32) string` (lower-case base name), `win.FocusWindow(hwnd uintptr) error`, `win.ShowWindowCmd(hwnd uintptr, cmd int32)`, `win.SetWindowRect(hwnd uintptr, x, y, w, h int) error`, `win.CloseWindow(hwnd uintptr) error`, `win.WindowFrameRect(hwnd uintptr) (RECT, bool)`, constants `win.SW_RESTORE=9, SW_MINIMIZE=6, SW_MAXIMIZE=3, SW_SHOWNOACTIVATE=4`.
  - `window.New() *Windows` (platform.Windows); `window.ExcludeClassPrefix = "CuOverlay"` (the overlay's window class prefix, filtered out of `List`).
  - `window.Match(wins []platform.WindowInfo, target string) (platform.WindowInfo, error)` — pure resolver: `"foreground"`, a decimal HWND, or a case-insensitive regexp on title or process (errors when nothing matches; ambiguous matches prefer the foreground window, then the first).

- [ ] **Step 1: Failing test for the target resolver**

`internal/window/filter_test.go`:
```go
package window

import (
	"testing"

	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

var wins = []platform.WindowInfo{
	{ID: 100, Title: "Untitled - Notepad", Process: "notepad.exe"},
	{ID: 200, Title: "Downloads - File Explorer", Process: "explorer.exe", Foreground: true},
	{ID: 300, Title: "GitHub - Google Chrome", Process: "chrome.exe"},
}

func TestMatchByIDForegroundAndRegex(t *testing.T) {
	if w, _ := Match(wins, "300"); w.ID != 300 {
		t.Fatalf("id match failed: %+v", w)
	}
	if w, _ := Match(wins, "foreground"); w.ID != 200 {
		t.Fatalf("foreground match failed: %+v", w)
	}
	if w, _ := Match(wins, "notepad"); w.ID != 100 {
		t.Fatalf("regex on title/process failed: %+v", w)
	}
	if w, _ := Match(wins, "(?i)chrome$"); w.ID != 300 {
		t.Fatalf("regex with flags failed: %+v", w)
	}
	if _, err := Match(wins, "nothing-here"); err == nil {
		t.Fatalf("no match must error")
	}
	if w, _ := Match(wins, "e"); w.ID != 200 {
		t.Fatalf("ambiguous match must prefer the foreground window: %+v", w)
	}
}
```
Run: `go test ./internal/window/` → FAIL.

- [ ] **Step 2: Implement filter.go**

`internal/window/filter.go`:
```go
// Package window implements platform.Windows and window target resolution.
package window

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

// ExcludeClassPrefix hides cu's own overlay windows from List.
const ExcludeClassPrefix = "CuOverlay"

// Match resolves "foreground", a decimal HWND, or a case-insensitive regexp over title and process.
func Match(wins []platform.WindowInfo, target string) (platform.WindowInfo, error) {
	if target == "" || target == "foreground" {
		for _, w := range wins {
			if w.Foreground {
				return w, nil
			}
		}
		return platform.WindowInfo{}, fmt.Errorf("no foreground window")
	}
	if id, err := strconv.ParseUint(target, 10, 64); err == nil {
		for _, w := range wins {
			if w.ID == uintptr(id) {
				return w, nil
			}
		}
		return platform.WindowInfo{}, fmt.Errorf("no window with id %d (call windows to list them)", id)
	}
	re, err := regexp.Compile("(?i)" + target)
	if err != nil {
		return platform.WindowInfo{}, fmt.Errorf("bad window pattern %q: %v", target, err)
	}
	var first *platform.WindowInfo
	for i := range wins {
		w := &wins[i]
		if re.MatchString(w.Title) || re.MatchString(w.Process) {
			if w.Foreground {
				return *w, nil
			}
			if first == nil {
				first = w
			}
		}
	}
	if first == nil {
		return platform.WindowInfo{}, fmt.Errorf("no window matches %q (call windows to list them)", target)
	}
	return *first, nil
}
```
Run: `go test ./internal/window/` → PASS.

- [ ] **Step 3: Win32 window bindings**

`internal/win/window.go`:
```go
//go:build windows

package win

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procGetClassNameW            = user32.NewProc("GetClassNameW")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetWindowLongPtrW        = user32.NewProc("GetWindowLongPtrW")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop         = user32.NewProc("BringWindowToTop")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procIsIconic                 = user32.NewProc("IsIconic")
	procIsZoomed                 = user32.NewProc("IsZoomed")
	procShowWindow               = user32.NewProc("ShowWindow")
	procSetWindowPos             = user32.NewProc("SetWindowPos")
	procPostMessageW             = user32.NewProc("PostMessageW")
	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
	procGetCurrentThreadId       = kernel32.NewProc("GetCurrentThreadId")
	procDwmGetWindowAttribute    = dwmapi.NewProc("DwmGetWindowAttribute")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
)

const (
	gwlExStyle       = ^uintptr(19) // -20
	wsExToolWindow   = 0x00000080
	dwmwaCloaked     = 14
	dwmwaFrameBounds = 9
	wmClose          = 0x0010

	SW_MAXIMIZE       = 3
	SW_SHOWNOACTIVATE = 4
	SW_MINIMIZE       = 6
	SW_RESTORE        = 9

	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
)

type RawWindow struct {
	HWND      uintptr
	Title     string
	Class     string
	PID, TID  uint32
	Rect      RECT
	Minimized bool
	Maximized bool
}

func windowText(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}

func className(hwnd uintptr) string {
	var buf [256]uint16
	procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf[:])
}

func isCloaked(hwnd uintptr) bool {
	var cloaked uint32
	procDwmGetWindowAttribute.Call(hwnd, dwmwaCloaked, uintptr(unsafe.Pointer(&cloaked)), 4)
	return cloaked != 0
}

// WindowFrameRect returns the visible frame bounds (without the invisible resize borders).
func WindowFrameRect(hwnd uintptr) (RECT, bool) {
	var r RECT
	if hr, _, _ := procDwmGetWindowAttribute.Call(hwnd, dwmwaFrameBounds, uintptr(unsafe.Pointer(&r)), unsafe.Sizeof(r)); hr == 0 {
		return r, true
	}
	if ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ok != 0 {
		return r, false
	}
	return RECT{}, false
}

var (
	enumWinMu  sync.Mutex
	enumWinOut []RawWindow
	enumWinCb  = windows.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		if v, _, _ := procIsWindowVisible.Call(hwnd); v == 0 {
			return 1
		}
		ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
		if ex&wsExToolWindow != 0 || isCloaked(hwnd) {
			return 1
		}
		title := windowText(hwnd)
		if title == "" {
			return 1
		}
		var pid uint32
		tid, _, _ := procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		rect, _ := WindowFrameRect(hwnd)
		iconic, _, _ := procIsIconic.Call(hwnd)
		zoomed, _, _ := procIsZoomed.Call(hwnd)
		enumWinOut = append(enumWinOut, RawWindow{
			HWND: hwnd, Title: title, Class: className(hwnd), PID: pid, TID: uint32(tid),
			Rect: rect, Minimized: iconic != 0, Maximized: zoomed != 0,
		})
		return 1
	})
)

// EnumTopLevelWindows lists visible, titled, non-tool, non-cloaked top-level windows in Z order.
func EnumTopLevelWindows() ([]RawWindow, error) {
	enumWinMu.Lock()
	defer enumWinMu.Unlock()
	enumWinOut = nil
	r, _, e := procEnumWindows.Call(enumWinCb, 0)
	if err := callErr("EnumWindows", r, e); err != nil {
		return nil, err
	}
	out := make([]RawWindow, len(enumWinOut))
	copy(out, enumWinOut)
	return out, nil
}

func ForegroundWindow() uintptr {
	h, _, _ := procGetForegroundWindow.Call()
	return h
}

// ProcessImageName returns the lower-case executable name for a PID ("notepad.exe"), or "".
func ProcessImageName(pid uint32) string {
	h, _, _ := procOpenProcess.Call(0x1000 /* PROCESS_QUERY_LIMITED_INFORMATION */, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)
	var buf [1024]uint16
	size := uint32(len(buf))
	if r, _, _ := procQueryFullProcessImageNameW.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r == 0 {
		return ""
	}
	return strings.ToLower(filepath.Base(windows.UTF16ToString(buf[:size])))
}

func ShowWindowCmd(hwnd uintptr, cmd int32) { procShowWindow.Call(hwnd, uintptr(cmd)) }

// FocusWindow brings hwnd to the foreground, working around SetForegroundWindow's restrictions.
func FocusWindow(hwnd uintptr) error {
	if iconic, _, _ := procIsIconic.Call(hwnd); iconic != 0 {
		ShowWindowCmd(hwnd, SW_RESTORE)
		time.Sleep(50 * time.Millisecond)
	}
	try := func() bool {
		procSetForegroundWindow.Call(hwnd)
		procBringWindowToTop.Call(hwnd)
		time.Sleep(30 * time.Millisecond)
		return ForegroundWindow() == hwnd
	}
	if try() {
		return nil
	}
	// Trick 1: a synthetic Alt press marks our process as "last input", unlocking SetForegroundWindow.
	KeyEvent(0x12, MapVirtualKeyToScan(0x12), 0)
	KeyEvent(0x12, MapVirtualKeyToScan(0x12), KeyEventfKeyUp)
	if try() {
		return nil
	}
	// Trick 2: attach our input queue to the current foreground thread.
	fg := ForegroundWindow()
	var pid uint32
	fgTid, _, _ := procGetWindowThreadProcessId.Call(fg, uintptr(unsafe.Pointer(&pid)))
	cur, _, _ := procGetCurrentThreadId.Call()
	if fgTid != 0 && fgTid != cur {
		procAttachThreadInput.Call(cur, fgTid, 1)
		ok := try()
		procAttachThreadInput.Call(cur, fgTid, 0)
		if ok {
			return nil
		}
	}
	return fmt.Errorf("could not bring window %d to the foreground (Windows only flashed it in the taskbar)", hwnd)
}

// SetWindowRect moves/resizes so the *visible frame* matches (x,y,w,h), compensating the DWM border.
func SetWindowRect(hwnd uintptr, x, y, w, h int) error {
	var wr RECT
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	fr, ok := WindowFrameRect(hwnd)
	dl, dt, dr, db := int32(0), int32(0), int32(0), int32(0)
	if ok {
		dl, dt, dr, db = fr.Left-wr.Left, fr.Top-wr.Top, wr.Right-fr.Right, wr.Bottom-fr.Bottom
	}
	r, _, e := procSetWindowPos.Call(hwnd, 0,
		uintptr(int32(x)-dl), uintptr(int32(y)-dt), uintptr(int32(w)+dl+dr), uintptr(int32(h)+dt+db),
		swpNoZOrder|swpNoActivate)
	return callErr("SetWindowPos", r, e)
}

func CloseWindow(hwnd uintptr) error {
	r, _, e := procPostMessageW.Call(hwnd, wmClose, 0, 0)
	return callErr("PostMessage(WM_CLOSE)", r, e)
}
```

- [ ] **Step 4: platform.Windows implementation**

`internal/window/window_windows.go`:
```go
//go:build windows

package window

import (
	"strings"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

type Windows struct{}

func New() *Windows { return &Windows{} }

func toInfo(w win.RawWindow, fg uintptr) platform.WindowInfo {
	st := platform.WindowNormal
	if w.Minimized {
		st = platform.WindowMinimized
	} else if w.Maximized {
		st = platform.WindowMaximized
	}
	return platform.WindowInfo{
		ID: w.HWND, Title: w.Title, Process: win.ProcessImageName(w.PID), PID: w.PID,
		Rect:  geom.Rect{X: int(w.Rect.Left), Y: int(w.Rect.Top), W: int(w.Rect.Width()), H: int(w.Rect.Height())},
		State: st, Foreground: w.HWND == fg,
	}
}

func (*Windows) List() ([]platform.WindowInfo, error) {
	raw, err := win.EnumTopLevelWindows()
	if err != nil {
		return nil, err
	}
	fg := win.ForegroundWindow()
	out := make([]platform.WindowInfo, 0, len(raw))
	for _, w := range raw {
		if strings.HasPrefix(w.Class, ExcludeClassPrefix) {
			continue
		}
		out = append(out, toInfo(w, fg))
	}
	return out, nil
}

func (ws *Windows) Foreground() (platform.WindowInfo, error) {
	list, err := ws.List()
	if err != nil {
		return platform.WindowInfo{}, err
	}
	return Match(list, "foreground")
}

func (*Windows) Focus(id uintptr) error { return win.FocusWindow(id) }

func (*Windows) SetState(id uintptr, s platform.WindowState) error {
	switch s {
	case platform.WindowMinimized:
		win.ShowWindowCmd(id, win.SW_MINIMIZE)
	case platform.WindowMaximized:
		win.ShowWindowCmd(id, win.SW_MAXIMIZE)
	default:
		win.ShowWindowCmd(id, win.SW_RESTORE)
	}
	return nil
}

func (*Windows) Close(id uintptr) error            { return win.CloseWindow(id) }
func (*Windows) Move(id uintptr, r geom.Rect) error { return win.SetWindowRect(id, r.X, r.Y, r.W, r.H) }

var _ platform.Windows = (*Windows)(nil)
```

- [ ] **Step 5: Extend `cu doctor` with a window list and check manually**

In `cmd/cu/cmd_doctor.go`, after the cursor line add:
```go
	wl, err := window.New().List()
	if err != nil {
		return err
	}
	fmt.Printf("windows: %d\n", len(wl))
	for _, w := range wl {
		mark := " "
		if w.Foreground {
			mark = "*"
		}
		fmt.Printf(" %s %d %-20s %-9s %v %q\n", mark, w.ID, w.Process, w.State, w.Rect, w.Title)
	}
```
(import `github.com/racass-pixel/claude-computer-use/internal/window`). Run `go run ./cmd/cu doctor`: the terminal window is marked `*`; Notepad shows as `notepad.exe`; no invisible/cloaked windows (e.g. no "Settings" if it is not open). Rects must match the visible frames.

- [ ] **Step 6: Commit**

```bash
go build ./... && go vet ./... && go test ./...
git add -A && git commit -m "feat: window enumeration, focus workarounds, state and move"
```

---

### Task 7: Configuration

**Files:**
- Create: `internal/config/config.go`, `internal/config/config_test.go`

**Interfaces:**
- Produces:
```go
type Config struct {
	Hotkey             string `json:"hotkey"`               // "esc esc" | "ctrl+alt+esc"
	AutoPause          bool   `json:"auto_pause"`
	MouseThresholdPx   int    `json:"mouse_threshold_px"`
	ScreenshotLongEdge int    `json:"screenshot_long_edge"`
	ScreenshotFormat   string `json:"screenshot_format"`    // png | jpeg
	JPEGQuality        int    `json:"jpeg_quality"`
	Lang               string `json:"lang"`                 // auto | ru | en
	Accent             string `json:"accent"`               // "#D97757"
	Overlay            bool   `json:"overlay"`
	IdleReleaseMs      int    `json:"idle_release_ms"`
	PauseWaitMs        int    `json:"pause_wait_ms"`
	PasteThreshold     int    `json:"paste_threshold"`
	LogFile            string `json:"log_file"`             // "" = stderr only
}
func Default() Config
func Path() (string, error)                      // %APPDATA%\claude-computer-use\config.json
func Load() (Config, error)                      // Default ← file (if exists) ← env CU_*
func applyFile(c *Config, data []byte) error
func applyEnv(c *Config, getenv func(string) string) error
func (c Config) AccentRGB() (r, g, b uint8, err error)
```
  Env names: `CU_HOTKEY`, `CU_AUTO_PAUSE`, `CU_MOUSE_THRESHOLD_PX`, `CU_SCREENSHOT_LONG_EDGE`, `CU_SCREENSHOT_FORMAT`, `CU_JPEG_QUALITY`, `CU_LANG`, `CU_ACCENT`, `CU_OVERLAY`, `CU_IDLE_RELEASE_MS`, `CU_PAUSE_WAIT_MS`, `CU_PASTE_THRESHOLD`, `CU_LOG_FILE`.

- [ ] **Step 1: Failing tests**

`internal/config/config_test.go`:
```go
package config

import "testing"

func TestDefaults(t *testing.T) {
	c := Default()
	if c.Hotkey != "esc esc" || !c.AutoPause || c.MouseThresholdPx != 12 || c.ScreenshotLongEdge != 1366 ||
		c.ScreenshotFormat != "png" || c.JPEGQuality != 85 || c.Lang != "auto" || c.Accent != "#D97757" ||
		!c.Overlay || c.IdleReleaseMs != 120000 || c.PauseWaitMs != 20000 || c.PasteThreshold != 200 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestFileThenEnvOverride(t *testing.T) {
	c := Default()
	if err := applyFile(&c, []byte(`{"hotkey":"ctrl+alt+esc","screenshot_long_edge":1024,"overlay":false}`)); err != nil {
		t.Fatal(err)
	}
	if c.Hotkey != "ctrl+alt+esc" || c.ScreenshotLongEdge != 1024 || c.Overlay || c.AutoPause != true {
		t.Fatalf("file merge wrong: %+v", c)
	}
	env := map[string]string{"CU_SCREENSHOT_LONG_EDGE": "1568", "CU_AUTO_PAUSE": "false", "CU_LANG": "ru", "CU_ACCENT": "#3366FF"}
	if err := applyEnv(&c, func(k string) string { return env[k] }); err != nil {
		t.Fatal(err)
	}
	if c.ScreenshotLongEdge != 1568 || c.AutoPause || c.Lang != "ru" || c.Accent != "#3366FF" {
		t.Fatalf("env override wrong: %+v", c)
	}
	r, g, b, err := c.AccentRGB()
	if err != nil || r != 0x33 || g != 0x66 || b != 0xFF {
		t.Fatalf("AccentRGB = %d %d %d %v", r, g, b, err)
	}
}

func TestInvalidValues(t *testing.T) {
	c := Default()
	if err := applyEnv(&c, func(k string) string { if k == "CU_SCREENSHOT_LONG_EDGE" { return "abc" }; return "" }); err == nil {
		t.Fatal("non-numeric int must error")
	}
	c.Accent = "red"
	if _, _, _, err := c.AccentRGB(); err == nil {
		t.Fatal("accent must be #RRGGBB")
	}
	if err := applyFile(&c, []byte(`{"screenshot_format":"bmp"}`)); err == nil {
		t.Fatal("unknown format must error")
	}
}
```
Run: `go test ./internal/config/` → FAIL.

- [ ] **Step 2: Implement config.go**

`internal/config/config.go`:
```go
// Package config loads cu settings: defaults ← %APPDATA%\claude-computer-use\config.json ← CU_* env.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Hotkey             string `json:"hotkey"`
	AutoPause          bool   `json:"auto_pause"`
	MouseThresholdPx   int    `json:"mouse_threshold_px"`
	ScreenshotLongEdge int    `json:"screenshot_long_edge"`
	ScreenshotFormat   string `json:"screenshot_format"`
	JPEGQuality        int    `json:"jpeg_quality"`
	Lang               string `json:"lang"`
	Accent             string `json:"accent"`
	Overlay            bool   `json:"overlay"`
	IdleReleaseMs      int    `json:"idle_release_ms"`
	PauseWaitMs        int    `json:"pause_wait_ms"`
	PasteThreshold     int    `json:"paste_threshold"`
	LogFile            string `json:"log_file"`
}

func Default() Config {
	return Config{
		Hotkey: "esc esc", AutoPause: true, MouseThresholdPx: 12,
		ScreenshotLongEdge: 1366, ScreenshotFormat: "png", JPEGQuality: 85,
		Lang: "auto", Accent: "#D97757", Overlay: true,
		IdleReleaseMs: 120000, PauseWaitMs: 20000, PasteThreshold: 200,
	}
}

func Path() (string, error) {
	dir, err := os.UserConfigDir() // %APPDATA% on Windows
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "claude-computer-use", "config.json"), nil
}

// Load never fails on a missing file; a malformed file or env value is an error.
func Load() (Config, error) {
	c := Default()
	p, err := Path()
	if err == nil {
		if data, rerr := os.ReadFile(p); rerr == nil {
			if err := applyFile(&c, data); err != nil {
				return c, fmt.Errorf("%s: %w", p, err)
			}
		} else if !errors.Is(rerr, os.ErrNotExist) {
			return c, rerr
		}
	}
	if err := applyEnv(&c, os.Getenv); err != nil {
		return c, err
	}
	return c, c.validate()
}

func applyFile(c *Config, data []byte) error {
	if err := json.Unmarshal(data, c); err != nil {
		return err
	}
	return c.validate()
}

func applyEnv(c *Config, getenv func(string) string) error {
	str := func(key string, dst *string) {
		if v := getenv(key); v != "" {
			*dst = v
		}
	}
	num := func(key string, dst *int) error {
		v := getenv(key)
		if v == "" {
			return nil
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s: %q is not a number", key, v)
		}
		*dst = n
		return nil
	}
	boolean := func(key string, dst *bool) error {
		v := getenv(key)
		if v == "" {
			return nil
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("%s: %q is not a boolean", key, v)
		}
		*dst = b
		return nil
	}
	str("CU_HOTKEY", &c.Hotkey)
	str("CU_SCREENSHOT_FORMAT", &c.ScreenshotFormat)
	str("CU_LANG", &c.Lang)
	str("CU_ACCENT", &c.Accent)
	str("CU_LOG_FILE", &c.LogFile)
	for _, e := range []error{
		boolean("CU_AUTO_PAUSE", &c.AutoPause),
		boolean("CU_OVERLAY", &c.Overlay),
		num("CU_MOUSE_THRESHOLD_PX", &c.MouseThresholdPx),
		num("CU_SCREENSHOT_LONG_EDGE", &c.ScreenshotLongEdge),
		num("CU_JPEG_QUALITY", &c.JPEGQuality),
		num("CU_IDLE_RELEASE_MS", &c.IdleReleaseMs),
		num("CU_PAUSE_WAIT_MS", &c.PauseWaitMs),
		num("CU_PASTE_THRESHOLD", &c.PasteThreshold),
	} {
		if e != nil {
			return e
		}
	}
	return c.validate()
}

func (c Config) validate() error {
	switch c.ScreenshotFormat {
	case "png", "jpeg", "jpg":
	default:
		return fmt.Errorf("screenshot_format must be png or jpeg, got %q", c.ScreenshotFormat)
	}
	switch c.Lang {
	case "auto", "ru", "en":
	default:
		return fmt.Errorf("lang must be auto, ru or en, got %q", c.Lang)
	}
	return nil
}

// AccentRGB parses "#RRGGBB".
func (c Config) AccentRGB() (r, g, b uint8, err error) {
	s := strings.TrimPrefix(c.Accent, "#")
	if len(s) != 6 {
		return 0, 0, 0, fmt.Errorf("accent must be #RRGGBB, got %q", c.Accent)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("accent must be #RRGGBB, got %q", c.Accent)
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v), nil
}
```
Run: `go test ./internal/config/` → PASS.

- [ ] **Step 3: Commit**

```bash
go build ./... && go vet ./... && go test ./...
git add -A && git commit -m "feat: config with file and env overrides"
```

---

### Task 8: MCP server core — session, view state, `screenshot` and `monitors`, `cu serve`

**Files:**
- Create: `internal/server/server.go`, `internal/server/respond.go`, `internal/server/session.go`, `internal/server/tools_screen.go`, `internal/server/server_test.go`
- Create: `cmd/cu/cmd_serve.go`, `cmd/cu/cmd_serve_other.go`; Modify: `cmd/cu/stubs.go` (remove `runServe`)

**Interfaces:**
- Consumes: `platform.*`, `screen.View/NewView/AutoScale/ZoomScale/Grab/Shot/ActiveMonitor/VirtualScreen/MonitorByID`, `config.Config`, `actions.Actor`, `fake.*`.
- Produces:
```go
package server

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

type Session struct { /* unexported */ }
func New(d Deps, cfg config.Config, logger *log.Logger) *Session
func (s *Session) Register(srv *mcp.Server)
func Run(ctx context.Context, d Deps, cfg config.Config, logger *log.Logger) error

// respond.go
type ForegroundInfo struct { ID uintptr `json:"id"`; Title string `json:"title"`; Process string `json:"process"` }
type Meta struct {
	Monitor    int             `json:"monitor"`
	Image      [2]int          `json:"image"`
	Scale      float64         `json:"scale"`
	Screen     geom.Rect       `json:"screen"`
	Cursor     [2]int          `json:"cursor"`
	Foreground *ForegroundInfo `json:"foreground,omitempty"`
	Paused     bool            `json:"paused,omitempty"`
}
func (m Meta) into(fields map[string]any)
func jsonText(v any) *mcp.TextContent
func okResult(fields map[string]any, shot *screen.Shot) *mcp.CallToolResult   // sets "ok": true unless already set
func errResult(code, msg string) *mcp.CallToolResult                          // IsError = true

// session.go
type captureSpec struct { monitor string; region *RegionIn; scale float64; format string }
type RegionIn struct { X int `json:"x"`; Y int `json:"y"`; W int `json:"w"`; H int `json:"h"` }
func (s *Session) capture(spec captureSpec) (*screen.Shot, Meta, error)  // updates the view
func (s *Session) currentView() screen.View                             // default = active monitor at auto scale
func (s *Session) metaFor(v screen.View) Meta
func (s *Session) monitors() ([]platform.Monitor, error)                 // cached for 2 s
func (s *Session) foreground() platform.WindowInfo                       // zero value if none
func (s *Session) logTiming(tool string, t0 time.Time)
```

- [ ] **Step 1: Failing server test (screenshot sets the view and returns an image)**

`internal/server/server_test.go`:
```go
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
```
Run: `go get github.com/modelcontextprotocol/go-sdk@latest` then `go test ./internal/server/` → FAIL.

- [ ] **Step 2: respond.go**

`internal/server/respond.go`:
```go
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
```

- [ ] **Step 3: server.go and session.go**

`internal/server/server.go`:
```go
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

type Controller interface {
	IsPaused() bool
	Acquire(now time.Time)
	Touch(now time.Time)
	Release(now time.Time)
	WaitResume(ctx context.Context) bool
	Status() ControllerStatus
}

type ControllerStatus struct {
	State  string `json:"state"`
	Hotkey string `json:"hotkey"`
	IdleMs int64  `json:"idle_ms"`
}

type Deps struct {
	Screen     platform.Screen
	Input      platform.Input
	Clip       platform.Clipboard
	Wins       platform.Windows
	Access     platform.Accessibility
	Overlay    platform.Overlay
	Controller Controller
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
```

`internal/server/session.go`:
```go
package server

import (
	"fmt"
	"strconv"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/screen"
)

type RegionIn struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type captureSpec struct {
	monitor string
	region  *RegionIn
	scale   float64
	format  string
}

func (s *Session) monitors() ([]platform.Monitor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mons != nil && time.Since(s.monsAt) < 2*time.Second {
		return s.mons, nil
	}
	mons, err := s.d.Screen.Monitors()
	if err != nil {
		return nil, err
	}
	if len(mons) == 0 {
		return nil, fmt.Errorf("no monitors found")
	}
	s.mons, s.monsAt = mons, time.Now()
	return mons, nil
}

func (s *Session) foreground() platform.WindowInfo {
	if s.d.Wins == nil {
		return platform.WindowInfo{}
	}
	w, err := s.d.Wins.Foreground()
	if err != nil {
		return platform.WindowInfo{}
	}
	return w
}

func (s *Session) cursor() geom.Point {
	p, _ := s.d.Screen.CursorPos()
	return p
}

func (s *Session) activeMonitor() platform.Monitor {
	mons, err := s.monitors()
	if err != nil {
		return platform.Monitor{}
	}
	return screen.ActiveMonitor(mons, s.foreground().Rect, s.cursor())
}

// currentView is the coordinate space tool inputs are in right now.
func (s *Session) currentView() screen.View {
	s.mu.Lock()
	if s.hasView {
		v := s.view
		s.mu.Unlock()
		return v
	}
	s.mu.Unlock()
	m := s.activeMonitor()
	return screen.NewView(m.ID, m.Rect, screen.AutoScale(m.Rect, s.cfg.ScreenshotLongEdge))
}

func (s *Session) setView(v screen.View) {
	s.mu.Lock()
	s.view, s.hasView = v, true
	s.mu.Unlock()
}

func (s *Session) metaFor(v screen.View) Meta {
	m := Meta{Monitor: v.Monitor, Image: [2]int{v.Image.W, v.Image.H}, Scale: v.Scale, Screen: v.Screen}
	c := v.ToImage(s.cursor())
	m.Cursor = [2]int{c.X, c.Y}
	if fg := s.foreground(); fg.ID != 0 {
		m.Foreground = &ForegroundInfo{ID: fg.ID, Title: fg.Title, Process: fg.Process}
	}
	if s.d.Controller != nil && s.d.Controller.IsPaused() {
		m.Paused = true
	}
	return m
}

// capture takes a screenshot per spec and makes it the current view.
func (s *Session) capture(spec captureSpec) (*screen.Shot, Meta, error) {
	mons, err := s.monitors()
	if err != nil {
		return nil, Meta{}, err
	}
	var rect geom.Rect
	monID := 0
	scale := spec.scale
	switch {
	case spec.region != nil:
		v := s.currentView()
		r := geom.Rect{X: spec.region.X, Y: spec.region.Y, W: spec.region.W, H: spec.region.H}
		rect = v.RectToScreen(r).Intersect(v.Screen)
		if rect.Empty() {
			return nil, Meta{}, fmt.Errorf("region %+v is outside the current view (%dx%d)", r, v.Image.W, v.Image.H)
		}
		monID = v.Monitor
		if scale <= 0 {
			scale = screen.ZoomScale(rect, s.cfg.ScreenshotLongEdge, 2)
		}
	case spec.monitor == "all":
		rect = screen.VirtualScreen(mons)
		if scale <= 0 {
			scale = screen.AutoScale(rect, s.cfg.ScreenshotLongEdge*3/2)
		}
	case spec.monitor == "" || spec.monitor == "active":
		m := screen.ActiveMonitor(mons, s.foreground().Rect, s.cursor())
		rect, monID = m.Rect, m.ID
		if scale <= 0 {
			scale = screen.AutoScale(rect, s.cfg.ScreenshotLongEdge)
		}
	default:
		id, perr := strconv.Atoi(spec.monitor)
		m, ok := screen.MonitorByID(mons, id)
		if perr != nil || !ok {
			return nil, Meta{}, fmt.Errorf("unknown monitor %q (use active, all, or an id from monitors)", spec.monitor)
		}
		rect, monID = m.Rect, m.ID
		if scale <= 0 {
			scale = screen.AutoScale(rect, s.cfg.ScreenshotLongEdge)
		}
	}
	format := spec.format
	if format == "" {
		format = s.cfg.ScreenshotFormat
	}
	shot, err := screen.Grab(s.d.Screen, rect, scale, format, s.cfg.JPEGQuality)
	if err != nil {
		return nil, Meta{}, err
	}
	v := screen.NewView(monID, rect, scale)
	s.setView(v)
	return shot, s.metaFor(v), nil
}

func (s *Session) logTiming(tool string, t0 time.Time) {
	s.log.Printf("tool=%s ms=%d", tool, time.Since(t0).Milliseconds())
}
```

- [ ] **Step 4: tools_screen.go**

```go
package server

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
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
	shot, meta, err := s.capture(captureSpec{monitor: in.Monitor, region: in.Region, scale: in.Scale, format: in.Format})
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

var _ = geom.Point{}
```
(remove the `var _ = geom.Point{}` line if `geom` ends up unused). Run: `go test ./internal/server/` → PASS.

- [ ] **Step 5: Schema sanity test**

Append to `server_test.go`:
```go
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
	for _, tool := range tools.Tools {
		if tool.Name == "screenshot" {
			b, _ := json.Marshal(tool.InputSchema)
			if bytes.Contains(b, []byte(`"required"`)) {
				t.Fatalf("screenshot schema must have no required fields: %s", b)
			}
		}
	}
}
```
Adjust the in-memory transport helper name to whatever the installed go-sdk version exports (`mcp.NewInMemoryTransports` in v1.x); check with `go doc github.com/modelcontextprotocol/go-sdk/mcp NewInMemoryTransports`. Run: `go test ./internal/server/` → PASS.

- [ ] **Step 6: `cu serve`**

`cmd/cu/cmd_serve.go` (delete `runServe` from stubs):
```go
//go:build windows

package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"

	"github.com/racass-pixel/claude-computer-use/internal/config"
	"github.com/racass-pixel/claude-computer-use/internal/input"
	"github.com/racass-pixel/claude-computer-use/internal/screen"
	"github.com/racass-pixel/claude-computer-use/internal/server"
	"github.com/racass-pixel/claude-computer-use/internal/win"
	"github.com/racass-pixel/claude-computer-use/internal/window"
)

func runServe(args []string) error {
	if err := win.SetPerMonitorDPIAwareV2(); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var w io.Writer = os.Stderr
	if cfg.LogFile != "" {
		if f, ferr := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); ferr == nil {
			defer f.Close()
			w = io.MultiWriter(os.Stderr, f)
		}
	}
	logger := log.New(w, "cu: ", log.Ltime|log.Lmicroseconds)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	deps := server.Deps{
		Screen:  screen.New(),
		Input:   input.New(),
		Clip:    input.NewClipboard(),
		Wins:    window.New(),
		Version: version,
	}
	return server.Run(ctx, deps, cfg, logger)
}
```
`cmd/cu/cmd_serve_other.go` returns an error on non-Windows.

- [ ] **Step 7: Smoke test inside Claude Code**

1. `powershell -NoProfile -File scripts/build.ps1`.
2. In a new terminal: `claude --plugin-dir C:\Users\test\Desktop\claude-computer-use`, then `/mcp` → server `desktop` is connected with tools `screenshot` and `monitors`.
3. Ask: "Возьми скриншот активного монитора и скажи, что на нём" → Claude calls the tool, sees the image, describes it correctly; "Сколько у меня мониторов?" → uses `monitors`.
4. Check stderr timing lines in the MCP log (`/mcp` → desktop → view logs) — `tool=screenshot ms=<n>` under 100 ms on 1080p.

- [ ] **Step 8: Commit**

```bash
go build ./... && go vet ./... && go test ./...
git add -A && git commit -m "feat: MCP server core with screenshot and monitors tools"
```

---

### Task 9: Action tools (click/move/drag/scroll/type/key/clipboard/windows/window), `batch`, `wait`, `control`

**Files:**
- Create: `internal/server/tools_mouse.go`, `internal/server/tools_keyboard.go`, `internal/server/tools_window.go`, `internal/server/tools_batch.go`, `internal/server/tools_batch_test.go`, `internal/server/tools_wait.go`, `internal/server/tools_wait_test.go`, `internal/server/tools_control.go`, `internal/server/tools_test.go`
- Modify: `internal/server/session.go` (add `begin`, `finish`, `resolvePoint`, `settle`), `internal/server/server.go` (`Register` adds the tools; `batchable` registry), `internal/input/keys.go` (add `ModifierVK`)

**Interfaces:**
- Consumes: everything from Task 8, `actions.Actor`, `input.ParseChord`, `window.Match`.
- Produces:
```go
// input
func ModifierVK(name string) (uint16, bool)   // "ctrl"|"control"|"alt"|"shift"|"win"|"cmd"… → VK

// server/session.go
// begin gates an action on the Controller and shows the overlay. A non-nil result must be returned to the caller as-is.
func (s *Session) begin(ctx context.Context, action, summary string) *mcp.CallToolResult
// finish sleeps settle, optionally captures, and builds the standard result.
func (s *Session) finish(action string, t0 time.Time, extra map[string]any, withShot bool, settle time.Duration) *mcp.CallToolResult
func (s *Session) wantShot(p *bool) bool                        // nil → true
func (s *Session) resolvePoint(x, y *int, element string) (geom.Point, error) // image space or element → screen
func (s *Session) settleFor(action string) time.Duration        // click 100ms, type 60ms, key 100ms, scroll 120ms, drag 150ms, window 200ms

// server/server.go
type batchFn func(ctx context.Context, args json.RawMessage) (*mcp.CallToolResult, error)
func (s *Session) batchable() map[string]batchFn                // click, move, drag, scroll, type, key, wait, window, clipboard (find added in Task 15)
```
Input structs (all optional fields `omitempty`; bool options are `*bool`):
```go
type ClickIn struct {
	X *int `json:"x,omitempty"`; Y *int `json:"y,omitempty"`
	Element string `json:"element,omitempty"`
	Button string `json:"button,omitempty"`      // left|right|middle
	Count int `json:"count,omitempty"`           // 1|2|3
	Modifiers []string `json:"modifiers,omitempty"`
	Screenshot *bool `json:"screenshot,omitempty"`
}
type MoveIn struct { X *int; Y *int; Element string; Screenshot *bool }                       // same tags
type PointIn struct { X *int `json:"x,omitempty"`; Y *int `json:"y,omitempty"`; Element string `json:"element,omitempty"` }
type DragIn struct { From PointIn `json:"from"`; To PointIn `json:"to"`; Button string; DurationMs int `json:"duration_ms,omitempty"`; Screenshot *bool }
type ScrollIn struct { X *int; Y *int; Dx int `json:"dx,omitempty"`; Dy int `json:"dy,omitempty"`; Screenshot *bool }
type TypeIn struct { Text string `json:"text"`; Mode string `json:"mode,omitempty"`; DelayMs int `json:"delay_ms,omitempty"`; Screenshot *bool }
type KeyIn struct { Key string `json:"key,omitempty"`; Keys []string `json:"keys,omitempty"`; HoldMs int `json:"hold_ms,omitempty"`; Screenshot *bool }
type ClipboardIn struct { Action string `json:"action"`; Text string `json:"text,omitempty"` }
type WindowsIn struct { Filter string `json:"filter,omitempty"`; Monitor int `json:"monitor,omitempty"` }
type WindowIn struct { Action string `json:"action"`; Target string `json:"target,omitempty"`; Rect *RegionIn `json:"rect,omitempty"`; Screenshot *bool }
type BatchAction struct { Tool string `json:"tool"`; Args map[string]any `json:"args,omitempty"` }
type BatchIn struct { Actions []BatchAction `json:"actions"`; StopOnError *bool `json:"stop_on_error,omitempty"`; Screenshot *bool }
type WaitIn struct { Ms int `json:"ms,omitempty"`; Window string `json:"window,omitempty"`; Stable *bool `json:"stable,omitempty"`; TimeoutMs int `json:"timeout_ms,omitempty"`; Screenshot *bool }
type ControlIn struct { Action string `json:"action"`; Task string `json:"task,omitempty"`; Note string `json:"note,omitempty"` }
```
(Every field above carries a `jsonschema:"..."` description in the real code; write one sentence each, mentioning the coordinate space for x/y.)

- [ ] **Step 1: Failing tests for point resolution, click mapping, batch and frame diff**

`internal/server/tools_test.go`:
```go
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
```

`internal/server/tools_batch_test.go`:
```go
package server

import (
	"context"
	"strings"
	"testing"
)

func TestBatchRunsInOrderAndTakesOneScreenshot(t *testing.T) {
	h := newHarness(t)
	res, _, _ := h.s.toolBatch(context.Background(), nil, BatchIn{Actions: []BatchAction{
		{Tool: "click", Args: map[string]any{"x": 10, "y": 10}},
		{Tool: "type", Args: map[string]any{"text": "hi"}},
		{Tool: "key", Args: map[string]any{"key": "enter"}},
	}})
	fields, img := decode(t, res)
	if res.IsError || img == nil {
		t.Fatalf("batch failed: %v", fields)
	}
	steps := fields["steps"].([]any)
	if len(steps) != 3 {
		t.Fatalf("steps = %v", steps)
	}
	got := strings.Join(h.in.Calls, "|")
	if !strings.HasPrefix(got, "move ") || !strings.Contains(got, "|type hi|key_down 13|key_up 13") {
		t.Fatalf("calls = %q", got)
	}
}

func TestBatchStopsOnError(t *testing.T) {
	h := newHarness(t)
	res, _, _ := h.s.toolBatch(context.Background(), nil, BatchIn{Actions: []BatchAction{
		{Tool: "nope", Args: nil},
		{Tool: "type", Args: map[string]any{"text": "never"}},
	}})
	fields, _ := decode(t, res)
	if fields["ok"] != false || len(h.in.Calls) != 0 {
		t.Fatalf("batch must stop at the first failing step: %v / %v", fields, h.in.Calls)
	}
	if steps := fields["steps"].([]any); len(steps) != 1 || steps[0].(map[string]any)["ok"] != false {
		t.Fatalf("steps = %v", steps)
	}
}
```

`internal/server/tools_wait_test.go`:
```go
package server

import (
	"image"
	"testing"
)

func TestFrameDiff(t *testing.T) {
	a := image.NewRGBA(image.Rect(0, 0, 10, 10))
	b := image.NewRGBA(image.Rect(0, 0, 10, 10))
	if d := frameDiff(a, b); d != 0 {
		t.Fatalf("identical frames diff = %v", d)
	}
	for i := 0; i < len(b.Pix); i += 4 {
		b.Pix[i] = 255
	}
	if d := frameDiff(a, b); d < 60 {
		t.Fatalf("changed frames diff = %v, want >= 60", d)
	}
}
```
Run: `go test ./internal/server/` → FAIL.

- [ ] **Step 2: session.go additions**

Append to `internal/server/session.go`:
```go
// begin gates an action on the Controller (pause semantics, spec §7) and updates the overlay.
func (s *Session) begin(ctx context.Context, action, summary string) *mcp.CallToolResult {
	now := time.Now()
	if c := s.d.Controller; c != nil {
		if c.IsPaused() {
			wctx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.PauseWaitMs)*time.Millisecond)
			resumed := c.WaitResume(wctx)
			cancel()
			if !resumed {
				return errResult("user_took_control", "The user took control of the computer (hotkey or physical input). Stop now, report what was done and what remains, and wait for the user to ask you to continue.")
			}
			shot, meta, err := s.capture(captureSpec{})
			f := map[string]any{"ok": false, "resumed": true, "note": "The user handed control back. This action was NOT performed; look at the fresh screenshot and continue from the current state."}
			if err == nil {
				meta.into(f)
			}
			return okResult(f, shot)
		}
		c.Acquire(now)
		c.Touch(now)
	}
	s.d.Overlay.SetAction(summary)
	if m := s.activeMonitor(); m.ID != 0 {
		s.d.Overlay.Show(m, platform.OverlayControlling)
	}
	return nil
}

func (s *Session) settleFor(action string) time.Duration {
	switch action {
	case "click", "key":
		return 100 * time.Millisecond
	case "type":
		return 60 * time.Millisecond
	case "scroll":
		return 120 * time.Millisecond
	case "drag":
		return 150 * time.Millisecond
	case "window":
		return 200 * time.Millisecond
	}
	return 50 * time.Millisecond
}

// finish waits for the UI to settle, captures if wanted, and builds the standard result.
func (s *Session) finish(action string, t0 time.Time, extra map[string]any, withShot bool, settle time.Duration) *mcp.CallToolResult {
	f := map[string]any{"action": action}
	for k, v := range extra {
		f[k] = v
	}
	var shot *screen.Shot
	if withShot {
		time.Sleep(settle)
		var err error
		var meta Meta
		shot, meta, err = s.capture(captureSpec{monitor: s.sameMonitorSpec()})
		if err == nil {
			meta.into(f)
		} else {
			f["screenshot_error"] = err.Error()
		}
	} else {
		s.metaFor(s.currentView()).into(f)
	}
	f["ms"] = time.Since(t0).Milliseconds()
	s.logTiming(action, t0)
	return okResult(f, shot)
}

// sameMonitorSpec keeps follow-up screenshots on the monitor of the current view (region views reset to full monitor).
func (s *Session) sameMonitorSpec() string {
	v := s.currentView()
	if v.Monitor == 0 {
		return "all"
	}
	return strconv.Itoa(v.Monitor)
}

func (s *Session) wantShot(p *bool) bool { return p == nil || *p }

// resolvePoint turns image-space x,y or an element id into a screen point.
func (s *Session) resolvePoint(x, y *int, element string) (geom.Point, error) {
	if element != "" {
		s.mu.Lock()
		el, ok := s.elements[element]
		s.mu.Unlock()
		if !ok {
			return geom.Point{}, fmt.Errorf("unknown element %q (ids are valid only until the next find)", element)
		}
		r := el.Rect
		if s.d.Access != nil {
			if fresh, err := s.d.Access.Rect(el.Ref); err == nil && !fresh.Empty() {
				r = fresh
			}
		}
		return r.Center(), nil
	}
	if x == nil || y == nil {
		return geom.Point{}, fmt.Errorf("give x and y (pixels of the last screenshot) or an element id from find")
	}
	v := s.currentView()
	return v.ToScreen(v.ClampImage(geom.Point{X: *x, Y: *y})), nil
}

func parseModifiers(names []string) ([]uint16, error) {
	var out []uint16
	for _, n := range names {
		vk, ok := input.ModifierVK(n)
		if !ok {
			return nil, fmt.Errorf("unknown modifier %q (ctrl, alt, shift, win)", n)
		}
		out = append(out, vk)
	}
	return out, nil
}
```
(add imports `context`, `github.com/modelcontextprotocol/go-sdk/mcp`, `.../internal/input`.) Add to `internal/input/keys.go`:
```go
// ModifierVK maps a modifier name ("ctrl", "control", "alt", "shift", "win", "cmd"...) to its VK.
func ModifierVK(name string) (uint16, bool) {
	vk, ok := modifierNames[strings.ToLower(strings.TrimSpace(name))]
	return vk, ok
}
```

- [ ] **Step 3: tools_mouse.go**

```go
package server

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

type ClickIn struct {
	X          *int     `json:"x,omitempty" jsonschema:"x in pixels of the last screenshot"`
	Y          *int     `json:"y,omitempty" jsonschema:"y in pixels of the last screenshot"`
	Element    string   `json:"element,omitempty" jsonschema:"element id from find, used instead of x,y"`
	Button     string   `json:"button,omitempty" jsonschema:"left (default), right or middle"`
	Count      int      `json:"count,omitempty" jsonschema:"1 (default), 2 for double click, 3 for triple"`
	Modifiers  []string `json:"modifiers,omitempty" jsonschema:"keys held during the click: ctrl, alt, shift, win"`
	Screenshot *bool    `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func (s *Session) toolClick(ctx context.Context, req *mcp.CallToolRequest, in ClickIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	target := in.Element
	if in.X != nil && in.Y != nil {
		target = fmt.Sprintf("%d,%d", *in.X, *in.Y)
	}
	p, err := s.resolvePoint(in.X, in.Y, in.Element)
	if err != nil {
		return errResult("bad_target", err.Error()), nil, nil
	}
	mods, err := parseModifiers(in.Modifiers)
	if err != nil {
		return errResult("bad_modifier", err.Error()), nil, nil
	}
	if early := s.begin(ctx, "click", "click "+target); early != nil {
		return early, nil, nil
	}
	if err := s.actor.Click(p, platform.MouseButton(in.Button), in.Count, mods); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}
	s.d.Overlay.Ripple(p)
	return s.finish("click", t0, map[string]any{"screen_point": [2]int{p.X, p.Y}}, s.wantShot(in.Screenshot), s.settleFor("click")), nil, nil
}

type MoveIn struct {
	X          *int   `json:"x,omitempty" jsonschema:"x in pixels of the last screenshot"`
	Y          *int   `json:"y,omitempty" jsonschema:"y in pixels of the last screenshot"`
	Element    string `json:"element,omitempty" jsonschema:"element id from find, used instead of x,y"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default false for move)"`
}

func (s *Session) toolMove(ctx context.Context, req *mcp.CallToolRequest, in MoveIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	p, err := s.resolvePoint(in.X, in.Y, in.Element)
	if err != nil {
		return errResult("bad_target", err.Error()), nil, nil
	}
	if early := s.begin(ctx, "move", fmt.Sprintf("move %d,%d", p.X, p.Y)); early != nil {
		return early, nil, nil
	}
	if err := s.d.Input.MouseMove(p); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}
	return s.finish("move", t0, map[string]any{"screen_point": [2]int{p.X, p.Y}}, in.Screenshot != nil && *in.Screenshot, s.settleFor("move")), nil, nil
}

type PointIn struct {
	X       *int   `json:"x,omitempty" jsonschema:"x in pixels of the last screenshot"`
	Y       *int   `json:"y,omitempty" jsonschema:"y in pixels of the last screenshot"`
	Element string `json:"element,omitempty" jsonschema:"element id from find, used instead of x,y"`
}

type DragIn struct {
	From       PointIn `json:"from" jsonschema:"where to press"`
	To         PointIn `json:"to" jsonschema:"where to release"`
	Button     string  `json:"button,omitempty" jsonschema:"left (default), right or middle"`
	DurationMs int     `json:"duration_ms,omitempty" jsonschema:"drag duration in ms (default 250); use 600+ for drag-and-drop into other apps"`
	Screenshot *bool   `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func (s *Session) toolDrag(ctx context.Context, req *mcp.CallToolRequest, in DragIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	from, err := s.resolvePoint(in.From.X, in.From.Y, in.From.Element)
	if err != nil {
		return errResult("bad_target", "from: "+err.Error()), nil, nil
	}
	to, err := s.resolvePoint(in.To.X, in.To.Y, in.To.Element)
	if err != nil {
		return errResult("bad_target", "to: "+err.Error()), nil, nil
	}
	if early := s.begin(ctx, "drag", fmt.Sprintf("drag → %d,%d", to.X, to.Y)); early != nil {
		return early, nil, nil
	}
	if err := s.actor.Drag(from, to, platform.MouseButton(in.Button), time.Duration(in.DurationMs)*time.Millisecond); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}
	s.d.Overlay.Ripple(to)
	return s.finish("drag", t0, map[string]any{"from": [2]int{from.X, from.Y}, "to": [2]int{to.X, to.Y}}, s.wantShot(in.Screenshot), s.settleFor("drag")), nil, nil
}

type ScrollIn struct {
	X          *int  `json:"x,omitempty" jsonschema:"optional x (last-screenshot pixels) to scroll at"`
	Y          *int  `json:"y,omitempty" jsonschema:"optional y (last-screenshot pixels) to scroll at"`
	Dx         int   `json:"dx,omitempty" jsonschema:"horizontal wheel ticks; positive scrolls right"`
	Dy         int   `json:"dy,omitempty" jsonschema:"vertical wheel ticks; positive scrolls DOWN (3 ≈ one small step, 10 ≈ a page)"`
	Screenshot *bool `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func (s *Session) toolScroll(ctx context.Context, req *mcp.CallToolRequest, in ScrollIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	var at *geom.Point
	if in.X != nil && in.Y != nil {
		p, err := s.resolvePoint(in.X, in.Y, "")
		if err != nil {
			return errResult("bad_target", err.Error()), nil, nil
		}
		at = &p
	}
	if in.Dx == 0 && in.Dy == 0 {
		return errResult("bad_args", "give dx and/or dy in wheel ticks"), nil, nil
	}
	if early := s.begin(ctx, "scroll", fmt.Sprintf("scroll %d,%d", in.Dx, in.Dy)); early != nil {
		return early, nil, nil
	}
	if err := s.actor.Scroll(at, in.Dx, in.Dy); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}
	return s.finish("scroll", t0, nil, s.wantShot(in.Screenshot), s.settleFor("scroll")), nil, nil
}
```

- [ ] **Step 4: tools_keyboard.go**

```go
package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/input"
)

type TypeIn struct {
	Text       string `json:"text" jsonschema:"text to type into the focused control; newlines press Enter, tabs press Tab"`
	Mode       string `json:"mode,omitempty" jsonschema:"auto (default: unicode, paste when long), unicode, paste (clipboard + Ctrl+V), keys (slow per-character for apps that drop fast input)"`
	DelayMs    int    `json:"delay_ms,omitempty" jsonschema:"delay between characters in ms (default 0)"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func summarizeText(t string) string {
	t = strings.ReplaceAll(t, "\n", "⏎")
	r := []rune(t)
	if len(r) > 28 {
		return fmt.Sprintf("type %q… (%d chars)", string(r[:28]), len(r))
	}
	return fmt.Sprintf("type %q", t)
}

func (s *Session) toolType(ctx context.Context, req *mcp.CallToolRequest, in TypeIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	if in.Text == "" {
		return errResult("bad_args", "text is empty"), nil, nil
	}
	if early := s.begin(ctx, "type", summarizeText(in.Text)); early != nil {
		return early, nil, nil
	}
	if err := s.actor.Type(in.Text, in.Mode, time.Duration(in.DelayMs)*time.Millisecond); err != nil {
		return errResult("input_failed", err.Error()), nil, nil
	}
	return s.finish("type", t0, map[string]any{"chars": len([]rune(in.Text))}, s.wantShot(in.Screenshot), s.settleFor("type")), nil, nil
}

type KeyIn struct {
	Key        string   `json:"key,omitempty" jsonschema:"one chord such as ctrl+s, alt+f4, win+r, enter, f5"`
	Keys       []string `json:"keys,omitempty" jsonschema:"a sequence of chords pressed one after another, e.g. [\"win+r\",\"enter\"]"`
	HoldMs     int      `json:"hold_ms,omitempty" jsonschema:"how long to hold each chord (default 10)"`
	Screenshot *bool    `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func (s *Session) toolKey(ctx context.Context, req *mcp.CallToolRequest, in KeyIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	names := in.Keys
	if in.Key != "" {
		names = append([]string{in.Key}, names...)
	}
	if len(names) == 0 {
		return errResult("bad_args", "give key (one chord) or keys (a sequence)"), nil, nil
	}
	chords := make([]input.Chord, 0, len(names))
	for _, n := range names {
		c, err := input.ParseChord(n)
		if err != nil {
			return errResult("bad_key", err.Error()), nil, nil
		}
		chords = append(chords, c)
	}
	if early := s.begin(ctx, "key", "key "+strings.Join(names, " ")); early != nil {
		return early, nil, nil
	}
	for i, c := range chords {
		if err := s.actor.Chord(c, time.Duration(in.HoldMs)*time.Millisecond); err != nil {
			return errResult("input_failed", err.Error()), nil, nil
		}
		if i < len(chords)-1 {
			time.Sleep(40 * time.Millisecond)
		}
	}
	return s.finish("key", t0, map[string]any{"keys": names}, s.wantShot(in.Screenshot), s.settleFor("key")), nil, nil
}

type ClipboardIn struct {
	Action string `json:"action" jsonschema:"get or set"`
	Text   string `json:"text,omitempty" jsonschema:"text to place on the clipboard when action is set"`
}

func (s *Session) toolClipboard(ctx context.Context, req *mcp.CallToolRequest, in ClipboardIn) (*mcp.CallToolResult, any, error) {
	if s.d.Clip == nil {
		return errResult("unsupported", "clipboard is not available"), nil, nil
	}
	switch in.Action {
	case "get":
		t, err := s.d.Clip.GetText()
		if err != nil {
			return errResult("clipboard_failed", err.Error()), nil, nil
		}
		return okResult(map[string]any{"text": t}, nil), nil, nil
	case "set":
		if err := s.d.Clip.SetText(in.Text); err != nil {
			return errResult("clipboard_failed", err.Error()), nil, nil
		}
		return okResult(map[string]any{"chars": len([]rune(in.Text))}, nil), nil, nil
	}
	return errResult("bad_args", "action must be get or set"), nil, nil
}
```

- [ ] **Step 5: tools_window.go**

```go
package server

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/window"
)

type WindowsIn struct {
	Filter  string `json:"filter,omitempty" jsonschema:"case-insensitive regexp on title or process name"`
	Monitor int    `json:"monitor,omitempty" jsonschema:"only windows whose center is on this monitor id"`
}

type windowOut struct {
	platform.WindowInfo
	Monitor int `json:"monitor"`
}

func (s *Session) monitorOf(r geom.Rect) int {
	mons, err := s.monitors()
	if err != nil {
		return 0
	}
	c := r.Center()
	for _, m := range mons {
		if m.Rect.Contains(c) {
			return m.ID
		}
	}
	return 0
}

func (s *Session) toolWindows(ctx context.Context, req *mcp.CallToolRequest, in WindowsIn) (*mcp.CallToolResult, any, error) {
	list, err := s.d.Wins.List()
	if err != nil {
		return errResult("windows_failed", err.Error()), nil, nil
	}
	var re *regexp.Regexp
	if in.Filter != "" {
		if re, err = regexp.Compile("(?i)" + in.Filter); err != nil {
			return errResult("bad_args", "filter: "+err.Error()), nil, nil
		}
	}
	out := make([]windowOut, 0, len(list))
	for _, w := range list {
		if re != nil && !re.MatchString(w.Title) && !re.MatchString(w.Process) {
			continue
		}
		mon := s.monitorOf(w.Rect)
		if in.Monitor != 0 && mon != in.Monitor {
			continue
		}
		out = append(out, windowOut{WindowInfo: w, Monitor: mon})
	}
	return okResult(map[string]any{"windows": out, "count": len(out)}, nil), nil, nil
}

type WindowIn struct {
	Action     string    `json:"action" jsonschema:"focus, minimize, maximize, restore, close, move or resize"`
	Target     string    `json:"target,omitempty" jsonschema:"\"foreground\" (default), a window id from windows, or a case-insensitive regexp on title/process"`
	Rect       *RegionIn `json:"rect,omitempty" jsonschema:"for move/resize: target frame rect in SCREEN pixels (use monitors for bounds)"`
	Screenshot *bool     `json:"screenshot,omitempty" jsonschema:"return a screenshot after the action (default true)"`
}

func (s *Session) toolWindow(ctx context.Context, req *mcp.CallToolRequest, in WindowIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	list, err := s.d.Wins.List()
	if err != nil {
		return errResult("windows_failed", err.Error()), nil, nil
	}
	w, err := window.Match(list, in.Target)
	if err != nil {
		return errResult("no_window", err.Error()), nil, nil
	}
	if early := s.begin(ctx, "window", fmt.Sprintf("window %s %q", in.Action, w.Title)); early != nil {
		return early, nil, nil
	}
	switch in.Action {
	case "focus":
		err = s.d.Wins.Focus(w.ID)
	case "minimize":
		err = s.d.Wins.SetState(w.ID, platform.WindowMinimized)
	case "maximize":
		err = s.d.Wins.SetState(w.ID, platform.WindowMaximized)
	case "restore":
		err = s.d.Wins.SetState(w.ID, platform.WindowNormal)
	case "close":
		err = s.d.Wins.Close(w.ID)
	case "move", "resize":
		if in.Rect == nil {
			return errResult("bad_args", "move/resize need rect {x,y,w,h} in screen pixels"), nil, nil
		}
		r := geom.Rect{X: in.Rect.X, Y: in.Rect.Y, W: in.Rect.W, H: in.Rect.H}
		if in.Action == "resize" && (r.W == 0 || r.H == 0) {
			return errResult("bad_args", "resize needs w and h"), nil, nil
		}
		if r.W == 0 || r.H == 0 { // move keeps the size
			r.W, r.H = w.Rect.W, w.Rect.H
		}
		err = s.d.Wins.Move(w.ID, r)
	default:
		return errResult("bad_args", "unknown window action "+in.Action), nil, nil
	}
	if err != nil {
		return errResult("window_failed", err.Error()), nil, nil
	}
	s.mu.Lock()
	s.mons = nil // a moved window may change the active monitor
	s.mu.Unlock()
	return s.finish("window", t0, map[string]any{"window": w.ID, "title": w.Title, "did": in.Action}, s.wantShot(in.Screenshot), s.settleFor("window")), nil, nil
}
```

- [ ] **Step 6: tools_batch.go**

```go
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type BatchAction struct {
	Tool string         `json:"tool" jsonschema:"click, move, drag, scroll, type, key, wait, window, clipboard or find"`
	Args map[string]any `json:"args,omitempty" jsonschema:"the tool's arguments; screenshot is forced off for steps"`
}

type BatchIn struct {
	Actions     []BatchAction `json:"actions" jsonschema:"steps executed in order"`
	StopOnError *bool         `json:"stop_on_error,omitempty" jsonschema:"stop at the first failing step (default true)"`
	Screenshot  *bool         `json:"screenshot,omitempty" jsonschema:"one screenshot after the last step (default true)"`
}

type batchFn func(ctx context.Context, args json.RawMessage) (*mcp.CallToolResult, error)

// wrap adapts a typed handler into a batchFn that forces screenshot=false.
func wrap[In any](h func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, any, error)) batchFn {
	return func(ctx context.Context, args json.RawMessage) (*mcp.CallToolResult, error) {
		var m map[string]any
		if len(args) > 0 {
			if err := json.Unmarshal(args, &m); err != nil {
				return nil, err
			}
		}
		if m == nil {
			m = map[string]any{}
		}
		m["screenshot"] = false
		b, _ := json.Marshal(m)
		var in In
		if err := json.Unmarshal(b, &in); err != nil {
			return nil, err
		}
		res, _, err := h(ctx, nil, in)
		return res, err
	}
}

func (s *Session) batchable() map[string]batchFn {
	return map[string]batchFn{
		"click":     wrap(s.toolClick),
		"move":      wrap(s.toolMove),
		"drag":      wrap(s.toolDrag),
		"scroll":    wrap(s.toolScroll),
		"type":      wrap(s.toolType),
		"key":       wrap(s.toolKey),
		"wait":      wrap(s.toolWait),
		"window":    wrap(s.toolWindow),
		"clipboard": wrap(s.toolClipboard),
	}
}

func (s *Session) toolBatch(ctx context.Context, req *mcp.CallToolRequest, in BatchIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	if len(in.Actions) == 0 {
		return errResult("bad_args", "actions is empty"), nil, nil
	}
	reg := s.batchable()
	stop := in.StopOnError == nil || *in.StopOnError
	steps := make([]map[string]any, 0, len(in.Actions))
	allOK := true
	for i, a := range in.Actions {
		step := map[string]any{"i": i, "tool": a.Tool}
		fn, ok := reg[a.Tool]
		var res *mcp.CallToolResult
		var err error
		if !ok {
			err = fmt.Errorf("tool %q cannot be used in batch", a.Tool)
		} else {
			args, _ := json.Marshal(a.Args)
			res, err = fn(ctx, args)
		}
		if err == nil && res != nil && res.IsError {
			if tc, ok := res.Content[0].(*mcp.TextContent); ok {
				err = fmt.Errorf("%s", tc.Text)
			} else {
				err = fmt.Errorf("step failed")
			}
		}
		if err != nil {
			step["ok"] = false
			step["error"] = err.Error()
			steps = append(steps, step)
			allOK = false
			if stop {
				break
			}
			continue
		}
		step["ok"] = true
		if tc, ok := res.Content[0].(*mcp.TextContent); ok {
			var f map[string]any
			if json.Unmarshal([]byte(tc.Text), &f) == nil {
				if r, ok := f["resumed"]; ok && r == true { // user handed control back mid-batch: stop, re-observe
					step["resumed"] = true
					steps = append(steps, step)
					allOK = false
					break
				}
			}
		}
		steps = append(steps, step)
	}
	f := map[string]any{"ok": allOK, "steps": steps, "completed": len(steps)}
	var res *mcp.CallToolResult
	if s.wantShot(in.Screenshot) {
		res = s.finish("batch", t0, f, true, 120*time.Millisecond)
	} else {
		res = s.finish("batch", t0, f, false, 0)
	}
	return res, nil, nil
}
```
Note `finish` copies `extra` over `f` including `"ok"`, and `okResult` only defaults `ok` when unset — so `ok:false` survives.

- [ ] **Step 7: tools_wait.go**

```go
package server

import (
	"context"
	"image"
	"regexp"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/screen"
	"github.com/racass-pixel/claude-computer-use/internal/window"
)

type WaitIn struct {
	Ms         int    `json:"ms,omitempty" jsonschema:"plain sleep in milliseconds"`
	Window     string `json:"window,omitempty" jsonschema:"wait until a window matching this regexp (title or process) exists"`
	Stable     *bool  `json:"stable,omitempty" jsonschema:"wait until the active monitor stops changing (animations, page loads)"`
	TimeoutMs  int    `json:"timeout_ms,omitempty" jsonschema:"give up after this long (default 10000)"`
	Screenshot *bool  `json:"screenshot,omitempty" jsonschema:"return a screenshot when done (default true)"`
}

// frameDiff is the mean absolute RGB difference (0..255) between two equally sized frames.
func frameDiff(a, b *image.RGBA) float64 {
	if len(a.Pix) != len(b.Pix) || len(a.Pix) == 0 {
		return 255
	}
	var sum int64
	n := 0
	for i := 0; i+3 < len(a.Pix); i += 4 {
		for k := 0; k < 3; k++ {
			d := int(a.Pix[i+k]) - int(b.Pix[i+k])
			if d < 0 {
				d = -d
			}
			sum += int64(d)
		}
		n += 3
	}
	return float64(sum) / float64(n)
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func (s *Session) toolWait(ctx context.Context, req *mcp.CallToolRequest, in WaitIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	timeout := time.Duration(in.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	deadline := time.Now().Add(timeout)
	cond := "ms"
	switch {
	case in.Window != "":
		cond = "window"
		re, err := regexp.Compile("(?i)" + in.Window)
		if err != nil {
			return errResult("bad_args", "window: "+err.Error()), nil, nil
		}
		for {
			list, _ := s.d.Wins.List()
			if w, err := window.Match(list, in.Window); err == nil && re != nil {
				return s.finish("wait", t0, map[string]any{"condition": cond, "matched": map[string]any{"id": w.ID, "title": w.Title, "process": w.Process}}, s.wantShot(in.Screenshot), 50*time.Millisecond), nil, nil
			}
			if time.Now().After(deadline) || !sleepCtx(ctx, 150*time.Millisecond) {
				break
			}
		}
	case in.Stable != nil && *in.Stable:
		cond = "stable"
		m := s.activeMonitor()
		var prev *image.RGBA
		quiet := 0
		for {
			img, err := s.d.Screen.Capture(m.Rect)
			if err != nil {
				return errResult("capture_failed", err.Error()), nil, nil
			}
			small := screen.Scale(img, 0.2)
			if prev != nil && frameDiff(prev, small) < 1.0 {
				quiet++
				if quiet >= 2 {
					return s.finish("wait", t0, map[string]any{"condition": cond, "stable_after_ms": time.Since(t0).Milliseconds()}, s.wantShot(in.Screenshot), 0), nil, nil
				}
			} else {
				quiet = 0
			}
			prev = small
			if time.Now().After(deadline) || !sleepCtx(ctx, 150*time.Millisecond) {
				break
			}
		}
	default:
		d := time.Duration(in.Ms) * time.Millisecond
		if d <= 0 {
			d = 500 * time.Millisecond
		}
		sleepCtx(ctx, min(d, timeout))
		return s.finish("wait", t0, map[string]any{"condition": cond, "waited_ms": time.Since(t0).Milliseconds()}, s.wantShot(in.Screenshot), 0), nil, nil
	}
	f := map[string]any{"ok": false, "timeout": true, "condition": cond, "waited_ms": time.Since(t0).Milliseconds()}
	return s.finish("wait", t0, f, s.wantShot(in.Screenshot), 0), nil, nil
}
```

- [ ] **Step 8: tools_control.go**

```go
package server

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

type ControlIn struct {
	Action string `json:"action" jsonschema:"status, acquire (show the overlay now), release (hide it), or hud (set the task title shown to the user)"`
	Task   string `json:"task,omitempty" jsonschema:"short task title for the HUD, e.g. \"Заполняю форму заказа\""`
	Note   string `json:"note,omitempty" jsonschema:"optional current step shown after the hotkey hint"`
}

func (s *Session) controlStatus() map[string]any {
	f := map[string]any{"controlling": false, "paused": false, "hotkey": s.cfg.Hotkey}
	if c := s.d.Controller; c != nil {
		st := c.Status()
		f["state"] = st.State
		f["controlling"] = st.State == "controlling"
		f["paused"] = st.State == "paused"
		f["hotkey"] = st.Hotkey
		f["idle_ms"] = st.IdleMs
	}
	if m := s.activeMonitor(); m.ID != 0 {
		f["active_monitor"] = m.ID
	}
	return f
}

func (s *Session) toolControl(ctx context.Context, req *mcp.CallToolRequest, in ControlIn) (*mcp.CallToolResult, any, error) {
	now := time.Now()
	switch in.Action {
	case "", "status":
	case "acquire":
		if c := s.d.Controller; c != nil {
			if c.IsPaused() {
				return errResult("user_took_control", "The user has control. Wait for the user to hand it back or to ask you to continue."), nil, nil
			}
			c.Acquire(now)
		}
		if in.Task != "" {
			s.d.Overlay.SetTitle(in.Task)
		}
		if m := s.activeMonitor(); m.ID != 0 {
			s.d.Overlay.Show(m, platform.OverlayControlling)
		}
	case "release":
		if c := s.d.Controller; c != nil {
			c.Release(now)
		}
		s.d.Overlay.Hide()
	case "hud":
		s.d.Overlay.SetTitle(in.Task)
		if in.Note != "" {
			s.d.Overlay.SetAction(in.Note)
		}
	default:
		return errResult("bad_args", "action must be status, acquire, release or hud"), nil, nil
	}
	return okResult(s.controlStatus(), nil), nil, nil
}
```

- [ ] **Step 9: Register the tools**

In `server.go` `Register`, after `monitors` add (one `mcp.AddTool` per line, descriptions verbatim):
```go
	mcp.AddTool(srv, &mcp.Tool{Name: "click", Description: "Click at x,y (pixels of the last screenshot) or on an element id from find. Supports right/middle button, double/triple click and held modifiers. Returns a screenshot after the click by default."}, s.toolClick)
	mcp.AddTool(srv, &mcp.Tool{Name: "move", Description: "Move the mouse (hover) to x,y of the last screenshot or to an element. No screenshot by default."}, s.toolMove)
	mcp.AddTool(srv, &mcp.Tool{Name: "drag", Description: "Press at from, move smoothly, release at to (drag-and-drop, selections, sliders, window moves). Coordinates are pixels of the last screenshot or element ids."}, s.toolDrag)
	mcp.AddTool(srv, &mcp.Tool{Name: "scroll", Description: "Scroll the mouse wheel at an optional x,y. dy>0 scrolls down, dx>0 scrolls right, in ticks."}, s.toolScroll)
	mcp.AddTool(srv, &mcp.Tool{Name: "type", Description: "Type text into the focused control (Unicode, any language). Long texts are pasted via the clipboard. Newlines press Enter."}, s.toolType)
	mcp.AddTool(srv, &mcp.Tool{Name: "key", Description: "Press one chord (key: \"ctrl+s\") or a sequence (keys: [\"win+r\",\"enter\"]). Names: ctrl, alt, shift, win, enter, esc, tab, space, backspace, delete, home, end, pageup, pagedown, arrows, f1-f24, letters, digits."}, s.toolKey)
	mcp.AddTool(srv, &mcp.Tool{Name: "clipboard", Description: "Read (get) or write (set) the text clipboard."}, s.toolClipboard)
	mcp.AddTool(srv, &mcp.Tool{Name: "windows", Description: "List open top-level windows: id, title, process, rect (screen px), monitor, state, is_foreground. Optional regexp filter."}, s.toolWindows)
	mcp.AddTool(srv, &mcp.Tool{Name: "window", Description: "Act on a window: focus, minimize, maximize, restore, close, move, resize. Target by id, \"foreground\", or a regexp on title/process. Use this to switch apps instead of clicking the taskbar."}, s.toolWindow)
	mcp.AddTool(srv, &mcp.Tool{Name: "wait", Description: "Wait for something instead of polling with screenshots: ms (sleep), window (regexp appears), or stable (screen stops changing). Returns a screenshot when done; ok:false with timeout:true if it did not happen."}, s.toolWait)
	mcp.AddTool(srv, &mcp.Tool{Name: "batch", Description: "Run several actions in one call when you are confident of the sequence (e.g. click a field, type, press Enter). Steps run without screenshots; one screenshot is returned at the end. Stops at the first failure."}, s.toolBatch)
	mcp.AddTool(srv, &mcp.Tool{Name: "control", Description: "Session control: status (are you controlling / did the user pause), acquire (show the take-over overlay now), release (hide it when the task is done), hud (set the task title the user sees)."}, s.toolControl)
```
Run: `go test ./internal/server/` → PASS (all tests from Step 1).

- [ ] **Step 10: Real smoke test in Claude Code**

Build, run `claude --plugin-dir .` and ask, in Russian: «Открой Блокнот через Win+R, напиши "привет из claude", сохрани файл на рабочий стол как cu-test.txt». Expected: the model uses `key`, `wait`/`screenshot`, `type`, `key ctrl+s`, types the path, presses Enter; the file exists on the Desktop. Check every action's stderr `ms=` < 100 (excluding wait). Also ask «Переключись на окно Проводника и сверни его» → uses `windows` then `window`.

- [ ] **Step 11: Commit**

```bash
go build ./... && go vet ./... && go test ./...
git add -A && git commit -m "feat: mouse, keyboard, window, batch, wait and control tools"
```

---

### Task 10: Guard — hotkey grammar, tap matcher, pause state machine (pure, tested)

**Files:**
- Create: `internal/guard/hotkey.go`, `internal/guard/hotkey_test.go`, `internal/guard/machine.go`, `internal/guard/machine_test.go`

**Interfaces:**
- Consumes: `input.Chord`, `input.ParseChord`, `input.NormalizeModifier`, `input.IsModifier`, `geom.Point`.
- Produces:
```go
package guard

type Hotkey struct { Chord input.Chord; Taps int }
func ParseHotkey(s string) (Hotkey, error)   // "esc esc" → Esc ×2; "ctrl+alt+esc" → 1 tap; whitespace-separated tokens must be identical
func (h Hotkey) String() string              // "Esc Esc" | "Ctrl+Alt+Esc" (HUD label)

type Kind int
const ( KeyDown Kind = iota; KeyUp; MouseMove; MouseDown )
type Event struct { Kind Kind; VK uint16; Pos geom.Point; Injected bool; At time.Time }

type TapMatcher struct{ /* unexported */ }
func NewTapMatcher(h Hotkey, window time.Duration) *TapMatcher
func (m *TapMatcher) Feed(ev Event) bool          // true when the full gesture completes
func (m *TapMatcher) Involves(vk uint16) bool     // vk is the hotkey key or one of its modifiers

type State int
const ( Idle State = iota; Controlling; Paused )
func (s State) String() string                    // "idle" | "controlling" | "paused"

type Reason string
const (
	ReasonAction        Reason = "action"
	ReasonHotkey        Reason = "hotkey"
	ReasonPhysicalKey   Reason = "physical_key"
	ReasonPhysicalMouse Reason = "physical_mouse"
	ReasonIdle          Reason = "idle"
	ReasonRelease       Reason = "release"
	ReasonResume        Reason = "resume"
	ReasonPrompt        Reason = "prompt"
)
type Transition struct { From, To State; Reason Reason; At time.Time }

type Config struct {
	AutoPause        bool
	MouseThresholdPx int
	MouseWindow      time.Duration // 300ms
	IdleRelease      time.Duration
	Hotkey           Hotkey
	TapWindow        time.Duration // 400ms
}
type Status struct { State string; Hotkey string; IdleMs int64 }

type Machine struct{ /* unexported */ }
func New(cfg Config, onChange func(Transition)) *Machine   // onChange may be nil; called outside the lock
func (m *Machine) State() State
func (m *Machine) IsPaused() bool
func (m *Machine) Acquire(now time.Time)                    // Idle → Controlling (no-op when Paused)
func (m *Machine) Touch(now time.Time)                      // records activity (idle timer)
func (m *Machine) Release(now time.Time)                    // any → Idle
func (m *Machine) Pause(now time.Time, r Reason)            // Controlling → Paused
func (m *Machine) Resume(now time.Time, r Reason)           // Paused → Controlling
func (m *Machine) HandleEvent(ev Event)                     // called from the hook thread; never blocks
func (m *Machine) CheckIdle(now time.Time)                  // Controlling and idle too long → Idle
func (m *Machine) WaitResume(ctx context.Context) bool      // true once state != Paused
func (m *Machine) Status() Status
```

- [ ] **Step 1: Failing hotkey tests**

`internal/guard/hotkey_test.go`:
```go
package guard

import (
	"testing"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/input"
)

func TestParseHotkey(t *testing.T) {
	h, err := ParseHotkey("esc esc")
	if err != nil || h.Taps != 2 || h.Chord.Key != input.VK_ESCAPE || h.String() != "Esc Esc" {
		t.Fatalf("esc esc: %+v %v %q", h, err, h.String())
	}
	h, err = ParseHotkey("ctrl+alt+esc")
	if err != nil || h.Taps != 1 || len(h.Chord.Mods) != 2 || h.String() != "Ctrl+Alt+Esc" {
		t.Fatalf("chord: %+v %v", h, err)
	}
	for _, bad := range []string{"", "esc enter", "bogus bogus"} {
		if _, err := ParseHotkey(bad); err == nil {
			t.Fatalf("%q must fail", bad)
		}
	}
}

func at(ms int) time.Time { return time.Unix(0, 0).Add(time.Duration(ms) * time.Millisecond) }

func TestDoubleTapWithinWindow(t *testing.T) {
	h, _ := ParseHotkey("esc esc")
	m := NewTapMatcher(h, 400*time.Millisecond)
	if m.Feed(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(0)}) {
		t.Fatal("first tap must not complete")
	}
	m.Feed(Event{Kind: KeyUp, VK: input.VK_ESCAPE, At: at(50)})
	if !m.Feed(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(300)}) {
		t.Fatal("second tap within 400ms must complete")
	}
	// gesture resets after completing
	if m.Feed(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(500)}) {
		t.Fatal("a new sequence must start after completion")
	}
	if m.Feed(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(1200)}) {
		t.Fatal("taps 700ms apart must not complete")
	}
}

func TestChordNeedsAllModifiersHeld(t *testing.T) {
	h, _ := ParseHotkey("ctrl+alt+esc")
	m := NewTapMatcher(h, 400*time.Millisecond)
	if m.Feed(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(0)}) {
		t.Fatal("esc alone must not match")
	}
	m.Feed(Event{Kind: KeyDown, VK: input.VK_LCONTROL, At: at(10)})
	m.Feed(Event{Kind: KeyDown, VK: input.VK_RMENU, At: at(20)})
	if !m.Feed(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(30)}) {
		t.Fatal("ctrl+alt+esc must match with left/right variants held")
	}
	m.Feed(Event{Kind: KeyUp, VK: input.VK_LCONTROL, At: at(40)})
	if m.Feed(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(50)}) {
		t.Fatal("after releasing ctrl the chord must not match")
	}
	if !m.Involves(input.VK_RCONTROL) || m.Involves(0x41) {
		t.Fatal("Involves must cover the key and its modifiers")
	}
}
```

- [ ] **Step 2: Implement hotkey.go**

`internal/guard/hotkey.go`:
```go
// Package guard decides when Claude may act: it owns the idle/controlling/paused state machine
// and recognises the user's take-over gesture.
package guard

import (
	"fmt"
	"strings"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/input"
)

type Hotkey struct {
	Chord input.Chord
	Taps  int
}

// ParseHotkey accepts "esc esc" (double tap) or a single chord like "ctrl+alt+esc".
func ParseHotkey(s string) (Hotkey, error) {
	tokens := strings.Fields(strings.ToLower(s))
	if len(tokens) == 0 {
		return Hotkey{}, fmt.Errorf("empty hotkey")
	}
	for _, tok := range tokens[1:] {
		if tok != tokens[0] {
			return Hotkey{}, fmt.Errorf("hotkey %q: repeated taps must use the same key (e.g. \"esc esc\")", s)
		}
	}
	c, err := input.ParseChord(tokens[0])
	if err != nil {
		return Hotkey{}, fmt.Errorf("hotkey %q: %w", s, err)
	}
	return Hotkey{Chord: c, Taps: len(tokens)}, nil
}

func (h Hotkey) String() string {
	parts := make([]string, h.Taps)
	for i := range parts {
		parts[i] = h.Chord.String()
	}
	return strings.Join(parts, " ")
}

type Kind int

const (
	KeyDown Kind = iota
	KeyUp
	MouseMove
	MouseDown
)

type Event struct {
	Kind     Kind
	VK       uint16
	Pos      geom.Point
	Injected bool
	At       time.Time
}

// TapMatcher recognises the hotkey gesture from a stream of key events.
type TapMatcher struct {
	h       Hotkey
	window  time.Duration
	held    map[uint16]bool // normalised modifiers currently down
	count   int
	lastTap time.Time
}

func NewTapMatcher(h Hotkey, window time.Duration) *TapMatcher {
	return &TapMatcher{h: h, window: window, held: map[uint16]bool{}}
}

func (m *TapMatcher) Involves(vk uint16) bool {
	n := input.NormalizeModifier(vk)
	if n == m.h.Chord.Key {
		return true
	}
	for _, mod := range m.h.Chord.Mods {
		if mod == n {
			return true
		}
	}
	return false
}

func (m *TapMatcher) modsHeld() bool {
	want := map[uint16]bool{}
	for _, mod := range m.h.Chord.Mods {
		want[mod] = true
	}
	for mod := range want {
		if !m.held[mod] {
			return false
		}
	}
	for mod := range m.held {
		if m.held[mod] && !want[mod] {
			return false // an extra modifier is held
		}
	}
	return true
}

func (m *TapMatcher) Feed(ev Event) bool {
	if input.IsModifier(ev.VK) {
		n := input.NormalizeModifier(ev.VK)
		switch ev.Kind {
		case KeyDown:
			m.held[n] = true
		case KeyUp:
			delete(m.held, n)
		}
		return false
	}
	if ev.Kind != KeyDown || ev.VK != m.h.Chord.Key || !m.modsHeld() {
		return false
	}
	if m.count > 0 && ev.At.Sub(m.lastTap) > m.window {
		m.count = 0
	}
	m.count++
	m.lastTap = ev.At
	if m.count >= m.h.Taps {
		m.count = 0
		return true
	}
	return false
}
```
Run: `go test ./internal/guard/ -run 'Hotkey|Tap|Chord'` → PASS.

- [ ] **Step 3: Failing machine tests**

`internal/guard/machine_test.go`:
```go
package guard

import (
	"context"
	"testing"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/input"
)

func newMachine(autoPause bool) (*Machine, *[]Transition) {
	h, _ := ParseHotkey("esc esc")
	var log []Transition
	m := New(Config{
		AutoPause: autoPause, MouseThresholdPx: 12, MouseWindow: 300 * time.Millisecond,
		IdleRelease: 2 * time.Second, Hotkey: h, TapWindow: 400 * time.Millisecond,
	}, func(tr Transition) { log = append(log, tr) })
	return m, &log
}

func TestAcquireTouchIdleRelease(t *testing.T) {
	m, log := newMachine(true)
	m.Acquire(at(0))
	if m.State() != Controlling {
		t.Fatalf("state = %v", m.State())
	}
	m.Touch(at(1000))
	m.CheckIdle(at(2500))
	if m.State() != Controlling {
		t.Fatal("must stay controlling within IdleRelease of the last action")
	}
	m.CheckIdle(at(3100))
	if m.State() != Idle || (*log)[len(*log)-1].Reason != ReasonIdle {
		t.Fatalf("idle release failed: %v %v", m.State(), *log)
	}
}

func TestPhysicalKeyPausesInjectedDoesNot(t *testing.T) {
	m, log := newMachine(true)
	m.Acquire(at(0))
	m.HandleEvent(Event{Kind: KeyDown, VK: 0x41, Injected: true, At: at(10)})
	if m.State() != Controlling {
		t.Fatal("injected input must be ignored")
	}
	m.HandleEvent(Event{Kind: KeyDown, VK: 0x41, At: at(20)})
	if m.State() != Paused || (*log)[len(*log)-1].Reason != ReasonPhysicalKey {
		t.Fatalf("physical key must pause: %v", *log)
	}
}

func TestSingleEscIsReservedForTheGesture(t *testing.T) {
	m, _ := newMachine(true)
	m.Acquire(at(0))
	m.HandleEvent(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(10)})
	if m.State() != Controlling {
		t.Fatal("a single Esc must not pause (it is part of the hotkey)")
	}
	m.HandleEvent(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(200)})
	if m.State() != Paused {
		t.Fatal("Esc Esc must pause")
	}
	m.HandleEvent(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(1000)})
	m.HandleEvent(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(1200)})
	if m.State() != Controlling {
		t.Fatal("Esc Esc while paused must hand control back")
	}
}

func TestMouseThresholdAndInjectedBaseline(t *testing.T) {
	m, _ := newMachine(true)
	m.Acquire(at(0))
	m.HandleEvent(Event{Kind: MouseMove, Pos: geom.Point{X: 1000, Y: 1000}, Injected: true, At: at(10)})
	m.HandleEvent(Event{Kind: MouseMove, Pos: geom.Point{X: 1004, Y: 1000}, At: at(20)})
	m.HandleEvent(Event{Kind: MouseMove, Pos: geom.Point{X: 1007, Y: 1002}, At: at(40)})
	if m.State() != Controlling {
		t.Fatal("jitter under the threshold must not pause")
	}
	m.HandleEvent(Event{Kind: MouseMove, Pos: geom.Point{X: 1030, Y: 1002}, At: at(60)})
	if m.State() != Paused {
		t.Fatal("moving past the threshold within the window must pause")
	}
}

func TestMouseButtonPausesAndAutoPauseOff(t *testing.T) {
	m, _ := newMachine(true)
	m.Acquire(at(0))
	m.HandleEvent(Event{Kind: MouseDown, At: at(10)})
	if m.State() != Paused {
		t.Fatal("physical click must pause")
	}
	m2, _ := newMachine(false)
	m2.Acquire(at(0))
	m2.HandleEvent(Event{Kind: KeyDown, VK: 0x41, At: at(10)})
	m2.HandleEvent(Event{Kind: MouseDown, At: at(20)})
	if m2.State() != Controlling {
		t.Fatal("auto_pause=false must ignore physical input")
	}
	m2.HandleEvent(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(30)})
	m2.HandleEvent(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(100)})
	if m2.State() != Paused {
		t.Fatal("the hotkey must work even with auto_pause=false")
	}
}

func TestWaitResumeAndRelease(t *testing.T) {
	m, _ := newMachine(true)
	m.Acquire(at(0))
	m.Pause(at(10), ReasonHotkey)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if m.WaitResume(ctx) {
		t.Fatal("WaitResume must time out while paused")
	}
	go func() { time.Sleep(10 * time.Millisecond); m.Resume(at(20), ReasonPrompt) }()
	if !m.WaitResume(context.Background()) {
		t.Fatal("WaitResume must return true after Resume")
	}
	m.Pause(at(30), ReasonHotkey)
	m.Release(at(40))
	if m.State() != Idle || m.IsPaused() {
		t.Fatal("Release must reach Idle from Paused")
	}
	if st := m.Status(); st.State != "idle" || st.Hotkey != "Esc Esc" {
		t.Fatalf("status = %+v", st)
	}
}
```
Run: `go test ./internal/guard/` → FAIL.

- [ ] **Step 4: Implement machine.go**

`internal/guard/machine.go`:
```go
package guard

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
)

type State int

const (
	Idle State = iota
	Controlling
	Paused
)

func (s State) String() string {
	switch s {
	case Controlling:
		return "controlling"
	case Paused:
		return "paused"
	}
	return "idle"
}

type Reason string

const (
	ReasonAction        Reason = "action"
	ReasonHotkey        Reason = "hotkey"
	ReasonPhysicalKey   Reason = "physical_key"
	ReasonPhysicalMouse Reason = "physical_mouse"
	ReasonIdle          Reason = "idle"
	ReasonRelease       Reason = "release"
	ReasonResume        Reason = "resume"
	ReasonPrompt        Reason = "prompt"
)

type Transition struct {
	From, To State
	Reason   Reason
	At       time.Time
}

type Config struct {
	AutoPause        bool
	MouseThresholdPx int
	MouseWindow      time.Duration
	IdleRelease      time.Duration
	Hotkey           Hotkey
	TapWindow        time.Duration
}

type Status struct {
	State  string
	Hotkey string
	IdleMs int64
}

type Machine struct {
	mu         sync.Mutex
	cfg        Config
	state      State
	lastAction time.Time
	taps       *TapMatcher
	onChange   func(Transition)

	mouseAcc      float64
	mouseWinStart time.Time
	lastMouse     geom.Point
	haveMouse     bool

	resumeCh chan struct{} // closed when leaving Paused
}

func New(cfg Config, onChange func(Transition)) *Machine {
	if cfg.MouseWindow <= 0 {
		cfg.MouseWindow = 300 * time.Millisecond
	}
	if cfg.TapWindow <= 0 {
		cfg.TapWindow = 400 * time.Millisecond
	}
	return &Machine{cfg: cfg, taps: NewTapMatcher(cfg.Hotkey, cfg.TapWindow), onChange: onChange, resumeCh: make(chan struct{})}
}

// set changes state under the lock and reports the transition after unlocking.
func (m *Machine) set(to State, r Reason, now time.Time) {
	from := m.state
	if from == to {
		return
	}
	m.state = to
	if from == Paused {
		close(m.resumeCh)
	}
	if to == Paused {
		m.resumeCh = make(chan struct{})
	}
	if to == Controlling {
		m.lastAction = now
		m.mouseAcc, m.haveMouse = 0, false
	}
	if m.onChange != nil {
		tr := Transition{From: from, To: to, Reason: r, At: now}
		m.mu.Unlock()
		m.onChange(tr)
		m.mu.Lock()
	}
}

func (m *Machine) State() State   { m.mu.Lock(); defer m.mu.Unlock(); return m.state }
func (m *Machine) IsPaused() bool { return m.State() == Paused }

func (m *Machine) Acquire(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == Idle {
		m.set(Controlling, ReasonAction, now)
	}
}

func (m *Machine) Touch(now time.Time) {
	m.mu.Lock()
	m.lastAction = now
	m.mu.Unlock()
}

func (m *Machine) Release(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.set(Idle, ReasonRelease, now)
}

func (m *Machine) Pause(now time.Time, r Reason) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == Controlling {
		m.set(Paused, r, now)
	}
}

func (m *Machine) Resume(now time.Time, r Reason) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == Paused {
		m.set(Controlling, r, now)
	}
}

func (m *Machine) CheckIdle(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == Controlling && m.cfg.IdleRelease > 0 && now.Sub(m.lastAction) > m.cfg.IdleRelease {
		m.set(Idle, ReasonIdle, now)
	}
}

// HandleEvent applies the take-over rules (spec §7). Injected events only refresh the mouse baseline.
func (m *Machine) HandleEvent(ev Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ev.Injected {
		if ev.Kind == MouseMove {
			m.lastMouse, m.haveMouse = ev.Pos, true
		}
		return
	}
	if ev.Kind == KeyDown || ev.Kind == KeyUp {
		if m.taps.Feed(ev) {
			switch m.state {
			case Controlling:
				m.set(Paused, ReasonHotkey, ev.At)
			case Paused:
				m.set(Controlling, ReasonHotkey, ev.At)
			}
			return
		}
	}
	if m.state != Controlling || !m.cfg.AutoPause {
		return
	}
	switch ev.Kind {
	case KeyDown:
		if m.taps.Involves(ev.VK) {
			return // reserved for the gesture
		}
		m.set(Paused, ReasonPhysicalKey, ev.At)
	case MouseDown:
		m.set(Paused, ReasonPhysicalMouse, ev.At)
	case MouseMove:
		if !m.haveMouse || ev.At.Sub(m.mouseWinStart) > m.cfg.MouseWindow {
			m.mouseAcc, m.mouseWinStart = 0, ev.At
			if !m.haveMouse {
				m.lastMouse, m.haveMouse = ev.Pos, true
				return
			}
		}
		dx, dy := float64(ev.Pos.X-m.lastMouse.X), float64(ev.Pos.Y-m.lastMouse.Y)
		m.mouseAcc += math.Hypot(dx, dy)
		m.lastMouse = ev.Pos
		if m.mouseAcc > float64(m.cfg.MouseThresholdPx) {
			m.set(Paused, ReasonPhysicalMouse, ev.At)
		}
	}
}

// WaitResume blocks until the machine is no longer Paused (true) or ctx ends (false).
func (m *Machine) WaitResume(ctx context.Context) bool {
	m.mu.Lock()
	if m.state != Paused {
		m.mu.Unlock()
		return true
	}
	ch := m.resumeCh
	m.mu.Unlock()
	select {
	case <-ch:
		return true
	case <-ctx.Done():
		return false
	}
}

func (m *Machine) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	idle := int64(0)
	if !m.lastAction.IsZero() {
		idle = time.Since(m.lastAction).Milliseconds()
	}
	return Status{State: m.state.String(), Hotkey: m.cfg.Hotkey.String(), IdleMs: idle}
}
```
Run: `go test ./internal/guard/` → PASS. (If `TestMouseThresholdAndInjectedBaseline` fails on the "jitter" step, re-check that the injected move sets `lastMouse` and the first physical move only starts the window.)

- [ ] **Step 5: Commit**

```bash
go build ./... && go vet ./... && go test ./...
git add -A && git commit -m "feat: guard state machine and take-over hotkey matcher"
```

---

### Task 11: UI thread, low-level input hooks, parent watch, guard wiring into `cu serve`

**Files:**
- Create: `internal/win/msgwindow.go`, `internal/win/hooks.go`, `internal/win/process.go`
- Create: `internal/uithread/uithread_windows.go`
- Create: `internal/guard/runner_windows.go`
- Modify: `cmd/cu/cmd_serve.go` (guard + adapter + parent watch + clean shutdown)

**Interfaces:**
- Consumes: `guard.Machine`, `guard.Event`, `server.Controller`, `win.InputTag`, `win.POINT`.
- Produces:
```go
// win/msgwindow.go
const WM_APP = 0x8000
func RegisterClass(name string, wndProc uintptr) error                 // idempotent per name
func CreateMessageWindow(class string) (uintptr, error)                // HWND_MESSAGE child
func DefWindowProc(hwnd, msg, wparam, lparam uintptr) uintptr
func PostMessage(hwnd uintptr, msg uint32, wparam, lparam uintptr) error
func RunMessageLoop()                                                  // GetMessage/TranslateMessage/DispatchMessage until WM_QUIT
func PostQuitMessage()
func CurrentThreadID() uint32

// win/hooks.go
type HookKind int
const ( HookKeyDown HookKind = iota; HookKeyUp; HookMouseMove; HookMouseDown; HookWheel )
type HookEvent struct { Kind HookKind; VK uint16; X, Y int32; Injected bool }
func InstallLLHooks(sink func(HookEvent)) (remove func(), err error)   // call on a thread that runs a message loop

// win/process.go
func ParentPID() (uint32, error)
func WaitForProcessExit(pid uint32) error                              // blocks until the process ends
func UserUILanguage() string                                           // "ru" | "en" | other ISO-639-1 guesses → "en"

// uithread
type Thread struct{ /* unexported */ }
func New() (*Thread, error)                 // locked OS thread with a message loop
func (t *Thread) Do(f func())               // run f on the thread, asynchronously
func (t *Thread) DoSync(f func())           // run f and wait
func (t *Thread) ID() uint32
func (t *Thread) Close()                    // posts WM_QUIT and waits

// guard/runner_windows.go
type Runner struct{ /* unexported */ }
func Start(m *Machine, t *uithread.Thread) (*Runner, error)   // installs hooks on t, idle ticker every second
func (r *Runner) Stop()
```

- [ ] **Step 1: Message window and loop**

`internal/win/msgwindow.go`:
```go
//go:build windows

package win

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procRegisterClassExW  = user32.NewProc("RegisterClassExW")
	procCreateWindowExW   = user32.NewProc("CreateWindowExW")
	procDefWindowProcW    = user32.NewProc("DefWindowProcW")
	procGetMessageW       = user32.NewProc("GetMessageW")
	procTranslateMessage  = user32.NewProc("TranslateMessage")
	procDispatchMessageW  = user32.NewProc("DispatchMessageW")
	procPostQuitMessage   = user32.NewProc("PostQuitMessage")
	procGetModuleHandleW  = kernel32.NewProc("GetModuleHandleW")
	procGetCurrentThreadIdK = kernel32.NewProc("GetCurrentThreadId")
)

const WM_APP = 0x8000

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

var (
	classMu   sync.Mutex
	classDone = map[string]bool{}
)

func moduleHandle() uintptr {
	h, _, _ := procGetModuleHandleW.Call(0)
	return h
}

// RegisterClass registers a window class once. wndProc comes from windows.NewCallback.
func RegisterClass(name string, wndProc uintptr) error {
	classMu.Lock()
	defer classMu.Unlock()
	if classDone[name] {
		return nil
	}
	cn, _ := windows.UTF16PtrFromString(name)
	wc := wndClassEx{WndProc: wndProc, Instance: moduleHandle(), ClassName: cn}
	wc.Size = uint32(unsafe.Sizeof(wc))
	r, _, e := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if err := callErr("RegisterClassExW", r, e); err != nil {
		return err
	}
	classDone[name] = true
	return nil
}

// CreateMessageWindow creates an invisible message-only window (parent HWND_MESSAGE).
func CreateMessageWindow(class string) (uintptr, error) {
	cn, _ := windows.UTF16PtrFromString(class)
	const hwndMessage = ^uintptr(2) // (HWND)-3
	h, _, e := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cn)), 0, 0, 0, 0, 0, 0, hwndMessage, 0, moduleHandle(), 0)
	return h, callErr("CreateWindowExW", h, e)
}

func DefWindowProc(hwnd, m, wparam, lparam uintptr) uintptr {
	r, _, _ := procDefWindowProcW.Call(hwnd, m, wparam, lparam)
	return r
}

func PostMessage(hwnd uintptr, m uint32, wparam, lparam uintptr) error {
	r, _, e := procPostMessageW.Call(hwnd, uintptr(m), wparam, lparam)
	return callErr("PostMessageW", r, e)
}

// RunMessageLoop pumps messages for the calling thread until WM_QUIT.
func RunMessageLoop() {
	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func PostQuitMessage() { procPostQuitMessage.Call(0) }

func CurrentThreadID() uint32 {
	r, _, _ := procGetCurrentThreadIdK.Call()
	return uint32(r)
}
```
(`procPostMessageW` already exists in window.go; do not redeclare.)

- [ ] **Step 2: uithread**

`internal/uithread/uithread_windows.go`:
```go
//go:build windows

// Package uithread runs closures on one locked OS thread that pumps a Win32 message loop.
// Overlay windows and low-level hooks must live on such a thread.
package uithread

import (
	"log"
	"runtime"
	"sync"

	"golang.org/x/sys/windows"

	"github.com/racass-pixel/claude-computer-use/internal/win"
)

const className = "CuDispatch"

type Thread struct {
	hwnd uintptr
	tid  uint32
	q    chan func()
	done chan struct{}
}

var (
	regMu   sync.Mutex
	threads = map[uintptr]*Thread{}
	wndProc = windows.NewCallback(func(hwnd, msg, wparam, lparam uintptr) uintptr {
		if msg == win.WM_APP {
			regMu.Lock()
			t := threads[hwnd]
			regMu.Unlock()
			if t != nil {
				t.drain()
			}
			return 0
		}
		return win.DefWindowProc(hwnd, msg, wparam, lparam)
	})
)

func New() (*Thread, error) {
	t := &Thread{q: make(chan func(), 1024), done: make(chan struct{})}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		if err := win.RegisterClass(className, wndProc); err != nil {
			ready <- err
			return
		}
		hwnd, err := win.CreateMessageWindow(className)
		if err != nil {
			ready <- err
			return
		}
		t.hwnd, t.tid = hwnd, win.CurrentThreadID()
		regMu.Lock()
		threads[hwnd] = t
		regMu.Unlock()
		ready <- nil
		win.RunMessageLoop()
		regMu.Lock()
		delete(threads, hwnd)
		regMu.Unlock()
		close(t.done)
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Thread) drain() {
	for {
		select {
		case f := <-t.q:
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("uithread: panic in closure: %v", r)
					}
				}()
				f()
			}()
		default:
			return
		}
	}
}

func (t *Thread) Do(f func()) {
	t.q <- f
	_ = win.PostMessage(t.hwnd, win.WM_APP, 0, 0)
}

func (t *Thread) DoSync(f func()) {
	done := make(chan struct{})
	t.Do(func() { defer close(done); f() })
	<-done
}

func (t *Thread) ID() uint32 { return t.tid }

func (t *Thread) Close() {
	t.Do(win.PostQuitMessage)
	<-t.done
}
```

- [ ] **Step 3: Low-level hooks**

`internal/win/hooks.go`:
```go
//go:build windows

package win

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
)

const (
	whKeyboardLL  = 13
	whMouseLL     = 14
	llkhfInjected = 0x10
	llmhfInjected = 0x01

	wmKeyDown     = 0x0100
	wmKeyUp       = 0x0101
	wmSysKeyDown  = 0x0104
	wmSysKeyUp    = 0x0105
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmRButtonDown = 0x0204
	wmMButtonDown = 0x0207
	wmMouseWheel  = 0x020A
	wmMouseHWheel = 0x020E
	wmXButtonDown = 0x020B
)

type kbdLLHookStruct struct {
	VkCode, ScanCode, Flags, Time uint32
	ExtraInfo                     uintptr
}

type msLLHookStruct struct {
	Pt                     POINT
	MouseData, Flags, Time uint32
	ExtraInfo              uintptr
}

type HookKind int

const (
	HookKeyDown HookKind = iota
	HookKeyUp
	HookMouseMove
	HookMouseDown
	HookWheel
)

type HookEvent struct {
	Kind     HookKind
	VK       uint16
	X, Y     int32
	Injected bool
}

var (
	hookMu   sync.RWMutex
	hookSink func(HookEvent)

	kbdHookProc = windows.NewCallback(func(nCode int32, wparam, lparam uintptr) uintptr {
		if nCode >= 0 {
			k := (*kbdLLHookStruct)(unsafe.Pointer(lparam))
			ev := HookEvent{VK: uint16(k.VkCode), Injected: k.Flags&llkhfInjected != 0 || k.ExtraInfo == InputTag}
			switch wparam {
			case wmKeyDown, wmSysKeyDown:
				ev.Kind = HookKeyDown
			default:
				ev.Kind = HookKeyUp
			}
			hookMu.RLock()
			s := hookSink
			hookMu.RUnlock()
			if s != nil {
				s(ev)
			}
		}
		r, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wparam, lparam)
		return r
	})

	mouseHookProc = windows.NewCallback(func(nCode int32, wparam, lparam uintptr) uintptr {
		if nCode >= 0 {
			m := (*msLLHookStruct)(unsafe.Pointer(lparam))
			ev := HookEvent{X: m.Pt.X, Y: m.Pt.Y, Injected: m.Flags&llmhfInjected != 0 || m.ExtraInfo == InputTag}
			deliver := true
			switch wparam {
			case wmMouseMove:
				ev.Kind = HookMouseMove
			case wmLButtonDown, wmRButtonDown, wmMButtonDown, wmXButtonDown:
				ev.Kind = HookMouseDown
			case wmMouseWheel, wmMouseHWheel:
				ev.Kind = HookWheel
			default:
				deliver = false
			}
			if deliver {
				hookMu.RLock()
				s := hookSink
				hookMu.RUnlock()
				if s != nil {
					s(ev)
				}
			}
		}
		r, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wparam, lparam)
		return r
	})
)

// InstallLLHooks installs global keyboard and mouse hooks on the calling thread, which must pump messages.
// sink must return quickly (Windows silently removes hooks that take longer than a few hundred ms).
func InstallLLHooks(sink func(HookEvent)) (func(), error) {
	hookMu.Lock()
	hookSink = sink
	hookMu.Unlock()
	hk, _, e := procSetWindowsHookExW.Call(whKeyboardLL, kbdHookProc, 0, 0)
	if err := callErr("SetWindowsHookExW(keyboard)", hk, e); err != nil {
		return nil, err
	}
	hm, _, e := procSetWindowsHookExW.Call(whMouseLL, mouseHookProc, 0, 0)
	if err := callErr("SetWindowsHookExW(mouse)", hm, e); err != nil {
		procUnhookWindowsHookEx.Call(hk)
		return nil, err
	}
	return func() {
		procUnhookWindowsHookEx.Call(hk)
		procUnhookWindowsHookEx.Call(hm)
		hookMu.Lock()
		hookSink = nil
		hookMu.Unlock()
	}, nil
}
```

- [ ] **Step 4: Process helpers**

`internal/win/process.go`:
```go
//go:build windows

package win

import (
	"fmt"

	"golang.org/x/sys/windows"
)

var procGetUserDefaultUILanguage = kernel32.NewProc("GetUserDefaultUILanguage")

// ParentPID finds the parent of the current process via the toolhelp snapshot.
func ParentPID() (uint32, error) {
	me := windows.GetCurrentProcessId()
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(snap)
	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	for err = windows.Process32First(snap, &pe); err == nil; err = windows.Process32Next(snap, &pe) {
		if pe.ProcessID == me {
			return pe.ParentProcessID, nil
		}
	}
	return 0, fmt.Errorf("parent process not found")
}

// WaitForProcessExit blocks until pid exits.
func WaitForProcessExit(pid uint32) error {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	_, err = windows.WaitForSingleObject(h, windows.INFINITE)
	return err
}

// UserUILanguage maps the user's UI language to "ru" or "en".
func UserUILanguage() string {
	r, _, _ := procGetUserDefaultUILanguage.Call()
	if r&0x3FF == 0x19 { // LANG_RUSSIAN
		return "ru"
	}
	return "en"
}
```
(add `"unsafe"` to the imports.)

- [ ] **Step 5: Guard runner**

`internal/guard/runner_windows.go`:
```go
//go:build windows

package guard

import (
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/uithread"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

type Runner struct {
	m      *Machine
	remove func()
	stop   chan struct{}
	t      *uithread.Thread
}

// Start installs the low-level hooks on t and starts the idle ticker.
func Start(m *Machine, t *uithread.Thread) (*Runner, error) {
	r := &Runner{m: m, stop: make(chan struct{}), t: t}
	var err error
	t.DoSync(func() {
		r.remove, err = win.InstallLLHooks(func(h win.HookEvent) {
			ev := Event{VK: h.VK, Pos: geom.Point{X: int(h.X), Y: int(h.Y)}, Injected: h.Injected, At: time.Now()}
			switch h.Kind {
			case win.HookKeyDown:
				ev.Kind = KeyDown
			case win.HookKeyUp:
				ev.Kind = KeyUp
			case win.HookMouseMove:
				ev.Kind = MouseMove
			case win.HookMouseDown, win.HookWheel:
				ev.Kind = MouseDown
			}
			m.HandleEvent(ev)
		})
	})
	if err != nil {
		return nil, err
	}
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-r.stop:
				return
			case now := <-tick.C:
				m.CheckIdle(now)
			}
		}
	}()
	return r, nil
}

func (r *Runner) Stop() {
	close(r.stop)
	if r.remove != nil {
		r.t.DoSync(r.remove)
	}
}
```

- [ ] **Step 6: Wire into `cu serve`**

Replace the body of `runServe` in `cmd/cu/cmd_serve.go` with:
```go
func runServe(args []string) error {
	if err := win.SetPerMonitorDPIAwareV2(); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var w io.Writer = os.Stderr
	if cfg.LogFile != "" {
		if f, ferr := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); ferr == nil {
			defer f.Close()
			w = io.MultiWriter(os.Stderr, f)
		}
	}
	logger := log.New(w, "cu: ", log.Ltime|log.Lmicroseconds)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { // exit with Claude Code even if stdio is not closed cleanly
		if ppid, err := win.ParentPID(); err == nil {
			_ = win.WaitForProcessExit(ppid)
			logger.Printf("parent %d exited", ppid)
			cancel()
		}
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() { <-sig; cancel() }()

	hotkey, err := guard.ParseHotkey(cfg.Hotkey)
	if err != nil {
		return err
	}
	ui, err := uithread.New()
	if err != nil {
		return err
	}
	defer ui.Close()
	machine := guard.New(guard.Config{
		AutoPause: cfg.AutoPause, MouseThresholdPx: cfg.MouseThresholdPx,
		IdleRelease: time.Duration(cfg.IdleReleaseMs) * time.Millisecond, Hotkey: hotkey,
	}, func(tr guard.Transition) {
		logger.Printf("guard: %s -> %s (%s)", tr.From, tr.To, tr.Reason)
	})
	runner, err := guard.Start(machine, ui)
	if err != nil {
		return err
	}
	defer runner.Stop()

	deps := server.Deps{
		Screen:     screen.New(),
		Input:      input.New(),
		Clip:       input.NewClipboard(),
		Wins:       window.New(),
		Controller: controller{machine},
		Version:    version,
	}
	return server.Run(ctx, deps, cfg, logger)
}

// controller adapts guard.Machine to server.Controller.
type controller struct{ m *guard.Machine }

func (c controller) IsPaused() bool                     { return c.m.IsPaused() }
func (c controller) Acquire(now time.Time)              { c.m.Acquire(now) }
func (c controller) Touch(now time.Time)                { c.m.Touch(now) }
func (c controller) Release(now time.Time)              { c.m.Release(now) }
func (c controller) WaitResume(ctx context.Context) bool { return c.m.WaitResume(ctx) }
func (c controller) Status() server.ControllerStatus {
	st := c.m.Status()
	return server.ControllerStatus{State: st.State, Hotkey: st.Hotkey, IdleMs: st.IdleMs}
}
```
(imports: `time`, `.../internal/guard`, `.../internal/uithread`.) Note: `server.Run` returns when stdin closes; `ctx` cancel from the parent watcher also stops it.

- [ ] **Step 7: Manual verification**

Build; `claude --plugin-dir .`; ask for «Открой Блокнот и напиши три строки текста по одной». While it types, wiggle the mouse 3 cm: the next tool call returns `user_took_control`; Claude stops and says so. Then type «продолжай» — nothing resumes yet (Task 14 adds the hook), so press Esc Esc and ask again: control resumes and the task finishes. Check the stderr log shows `guard: controlling -> paused (physical_mouse)` and `paused -> controlling (hotkey)`. Kill the Claude Code terminal window: `cu.exe` must disappear from Task Manager within a second.

- [ ] **Step 8: Commit**

```bash
go build ./... && go vet ./... && go test ./...
git add -A && git commit -m "feat: low-level input hooks, UI thread and guard wiring with pause semantics"
```

---

### Task 12: Overlay — layered windows, glowing border, capture exclusion, `cu demo`

**Files:**
- Create: `internal/win/layered.go`
- Create: `internal/overlay/draw.go`, `internal/overlay/draw_test.go`, `internal/overlay/border.go`, `internal/overlay/border_test.go`, `internal/overlay/overlay.go`, `internal/overlay/overlay_windows.go`, `internal/overlay/overlay_other.go`
- Create: `cmd/cu/cmd_demo.go`, `cmd/cu/cmd_demo_other.go`; Modify: `cmd/cu/stubs.go` (remove `runDemo`), `cmd/cu/cmd_serve.go` (overlay + guard transitions), `internal/screen/capture.go` (`BeforeCapture`/`AfterCapture` hooks)

**Interfaces:**
- Consumes: `uithread.Thread`, `win.RegisterClass`, `win.DefWindowProc`, `platform.Overlay`, `platform.Monitor`, `config.Config.AccentRGB`.
- Produces:
```go
// win/layered.go
const OverlayClass = "CuOverlay"                                   // window.ExcludeClassPrefix matches this
var CaptureExclusionSupported = true                               // false if SetWindowDisplayAffinity failed
func CreateOverlayWindow(x, y, w, h int) (uintptr, error)          // WS_POPUP + layered/transparent/topmost/toolwindow/noactivate, excluded from capture, hidden
type LayeredSurface struct{ /* cached DIB */ }
func NewLayeredSurface(w, h int) (*LayeredSurface, error)
func (s *LayeredSurface) Update(hwnd uintptr, x, y int, img *image.RGBA) error   // premultiplies, UpdateLayeredWindow(ULW_ALPHA)
func (s *LayeredSurface) Close()
func ShowNoActivate(hwnd uintptr)
func HideWindow(hwnd uintptr)
func DestroyWindow(hwnd uintptr)

// overlay/draw.go (pure)
func premultiplyBGRA(dst []byte, src *image.RGBA)
func fillRoundedRect(img *image.RGBA, r image.Rectangle, radius float64, c color.RGBA)
func strokeRing(img *image.RGBA, cx, cy, radius, width float64, c color.RGBA)
func blendPixel(img *image.RGBA, x, y int, c color.RGBA, coverage float64)

// overlay/border.go (pure)
type borderSpec struct { W, H, Thick int; Color color.RGBA; Peak float64 }
type stripSet struct { Top, Bottom, Left, Right *image.RGBA; Alpha [4][]float64 /* base alpha per pixel */ }
func renderStrips(spec borderSpec) *stripSet
func (s *stripSet) apply(breath float64)          // rewrites each strip's alpha channel = base * breath

// overlay/overlay.go
type Config struct { Accent color.RGBA; Lang string; HotkeyLabel string; Thick int }   // Thick default 32 (logical px)
// overlay_windows.go
type Overlay struct{ /* unexported */ }
func New(t *uithread.Thread, cfg Config) (*Overlay, error)     // implements platform.Overlay
// overlay_other.go: func New(...) returns platform.NopOverlay-backed stub for non-Windows builds

// screen/capture.go additions
var BeforeCapture, AfterCapture func()   // called around every Grab when non-nil (fallback overlay hiding)
```

- [ ] **Step 1: Failing draw and border tests**

`internal/overlay/draw_test.go`:
```go
package overlay

import (
	"image"
	"image/color"
	"testing"
)

func TestPremultiplyBGRA(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1, 1))
	src.Pix[0], src.Pix[1], src.Pix[2], src.Pix[3] = 255, 0, 0, 128
	dst := make([]byte, 4)
	premultiplyBGRA(dst, src)
	if dst[0] != 0 || dst[1] != 0 || dst[2] != 128 || dst[3] != 128 {
		t.Fatalf("got %v want [0 0 128 128] (B G R A premultiplied)", dst)
	}
}

func TestRoundedRectCornersAreTransparent(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	fillRoundedRect(img, img.Bounds(), 8, color.RGBA{R: 20, G: 20, B: 20, A: 255})
	if a := img.RGBAAt(0, 0).A; a != 0 {
		t.Fatalf("corner alpha = %d, want 0", a)
	}
	if a := img.RGBAAt(20, 10).A; a != 255 {
		t.Fatalf("center alpha = %d, want 255", a)
	}
	if a := img.RGBAAt(20, 0).A; a != 255 {
		t.Fatalf("top edge middle alpha = %d, want 255", a)
	}
}

func TestRingHasHoleAndEdge(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 60, 60))
	strokeRing(img, 30, 30, 20, 3, color.RGBA{R: 255, A: 255})
	if img.RGBAAt(30, 30).A != 0 {
		t.Fatal("ring center must be empty")
	}
	if img.RGBAAt(50, 30).A == 0 {
		t.Fatal("ring edge must be painted")
	}
}
```

`internal/overlay/border_test.go`:
```go
package overlay

import (
	"image/color"
	"testing"
)

func TestStripsSizesAndFalloff(t *testing.T) {
	s := renderStrips(borderSpec{W: 200, H: 100, Thick: 10, Color: color.RGBA{R: 217, G: 119, B: 87, A: 255}, Peak: 0.6})
	if s.Top.Bounds().Dx() != 200 || s.Top.Bounds().Dy() != 10 || s.Left.Bounds().Dx() != 10 || s.Left.Bounds().Dy() != 80 {
		t.Fatalf("strip sizes wrong: top %v left %v", s.Top.Bounds(), s.Left.Bounds())
	}
	s.apply(1)
	edge, inner := s.Top.RGBAAt(100, 0).A, s.Top.RGBAAt(100, 9).A
	if edge < 140 || edge > 160 || inner >= edge/4 {
		t.Fatalf("alpha must fall off from ~153 at the edge: edge=%d inner=%d", edge, inner)
	}
	s.apply(0.5)
	if half := s.Top.RGBAAt(100, 0).A; half < 70 || half > 80 {
		t.Fatalf("breath 0.5 must halve alpha: %d", half)
	}
}
```
Run: `go test ./internal/overlay/` → FAIL.

- [ ] **Step 2: Implement draw.go and border.go**

`internal/overlay/draw.go`:
```go
// Package overlay draws the take-over UI: glowing monitor border, HUD pill and click ripples.
package overlay

import (
	"image"
	"image/color"
	"math"
)

// premultiplyBGRA converts straight-alpha RGBA into the premultiplied BGRA layout UpdateLayeredWindow expects.
func premultiplyBGRA(dst []byte, src *image.RGBA) {
	p := src.Pix
	for i := 0; i+3 < len(p) && i+3 < len(dst); i += 4 {
		a := uint32(p[i+3])
		dst[i+0] = byte(uint32(p[i+2]) * a / 255)
		dst[i+1] = byte(uint32(p[i+1]) * a / 255)
		dst[i+2] = byte(uint32(p[i+0]) * a / 255)
		dst[i+3] = byte(a)
	}
}

// blendPixel draws c over img at (x,y) with extra coverage (0..1) for anti-aliasing.
func blendPixel(img *image.RGBA, x, y int, c color.RGBA, coverage float64) {
	if coverage <= 0 || !(image.Point{x, y}.In(img.Bounds())) {
		return
	}
	a := float64(c.A) / 255 * min(1, coverage)
	d := img.RGBAAt(x, y)
	da := float64(d.A) / 255
	outA := a + da*(1-a)
	if outA <= 0 {
		return
	}
	mix := func(s, dst uint8) uint8 {
		return uint8((float64(s)*a + float64(dst)*da*(1-a)) / outA)
	}
	img.SetRGBA(x, y, color.RGBA{R: mix(c.R, d.R), G: mix(c.G, d.G), B: mix(c.B, d.B), A: uint8(outA * 255)})
}

// sdRoundedRect is the signed distance from (px,py) to a rounded box centered at (cx,cy) with half sizes hw,hh.
func sdRoundedRect(px, py, cx, cy, hw, hh, r float64) float64 {
	qx := math.Abs(px-cx) - hw + r
	qy := math.Abs(py-cy) - hh + r
	return math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) + math.Min(math.Max(qx, qy), 0) - r
}

func fillRoundedRect(img *image.RGBA, r image.Rectangle, radius float64, c color.RGBA) {
	cx := float64(r.Min.X+r.Max.X) / 2
	cy := float64(r.Min.Y+r.Max.Y) / 2
	hw, hh := float64(r.Dx())/2, float64(r.Dy())/2
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			d := sdRoundedRect(float64(x)+0.5, float64(y)+0.5, cx, cy, hw, hh, radius)
			blendPixel(img, x, y, c, 0.5-d) // 1px anti-aliased edge
		}
	}
}

func strokeRing(img *image.RGBA, cx, cy, radius, width float64, c color.RGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			d := math.Abs(math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)-radius) - width/2
			blendPixel(img, x, y, c, 0.5-d)
		}
	}
}
```

`internal/overlay/border.go`:
```go
package overlay

import (
	"image"
	"image/color"
)

type borderSpec struct {
	W, H, Thick int
	Color       color.RGBA
	Peak        float64 // alpha at the very edge, 0..1
}

// stripSet holds the four edge strips and their base alpha so frames only rescale alpha.
type stripSet struct {
	Top, Bottom, Left, Right *image.RGBA
	Alpha                    [4][]float64
}

func (s *stripSet) images() [4]*image.RGBA { return [4]*image.RGBA{s.Top, s.Bottom, s.Left, s.Right} }

// falloff is 1 at the monitor edge and 0 at Thick pixels inward (quadratic).
func falloff(d, thick int) float64 {
	t := 1 - float64(d)/float64(thick)
	return t * t
}

func renderStrips(spec borderSpec) *stripSet {
	th := spec.Thick
	s := &stripSet{
		Top:    image.NewRGBA(image.Rect(0, 0, spec.W, th)),
		Bottom: image.NewRGBA(image.Rect(0, 0, spec.W, th)),
		Left:   image.NewRGBA(image.Rect(0, 0, th, spec.H-2*th)),
		Right:  image.NewRGBA(image.Rect(0, 0, th, spec.H-2*th)),
	}
	fill := func(idx int, img *image.RGBA, alphaAt func(x, y int) float64) {
		b := img.Bounds()
		s.Alpha[idx] = make([]float64, b.Dx()*b.Dy())
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				a := alphaAt(x, y) * spec.Peak
				s.Alpha[idx][y*b.Dx()+x] = a
				img.SetRGBA(x, y, color.RGBA{R: spec.Color.R, G: spec.Color.G, B: spec.Color.B, A: uint8(a * 255)})
			}
		}
	}
	corner := func(x, w int) float64 { // soften the strip ends so corners don't double up
		if x < th {
			return falloff(th-1-x, th)*0.5 + 0.5
		}
		if x >= w-th {
			return falloff(x-(w-th), th)*0.5 + 0.5
		}
		return 1
	}
	fill(0, s.Top, func(x, y int) float64 { return falloff(y, th) * corner(x, spec.W) })
	fill(1, s.Bottom, func(x, y int) float64 { return falloff(th-1-y, th) * corner(x, spec.W) })
	fill(2, s.Left, func(x, y int) float64 { return falloff(x, th) })
	fill(3, s.Right, func(x, y int) float64 { return falloff(th-1-x, th) })
	return s
}

// apply rescales every strip's alpha by breath (0..1) for the breathing animation.
func (s *stripSet) apply(breath float64) {
	for i, img := range s.images() {
		base := s.Alpha[i]
		for p := 0; p < len(base); p++ {
			img.Pix[p*4+3] = uint8(base[p] * breath * 255)
		}
	}
}
```
Run: `go test ./internal/overlay/` → PASS.

- [ ] **Step 3: Layered window bindings**

`internal/win/layered.go`:
```go
//go:build windows

package win

import (
	"fmt"
	"image"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procUpdateLayeredWindow     = user32.NewProc("UpdateLayeredWindow")
	procSetWindowDisplayAffinity = user32.NewProc("SetWindowDisplayAffinity")
	procDestroyWindow           = user32.NewProc("DestroyWindow")
)

const (
	OverlayClass = "CuOverlay"

	wsPopup         = 0x80000000
	wsExLayered     = 0x00080000
	wsExTransparent = 0x00000020
	wsExTopmost     = 0x00000008
	wsExNoActivate  = 0x08000000
	ulwAlpha        = 0x00000002
	acSrcAlpha      = 0x01
	wdaExcludeFromCapture = 0x00000011
)

var CaptureExclusionSupported = true

var overlayWndProc = windows.NewCallback(func(hwnd, msg, wparam, lparam uintptr) uintptr {
	return DefWindowProc(hwnd, msg, wparam, lparam)
})

type blendFunction struct{ BlendOp, BlendFlags, SourceConstantAlpha, AlphaFormat byte }
type size struct{ Cx, Cy int32 }

// CreateOverlayWindow creates a hidden, click-through, always-on-top layered window excluded from capture.
func CreateOverlayWindow(x, y, w, h int) (uintptr, error) {
	if err := RegisterClass(OverlayClass, overlayWndProc); err != nil {
		return 0, err
	}
	cn, _ := windows.UTF16PtrFromString(OverlayClass)
	hwnd, _, e := procCreateWindowExW.Call(
		wsExLayered|wsExTransparent|wsExTopmost|wsExToolWindow|wsExNoActivate,
		uintptr(unsafe.Pointer(cn)), 0, wsPopup,
		uintptr(int32(x)), uintptr(int32(y)), uintptr(int32(w)), uintptr(int32(h)),
		0, 0, moduleHandle(), 0)
	if hwnd == 0 {
		return 0, callErr("CreateWindowExW(overlay)", hwnd, e)
	}
	if r, _, _ := procSetWindowDisplayAffinity.Call(hwnd, wdaExcludeFromCapture); r == 0 {
		CaptureExclusionSupported = false
	}
	return hwnd, nil
}

// LayeredSurface caches a DIB so 30 fps updates do not allocate.
type LayeredSurface struct {
	w, h int
	hdc  uintptr
	hbm  uintptr
	old  uintptr
	bits []byte
}

func NewLayeredSurface(w, h int) (*LayeredSurface, error) {
	screen, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, screen)
	hdc, _, _ := procCreateCompatibleDC.Call(screen)
	if hdc == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	bi := bitmapInfo{Header: bitmapInfoHeader{Size: 40, Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}}
	var bits unsafe.Pointer
	hbm, _, e := procCreateDIBSection.Call(hdc, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbm == 0 || bits == nil {
		procDeleteDC.Call(hdc)
		return nil, callErr("CreateDIBSection", hbm, e)
	}
	old, _, _ := procSelectObject.Call(hdc, hbm)
	return &LayeredSurface{w: w, h: h, hdc: hdc, hbm: hbm, old: old, bits: unsafe.Slice((*byte)(bits), w*h*4)}, nil
}

// Update pushes img (straight alpha, same size as the surface) to hwnd at screen position x,y.
func (s *LayeredSurface) Update(hwnd uintptr, x, y int, img *image.RGBA) error {
	if img.Bounds().Dx() != s.w || img.Bounds().Dy() != s.h {
		return fmt.Errorf("image %v does not match surface %dx%d", img.Bounds(), s.w, s.h)
	}
	premultiplyInto(s.bits, img)
	pt := POINT{int32(x), int32(y)}
	src := POINT{}
	sz := size{int32(s.w), int32(s.h)}
	bf := blendFunction{SourceConstantAlpha: 255, AlphaFormat: acSrcAlpha}
	r, _, e := procUpdateLayeredWindow.Call(hwnd, 0, uintptr(unsafe.Pointer(&pt)), uintptr(unsafe.Pointer(&sz)),
		s.hdc, uintptr(unsafe.Pointer(&src)), 0, uintptr(unsafe.Pointer(&bf)), ulwAlpha)
	return callErr("UpdateLayeredWindow", r, e)
}

func premultiplyInto(dst []byte, src *image.RGBA) {
	p := src.Pix
	for i := 0; i+3 < len(p) && i+3 < len(dst); i += 4 {
		a := uint32(p[i+3])
		dst[i+0] = byte(uint32(p[i+2]) * a / 255)
		dst[i+1] = byte(uint32(p[i+1]) * a / 255)
		dst[i+2] = byte(uint32(p[i+0]) * a / 255)
		dst[i+3] = byte(a)
	}
}

func (s *LayeredSurface) Close() {
	procSelectObject.Call(s.hdc, s.old)
	procDeleteObject.Call(s.hbm)
	procDeleteDC.Call(s.hdc)
}

func ShowNoActivate(hwnd uintptr) { procShowWindow.Call(hwnd, SW_SHOWNOACTIVATE) }
func HideWindow(hwnd uintptr)     { procShowWindow.Call(hwnd, 0 /* SW_HIDE */) }
func DestroyWindow(hwnd uintptr)  { procDestroyWindow.Call(hwnd) }
```

- [ ] **Step 4: Overlay object (border only in this task)**

`internal/overlay/overlay.go`:
```go
package overlay

import "image/color"

// Config is what the overlay needs from the app config.
type Config struct {
	Accent      color.RGBA
	Lang        string // "ru" | "en"
	HotkeyLabel string // "Esc Esc"
	Thick       int    // logical px, default 32
}

var pausedColor = color.RGBA{R: 154, G: 154, B: 154, A: 255}
```

`internal/overlay/overlay_windows.go`:
```go
//go:build windows

package overlay

import (
	"image"
	"math"
	"sync"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/uithread"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

type layeredWin struct {
	hwnd    uintptr
	surf    *win.LayeredSurface
	x, y    int
	w, h    int
	visible bool
}

func newLayeredWin(x, y, w, h int) (*layeredWin, error) {
	hwnd, err := win.CreateOverlayWindow(x, y, w, h)
	if err != nil {
		return nil, err
	}
	surf, err := win.NewLayeredSurface(w, h)
	if err != nil {
		win.DestroyWindow(hwnd)
		return nil, err
	}
	return &layeredWin{hwnd: hwnd, surf: surf, x: x, y: y, w: w, h: h}, nil
}

func (l *layeredWin) update(img *image.RGBA) {
	_ = l.surf.Update(l.hwnd, l.x, l.y, img)
	if !l.visible {
		win.ShowNoActivate(l.hwnd)
		l.visible = true
	}
}

func (l *layeredWin) hide() {
	if l.visible {
		win.HideWindow(l.hwnd)
		l.visible = false
	}
}

func (l *layeredWin) destroy() {
	l.hide()
	l.surf.Close()
	win.DestroyWindow(l.hwnd)
}

// Overlay implements platform.Overlay. All window work happens on the UI thread via t.Do.
type Overlay struct {
	cfg Config
	t   *uithread.Thread

	mu     sync.Mutex // guards the fields below (read on the UI thread, written from anywhere)
	state  platform.OverlayState
	mon    platform.Monitor
	title  string
	action string

	// UI-thread-only state
	strips   [4]*layeredWin
	set      *stripSet
	anim     *time.Ticker
	stopAnim chan struct{}
	phase    float64
	hideAt   *time.Timer
}

func New(t *uithread.Thread, cfg Config) (*Overlay, error) {
	if cfg.Thick <= 0 {
		cfg.Thick = 32
	}
	return &Overlay{cfg: cfg, t: t}, nil
}

func (o *Overlay) Show(m platform.Monitor, s platform.OverlayState) {
	o.mu.Lock()
	same := o.state == s && o.mon.ID == m.ID && o.mon.Rect == m.Rect
	o.state, o.mon = s, m
	o.mu.Unlock()
	if same {
		return
	}
	o.t.Do(func() { o.rebuild(m, s) })
}

func (o *Overlay) Hide() {
	o.mu.Lock()
	o.state = platform.OverlayHidden
	o.mu.Unlock()
	o.t.Do(o.hideAll)
}

func (o *Overlay) SetTitle(title string)   { o.mu.Lock(); o.title = title; o.mu.Unlock(); o.t.Do(o.refreshHUD) }
func (o *Overlay) SetAction(action string) { o.mu.Lock(); o.action = action; o.mu.Unlock(); o.t.Do(o.refreshHUD) }
func (o *Overlay) Ripple(p geom.Point)     { o.t.Do(func() { o.ripple(p) }) }

func (o *Overlay) Close() {
	o.t.DoSync(func() {
		o.hideAll()
		for i, s := range o.strips {
			if s != nil {
				s.destroy()
				o.strips[i] = nil
			}
		}
		o.destroyHUD()
	})
}

// ---- UI thread ----

func (o *Overlay) rebuild(m platform.Monitor, s platform.OverlayState) {
	o.hideAll()
	if s == platform.OverlayHidden {
		return
	}
	th := int(math.Round(float64(o.cfg.Thick) * m.ScaleFactor))
	r := m.Rect
	rects := [4]geom.Rect{
		{X: r.X, Y: r.Y, W: r.W, H: th},
		{X: r.X, Y: r.Bottom() - th, W: r.W, H: th},
		{X: r.X, Y: r.Y + th, W: th, H: r.H - 2*th},
		{X: r.Right() - th, Y: r.Y + th, W: th, H: r.H - 2*th},
	}
	for i, rr := range rects {
		if o.strips[i] == nil || o.strips[i].w != rr.W || o.strips[i].h != rr.H {
			if o.strips[i] != nil {
				o.strips[i].destroy()
			}
			lw, err := newLayeredWin(rr.X, rr.Y, rr.W, rr.H)
			if err != nil {
				return
			}
			o.strips[i] = lw
		}
		o.strips[i].x, o.strips[i].y = rr.X, rr.Y
	}
	col, peak := o.cfg.Accent, 0.6
	if s == platform.OverlayPaused {
		col, peak = pausedColor, 0.45
	}
	o.set = renderStrips(borderSpec{W: r.W, H: r.H, Thick: th, Color: col, Peak: peak})
	o.frame()
	o.showHUD(m, s)
	o.startAnim()
	if s == platform.OverlayPaused {
		o.hideAt = time.AfterFunc(2500*time.Millisecond, func() { o.t.Do(o.hideAll) })
	}
}

func (o *Overlay) frame() {
	if o.set == nil {
		return
	}
	breath := 0.72 + 0.28*math.Sin(o.phase)
	o.set.apply(breath)
	for i, img := range o.set.images() {
		if o.strips[i] != nil {
			o.strips[i].update(img)
		}
	}
	o.pulseHUD()
}

func (o *Overlay) startAnim() {
	o.stopAnimLoop()
	o.anim = time.NewTicker(time.Second / 30)
	o.stopAnim = make(chan struct{})
	stop, tick := o.stopAnim, o.anim
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				o.t.Do(func() {
					o.phase += 2 * math.Pi / (2.4 * 30)
					o.frame()
				})
			}
		}
	}()
}

func (o *Overlay) stopAnimLoop() {
	if o.anim != nil {
		o.anim.Stop()
		close(o.stopAnim)
		o.anim = nil
	}
}

func (o *Overlay) hideAll() {
	o.stopAnimLoop()
	if o.hideAt != nil {
		o.hideAt.Stop()
		o.hideAt = nil
	}
	for _, s := range o.strips {
		if s != nil {
			s.hide()
		}
	}
	o.hideHUD()
}

// HUD and ripple are implemented in Task 13; keep these stubs compiling until then.
func (o *Overlay) showHUD(platform.Monitor, platform.OverlayState) {}
func (o *Overlay) refreshHUD()                                    {}
func (o *Overlay) pulseHUD()                                      {}
func (o *Overlay) hideHUD()                                       {}
func (o *Overlay) destroyHUD()                                    {}
func (o *Overlay) ripple(geom.Point)                              {}

var _ platform.Overlay = (*Overlay)(nil)
```
`internal/overlay/overlay_other.go` (`//go:build !windows`): `type Overlay = platform.NopOverlay` plus `func New(_ any, _ Config) (*Overlay, error) { return &Overlay{}, nil }`.

- [ ] **Step 5: Capture hooks and `cu demo`**

In `internal/screen/capture.go` add package vars and call them in `Grab`:
```go
// BeforeCapture/AfterCapture let the overlay hide itself around a capture when the OS cannot exclude it.
var BeforeCapture, AfterCapture func()
```
In `Grab`, before `s.Capture(rect)`: `if BeforeCapture != nil { BeforeCapture() }`; after it: `if AfterCapture != nil { AfterCapture() }`.

`cmd/cu/cmd_demo.go` (delete `runDemo` from stubs):
```go
//go:build windows

package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/config"
	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/overlay"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/screen"
	"github.com/racass-pixel/claude-computer-use/internal/uithread"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

func runDemo(args []string) error {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	secs := fs.Int("seconds", 5, "how long to show the overlay")
	out := fs.String("o", "", "also save a screenshot (to prove the overlay is excluded)")
	mon := fs.Int("m", 1, "monitor id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := win.SetPerMonitorDPIAwareV2(); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	r, g, b, err := cfg.AccentRGB()
	if err != nil {
		return err
	}
	ui, err := uithread.New()
	if err != nil {
		return err
	}
	defer ui.Close()
	lang := cfg.Lang
	if lang == "auto" {
		lang = win.UserUILanguage()
	}
	ov, err := overlay.New(ui, overlay.Config{Accent: color.RGBA{R: r, G: g, B: b, A: 255}, Lang: lang, HotkeyLabel: "Esc Esc"})
	if err != nil {
		return err
	}
	defer ov.Close()
	s := screen.New()
	mons, err := s.Monitors()
	if err != nil {
		return err
	}
	m, ok := screen.MonitorByID(mons, *mon)
	if !ok {
		return fmt.Errorf("no monitor %d", *mon)
	}
	ov.SetTitle("Demo: Claude Computer Use")
	ov.Show(m, platform.OverlayControlling)
	for i := 0; i < *secs*2; i++ {
		ov.SetAction(fmt.Sprintf("click %d,%d", 100+i*40, 200))
		ov.Ripple(geom.Point{X: m.Rect.X + 200 + i*60, Y: m.Rect.Y + 300})
		time.Sleep(500 * time.Millisecond)
		if *out != "" && i == *secs {
			shot, err := screen.Grab(s, m.Rect, screen.AutoScale(m.Rect, 1366), "png", 85)
			if err != nil {
				return err
			}
			if err := os.WriteFile(*out, shot.Data, 0o644); err != nil {
				return err
			}
			fmt.Printf("saved %s (capture exclusion supported: %v)\n", *out, win.CaptureExclusionSupported)
		}
	}
	ov.Show(m, platform.OverlayPaused)
	time.Sleep(3 * time.Second)
	return nil
}
```
Plus `cmd/cu/cmd_demo_other.go`.

- [ ] **Step 6: The capture-exclusion spike (decides the fallback)**

Run: `go run ./cmd/cu demo -seconds 4 -o C:\Users\test\AppData\Local\Temp\demo.png`.
Observe: an orange breathing border on monitor 1 for 4 s, then a gray border that disappears after ~2.5 s. The border must not steal focus or block clicks (click a window behind the strip while it is shown).
Open `demo.png` with the Read tool: the border must NOT be visible.
- If it IS visible: set `win.IncludeLayeredWindows = false` in `cmd_serve.go`/`cmd_demo.go` before capturing and re-run. If still visible, install the fallback: in `cmd_serve.go` set `screen.BeforeCapture = func() { ui.DoSync(ov.HideForCapture) }` / `screen.AfterCapture = func() { ui.DoSync(ov.ShowAfterCapture) }` — add those two methods to `Overlay` (hide/show the strip and HUD windows without changing state). Record which path is active in `cu doctor` output as `overlay capture exclusion: native | hidden-during-capture`.

- [ ] **Step 7: Wire the overlay into `cu serve`**

In `runServe`, after creating `ui` and before the guard machine:
```go
	r, g, b, err := cfg.AccentRGB()
	if err != nil {
		return err
	}
	lang := cfg.Lang
	if lang == "auto" {
		lang = win.UserUILanguage()
	}
	var ov platform.Overlay = platform.NopOverlay{}
	if cfg.Overlay {
		o, err := overlay.New(ui, overlay.Config{Accent: color.RGBA{R: r, G: g, B: b, A: 255}, Lang: lang, HotkeyLabel: hotkey.String()})
		if err != nil {
			return err
		}
		ov = o
	}
	defer ov.Close()
```
and make the guard's `onChange` drive it (needs the active monitor; keep a tiny helper `activeMon := func() platform.Monitor { mons, _ := screen.ListMonitors(); p, _ := win.GetCursorPos(); fg := window.New(); w, _ := fg.Foreground(); return screen.ActiveMonitor(mons, w.Rect, geom.Point{X: int(p.X), Y: int(p.Y)}) }`):
```go
	}, func(tr guard.Transition) {
		logger.Printf("guard: %s -> %s (%s)", tr.From, tr.To, tr.Reason)
		switch tr.To {
		case guard.Paused:
			ov.Show(activeMon(), platform.OverlayPaused)
		case guard.Controlling:
			ov.Show(activeMon(), platform.OverlayControlling)
		case guard.Idle:
			ov.Hide()
		}
	})
```
Pass `Overlay: ov` in `server.Deps`. Build, run in Claude Code, ask for a small task: the border appears on the first action, follows the monitor of the action, turns gray when you move the mouse, and disappears 2 minutes after the last action or when the session ends.

- [ ] **Step 8: Commit**

```bash
go build ./... && go vet ./... && go test ./...
git add -A && git commit -m "feat: glowing monitor border overlay with capture exclusion and cu demo"
```

---

### Task 13: Overlay — HUD pill with text, click ripple, paused state, localization

**Files:**
- Create: `internal/overlay/hud.go`, `internal/overlay/hud_test.go`, `internal/overlay/ripple.go`
- Modify: `internal/overlay/overlay_windows.go` (replace the HUD/ripple stubs)

**Interfaces:**
- Consumes: `fillRoundedRect`, `strokeRing`, `blendPixel`, `layeredWin`, `golang.org/x/image/font`, `golang.org/x/image/font/opentype`, `golang.org/x/image/font/gofont/{gomedium,goregular}`.
- Produces:
```go
func hudText(lang, hotkeyLabel, title, action string, paused bool) (line1, line2 string)
type hudSpec struct { Title, Sub string; Scale float64; Accent color.RGBA; Paused bool; Pulse float64 /* 0..1 dot brightness */ }
func renderHUD(spec hudSpec) *image.RGBA          // sized to content
func renderRipple(size int, t float64, accent color.RGBA) *image.RGBA   // t 0..1
```

- [ ] **Step 1: Failing HUD tests**

`internal/overlay/hud_test.go`:
```go
package overlay

import (
	"image/color"
	"testing"
)

func TestHudTextLocalization(t *testing.T) {
	l1, l2 := hudText("ru", "Esc Esc", "", "click 1,2", false)
	if l1 != "Claude управляет компьютером" || l2 != "Esc Esc — забрать управление  ·  click 1,2" {
		t.Fatalf("ru: %q / %q", l1, l2)
	}
	l1, l2 = hudText("en", "Ctrl+Alt+Esc", "Filling the order form", "", false)
	if l1 != "Filling the order form" || l2 != "Ctrl+Alt+Esc — take control" {
		t.Fatalf("en custom title: %q / %q", l1, l2)
	}
	l1, l2 = hudText("en", "Esc Esc", "ignored while paused", "ignored", true)
	if l1 != "You are in control" || l2 != "Esc Esc — hand back to Claude" {
		t.Fatalf("paused: %q / %q", l1, l2)
	}
	if l1, _ := hudText("xx", "Esc Esc", "", "", false); l1 != "Claude is controlling the computer" {
		t.Fatalf("unknown lang must fall back to en: %q", l1)
	}
}

func TestRenderHUDProducesAPill(t *testing.T) {
	img := renderHUD(hudSpec{Title: "Claude управляет компьютером", Sub: "Esc Esc — забрать управление", Scale: 1, Accent: color.RGBA{R: 217, G: 119, B: 87, A: 255}, Pulse: 1})
	b := img.Bounds()
	if b.Dx() < 200 || b.Dy() < 40 || b.Dx() < b.Dy()*3 {
		t.Fatalf("unexpected HUD size %v", b)
	}
	if img.RGBAAt(0, 0).A != 0 {
		t.Fatal("pill corners must be transparent")
	}
	if img.RGBAAt(b.Dx()/2, b.Dy()/2).A < 200 {
		t.Fatal("pill body must be nearly opaque")
	}
}

func TestRippleFadesOut(t *testing.T) {
	a := renderRipple(120, 0.1, color.RGBA{R: 255, A: 255})
	z := renderRipple(120, 1.0, color.RGBA{R: 255, A: 255})
	var sumA, sumZ int
	for i := 3; i < len(a.Pix); i += 4 {
		sumA += int(a.Pix[i])
		sumZ += int(z.Pix[i])
	}
	if sumA == 0 || sumZ != 0 {
		t.Fatalf("ripple alpha early=%d late=%d", sumA, sumZ)
	}
}
```
Run: `go test ./internal/overlay/` → FAIL.

- [ ] **Step 2: hud.go and ripple.go**

`internal/overlay/hud.go`:
```go
package overlay

import (
	"image"
	"image/color"
	"math"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type hudStrings struct{ Title, Sub, PausedTitle, PausedSub string }

var texts = map[string]hudStrings{
	"en": {"Claude is controlling the computer", "%s — take control", "You are in control", "%s — hand back to Claude"},
	"ru": {"Claude управляет компьютером", "%s — забрать управление", "Управление у вас", "%s — вернуть Claude"},
}

func hudText(lang, hotkeyLabel, title, action string, paused bool) (string, string) {
	s, ok := texts[lang]
	if !ok {
		s = texts["en"]
	}
	if paused {
		return s.PausedTitle, sprintf(s.PausedSub, hotkeyLabel)
	}
	l1 := s.Title
	if title != "" {
		l1 = title
	}
	l2 := sprintf(s.Sub, hotkeyLabel)
	if action != "" {
		l2 += "  ·  " + action
	}
	return l1, l2
}

func sprintf(format, a string) string { // avoid fmt for the hot path; format has exactly one %s
	out := make([]byte, 0, len(format)+len(a))
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) && format[i+1] == 's' {
			out = append(out, a...)
			i++
			continue
		}
		out = append(out, format[i])
	}
	return string(out)
}

type hudSpec struct {
	Title, Sub string
	Scale      float64
	Accent     color.RGBA
	Paused     bool
	Pulse      float64
}

var (
	fontOnce sync.Once
	fontMed  *opentype.Font
	fontReg  *opentype.Font
)

func loadFonts() {
	fontMed, _ = opentype.Parse(gomedium.TTF)
	fontReg, _ = opentype.Parse(goregular.TTF)
}

func face(f *opentype.Font, px float64) font.Face {
	fc, _ := opentype.NewFace(f, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingFull})
	return fc
}

func textWidth(fc font.Face, s string) int { return font.MeasureString(fc, s).Ceil() }

func drawText(img *image.RGBA, fc font.Face, x, baseline int, s string, c color.RGBA) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: fc, Dot: fixed.P(x, baseline)}
	d.DrawString(s)
}

// renderHUD draws the pill: [dot] Title / Sub, dark translucent background, 1px light border.
func renderHUD(spec hudSpec) *image.RGBA {
	fontOnce.Do(loadFonts)
	sc := spec.Scale
	if sc <= 0 {
		sc = 1
	}
	titleFace := face(fontMed, 15*sc)
	subFace := face(fontReg, 12.5*sc)
	defer titleFace.Close()
	defer subFace.Close()

	padX, padY := int(16*sc), int(10*sc)
	dot := int(8 * sc)
	gap := int(9 * sc)
	lineGap := int(4 * sc)
	tw := max(textWidth(titleFace, spec.Title), textWidth(subFace, spec.Sub))
	titleH := titleFace.Metrics().Height.Ceil()
	subH := subFace.Metrics().Height.Ceil()
	w := padX*2 + dot + gap + tw
	h := padY*2 + titleH + lineGap + subH
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	radius := 14 * sc
	bg := color.RGBA{R: 22, G: 22, B: 24, A: 222}
	if spec.Paused {
		bg = color.RGBA{R: 44, G: 44, B: 46, A: 222}
	}
	fillRoundedRect(img, img.Bounds(), radius, color.RGBA{R: 255, G: 255, B: 255, A: 28}) // hairline border
	fillRoundedRect(img, image.Rect(1, 1, w-1, h-1), radius-1, bg)

	dotC := spec.Accent
	if spec.Paused {
		dotC = pausedColor
	}
	dotC.A = uint8(120 + 135*math.Max(0, math.Min(1, spec.Pulse)))
	cy := float64(padY) + float64(titleH)/2
	strokeRing(img, float64(padX)+float64(dot)/2, cy, float64(dot)/2, float64(dot), dotC) // a filled disc: width == diameter

	x := padX + dot + gap
	drawText(img, titleFace, x, padY+titleFace.Metrics().Ascent.Ceil(), spec.Title, color.RGBA{R: 245, G: 245, B: 245, A: 255})
	drawText(img, subFace, x, padY+titleH+lineGap+subFace.Metrics().Ascent.Ceil(), spec.Sub, color.RGBA{R: 200, G: 200, B: 205, A: 255})
	return img
}
```

`internal/overlay/ripple.go`:
```go
package overlay

import (
	"image"
	"image/color"
)

// renderRipple draws an expanding, fading ring; t runs 0..1 over the animation.
func renderRipple(size int, t float64, accent color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	if t >= 1 {
		return img
	}
	r := float64(size) * (0.18 + 0.30*t)
	c := accent
	c.A = uint8(230 * (1 - t))
	strokeRing(img, float64(size)/2, float64(size)/2, r, 3, c)
	return img
}
```
Run: `go test ./internal/overlay/` → PASS.

- [ ] **Step 3: Replace the stubs in overlay_windows.go**

Add fields to `Overlay`: `hud *layeredWin; hudImg *image.RGBA; hudMon platform.Monitor; hudPaused bool; rip *layeredWin; ripStop chan struct{}`.

```go
func (o *Overlay) hudSpecNow() (hudSpec, platform.Monitor, platform.OverlayState) {
	o.mu.Lock()
	defer o.mu.Unlock()
	paused := o.state == platform.OverlayPaused
	l1, l2 := hudText(o.cfg.Lang, o.cfg.HotkeyLabel, o.title, o.action, paused)
	return hudSpec{Title: l1, Sub: l2, Scale: o.mon.ScaleFactor, Accent: o.cfg.Accent, Paused: paused, Pulse: 0.5 + 0.5*math.Sin(o.phase*2)}, o.mon, o.state
}

func (o *Overlay) showHUD(m platform.Monitor, s platform.OverlayState) { o.refreshHUD() }

func (o *Overlay) refreshHUD() {
	spec, m, st := o.hudSpecNow()
	if st == platform.OverlayHidden || m.ID == 0 {
		return
	}
	img := renderHUD(spec)
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	x := m.Rect.X + (m.Rect.W-w)/2
	y := m.Rect.Y + int(14*m.ScaleFactor)
	if o.hud == nil || o.hud.w != w || o.hud.h != h {
		if o.hud != nil {
			o.hud.destroy()
		}
		lw, err := newLayeredWin(x, y, w, h)
		if err != nil {
			return
		}
		o.hud = lw
	}
	o.hud.x, o.hud.y = x, y
	o.hudImg = img
	o.hud.update(img)
}

// pulseHUD is called every frame; re-rendering text at 30 fps is ~1 ms, acceptable.
func (o *Overlay) pulseHUD() {
	if o.hud != nil && o.hud.visible {
		o.refreshHUD()
	}
}

func (o *Overlay) hideHUD() {
	if o.hud != nil {
		o.hud.hide()
	}
	if o.rip != nil {
		o.rip.hide()
	}
}

func (o *Overlay) destroyHUD() {
	if o.hud != nil {
		o.hud.destroy()
		o.hud = nil
	}
	if o.rip != nil {
		o.rip.destroy()
		o.rip = nil
	}
}

func (o *Overlay) ripple(p geom.Point) {
	o.mu.Lock()
	sc, st := o.mon.ScaleFactor, o.state
	o.mu.Unlock()
	if st != platform.OverlayControlling {
		return
	}
	size := int(120 * math.Max(1, sc))
	if o.rip == nil || o.rip.w != size {
		if o.rip != nil {
			o.rip.destroy()
		}
		lw, err := newLayeredWin(p.X-size/2, p.Y-size/2, size, size)
		if err != nil {
			return
		}
		o.rip = lw
	}
	o.rip.x, o.rip.y = p.X-size/2, p.Y-size/2
	if o.ripStop != nil {
		close(o.ripStop)
	}
	stop := make(chan struct{})
	o.ripStop = stop
	start := time.Now()
	go func() {
		tick := time.NewTicker(time.Second / 60)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				t := float64(time.Since(start)) / float64(350*time.Millisecond)
				o.t.Do(func() {
					if o.rip == nil {
						return
					}
					if t >= 1 {
						o.rip.hide()
						return
					}
					o.rip.update(renderRipple(size, t, o.cfg.Accent))
				})
				if t >= 1 {
					return
				}
			}
		}
	}()
}
```
Delete the stub methods from Task 12. Build and run `go run ./cmd/cu demo -seconds 6`: the pill shows the Russian (or English) title, the hotkey hint and the changing action text; a ring ripples along; then the paused state shows "Управление у вас · Esc Esc — вернуть Claude" in gray and fades. Check on a 125%/150% DPI monitor if available: text stays crisp and the pill scales.

- [ ] **Step 4: Commit**

```bash
go build ./... && go vet ./... && go test ./...
git add -A && git commit -m "feat: HUD pill with localized text, click ripple and paused state"
```

---

### Task 14: IPC named pipe, `cu ctl`, plugin hooks, clean shutdown

**Files:**
- Create: `internal/ipc/pipe_windows.go`, `internal/ipc/client_windows.go`, `internal/ipc/ipc_other.go`
- Create: `cmd/cu/cmd_ctl.go`, `cmd/cu/cmd_ctl_other.go`; Modify: `cmd/cu/stubs.go` (remove `runCtl`; the file should now be empty — delete it), `cmd/cu/cmd_serve.go`
- Create: `hooks/hooks.json`

**Interfaces:**
```go
package ipc
const Prefix = "claude-computer-use-"                       // pipe name = \\.\pipe\<Prefix><pid>
type Handler func(cmd string) (any, error)
func Serve(ctx context.Context, pid uint32, h Handler) error  // returns when ctx ends
type Reply struct { Pipe string; Body json.RawMessage; Err error }
func Broadcast(cmd string, timeout time.Duration) []Reply     // sends cmd to every cu server on this machine
```
Protocol: client writes one line `<cmd>\n`; server answers one line of JSON `{"ok":true,"result":...}` or `{"ok":false,"error":"..."}` and closes.

- [ ] **Step 1: Pipe server and client**

Run `go get github.com/Microsoft/go-winio@latest`.

`internal/ipc/pipe_windows.go`:
```go
//go:build windows

// Package ipc lets `cu ctl` (run by Claude Code hooks or the user) talk to running cu servers.
package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/Microsoft/go-winio"
)

const Prefix = "claude-computer-use-"

type Handler func(cmd string) (any, error)

func pipeName(pid uint32) string { return fmt.Sprintf(`\\.\pipe\%s%d`, Prefix, pid) }

func Serve(ctx context.Context, pid uint32, h Handler) error {
	l, err := winio.ListenPipe(pipeName(pid), nil)
	if err != nil {
		return err
	}
	go func() { <-ctx.Done(); l.Close() }()
	for {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go serveConn(conn, h)
	}
}

func serveConn(conn net.Conn, h Handler) {
	defer conn.Close()
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && line == "" {
		return
	}
	cmd := strings.TrimSpace(line)
	res, err := h(cmd)
	var out []byte
	if err != nil {
		out, _ = json.Marshal(map[string]any{"ok": false, "error": err.Error()})
	} else {
		out, _ = json.Marshal(map[string]any{"ok": true, "result": res})
	}
	conn.Write(append(out, '\n'))
}
```

`internal/ipc/client_windows.go`:
```go
//go:build windows

package ipc

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
)

type Reply struct {
	Pipe string
	Body json.RawMessage
	Err  error
}

func listServers() []string {
	entries, err := os.ReadDir(`\\.\pipe\`)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), Prefix) {
			out = append(out, `\\.\pipe\`+e.Name())
		}
	}
	return out
}

// Broadcast sends cmd to every running cu server and collects replies.
func Broadcast(cmd string, timeout time.Duration) []Reply {
	var replies []Reply
	for _, name := range listServers() {
		r := Reply{Pipe: name}
		conn, err := winio.DialPipe(name, &timeout)
		if err != nil {
			r.Err = err
			replies = append(replies, r)
			continue
		}
		conn.SetDeadline(time.Now().Add(timeout))
		if _, err := conn.Write([]byte(cmd + "\n")); err != nil {
			r.Err = err
		} else if line, err := bufio.NewReader(conn).ReadString('\n'); err != nil && line == "" {
			r.Err = err
		} else {
			r.Body = json.RawMessage(strings.TrimSpace(line))
		}
		conn.Close()
		replies = append(replies, r)
	}
	return replies
}
```
`internal/ipc/ipc_other.go` (`//go:build !windows`): same exported names returning `errors.New("ipc is Windows-only")` / nil.

- [ ] **Step 2: `cu ctl`**

`cmd/cu/cmd_ctl.go`:
```go
//go:build windows

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/ipc"
)

// runCtl never returns an error in --quiet mode: hooks must not fail Claude Code when no server runs.
func runCtl(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cu ctl status|resume|release|pause [--quiet]")
	}
	cmd := args[0]
	quiet := len(args) > 1 && args[1] == "--quiet"
	switch cmd {
	case "status", "resume", "release", "pause":
	default:
		return fmt.Errorf("unknown ctl command %q", cmd)
	}
	replies := ipc.Broadcast(cmd, 2*time.Second)
	if quiet {
		return nil
	}
	if len(replies) == 0 {
		fmt.Fprintln(os.Stderr, "no running cu server found")
		return nil
	}
	for _, r := range replies {
		if r.Err != nil {
			fmt.Printf("%s: error: %v\n", r.Pipe, r.Err)
			continue
		}
		fmt.Printf("%s: %s\n", r.Pipe, r.Body)
	}
	return nil
}
```
Plus `cmd_ctl_other.go`. Delete `cmd/cu/stubs.go` (all stubs are now real).

- [ ] **Step 3: Serve the pipe in `cu serve`**

In `runServe`, after `runner` is started:
```go
	go func() {
		err := ipc.Serve(ctx, windows.GetCurrentProcessId(), func(cmd string) (any, error) {
			now := time.Now()
			switch cmd {
			case "status":
				return machine.Status(), nil
			case "resume":
				machine.Resume(now, guard.ReasonPrompt)
				return machine.Status(), nil
			case "release":
				machine.Release(now)
				return machine.Status(), nil
			case "pause":
				machine.Pause(now, guard.ReasonHotkey)
				return machine.Status(), nil
			}
			return nil, fmt.Errorf("unknown command %q", cmd)
		})
		if err != nil {
			logger.Printf("ipc: %v", err)
		}
	}()
```
(imports `golang.org/x/sys/windows`, `.../internal/ipc`, `fmt`.)

- [ ] **Step 4: Plugin hooks**

`hooks/hooks.json`:
```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "\"${CLAUDE_PLUGIN_ROOT}/bin/cu.exe\" ctl resume --quiet",
            "shell": "bash",
            "async": true,
            "timeout": 5
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "\"${CLAUDE_PLUGIN_ROOT}/bin/cu.exe\" ctl release --quiet",
            "shell": "bash",
            "async": true,
            "timeout": 5
          }
        ]
      }
    ],
    "SubagentStop": [
      {
        "matcher": "operator",
        "hooks": [
          {
            "type": "command",
            "command": "\"${CLAUDE_PLUGIN_ROOT}/bin/cu.exe\" ctl release --quiet",
            "shell": "bash",
            "async": true,
            "timeout": 5
          }
        ]
      }
    ]
  }
}
```
Verify the shape with `claude plugin validate .`. If the validator rejects `shell` or `async`, drop those keys (the superpowers plugin on this machine uses both, so they should pass).

- [ ] **Step 5: Verify end to end**

1. Build. `bin\cu.exe ctl status` with no server → "no running cu server found".
2. `claude --plugin-dir .`; ask for a 3-step Notepad task; move the mouse mid-task → operator/model stops with `user_took_control`; stderr shows `paused (physical_mouse)`.
3. Type «продолжай» → stderr shows `guard: paused -> controlling (prompt)` before the model's next tool call, and the task completes.
4. When the model finishes its answer, the Stop hook fires → `controlling -> idle (release)` and the border disappears within a second.
5. In another terminal while the server runs: `bin\cu.exe ctl status` prints `{"ok":true,"result":{"State":"idle",...}}`.

- [ ] **Step 6: Commit**

```bash
go build ./... && go vet ./... && go test ./...
git add -A && git commit -m "feat: named-pipe control channel, cu ctl and plugin hooks for resume/release"
```

---

### Task 15: UI Automation — `find`, element targeting, `wait{element}`

**Files:**
- Create: `internal/uia/roles.go`, `internal/uia/roles_test.go`, `internal/uia/com_windows.go`, `internal/uia/uia_windows.go`, `internal/uia/uia_other.go`
- Create: `internal/server/tools_find.go`, `internal/server/tools_find_test.go`
- Modify: `internal/server/server.go` (register `find`, add to `batchable`), `internal/server/tools_wait.go` (`Element` condition), `cmd/cu/cmd_serve.go` (`Access: uia.New()`), `cmd/cu/cmd_doctor.go` (UIA check)

**Interfaces:**
```go
// uia
func RoleID(name string) (int32, bool)    // "button" → 50000 (case-insensitive)
func RoleName(id int32) string            // 50000 → "Button"; unknown → "Custom"
type UIA struct{ /* unexported */ }
func New() (*UIA, error)                  // dedicated COM (MTA) thread; implements platform.Accessibility
func (u *UIA) Close()

// server
type FindIn struct {
	Query        string `json:"query,omitempty"`
	Role         string `json:"role,omitempty"`
	Window       string `json:"window,omitempty"`        // "foreground" (default) | "all" | id | regexp
	AutomationID string `json:"automation_id,omitempty"`
	Limit        int    `json:"limit,omitempty"`
	Screenshot   *bool  `json:"screenshot,omitempty"`    // default false
}
type elementOut struct {
	ID string `json:"id"`; Name string `json:"name"`; Role string `json:"role"`
	AutomationID string `json:"automation_id,omitempty"`; Value string `json:"value,omitempty"`
	Rect geom.Rect `json:"rect"`; Center [2]int `json:"center"`; ScreenRect geom.Rect `json:"screen_rect"`
	Enabled bool `json:"enabled"`; Focused bool `json:"focused"`; InView bool `json:"in_view"`
}
func (s *Session) toolFind(ctx, req, in FindIn) (*mcp.CallToolResult, any, error)
func (s *Session) storeElements(els []platform.Element)   // releases previous refs, assigns ids e1..eN
```
`WaitIn` gains `Element *FindIn `json:"element,omitempty"`` (poll `find` every 200 ms until at least one match).

- [ ] **Step 1: Failing role and find tests**

`internal/uia/roles_test.go`:
```go
package uia

import "testing"

func TestRoles(t *testing.T) {
	if id, ok := RoleID("button"); !ok || id != 50000 {
		t.Fatalf("button → %d %v", id, ok)
	}
	if id, _ := RoleID("MenuItem"); id != 50011 {
		t.Fatalf("MenuItem → %d", id)
	}
	if RoleName(50004) != "Edit" || RoleName(99999) != "Custom" {
		t.Fatal("RoleName wrong")
	}
	if _, ok := RoleID("nope"); ok {
		t.Fatal("unknown role must not resolve")
	}
}
```

`internal/server/tools_find_test.go`:
```go
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
```
Run: `go test ./internal/uia/ ./internal/server/` → FAIL.

- [ ] **Step 2: roles.go**

```go
// Package uia implements platform.Accessibility with Windows UI Automation over raw COM (no cgo).
package uia

import "strings"

var roleIDs = map[string]int32{
	"button": 50000, "calendar": 50001, "checkbox": 50002, "combobox": 50003, "edit": 50004,
	"hyperlink": 50005, "image": 50006, "listitem": 50007, "list": 50008, "menu": 50009,
	"menubar": 50010, "menuitem": 50011, "progressbar": 50012, "radiobutton": 50013,
	"scrollbar": 50014, "slider": 50015, "spinner": 50016, "statusbar": 50017, "tab": 50018,
	"tabitem": 50019, "text": 50020, "toolbar": 50021, "tooltip": 50022, "tree": 50023,
	"treeitem": 50024, "custom": 50025, "group": 50026, "thumb": 50027, "datagrid": 50028,
	"dataitem": 50029, "document": 50030, "splitbutton": 50031, "window": 50032, "pane": 50033,
	"header": 50034, "headeritem": 50035, "table": 50036, "titlebar": 50037, "separator": 50038,
}

var roleNames = map[int32]string{}

func init() {
	pretty := map[string]string{
		"checkbox": "CheckBox", "combobox": "ComboBox", "listitem": "ListItem", "menubar": "MenuBar",
		"menuitem": "MenuItem", "progressbar": "ProgressBar", "radiobutton": "RadioButton",
		"scrollbar": "ScrollBar", "statusbar": "StatusBar", "tabitem": "TabItem", "toolbar": "ToolBar",
		"tooltip": "ToolTip", "treeitem": "TreeItem", "datagrid": "DataGrid", "dataitem": "DataItem",
		"splitbutton": "SplitButton", "headeritem": "HeaderItem", "titlebar": "TitleBar",
	}
	for k, v := range roleIDs {
		name, ok := pretty[k]
		if !ok {
			name = strings.ToUpper(k[:1]) + k[1:]
		}
		roleNames[v] = name
	}
}

func RoleID(name string) (int32, bool) {
	id, ok := roleIDs[strings.ToLower(strings.ReplaceAll(name, " ", ""))]
	return id, ok
}

func RoleName(id int32) string {
	if n, ok := roleNames[id]; ok {
		return n
	}
	return "Custom"
}
```

- [ ] **Step 3: Raw COM layer**

Run `go get github.com/go-ole/go-ole@latest`.

`internal/uia/com_windows.go`:
```go
//go:build windows

package uia

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
)

var (
	clsidCUIAutomation = ole.NewGUID("{ff48dba4-60ef-4201-aa87-54103eef594e}")
	iidIUIAutomation   = ole.NewGUID("{30cbe57d-d9d0-452a-ab13-7ac5ac4825ee}")
)

const (
	propBoundingRectangle = 30001
	propControlType       = 30003
	propName              = 30005
	propHasKeyboardFocus  = 30008
	propIsEnabled         = 30010
	propAutomationID      = 30011
	propIsControlElement  = 30016
	propIsOffscreen       = 30022
	propValueValue        = 30045

	treeScopeDescendants = 4
)

// Vtable slots from UIAutomationClient.h (IUnknown occupies 0-2). If any call returns E_NOTIMPL/garbage,
// compare against the interface definitions in github.com/sjuhan/w32uiautomation and fix the slot.
const (
	vtAutoGetRootElement      = 5
	vtAutoElementFromHandle   = 6
	vtAutoCreateCacheRequest  = 20
	vtAutoCreateTrueCondition = 21
	vtAutoCreatePropertyCond  = 23
	vtAutoCreateAndCondition  = 25

	vtElemFindAllBuildCache       = 8
	vtElemGetCurrentPropertyValue = 10
	vtElemGetCachedPropertyValue  = 12

	vtArrayGetLength  = 3
	vtArrayGetElement = 4

	vtCacheAddProperty = 3
)

type comObj struct{ *ole.IUnknown }

func (c comObj) call(slot int, args ...uintptr) uintptr {
	vt := (*[64]uintptr)(unsafe.Pointer(c.RawVTable))[slot]
	full := append([]uintptr{uintptr(unsafe.Pointer(c.IUnknown))}, args...)
	hr, _, _ := syscall.SyscallN(vt, full...)
	return hr
}

func (c comObj) release() {
	if c.IUnknown != nil {
		c.IUnknown.Release()
	}
}

func hrErr(what string, hr uintptr) error {
	if int32(hr) < 0 {
		return fmt.Errorf("%s: HRESULT 0x%08X", what, uint32(hr))
	}
	return nil
}

func outObj(c comObj, what string, slot int, args ...uintptr) (comObj, error) {
	var p *ole.IUnknown
	hr := c.call(slot, append(args, uintptr(unsafe.Pointer(&p)))...)
	if err := hrErr(what, hr); err != nil {
		return comObj{}, err
	}
	if p == nil {
		return comObj{}, fmt.Errorf("%s: null result", what)
	}
	return comObj{p}, nil
}

func createAutomation() (comObj, error) {
	unk, err := ole.CreateInstance(clsidCUIAutomation, iidIUIAutomation)
	if err != nil {
		return comObj{}, fmt.Errorf("CUIAutomation: %w", err)
	}
	return comObj{unk}, nil
}

func (a comObj) rootElement() (comObj, error) { return outObj(a, "GetRootElement", vtAutoGetRootElement) }

func (a comObj) elementFromHandle(hwnd uintptr) (comObj, error) {
	return outObj(a, "ElementFromHandle", vtAutoElementFromHandle, hwnd)
}

func (a comObj) trueCondition() (comObj, error) { return outObj(a, "CreateTrueCondition", vtAutoCreateTrueCondition) }

// propertyCondition passes the VARIANT by value; on x64 that is a pointer to a caller-owned copy.
func (a comObj) propertyCondition(prop int32, v *ole.VARIANT) (comObj, error) {
	return outObj(a, "CreatePropertyCondition", vtAutoCreatePropertyCond, uintptr(prop), uintptr(unsafe.Pointer(v)))
}

func (a comObj) andCondition(x, y comObj) (comObj, error) {
	return outObj(a, "CreateAndCondition", vtAutoCreateAndCondition, uintptr(unsafe.Pointer(x.IUnknown)), uintptr(unsafe.Pointer(y.IUnknown)))
}

func (a comObj) cacheRequest(props ...int32) (comObj, error) {
	cr, err := outObj(a, "CreateCacheRequest", vtAutoCreateCacheRequest)
	if err != nil {
		return comObj{}, err
	}
	for _, p := range props {
		if err := hrErr("CacheRequest.AddProperty", cr.call(vtCacheAddProperty, uintptr(p))); err != nil {
			cr.release()
			return comObj{}, err
		}
	}
	return cr, nil
}

func (e comObj) findAllBuildCache(scope int32, cond, cache comObj) (comObj, error) {
	return outObj(e, "FindAllBuildCache", vtElemFindAllBuildCache, uintptr(scope), uintptr(unsafe.Pointer(cond.IUnknown)), uintptr(unsafe.Pointer(cache.IUnknown)))
}

func (arr comObj) length() int32 {
	var n int32
	arr.call(vtArrayGetLength, uintptr(unsafe.Pointer(&n)))
	return n
}

func (arr comObj) element(i int32) (comObj, error) {
	return outObj(arr, "ElementArray.GetElement", vtArrayGetElement, uintptr(i))
}

func (e comObj) prop(slot int, prop int32) (ole.VARIANT, error) {
	var v ole.VARIANT
	ole.VariantInit(&v)
	hr := e.call(slot, uintptr(prop), uintptr(unsafe.Pointer(&v)))
	return v, hrErr("GetPropertyValue", hr)
}

func (e comObj) cachedString(prop int32) string {
	v, err := e.prop(vtElemGetCachedPropertyValue, prop)
	if err != nil {
		return ""
	}
	defer v.Clear()
	if v.VT == ole.VT_BSTR {
		return v.ToString()
	}
	return ""
}

func (e comObj) cachedBool(prop int32) bool {
	v, err := e.prop(vtElemGetCachedPropertyValue, prop)
	if err != nil {
		return false
	}
	defer v.Clear()
	b, _ := v.Value().(bool)
	return b
}

func (e comObj) cachedInt(prop int32) int32 {
	v, err := e.prop(vtElemGetCachedPropertyValue, prop)
	if err != nil {
		return 0
	}
	defer v.Clear()
	switch x := v.Value().(type) {
	case int32:
		return x
	case int64:
		return int32(x)
	}
	return 0
}

// rectFromVariant reads a UIA rectangle (SAFEARRAY of 4 doubles: left, top, width, height).
func rectFromVariant(v *ole.VARIANT) (l, t, w, h float64, ok bool) {
	arr := v.ToArray()
	if arr == nil {
		return
	}
	defer arr.Release()
	vals := arr.ToValueArray()
	if len(vals) != 4 {
		return
	}
	f := func(x any) float64 {
		switch n := x.(type) {
		case float64:
			return n
		case float32:
			return float64(n)
		}
		return 0
	}
	return f(vals[0]), f(vals[1]), f(vals[2]), f(vals[3]), true
}

func (e comObj) rect(slot int) (l, t, w, h float64, ok bool) {
	v, err := e.prop(slot, propBoundingRectangle)
	if err != nil {
		return
	}
	defer v.Clear()
	return rectFromVariant(&v)
}
```

- [ ] **Step 4: The UIA object on its own COM thread**

`internal/uia/uia_windows.go`:
```go
//go:build windows

package uia

import (
	"fmt"
	"regexp"
	"runtime"
	"time"

	"github.com/go-ole/go-ole"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

type UIA struct {
	req  chan func()
	auto comObj
}

// New starts the COM thread. Every UIA call runs there via run().
func New() (*UIA, error) {
	u := &UIA{req: make(chan func())}
	ready := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
			ready <- err
			return
		}
		a, err := createAutomation()
		if err != nil {
			ready <- err
			return
		}
		u.auto = a
		ready <- nil
		for f := range u.req {
			f()
		}
		a.release()
		ole.CoUninitialize()
	}()
	if err := <-ready; err != nil {
		return nil, err
	}
	return u, nil
}

func (u *UIA) run(f func() error) error {
	done := make(chan error, 1)
	select {
	case u.req <- func() { done <- f() }:
	case <-time.After(8 * time.Second):
		return fmt.Errorf("ui automation is busy (a previous query has not returned)")
	}
	select {
	case err := <-done:
		return err
	case <-time.After(8 * time.Second):
		return fmt.Errorf("ui automation timed out; narrow the query (role, window) or use vision")
	}
}

func (u *UIA) Close() { close(u.req) }

func (u *UIA) Find(q platform.FindQuery) ([]platform.Element, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 25
	}
	var re *regexp.Regexp
	if q.Name != "" {
		var err error
		if re, err = regexp.Compile("(?i)" + q.Name); err != nil {
			return nil, fmt.Errorf("query: %w", err)
		}
	}
	var out []platform.Element
	err := u.run(func() error {
		var root comObj
		var err error
		if q.Window != 0 {
			root, err = u.auto.elementFromHandle(q.Window)
		} else {
			root, err = u.auto.rootElement()
		}
		if err != nil {
			return err
		}
		defer root.release()

		vTrue := ole.NewVariant(ole.VT_BOOL, -1)
		cond, err := u.auto.propertyCondition(propIsControlElement, &vTrue)
		if err != nil {
			return err
		}
		defer cond.release()
		if q.Role != "" {
			id, ok := RoleID(q.Role)
			if !ok {
				return fmt.Errorf("unknown role %q (Button, Edit, CheckBox, ComboBox, MenuItem, ListItem, TreeItem, TabItem, Hyperlink, Text, Document, Window, Pane...)", q.Role)
			}
			vRole := ole.NewVariant(ole.VT_I4, int64(id))
			roleCond, err := u.auto.propertyCondition(propControlType, &vRole)
			if err != nil {
				return err
			}
			defer roleCond.release()
			both, err := u.auto.andCondition(cond, roleCond)
			if err != nil {
				return err
			}
			defer both.release()
			cond = both
		}
		cache, err := u.auto.cacheRequest(propName, propControlType, propBoundingRectangle, propAutomationID, propIsEnabled, propHasKeyboardFocus, propValueValue, propIsOffscreen)
		if err != nil {
			return err
		}
		defer cache.release()
		arr, err := root.findAllBuildCache(treeScopeDescendants, cond, cache)
		if err != nil {
			return err
		}
		defer arr.release()
		n := arr.length()
		for i := int32(0); i < n && len(out) < limit; i++ {
			el, err := arr.element(i)
			if err != nil {
				continue
			}
			name := el.cachedString(propName)
			autoID := el.cachedString(propAutomationID)
			if (re != nil && !re.MatchString(name)) || (q.AutomationID != "" && autoID != q.AutomationID) {
				el.release()
				continue
			}
			l, t, w, h, ok := el.rect(vtElemGetCachedPropertyValue)
			if !ok || w <= 0 || h <= 0 {
				el.release()
				continue
			}
			out = append(out, platform.Element{
				Name: name, Role: RoleName(el.cachedInt(propControlType)), AutomationID: autoID,
				Value:   el.cachedString(propValueValue),
				Rect:    geom.Rect{X: int(l), Y: int(t), W: int(w), H: int(h)},
				Enabled: el.cachedBool(propIsEnabled), Focused: el.cachedBool(propHasKeyboardFocus),
				Offscreen: el.cachedBool(propIsOffscreen), Ref: el,
			})
		}
		return nil
	})
	return out, err
}

func (u *UIA) Rect(ref any) (geom.Rect, error) {
	el, ok := ref.(comObj)
	if !ok {
		return geom.Rect{}, fmt.Errorf("bad element ref")
	}
	var r geom.Rect
	err := u.run(func() error {
		l, t, w, h, ok := el.rect(vtElemGetCurrentPropertyValue)
		if !ok {
			return fmt.Errorf("element has no bounding rectangle any more")
		}
		r = geom.Rect{X: int(l), Y: int(t), W: int(w), H: int(h)}
		return nil
	})
	return r, err
}

func (u *UIA) Release(refs []any) {
	_ = u.run(func() error {
		for _, r := range refs {
			if el, ok := r.(comObj); ok {
				el.release()
			}
		}
		return nil
	})
}

var _ platform.Accessibility = (*UIA)(nil)
```
`internal/uia/uia_other.go`: `type UIA struct{}`; `New()` returns an error `ui automation is Windows-only`.

Verification of the vtable slots before going further: add to `cmd/cu/cmd_doctor.go`:
```go
	if u, err := uia.New(); err != nil {
		fmt.Println("ui automation: FAIL", err)
	} else {
		fg := win.ForegroundWindow()
		t0 := time.Now()
		els, err := u.Find(platform.FindQuery{Window: fg, Limit: 8})
		fmt.Printf("ui automation: %d elements in foreground window in %dms (err=%v)\n", len(els), time.Since(t0).Milliseconds(), err)
		for _, e := range els {
			fmt.Printf("  %-10s %q %v\n", e.Role, e.Name, e.Rect)
		}
		u.Release(nil)
		u.Close()
	}
```
Run `go run ./cmd/cu doctor` with Notepad in the foreground: expect elements like `Document "Text Editor"`, `MenuItem "File"`, with sane rects. If names are empty or the call crashes, the slot constants are off: `go get github.com/sjuhan/w32uiautomation`, open its `IUIAutomation`/`IUIAutomationElement` vtable structs in `$(go env GOMODCACHE)`, count the slot positions, and correct the constants.

- [ ] **Step 5: tools_find.go and wait{element}**

```go
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
		return s.finish("find", t0, f, true, 0), nil, nil
	}
	return okResult(f, nil), nil, nil
}
```
In `tools_wait.go` add the field `Element *FindIn `json:"element,omitempty" jsonschema:"wait until find returns at least one element for this query"`` to `WaitIn` and, before the `Stable` case:
```go
	case in.Element != nil:
		cond = "element"
		for {
			out, _, err := s.findElements(*in.Element)
			if err == nil && len(out) > 0 {
				return s.finish("wait", t0, map[string]any{"condition": cond, "elements": out, "count": len(out)}, s.wantShot(in.Screenshot), 50*time.Millisecond), nil, nil
			}
			if time.Now().After(deadline) || !sleepCtx(ctx, 200*time.Millisecond) {
				break
			}
		}
```
Register in `server.go`:
```go
	mcp.AddTool(srv, &mcp.Tool{Name: "find", Description: "Find UI elements by name/role via Windows UI Automation (buttons, fields, menu items, list rows) in the foreground window by default. Returns ids you can pass to click/drag/move as element:\"e3\" plus rects in the last screenshot's coordinates. Faster and more precise than guessing pixels in native apps; use vision for web pages and canvases."}, s.toolFind)
```
and add `"find": wrap(s.toolFind)` to `batchable()`. In `cmd_serve.go`: `access, err := uia.New()` (log and continue with nil on error), `defer access.Close()`, `Access: access`.
Run: `go test ./...` → PASS.

- [ ] **Step 6: Real check and commit**

Build; in Claude Code: «Открой Блокнот, потом через find найди пункт меню "Файл" и кликни по нему через element id» → the menu opens without any pixel guessing. «В диалоге сохранения найди поле имени файла и кнопку Сохранить через find» → both found with correct rects.
```bash
go build ./... && go vet ./... && go test ./...
git add -A && git commit -m "feat: UI Automation find, element targeting and wait for element"
```

---

### Task 16: Operator agent, orchestrator skill, doctor and demo skills

**Files:**
- Create: `agents/operator.md`, `skills/computer-use/SKILL.md`, `skills/doctor/SKILL.md`, `skills/demo/SKILL.md`

- [ ] **Step 1: agents/operator.md**

```markdown
---
name: operator
description: Executes one bounded task on the user's Windows desktop with the desktop tools (mouse, keyboard, windows, screenshots, UI Automation). Use for any subtask that needs GUI interaction. Give it a single goal with a verifiable end state.
model: sonnet
effort: low
maxTurns: 80
tools: mcp__plugin_computer-use_desktop__screenshot, mcp__plugin_computer-use_desktop__monitors, mcp__plugin_computer-use_desktop__click, mcp__plugin_computer-use_desktop__move, mcp__plugin_computer-use_desktop__drag, mcp__plugin_computer-use_desktop__scroll, mcp__plugin_computer-use_desktop__type, mcp__plugin_computer-use_desktop__key, mcp__plugin_computer-use_desktop__clipboard, mcp__plugin_computer-use_desktop__windows, mcp__plugin_computer-use_desktop__window, mcp__plugin_computer-use_desktop__find, mcp__plugin_computer-use_desktop__wait, mcp__plugin_computer-use_desktop__batch, mcp__plugin_computer-use_desktop__control
---

You are the operator: you drive the user's Windows desktop to complete ONE bounded task, fast and precisely, like an expert user who knows every shortcut.

## Loop
1. Look first: `screenshot` (active monitor) or `find` for native apps. Never act on a stale image after something changed.
2. Act with one meaningful tool call. When you are confident of a short sequence (click a field → type → Enter), use `batch`.
3. Every action returns a fresh screenshot: read it, verify, continue. Use `wait` (stable / window / element) instead of taking repeated screenshots.
4. Stop when the end state is reached and verified.

## Coordinates
- x,y are pixels of the LAST screenshot you received. Never compute screen coordinates yourself.
- Small targets: `screenshot` with `region` to zoom, then click inside the zoomed image.
- Native apps (Explorer, Settings, Office, dialogs): prefer `find` + `click{element:"e2"}`. Web pages, canvases, games: use vision.
- Switch apps with `window{action:"focus"}` after `windows`, not by clicking the taskbar.
- Multi-monitor: `monitors` lists them; `screenshot{monitor:"2"}` looks at another one.

## Speed
- Do not narrate between actions. Do not re-screenshot without a reason. Type whole strings, never letter by letter.
- Use shortcuts: Ctrl+S, Ctrl+L (browser address bar), Win+R, Alt+F4, Ctrl+Shift+Esc, Win+arrows for snapping.
- `batch` when the sequence is obvious; `screenshot:false` for steps you do not need to see.

## Safety
- Never enter passwords, payment data or verification codes unless the task text gives them explicitly.
- Never delete files, send messages or emails, submit orders, or close unsaved work unless the task explicitly asks for that exact action.
- If a dialog asks for something outside the task, stop and report instead of guessing.

## Interruption
- If a tool returns the error `user_took_control`, stop immediately. Do not retry. Report what was done, what remains, and what the screen shows.
- If a tool returns `resumed: true`, the user handed control back: look at the returned screenshot and continue from the current state.

## Report
Reply with: outcome (done / partial / blocked), what you did in 2-5 bullets, what the final screen shows, anything the user must check. Under 120 words. Write in the language of the task.
```
Run `claude plugin validate .`. If `effort` is rejected, remove that line and add to the prompt: "Think briefly; act."

- [ ] **Step 2: skills/computer-use/SKILL.md**

```markdown
---
name: computer-use
description: Use when the user asks to do something on their computer or screen — open or operate apps, click/type in windows, handle files through the GUI, browse sites, fill forms, move windows between monitors, or "look at my screen and tell me…".
---

# Computer use (orchestrator)

You have the `desktop` MCP tools and a `computer-use:operator` agent. You plan and verify; the operator executes.

## Flow
1. Quick look: one `screenshot` (or `windows`) to see the current state.
2. Split the request into subtasks, each with a verifiable end state ("Notepad shows the text and the file exists at C:\...").
3. For each subtask dispatch `Agent(subagent_type: "computer-use:operator", prompt: ...)`.
   - Default model (Sonnet) for ordinary GUI work.
   - `model: "opus"` when the subtask needs judgment: reading long documents on screen, ambiguous UI, comparing options, anything irreversible.
   - `model: "haiku"` for trivial repeats ("click Next until Finish").
4. Verify the end state yourself (screenshot or `find`) before telling the user it is done.
5. Call `control{action:"release"}` when the whole job is finished so the overlay disappears.

## The operator prompt
Include: the goal, the exact end state, the app or window, data to enter (verbatim), what NOT to do, and "report when done". One subtask per dispatch. For a single click or a look, act yourself instead of dispatching.

## When the user takes control
If a tool or the operator reports `user_took_control`: stop, tell the user what happened and what is left, and wait. Do not resume on your own. The user's next message resumes control automatically; then re-dispatch from the current screen state.

## Cost
A screenshot is ~1.5k tokens. Prefer `find`, `batch`, `wait{stable:true}`, and `screenshot:false` on steps you do not need to see. Zoom with `screenshot{region}` only for small targets.
```

- [ ] **Step 3: skills/doctor/SKILL.md and skills/demo/SKILL.md**

`skills/doctor/SKILL.md`:
```markdown
---
name: doctor
description: Check the computer-use plugin environment (monitors, DPI, capture speed, UI Automation, hooks, permissions) and explain any problem. Use when desktop tools fail, when asked to check the setup, or after installing the plugin.
---

# /computer-use:doctor

1. The plugin root is two directories above this skill's base directory (`<root>/skills/doctor`). Run with Bash:
   `"<root>/bin/cu.exe" doctor` (build it first with `powershell -NoProfile -File "<root>/scripts/build.ps1"` if it is missing).
2. Read the report and explain, in the user's language: how many monitors and their scale factors, whether DPI awareness is per-monitor v2, capture time (should be under 100 ms), whether UI Automation returned elements, and whether capture exclusion for the overlay is native.
3. If the desktop MCP server is not connected (`/mcp` shows it disconnected), tell the user to run `/reload-plugins` or restart Claude Code, and to add the permission rule `mcp__plugin_computer-use_desktop` to `permissions.allow` in settings if they are not in bypass mode.
4. Known limits to mention only when relevant: elevated (admin) windows and UAC prompts cannot receive input unless cu runs elevated; exclusive-fullscreen games hide the overlay.
```

`skills/demo/SKILL.md`:
```markdown
---
name: demo
description: Show the Claude take-over overlay for a few seconds and prove it is excluded from screenshots. Use when the user wants to see how the overlay looks or to test it.
---

# /computer-use:demo

1. The plugin root is two directories above this skill's base directory. Run with Bash:
   `"<root>/bin/cu.exe" demo -seconds 5 -o "<scratchpad>/demo.png"`.
2. Read `demo.png` with the Read tool and confirm the glowing border and HUD are NOT in the capture.
3. Tell the user what they should have seen: the accent-colored breathing border on monitor 1, the HUD pill at the top with the hotkey hint and a changing action line, ripples, then a gray "you are in control" state that fades out. Mention the config file path (`%APPDATA%\claude-computer-use\config.json`) for `accent`, `lang`, `hotkey`.
```

- [ ] **Step 4: Validate in Claude Code and commit**

`claude plugin validate .` → OK. `claude --plugin-dir .` → `/computer-use:doctor` and `/computer-use:demo` work; ask «Открой Проводник в папке Загрузки и создай там папку test-cu» → the main session invokes the computer-use skill, dispatches the operator (visible as a subagent), the overlay appears, the folder is created, the border disappears when the answer completes.
```bash
git add -A && git commit -m "feat: operator agent, orchestrator skill, doctor and demo skills"
```

---

### Task 17: README (EN + RU), release pipeline, installer download, doctor polish

**Files:**
- Create: `README.md` (replace stub), `README.ru.md`, `.goreleaser.yaml`, `.github/workflows/release.yml`, `docs/media/.gitkeep`
- Modify: `scripts/install.ps1`, `CHANGELOG.md`, `cmd/cu/cmd_doctor.go` (capture timing + exclusion mode lines)

- [ ] **Step 1: Release pipeline**

`.goreleaser.yaml`:
```yaml
version: 2
project_name: cu
builds:
  - main: ./cmd/cu
    binary: cu
    goos: [windows]
    goarch: [amd64, arm64]
    env: [CGO_ENABLED=0]
    flags: [-trimpath]
    ldflags: ["-s -w -X main.version={{.Version}}"]
archives:
  - formats: [zip]
    name_template: "cu_{{ .Os }}_{{ .Arch }}"
checksum:
  name_template: checksums.txt
changelog:
  use: git
```
`.github/workflows/release.yml`:
```yaml
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  release:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - uses: actions/setup-go@v5
        with: { go-version: "1.26" }
      - run: go test ./...
      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 2: install.ps1 with release download and checksum**

Replace `scripts/install.ps1`:
```powershell
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $root "bin"
$exe = Join-Path $bin "cu.exe"
$manifest = Get-Content (Join-Path $root ".claude-plugin\plugin.json") -Raw | ConvertFrom-Json
$version = $manifest.version
$repo = "racass-pixel/claude-computer-use"
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
$asset = "cu_windows_$arch.zip"
$base = "https://github.com/$repo/releases/download/v$version"
New-Item -ItemType Directory -Force $bin | Out-Null
$tmp = Join-Path $env:TEMP "cu-install-$version-$PID"
New-Item -ItemType Directory -Force $tmp | Out-Null
try {
  Write-Host "cu: downloading $base/$asset"
  Invoke-WebRequest -Uri "$base/$asset" -OutFile (Join-Path $tmp $asset) -UseBasicParsing
  Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile (Join-Path $tmp "checksums.txt") -UseBasicParsing
  $line = Select-String -Path (Join-Path $tmp "checksums.txt") -Pattern ([regex]::Escape($asset)) | Select-Object -First 1
  if (-not $line) { throw "no checksum for $asset" }
  $expected = ($line.Line -split "\s+")[0].ToLower()
  $actual = (Get-FileHash (Join-Path $tmp $asset) -Algorithm SHA256).Hash.ToLower()
  if ($expected -ne $actual) { throw "checksum mismatch for $asset" }
  Expand-Archive -Path (Join-Path $tmp $asset) -DestinationPath $tmp -Force
  Copy-Item (Join-Path $tmp "cu.exe") $exe -Force
  Write-Host "cu: installed $exe ($version)"
} catch {
  Write-Host "cu: release download failed ($($_.Exception.Message)); trying go build"
  if (Get-Command go -ErrorAction SilentlyContinue) {
    & (Join-Path $PSScriptRoot "build.ps1")
  } else {
    throw "Neither a release download nor Go is available. Install Go from https://go.dev/dl/ and rerun."
  }
} finally {
  Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
```
Test the fallback path now (no release exists yet): delete `bin/cu.exe`, run `bin\cu.cmd version` → the script prints the download failure, builds with Go, and prints `0.1.0`. All of that output must go to stderr in the launcher path (it does: `1>&2` in `cu.cmd`).

- [ ] **Step 3: README.md (English) — write these sections in full**

1. Title + one-paragraph pitch + a placeholder image tag `![demo](docs/media/demo.gif)` (record the GIF in Task 18).
2. **What you get**: bullets — full mouse/keyboard/window control; screenshots in every action; UI Automation `find`; multi-monitor; glowing take-over overlay + HUD; instant interrupt (Esc Esc or any physical input) with hand-back; operator agent on Sonnet with per-task model override; pure Go, no cgo, single binary.
3. **Install** (exact commands):
   ```
   /plugin marketplace add racass-pixel/claude-computer-use
   /plugin install computer-use@claude-computer-use
   ```
   and for development `claude --plugin-dir <path>`. Note that the binary is downloaded from GitHub Releases on first start (checksum-verified) or built with Go if present.
4. **Permissions**: the settings block
   ```json
   { "permissions": { "allow": ["mcp__plugin_computer-use_desktop"] } }
   ```
   and the note that bypass mode needs nothing.
5. **Usage**: three example prompts (Notepad file, Explorer drag-and-drop, browser form) and what happens (skill → operator → overlay → report). How to take control (Esc Esc, or just move the mouse), how to hand back (Esc Esc) or continue (next message).
6. **Tools**: the table from spec §5 (tool, purpose, key args).
7. **Configuration**: path `%APPDATA%\claude-computer-use\config.json`, table of every key with default (from `config.Default()`), env override names.
8. **How it works**: the architecture diagram from the spec §3 and the coordinate model paragraph (spec §4).
9. **Limits**: from spec §10.
10. **Development**: `go build`, `go test ./...`, `cu doctor`, `cu demo`, `cu screenshot`, `cu input`, release process (`git tag v0.1.0 && git push --tags`).
11. **License**: MIT.

`README.ru.md`: the same sections in Russian; link each README to the other at the top.

- [ ] **Step 4: CHANGELOG and doctor polish**

`CHANGELOG.md`: under `## 0.1.0 — 2026-09-xx` list the features (one line each). In `cmd_doctor.go` add: a capture benchmark (`screen.Grab` of monitor 1 three times, print min ms), and `overlay capture exclusion: native` / `hidden-during-capture` according to `win.CaptureExclusionSupported` and whether `screen.BeforeCapture` is used.

- [ ] **Step 5: Commit**

```bash
go build ./... && go vet ./... && go test ./...
git add -A && git commit -m "docs: README (en/ru), release pipeline and checksum-verified installer"
```

---

### Task 18: End-to-end evaluation, tuning, first release

**Files:**
- Create: `docs/eval/2026-09-benchmark.md`, `docs/media/demo.gif`
- Modify: `agents/operator.md`, `skills/computer-use/SKILL.md` (only what the eval shows is needed), `.claude-plugin/plugin.json` (version), `CHANGELOG.md`

- [ ] **Step 1: Run the benchmark tasks in a fresh `claude --plugin-dir .` session**

Record for each: operator turns, wall time, success, notes. Targets in parentheses.
1. «Открой Блокнот, напиши "привет", сохрани на рабочий стол как cu-test.txt» (≤ 6 turns).
2. «В Проводнике открой Загрузки и создай папку cu-eval» (≤ 5).
3. «Перетащи файл cu-test.txt с рабочего стола в папку cu-eval» (drag-and-drop, ≤ 6).
4. «Открой Chrome, зайди на example.com и скажи заголовок страницы» (≤ 5).
5. «В Chrome открой https://httpbin.org/forms/post и заполни форму: имя Sasha, телефон 123, размер medium, отправь» (≤ 8).
6. «Переключись на окно Калькулятора (открой, если нет) и посчитай 12*34» (≤ 5).
7. Two monitors: «Перенеси окно Калькулятора на второй монитор и разверни» (≤ 4).
8. Interruption: start task 5 again, move the mouse mid-way → operator stops within one action and reports; type «продолжай» → completes.
9. «Открой Параметры → Экран и скажи текущий масштаб» (find on WinUI, ≤ 5).
10. Zoom: «В Блокноте кликни по маленькой кнопке закрытия вкладки/окна» (region zoom used, ≤ 4).

- [ ] **Step 2: Tune**

For every task over target: read the operator transcript; fix the cause in the operator prompt (missing rule), a tool description (model misused an argument), or the code (wrong coordinate mapping, slow settle). Re-run the failing task until it meets the target. Write the table and the changes made into `docs/eval/2026-09-benchmark.md`.

- [ ] **Step 3: Demo GIF**

Record a 15 s clip of task 1 with the overlay (Windows Game Bar `Win+G` or ShareX), convert to GIF ≤ 8 MB at 1280 px wide (ffmpeg if available: `ffmpeg -i clip.mp4 -vf "fps=12,scale=1280:-1" -loop 0 docs/media/demo.gif`), commit it.

- [ ] **Step 4: Release v0.1.0**

Ensure `plugin.json` version is `0.1.0`, CHANGELOG dated, README links valid. Then:
```bash
go build ./... && go vet ./... && go test ./... && claude plugin validate .
git add -A && git commit -m "docs: benchmark results and demo"
git remote add origin https://github.com/racass-pixel/claude-computer-use.git   # create the repo on GitHub first (public)
git push -u origin main
git tag v0.1.0 && git push --tags
```
Wait for the `release` workflow to publish `cu_windows_amd64.zip`, `cu_windows_arm64.zip` and `checksums.txt`. Then, on a machine (or after deleting `bin/cu.exe` and hiding Go from PATH), run `/plugin marketplace add racass-pixel/claude-computer-use` + `/plugin install computer-use@claude-computer-use` and confirm the binary downloads and the `desktop` server connects.

---

## Self-review notes

- Spec coverage: §3 architecture → Tasks 1-3, 8, 11; §4 coordinates → Task 4 (View) and 8-9 (resolvePoint); §5 tools → Tasks 8, 9, 15 (all 16 tools registered); §6 overlay → Tasks 12-13; §7 guard → Tasks 10, 11, 14 (hooks, IPC, pause-wait in `begin`); §8 plugin files → Tasks 1, 14, 16, 17; §9 models → Task 16 (operator frontmatter + skill overrides); §10 limits → Task 17 README + doctor.
- Type consistency checked: `platform.*` names (Task 2) are used verbatim in Tasks 3-15; `screen.View` methods (Task 4) in Task 8/9/15; `server.Controller` (Task 8) is satisfied by the adapter in Task 11 over `guard.Machine` (Task 10); `win.OverlayClass` = `window.ExcludeClassPrefix` = "CuOverlay".
- Known open points executors must resolve at the marked steps: `effort` frontmatter acceptance (Task 16), capture exclusion spike (Task 12 step 6), UIA vtable slots (Task 15 step 4), go-sdk in-memory transport helper name (Task 8 step 5).
