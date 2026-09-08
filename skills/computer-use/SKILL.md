---
name: computer-use
description: Use when the user asks to do something on their computer or screen — open or operate apps, click/type in windows, handle files through the GUI, browse sites, fill forms, move windows between monitors, or "look at my screen and tell me…".
---

# Computer use (orchestrator)

You have the `desktop` MCP tools and a `computer-use:operator` agent. You plan and verify; the operator executes.

## Flow
1. **Show the HUD**: call `control{action:"acquire", task:"<caption>"}` where `<caption>` is a short verb + object in the user's language, max 40 chars, no quotes — e.g. `Заполняю форму заказа`, `Ищу отчёт в почте`, `Opening Notepad`. This is what the user sees on screen while you work. **The response includes `suggested_recipes`** — check them immediately.
2. **Use a recipe if one matches.** If any `suggested_recipes` entry has `score >= 0.5`, dispatch the operator with `recipe{action:"run", slug:"<slug>", values:{...}}` as the FIRST instruction in the operator prompt. This replays the task without per-step screenshots — much faster. The operator verifies and finishes any failed steps by hand. **Always prefer running a matching recipe over doing it manually.**
3. Quick look: one `screenshot` (or `windows`) to see the current state (skip if running a recipe).
4. Split the request into subtasks, each with a verifiable end state ("Notepad shows the text and the file exists at C:\...").
5. For each subtask:
   - Update the HUD if the focus shifts: `control{action:"hud", task:"<new caption>"}`.
   - Dispatch `Agent(subagent_type: "computer-use:operator", prompt: ...)`.
   - Default model (Sonnet) for ordinary GUI work.
   - `model: "opus"` when the subtask needs judgment: reading long documents on screen, ambiguous UI, comparing options, anything irreversible.
   - `model: "haiku"` for trivial repeats ("click Next until Finish").
6. Verify the end state yourself (screenshot or `find`) before telling the user it is done.
7. Call `control{action:"release"}` when the whole job is finished so the overlay disappears. The server auto-records a draft recipe from the trace if the trace has >= 4 action steps and no recipe was run.

## The operator prompt
Include: the goal, the exact end state, the app or window, data to enter (verbatim), what NOT to do, and "report when done". One subtask per dispatch. For a single click or a look, act yourself instead of dispatching.

## When the user takes control
The user takes control ONLY with Esc Esc (by default); their mouse or typing does not pause you — so never fight the user's cursor: if the screen changes unexpectedly, re-observe rather than fight the cursor.
If a tool or the operator reports `user_took_control`: stop, tell the user what happened and what is left, and wait. Do not resume on your own. The user's next message resumes control automatically; then re-dispatch from the current screen state.

## Recipes
Recipes are procedural memory — saved sequences of desktop actions that can be replayed without screenshots between steps. The server surfaces them automatically and records drafts.

1. **`control acquire` returns `suggested_recipes`** — you no longer need to search manually. If a suggestion scores >= 0.5, you MUST tell the operator to run it: include `recipe{action:"run", slug:"<slug>", values:{...}}` as the first instruction. The operator replays the saved sequence much faster than doing it by hand.
2. **If `recipe run` reports `failed` steps**, the operator finishes the job by hand. Then call `recipe{action:"draft"}` to build an updated recipe from the trace, review the draft, and `recipe{action:"save", ...}` with the same name to replace it (saving on an existing slug resets the run counters so the repaired recipe gets a fresh start).
3. **`control release` auto-records** a draft recipe (`auto:true`) when the trace has >= 4 action steps, no recipe was run, and a task caption was set. Auto-recorded recipes are good but curated ones rank higher in search (+0.1 bonus), so after a novel job prefer `recipe draft` then edit and `save` it (curated beats auto).
4. **Manual save** for best quality: call `recipe{action:"draft"}` to get a recipe built from the trace (it drops non-action tools, collapses waits, parametrises long texts, adds wait steps after app transitions), then edit and save with `recipe{action:"save", name:"<name in the user's language>", description:"<description with synonyms so search finds it>", app:"<process>", params:[...], steps:[...]}`.
5. Recipes that have been run >= 2 times with zero successes are automatically excluded from suggestions.

## Cost
A screenshot is ~1.5k tokens. Prefer `find`, `batch`, `wait{stable:true}`, and `screenshot:false` on steps you do not need to see. Zoom with `screenshot{region}` only for small targets. For repetitive lists where every row needs the same action, use `pixel` to calibrate a visual cue and `click_until` to grind through rows server-side without per-click screenshots.
