---
name: doctor
description: Check the computer-use plugin environment (monitors, DPI, capture speed, UI Automation, hooks, permissions) and explain any problem. Use when desktop tools fail, when asked to check the setup, or after installing the plugin.
---

# /computer-use:doctor

1. The plugin root is two directories above this skill's base directory (`<root>/skills/doctor`). Run with Bash:
   `"<root>/bin/cu.exe" doctor` (build it first with `powershell -NoProfile -File "<root>/scripts/build.ps1"` if it is missing).
2. Read the report and explain, in the user's language: how many monitors and their scale factors, whether DPI awareness is per-monitor v2, capture time (should be under 100 ms), whether UI Automation returned elements, and whether capture exclusion for the overlay is native.
3. If the desktop MCP server is not connected (`/mcp` shows it disconnected), tell the user to run `/reload-plugins` or restart Claude Code, and to add the permission rule `mcp__plugin_computer-use_desktop` to `permissions.allow` in settings if they are not in bypass mode. If Claude Code was started with its working directory INSIDE this plugin's repository, the repo's `.mcp.json` is also loaded as a project-level server named `desktop` where `${CLAUDE_PLUGIN_ROOT}` is not expanded; that duplicate fails with CONNECTION_CLOSED — ignore it or start Claude Code from another directory. The plugin's own server is `plugin-computer-use-desktop`.
4. Known limits to mention only when relevant: elevated (admin) windows and UAC prompts cannot receive input unless cu runs elevated; exclusive-fullscreen games hide the overlay.
