---
name: demo
description: Show the Claude take-over overlay for a few seconds and prove it is excluded from screenshots. Use when the user wants to see how the overlay looks or to test it.
---

# /computer-use:demo

1. The plugin root is two directories above this skill's base directory (`<root>/skills/demo`). Run with Bash:
   `"<root>/bin/cu.exe" demo -seconds 5 -o "<scratchpad>/demo.png"`.
2. Read `demo.png` with the Read tool and confirm the glowing border and HUD are NOT in the capture — that is the exclusion proof. If the user instead wants to see a screenshot that DOES include the overlay (e.g. for a screen recording or to show someone else), re-run with `-show-in-capture` added, which skips the native capture exclusion so the overlay shows up in the saved image.
3. Tell the user what they should have seen: the accent-colored breathing border on monitor 1, the HUD pill at the top with the hotkey hint and a changing action line, ripples, then a gray "you are in control" state that fades out. Mention the config file path (`%APPDATA%\claude-computer-use\config.json`) for `accent`, `lang`, `hotkey`, `border_thickness`.
