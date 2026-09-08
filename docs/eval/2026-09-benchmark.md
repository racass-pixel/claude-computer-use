# Benchmark Results — 2026-09-08

Machine: Windows 11 Home (10.0.26200), 2560x1600 @150% DPI, single monitor.
Model: claude-sonnet-5 (via `claude -p` with `--dangerously-skip-permissions`).

## Results Table

| Task | Description | Target Turns | Actual Turns | Duration (s) | Success | Notes |
|------|-------------|:------------:|:------------:|:------------:|:-------:|-------|
| 1 | Notepad: write "привет", save as cu-test.txt | ≤6 | 7 | 117.7 | Yes | Used operator; opened via Win+R, typed, Ctrl+S save dialog. 1 turn over target. |
| 2 | Explorer: create cu-eval in Downloads | ≤5 | 7 | 74.3 | Yes | Used operator; navigated via Win+R to Downloads, created folder. 2 turns over target. Re-run after tuning: 8 turns (worse). |
| 3 | Drag cu-test.txt from Desktop into cu-eval | ≤6 | 7 | 62.6 | Yes | Drag-and-drop via operator. 1 turn over target. |
| 4 | Chrome: example.com title | ≤5 | 6 | 48.4 | Yes | Reported "Example Domain". 1 turn over target. |
| 5 | Chrome: httpbin form fill (Sasha, 123, medium) | ≤8 | 8 | 60.3 | Yes | On target. Form filled and submitted. |
| 6 | Calculator: 12*34 | ≤5 | 4 | 39.0 | Yes | Reported 408. Under target. |
| 7 | Move Calculator to second monitor | ≤4 | — | — | Manual | Skipped: single-monitor machine. |
| 8 | Esc-Esc interruption test | — | — | — | Manual | Skipped: requires human interaction. |
| 9 | Settings: display scale via `find` | ≤5 | 4 | 95.9 | Yes | Reported 150% (recommended). Used UIA `find`. Under target. |
| 10 | Notepad: click small close tab button | ≤4 | 9 | 36.3 | Partial | Closed the tab but took 9 turns (target 4). Re-run after tuning: improved from 13→9 but still over. Small tab close button is hard to locate via `find` or vision. |

**Summary: 6/8 runnable tasks succeeded. 3/8 within turn target (tasks 5, 6, 9). 5/8 over target (tasks 1-4, 10).**

## Recipe Speed Comparison (Task 1)

| Run | Turns | Duration (s) | Mechanism |
|-----|:-----:|:------------:|-----------|
| Run 1 (first time) | 7 | 117.7 | Operator: Win+R → notepad → type → Ctrl+S → save dialog |
| Run 2 (repeat) | 2 | 6.6 | Bypassed GUI — used Claude Code's file write tool directly |

**Speedup: 17.8x faster (117.7s → 6.6s), 7→2 turns.** On the second run, the model recognized it could write the file directly without GUI automation. This is a valid optimization path — the model learned the task could be done without the desktop tools. The existing Notepad recipe in the recipe store did not match (it was for opening Notepad without saving), so the recipe system was not the mechanism; the speedup came from model-level task routing.

## Tuning Changes

### Change 1: Operator prompt — small target guidance (agents/operator.md)

**Problem:** Task 10 (click small close tab button) took 13 turns on first run. The operator struggled to precisely locate and click Notepad's small tab close button using vision alone.

**Fix:** Expanded the "Small targets" guidance in the Coordinates section:
- Added explicit examples of using `find` with name/type filters for close buttons (`find{name:"Close", type:"Button"}`)
- Recommended `find` as the first strategy before falling back to `screenshot{region}` zoom
- Changed `click{element:"e2"}` to `click{element:"eN"}` to avoid implying a fixed element ID

**Result:** Improved from 13→9 turns. Still over target (4), but the Notepad tab close button is a genuinely difficult target — it's a custom WinUI control that UIA may not expose reliably as a distinct element.

### Change 2: Operator prompt — Explorer shortcuts (agents/operator.md)

**Problem:** Task 2 (create folder in Explorer) took 7 turns. The operator could be faster with keyboard shortcuts.

**Fix:** Added `Ctrl+Shift+N (new folder in Explorer)` and `Ctrl+E/Ctrl+L (Explorer address bar)` to the Speed section's shortcut list.

**Result:** Re-run was 8 turns (slightly worse due to model variance). The shortcut hint alone doesn't guarantee the model uses it — the model's planning path varies between runs.

### Change 3: CU_OVERLAY_VISIBLE_IN_CAPTURE env var (cmd/cu/cmd_serve.go)

**Addition:** Added environment variable check: when `CU_OVERLAY_VISIBLE_IN_CAPTURE=1` is set, `win.OverlayVisibleInCapture` is set to `true` before the overlay is created, making the overlay visible in screen recordings (ffmpeg gdigrab, OBS, etc.). Documented in README's Development section.

## Manual Items Left for the User

1. **Task 7 (second monitor):** "Move Calculator to second monitor and maximize" — requires a multi-monitor setup. Connect a second display and run:
   ```
   claude -p "Перенеси окно Калькулятора на второй монитор и разверни" --plugin-dir <path> --model sonnet --dangerously-skip-permissions --max-turns 60 --output-format json
   ```

2. **Task 8 (Esc-Esc interruption):** Start the form-fill task, move the mouse mid-way to trigger pause, verify the operator stops within one action and reports. Then type "продолжай" to resume. Requires real-time human interaction.

## Demo GIF

- File: `docs/media/demo.gif`
- Size: 3.21 MB (under 8 MB limit)
- Duration: 15 seconds @ 10 fps, 1280px wide
- Content: Shows the HUD banner ("Claude управляет компьютером"), status indicator (green dot), action labels (key/click/window operations), Notepad with typed text, Run dialog, and cursor activity.
- Source: 50s desktop recording trimmed to 17-32s (the active segment of Task 1).

## Files on User's Desktop/Downloads

Left for the user to inspect:
- `C:\Users\test\Desktop\cu-test.txt` — contains "привет"
- `C:\Users\test\Downloads\cu-eval\` — folder with copy of cu-test.txt from drag task
