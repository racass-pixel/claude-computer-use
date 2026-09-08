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
The user takes control ONLY with Esc Esc (by default); their mouse or typing does not pause you — so never fight the user's cursor: if the screen changes unexpectedly, re-observe rather than fight the cursor.
If a tool or the operator reports `user_took_control`: stop, tell the user what happened and what is left, and wait. Do not resume on your own. The user's next message resumes control automatically; then re-dispatch from the current screen state.

## Recipes
Recipes are procedural memory — saved sequences of desktop actions that can be replayed without screenshots between steps.

1. **Before multi-step work**, search for an existing recipe: `recipe{action:"search", query:"<what you are about to do>", app:"<process name if known>"}`.
2. If a match scores >= 0.5, pass it to the operator: tell it the recipe slug and any param values. The operator will run it with `recipe{action:"run", slug:"...", values:{...}}` and verify the end state.
3. **After a successful novel task** with >= 4 actions, distil it into a recipe:
   - Call `recipe{action:"trace"}` to see the actions performed.
   - Save with `recipe{action:"save", name:"<name in the user's language>", description:"<description with synonyms so search finds it>", app:"<process>", params:["<variable parts>"], steps:[...]}`.
   - Use `{{param}}` placeholders for text/paths that vary between runs.
   - Add `wait` steps (with `window`, `element`, or `stable`) between app transitions.

## Cost
A screenshot is ~1.5k tokens. Prefer `find`, `batch`, `wait{stable:true}`, and `screenshot:false` on steps you do not need to see. Zoom with `screenshot{region}` only for small targets.
