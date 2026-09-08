# Changelog

## 0.1.0 — 2026-09-08

- 16 MCP tools: screenshot, click, move, drag, scroll, type, key, clipboard, monitors, windows, window, find, wait, batch, control, recipe
- Glowing take-over overlay with HUD, click ripple and breathing animation
- Instant user interrupt: Esc Esc (configurable hotkey) takes control back
- Mouse glide: smooth cursor movement between points (configurable duration)
- UI Automation `find` for native apps (buttons, fields, menu items by name/role)
- Recipes: procedural memory — save, search, run and trace multi-step procedures
- Operator agent (Sonnet) with per-task model override (Opus / Haiku)
- Orchestrator skill: task decomposition, operator dispatch, result verification
- Doctor and demo CLI commands for diagnostics and overlay preview
- Hooks: auto-resume on prompt submit, auto-release on stop / operator stop
- Checksum-verified release installer with Go build fallback
- Multi-monitor aware with per-monitor DPI v2
- Coordinate model: all x,y relative to the last screenshot, server converts to screen pixels
- Configuration via `%APPDATA%\claude-computer-use\config.json` and `CU_*` environment variables
