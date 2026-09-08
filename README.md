[Русская версия](README.ru.md)

# Claude Computer Use

A Claude Code plugin that gives Claude full control of a Windows desktop — mouse, keyboard, windows, drag-and-drop, multiple monitors — at close to human speed. While Claude is in control, the active monitor gets a glowing border and a HUD. Press **Esc Esc** to take control back instantly; press it again to hand back.

![demo](docs/media/demo.gif)

## What you get

- **Full mouse and keyboard control** — click, drag, scroll, type (Unicode), keyboard shortcuts
- **Screenshot in every action** — the model sees the result of each step
- **UI Automation `find`** — target buttons, fields, menu items by name or role instead of guessing pixels
- **Multi-monitor aware** — switch between monitors, each with per-monitor DPI v2
- **Claude-style HUD and glow** — warm dark pill with the animated Claude spark, human-readable action captions (localized ru/en), key-cap hints, accent-colored breathing border with travelling shimmer, click ripple; the orchestrator writes a task caption the user sees the whole time
- **Instant interrupt** — Esc Esc takes control back; optionally pause on any physical input (`auto_pause: true`)
- **Smooth mouse glide** — natural cursor movement between points
- **Recipes** — procedural memory: the server suggests matching recipes when a task starts, auto-records drafts from the trace, and tracks success rates; curated recipes rank higher than auto-recorded ones
- **Operator agent** on Sonnet with per-task model override (Opus for judgment, Haiku for trivial repeats)
- **Pure Go, no cgo, single binary** — MIT license

## Install

From the Claude Code marketplace:

```
/plugin marketplace add racass-pixel/claude-computer-use
/plugin install computer-use@claude-computer-use
```

The binary `cu.exe` is downloaded from GitHub Releases on first start (checksum-verified). If the release is not available, it falls back to building with Go if present.

For development, clone the repository and start Claude Code with:

```
claude --plugin-dir C:\path\to\claude-computer-use
```

## Permissions

In bypass mode, no configuration is needed. Otherwise, add this to your Claude Code settings:

```json
{
  "permissions": {
    "allow": ["mcp__plugin_computer-use_desktop"]
  }
}
```

## Usage

Ask Claude to do things on your desktop:

> Open Notepad, write "Hello, world!" and save it to the Desktop as hello.txt

> In Explorer, drag report.pdf from Downloads to the Projects folder

> Open Chrome, go to github.com/settings/profile, and change my bio to "Building things"

Claude decomposes the task into subtasks, dispatches the operator agent, shows the overlay, and reports the result.

### Taking control back

Press **Esc Esc** at any time. Claude stops immediately, the overlay turns gray, and you are in control. The first Esc reaches the focused application (it may close a menu or dialog) — if that is a problem, set `hotkey` to `ctrl+alt+esc` in the config, which is fully swallowed.

### Handing back

Press **Esc Esc** again, or just send your next message — control resumes automatically.

### Auto-pause on physical input

By default, only Esc Esc takes control back. Set `auto_pause: true` in the config to also pause when you move the mouse (beyond a threshold) or press any key.

## Tools

The plugin exposes 18 MCP tools under the server name `desktop`:

| Tool | Purpose | Key arguments |
|---|---|---|
| `screenshot` | Capture the screen (active monitor, specific monitor, or a region) | `monitor`, `region`, `scale`, `format` |
| `monitors` | List monitors with rects, DPI scale, cursor and foreground info | — |
| `click` | Click at x,y or on a `find` element | `x`, `y`, `element`, `button`, `count`, `modifiers` |
| `move` | Move the mouse (hover) | `x`, `y`, `element` |
| `drag` | Drag from one point to another (drag-and-drop, selections, sliders) | `from`, `to`, `button`, `duration_ms` |
| `scroll` | Scroll the mouse wheel | `x`, `y`, `dy`, `dx` |
| `type` | Type text into the focused control (Unicode, any language) | `text`, `mode` |
| `key` | Press a chord or chord sequence | `keys` (`"ctrl+s"` or `["win+r","enter"]`) |
| `clipboard` | Read or write the text clipboard | `action` (`get`/`set`), `text` |
| `windows` | List open top-level windows | `filter` (regexp) |
| `window` | Focus, minimize, maximize, restore, close, move, resize a window | `action`, `target` |
| `find` | Find UI elements by name/role via Windows UI Automation | `query`, `role`, `window`, `limit` |
| `wait` | Wait for a condition: sleep, window appears, screen stabilizes | `ms`, `window`, `stable`, `timeout_ms` |
| `pixel` | Read the color of one or more pixels (for calibrating visual cues) | `x`, `y`, `points` |
| `click_until` | Click repeatedly until a probe pixel matches (or stops matching) a color | `x`, `y`, `probe`, `color`, `max`, `interval_ms` |
| `batch` | Run several actions in sequence, one screenshot at the end | `actions` |
| `control` | Session control: status, acquire overlay, release, set HUD title | `action` |
| `recipe` | Procedural memory: search, run, save, trace, get, list, delete | `action`, `slug`, `values` |

All coordinates are pixels of the **last screenshot**. The server converts to screen pixels; the model never does coordinate arithmetic.

### Grinding repetitive lists

When Claude needs to process many identical rows (accept/deny, check/uncheck), `click_until` clicks a point repeatedly server-side until a probe pixel turns a target color — for example, clicking Deny on the top row until the 5th star of the next applicant is yellow. Combined with `pixel` (to learn the cue's color) and `screenshot_region` (to keep the zoom on the working area), this replaces one-click-per-model-turn with dozens of clicks per tool call.

## Configuration

Config file: `%APPDATA%\claude-computer-use\config.json`

Every key can also be set via an environment variable (`CU_` prefix, uppercase, underscores):

| Key | Default | Env | Description |
|---|---|---|---|
| `hotkey` | `esc esc` | `CU_HOTKEY` | Key sequence to take/hand control (also supports chords like `ctrl+alt+esc`) |
| `auto_pause` | `false` | `CU_AUTO_PAUSE` | Pause on any physical mouse/keyboard input |
| `mouse_threshold_px` | `12` | `CU_MOUSE_THRESHOLD_PX` | Minimum mouse movement (px) to trigger auto-pause |
| `mouse_glide_ms` | `220` | `CU_MOUSE_GLIDE_MS` | Duration of smooth cursor glide between points (0 = instant) |
| `screenshot_long_edge` | `1366` | `CU_SCREENSHOT_LONG_EDGE` | Auto-scale screenshots so the long edge is at most this many pixels |
| `screenshot_format` | `png` | `CU_SCREENSHOT_FORMAT` | Screenshot format: `png` or `jpeg` |
| `jpeg_quality` | `85` | `CU_JPEG_QUALITY` | JPEG quality (1–100) |
| `lang` | `auto` | `CU_LANG` | HUD and overlay language: `auto`, `en`, or `ru` |
| `accent` | `#D97757` | `CU_ACCENT` | Overlay accent color (hex `#RRGGBB`) |
| `overlay` | `true` | `CU_OVERLAY` | Show the take-over overlay |
| `border_thickness` | `56` | `CU_BORDER_THICKNESS` | Overlay border strip thickness in pixels (before DPI) |
| `border_intensity` | `0.85` | `CU_BORDER_INTENSITY` | Overlay border opacity (0.0–1.0) |
| `border_shimmer` | `true` | `CU_BORDER_SHIMMER` | Travelling bright spot on the border glow while controlling |
| `idle_release_ms` | `120000` | `CU_IDLE_RELEASE_MS` | Hide overlay after this many ms of inactivity |
| `pause_wait_ms` | `20000` | `CU_PAUSE_WAIT_MS` | How long an action waits for the user to hand back before returning an error |
| `paste_threshold` | `200` | `CU_PASTE_THRESHOLD` | Character count above which `type` uses clipboard paste |
| `log_file` | *(empty)* | `CU_LOG_FILE` | Path to a log file (empty = stderr only) |

## How it works

```
Claude Code (main session = orchestrator)
 │  skill computer-use: decompose, pick model, verify
 │  Agent(operator, model: sonnet|opus|haiku) ── action loop
 │          │ MCP tools: mcp__plugin_computer-use_desktop__*
 ▼          ▼
cu.exe serve  (stdio MCP server, one per session)
 ├─ server    — tools, batch, wait, coordinate transform
 ├─ input     — SendInput: mouse, keyboard, clipboard
 ├─ screen    — monitors, DPI, BitBlt capture, scaling, PNG/JPEG
 ├─ window    — enumerate / focus / move / close windows
 ├─ uia       — UI Automation: find elements by name/role
 ├─ overlay   — glowing border, HUD, click ripple (own UI thread)
 ├─ guard     — low-level input hooks, Esc Esc, pause state machine
 ├─ recipes   — save/search/replay parameterised action sequences
 └─ config    — JSON file + CU_* env overrides
```

The server keeps a **view** (monitor, offset, scale) set by each `screenshot`. All x,y in tool inputs are pixels of the last screenshot image. The server converts them to physical screen coordinates using the view transform, so the model never does coordinate math. Auto-scale targets the long edge at 1366 px (configurable), giving about 1400 image tokens per screenshot.

## Recipes

Recipes are procedural memory — saved sequences of desktop actions that Claude can replay without taking screenshots between steps.

**How it works:**
1. Before multi-step work, the orchestrator searches for an existing recipe: `recipe{action:"search", query:"..."}`.
2. If a match is found, the operator replays it with `recipe{action:"run", slug:"...", values:{...}}` and verifies the end state.
3. After a successful novel task, Claude distils the action trace into a new recipe with `recipe{action:"save", ...}`, using `{{param}}` placeholders for variable parts.

Recipes are stored as JSON files in `%APPDATA%\claude-computer-use\recipes\`. Use `recipe{action:"list"}` to see them, or `recipe{action:"delete", slug:"..."}` to remove one.

## Limits

- **Elevated windows**: Claude cannot send input to admin/elevated windows or UAC prompts unless `cu.exe` itself runs elevated.
- **Exclusive fullscreen**: the overlay is not visible in exclusive-fullscreen games; low-level hooks still work.
- **SetForegroundWindow**: Windows restricts focus stealing; standard workarounds are used but rarely a window only flashes in the taskbar.
- **First Esc**: the first Esc of Esc Esc reaches the application (may close a menu); set `hotkey` to `ctrl+alt+esc` to avoid this.
- **Without bypass mode**: every tool call prompts for permission unless you add the allow rule above.
- **Windows only**: macOS and Linux are not supported yet; all platform code is behind interfaces for future ports.

## Development

Build:

```
go build ./cmd/cu
```

Test:

```
go test ./...
```

Vet (contributors — the `unsafeptr` check is disabled because of Win32 syscall patterns):

```
go vet -unsafeptr=false ./...
```

Format check:

```
gofmt -l .
```

Cross-compile check (ensures no accidental Windows-only code in shared packages):

```
GOOS=linux go build ./...
```

CLI helpers:

```
cu doctor                        # monitors, DPI, capture speed, UIA, config, recipes
cu demo                          # show the overlay for 5 seconds
cu screenshot -o screen.png      # save a screenshot
cu input click 500 300           # send a click
cu input type "hello"            # type text
```

Release:

```
git tag v0.1.0
git push --tags
```

The GitHub Actions workflow builds and publishes a release with GoReleaser.

### Recording demos

Set `CU_OVERLAY_VISIBLE_IN_CAPTURE=1` to make the overlay appear in screen
recordings (by default it is excluded via `SetWindowDisplayAffinity`). For
recording demos only — do not leave enabled in normal use.

### Developer caveat

Starting Claude Code with its working directory inside this repository also loads `.mcp.json` as a project-level server named `desktop` where `${CLAUDE_PLUGIN_ROOT}` is not expanded. That duplicate server fails with CONNECTION_CLOSED — ignore it or run Claude Code from another directory. The plugin's own server is `plugin-computer-use-desktop`.

## License

[MIT](LICENSE)
