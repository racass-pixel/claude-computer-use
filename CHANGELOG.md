# Changelog

## 0.1.0 — 2026-09-08

- Claude-style HUD: animated spark logo, warm dark pill, human-readable action captions (localized ru/en), key-cap hints, border glow shimmer; orchestrator writes the task title shown to the user
- `pixel` and `click_until` tools for grinding repetitive lists without per-click screenshots; `screenshot_region` on every action tool for zoomed follow-up captures
- `mouse_down` / `mouse_up` tools for held mouse buttons across calls; cross-window drag-and-drop via taskbar hover or Alt+Tab mid-drag; `drag` gains `via` waypoints and `hold_ms`; held-button safety: auto-release on pause, idle, control release, shutdown, and `drag_hold_timeout_ms` (default 60 s); `click` while held releases first (`released_held_button: true`)
- 20 MCP tools: screenshot, click, move, mouse_down, mouse_up, drag, scroll, type, key, clipboard, monitors, windows, window, find, wait, batch, control, recipe, pixel, click_until
- Glowing take-over overlay with HUD, click ripple and breathing animation
- Instant user interrupt: Esc Esc (configurable hotkey) takes control back
- Mouse glide: smooth cursor movement between points (configurable duration)
- UI Automation `find` for native apps (buttons, fields, menu items by name/role)
- Recipes: procedural memory — server-side suggestions on `control acquire`, auto-recorded drafts on `control release`, trace-based `recipe draft` action, success tracking with `LastError`, curated/auto ranking, failed-recipe exclusion; `recipe run` reports `failed` steps for repair
- Operator agent (Sonnet) with per-task model override (Opus / Haiku)
- Orchestrator skill: task decomposition, operator dispatch, result verification
- Doctor and demo CLI commands for diagnostics and overlay preview
- Hooks: auto-resume on prompt submit, auto-release on stop / operator stop
- Checksum-verified release installer with Go build fallback
- Multi-monitor aware with per-monitor DPI v2
- Coordinate model: all x,y relative to the last screenshot, server converts to screen pixels
- Configuration via `%APPDATA%\claude-computer-use\config.json` and `CU_*` environment variables
