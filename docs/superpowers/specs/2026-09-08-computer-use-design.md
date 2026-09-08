# Claude Computer Use — design spec

Date: 2026-09-08. Status: approved.

## 1. Goal

A Claude Code plugin that gives Claude full control of a Windows desktop (mouse, keyboard, windows, drag-and-drop, multiple monitors) at close to human speed, with a polished and safe UX: while Claude is in control, the active monitor gets a glowing border and a HUD at the top tells the user how to take control back; any physical input from the user, or a double Esc, stops Claude immediately. The bar: nicer, more convenient and more capable than OpenAI Operator, and a repository worth publishing.

## 2. Decisions

| Question | Decision |
|---|---|
| Taking control back | Double Esc (configurable) plus auto-pause on any physical input (mouse with a movement threshold, any key) |
| Targeting UI elements | Vision (screenshots) plus Windows UI Automation (`find` returns elements with coordinates) |
| Operator model | Sonnet 5 by default; the orchestrator (main session) can override to opus/haiku per subtask |
| Platform | Windows only; all platform code behind `internal/platform` interfaces so macOS/Linux can be added later |
| Process model | One binary `cu.exe`, started by Claude Code as a stdio MCP server per session; tools, overlay, input hooks and UIA live inside it |
| Language | Go, no cgo. MIT license |

Module path: `github.com/racass-pixel/claude-computer-use`.

## 3. Architecture

```
Claude Code (main session = orchestrator, user's model)
 │  skill computer-use: decompose task, pick model, verify result
 │  Agent(operator, model: sonnet|opus|haiku) ── action loop
 │          │ MCP tools: mcp__plugin_computer-use_desktop__*
 ▼          ▼
cu.exe serve  (stdio MCP server, lives for the whole session)
 ├─ internal/server   — tools, batch, wait, view transform for coordinates
 ├─ internal/input    — SendInput: mouse, keyboard (unicode + VK), chord parser, clipboard
 ├─ internal/screen   — monitors, DPI, BitBlt capture, scaling, PNG/JPEG
 ├─ internal/window   — enumerate/focus/move/close windows
 ├─ internal/uia      — UI Automation: find elements by name/role, rects
 ├─ internal/overlay  — UI thread: glowing border, HUD, click ripple
 ├─ internal/guard    — low-level input hooks, double Esc, pause/control state machine, idle release
 ├─ internal/ipc      — named pipe for `cu ctl resume|release|status`
 └─ internal/config   — %APPDATA%\claude-computer-use\config.json + env CU_*
hooks: UserPromptSubmit → cu ctl resume; Stop/SubagentStop → cu ctl release
```

Threads: the main goroutine serves MCP over stdio. One OS thread (`runtime.LockOSThread`) owns the overlay windows, the low-level hooks and the message loop. Input actions call SendInput directly from tool handlers (thread-safe). The server exits on stdin EOF and when the parent process dies (it watches the parent handle); on exit it always hides the overlay and removes the hooks.

DPI: `SetProcessDpiAwarenessContext(PER_MONITOR_AWARE_V2)` at startup; all internal coordinates are physical pixels of the virtual screen.

## 4. Coordinate model

The server keeps a current `view = {monitor, offset (screen x,y), scale}`. Every `screenshot` sets the view (full monitor or a region). All `x,y` in tool inputs and every `rect` returned by `find` are in pixels of the **last screenshot**. The server converts to screen coordinates; the model never does arithmetic. Before the first screenshot the view is the active monitor at auto scale (deterministic from the monitor size).

Auto scale: long edge ≤ 1366 px (config `screenshot_long_edge`); 1920×1080 → 1366×768 (~1400 image tokens). For small targets use `screenshot{region}` (zoom); after it, coordinates are in region space. Every tool response carries view metadata.

Active monitor = the one containing the center of the foreground window (fallback: cursor). The overlay border glows on the monitor of the last action.

## 5. MCP tools (server `desktop`)

Names in Claude Code: `mcp__plugin_computer-use_desktop__<tool>`; allow rule: `mcp__plugin_computer-use_desktop`.

| Tool | Input | Output |
|---|---|---|
| `screenshot` | `monitor?: "active"\|"all"\|int, region?: {x,y,w,h}, scale?: number, format?: "png"\|"jpeg"` | image + JSON `{monitor, image:{w,h}, screen:{x,y,w,h}, scale, cursor:{x,y}, foreground:{id,title,process}, paused}` |
| `click` | `x?,y?, element?: id, button?: left\|right\|middle, count?: 1\|2\|3, modifiers?: [], screenshot?: true` | status JSON + screenshot |
| `move` | `x,y \| element` | status |
| `drag` | `from: {x,y}\|{element}, to: {x,y}\|{element}, button?, duration_ms?: 250, screenshot?` | status + screenshot (interpolated steps, held button, pause before release; works with drag-and-drop in Explorer and browsers) |
| `scroll` | `x?,y?, dy?: ticks, dx?: ticks, screenshot?` | status + screenshot (dy>0 = down) |
| `type` | `text, mode?: auto\|unicode\|paste\|keys, delay_ms?: 0, screenshot?` | status + screenshot. `auto`: unicode via SendInput; >200 chars via clipboard with restore |
| `key` | `keys: "ctrl+shift+t" \| ["win+r","enter"], hold_ms?, screenshot?` | status + screenshot. Grammar: modifiers ctrl/alt/shift/win + key names (enter, esc, tab, f1..f24, arrows, letters/digits) |
| `monitors` | — | `[{id, name, rect, scale_factor, primary, has_cursor, has_foreground}]` |
| `windows` | `filter?: regex, monitor?` | `[{id, title, process, pid, rect, monitor, state, is_foreground}]` |
| `window` | `action: focus\|minimize\|maximize\|restore\|close\|move\|resize, target: id\|regex\|"foreground", rect?, screenshot?` | status + screenshot. Focus works around SetForegroundWindow restrictions (AttachThreadInput / Alt trick) and restores minimized windows |
| `find` | `query?: substring\|regex on Name, role?: Button\|Edit\|MenuItem\|…, window?: id\|"foreground"\|"all", automation_id?, limit?: 25` | `[{id, name, role, rect (view space), screen_rect, enabled, focused, value}]`; ids are valid until the next `find` |
| `wait` | `ms? \| window?: regex \| element?: {query, role} \| stable?: true, timeout_ms?: 10000, screenshot?: true` | waits for the condition (window appeared, element found, screen stopped changing) |
| `batch` | `actions: [{tool, args}], stop_on_error?: true, screenshot?: true` | per-step results + one final screenshot |
| `control` | `action: status\|acquire\|release\|hud, task?: string, note?: string` | `{controlling, paused, hotkey, active_monitor, idle_ms}`; `hud` sets the task title in the HUD |
| `clipboard` | `action: get\|set, text?` | text |

Common rules: any action automatically acquires control (shows the overlay) if it is not shown. Every action is displayed in the HUD (`click 640,412`, `type "hello"`). While paused, an action returns `is_error` with `{"error":"user_took_control"}` (see §7). Results are compact: one-line status JSON plus the screenshot as a separate `ImageContent`.

## 6. Overlay

- Border: four strip windows along the edges of the active monitor (32 px × DPI thick), alpha gradient from the edge inward, slow "breathing" at ~30 fps. Strips are small, so pixels are computed in Go cheaply. Accent color from config (`accent`, default Claude orange `#D97757`).
- HUD: a pill at the top center of the active monitor: dark translucent, rounded, two lines: "Claude is controlling the computer" and "Esc Esc — take control · click 640,412". Pulsing dot indicator. Task title via `control{hud}`.
- Ripple: ring 40→90 px over 350 ms at the click point.
- Paused: border turns gray, HUD says "You are in control · Esc Esc — hand back to Claude", then fades out.
- HUD localization: `lang: auto|ru|en` (system language by default).
- Implementation: `WS_POPUP` windows with `WS_EX_LAYERED|WS_EX_TRANSPARENT|WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE`; rendered into an RGBA buffer (`image/draw`, `x/image/vector` for rounded shapes, `x/image/font/opentype` with an embedded Inter OFL font); displayed via `UpdateLayeredWindow` (premultiplied BGRA DIB). Click-through.
- Excluded from screenshots via `SetWindowDisplayAffinity(WDA_EXCLUDEFROMCAPTURE)`; verified by a spike against BitBlt; fallback: hide the overlay windows for the duration of a capture.

## 7. Guard: taking control and pausing

State machine (pure logic, unit-tested with a fake clock): `idle → controlling → paused → controlling|idle`.

- Low-level hooks `WH_KEYBOARD_LL` / `WH_MOUSE_LL` on the UI thread. Claude's input carries `LLKHF_INJECTED`/`LLMHF_INJECTED` and is ignored. The hook procedure only pushes an event to a channel (slow hooks get removed by Windows).
- While `controlling`, physical input: any key → pause; mouse movement summing to more than `mouse_threshold_px` (12 px within 300 ms) or any button → pause. Disabled with `auto_pause: false`.
- Double Esc (`hotkey: "esc esc"`, gap ≤ 400 ms; the grammar also supports chords like `ctrl+alt+esc`): in `controlling` → pause; in `paused` → hand control back to Claude. Works even with auto-pause off.
- Pause: an atomic flag in the server; all actions are rejected. An action called while paused waits up to `pause_wait_ms` (20 s): if the user hands control back (Esc Esc), the action is NOT performed; a fresh screenshot and `{"resumed":true}` are returned so the operator re-observes the screen. Otherwise the error `user_took_control` is returned and the operator must stop and report.
- Only the user can lift a pause: the Esc Esc gesture, a new prompt (hook `UserPromptSubmit` → `cu ctl resume`), or `cu ctl resume` by hand. The model has no resume tool.
- Idle release: no actions for `idle_release_ms` (120 s) → overlay hides, state `idle`. Hooks `Stop`/`SubagentStop(operator)` → `cu ctl release`. Parent death / EOF → everything is cleaned up.
- IPC: named pipe `\\.\pipe\claude-computer-use-<pid>`; `cu ctl` sends the command to every pipe with this prefix (safe with several sessions: it is always a user action).

## 8. Plugin: agents, skills, hooks

`.claude-plugin/plugin.json`: name `computer-use`, version, description, author, repository, license MIT, keywords.

`.mcp.json`:
```json
{"mcpServers":{"desktop":{"type":"stdio","command":"cmd","args":["/c","${CLAUDE_PLUGIN_ROOT}/bin/cu.cmd","serve"]}}}
```
`bin/cu.cmd` is a launcher: if `bin/cu.exe` is missing it runs `scripts/install.ps1` (download the GitHub release matching the version in plugin.json, verify sha256; fallback `go build`), with all install output on stderr, then runs `cu.exe serve` with pass-through stdio. This sidesteps the "SessionStart hook vs MCP startup order" question.

`agents/operator.md` (frontmatter: `name: operator`, `model: sonnet`, `effort: low` if accepted by `claude plugin validate`, else `medium`; `tools:` only the `desktop` tools; `maxTurns: 80`). Body: loop discipline — `screenshot`/`find` first, one meaningful action or a `batch` when confident, coordinates only from the last screenshot, `find` first for native apps, vision for web/canvas, `wait` instead of repeated screenshots, verify after risky actions, no passwords and no irreversible actions (delete, send messages, payments) without explicit permission in the task; on `user_took_control` stop immediately and report what was done, what remains, and the screen state.

`skills/computer-use/SKILL.md` (for the orchestrator; triggers when the user asks to do something on the computer / in an app / in a window). Covers: splitting the task into verifiable subtasks; dispatching `Agent(subagent_type: "computer-use:operator")` with `model: "opus"` for judgment-heavy subtasks (complex documents, ambiguous UI) and `haiku` for trivial repeats; verifying the final result with a screenshot; the interruption protocol (never restart without the user's word); cost hints (region zoom, `batch`).

`skills/doctor/SKILL.md` → `/computer-use:doctor`: runs `cu doctor` (DPI, monitors, privileges, hooks, UIA, screenshot speed) and explains the result. `skills/demo/SKILL.md` → `/computer-use:demo`: show the overlay for 5 s and take a screenshot without the overlay.

`hooks/hooks.json`: `UserPromptSubmit` → `cu ctl resume --quiet` (async); `Stop` → `cu ctl release --quiet`; `SubagentStop` (matcher `operator`) → `cu ctl release --quiet`. No `PreToolUse` hooks on purpose: each hook is a process per call and would kill speed; the pause check lives in the server.

`.claude-plugin/marketplace.json`: `{"name":"claude-computer-use","owner":{"name":"..."},"plugins":[{"name":"computer-use","source":"./"}]}`. Install: `/plugin marketplace add racass-pixel/claude-computer-use` → `/plugin install computer-use@claude-computer-use`. Development: `claude --plugin-dir .`, `/reload-plugins`.

Permissions: a plugin cannot ship an allow list. The README gives a ready block for `settings.json`: `"permissions":{"allow":["mcp__plugin_computer-use_desktop"]}`.

## 9. Models per task

| Role | Model | Why |
|---|---|---|
| Orchestrator | session model | planning, verification, talking to the user |
| Operator | `sonnet` (Sonnet 5), `effort: low` | fast screenshot → action loop, strong vision |
| Operator for hard subtasks | `opus` via dispatch override | judgment, complex documents |
| Trivial repeats | `haiku` via dispatch override | cheap and fast |

Honest note on speed: the binary executes an action in under 100 ms (BitBlt + scale + PNG ≈ 60–100 ms; SendInput is microseconds). The loop is bounded by model latency (1.5–4 s per Sonnet turn). The design therefore minimizes turns: `find` instead of guessing, `batch`, `wait`, and a screenshot in every action response.

## 10. Risks and known limits

- `SetForegroundWindow` is restricted by Windows; standard workarounds are used; rarely a window only flashes in the taskbar.
- UIPI: Claude cannot send input to elevated windows (UAC dialogs, admin windows); `doctor` warns; running `cu.exe` elevated lifts the limit (documented).
- Games / exclusive fullscreen: the overlay is not visible; low-level hooks still work.
- `WDA_EXCLUDEFROMCAPTURE` with BitBlt is confirmed by a spike; a fallback exists.
- The `effort` agent frontmatter field: values undocumented; validated with the plugin validator; otherwise a "don't deliberate" instruction in the prompt.
- Without bypass mode every tool call asks for permission; the README gives the allow rule.
- Double Esc: the first Esc reaches the application (may close a dialog); documented; `ctrl+alt+esc` in config is fully swallowed.

## 11. Later (not in v0.1)

DXGI Desktop Duplication for sub-10 ms capture; a tray daemon variant on top of `ipc`; macOS/Linux `platform` implementations; OCR fallback for apps without UIA; session recording to GIF for reports.
