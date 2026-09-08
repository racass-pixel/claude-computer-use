package server

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/recipes"
)

type ControlIn struct {
	Action string `json:"action" jsonschema:"status, acquire (show the overlay now — returns suggested_recipes when task is given), release (hide it; auto-records a recipe draft if the trace has enough action steps), or hud (set the task title shown to the user)"`
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
			s.clearTrace()

			// Remember task caption and foreground app for auto-record.
			fg := s.foreground()
			s.mu.Lock()
			s.taskCaption = in.Task
			s.taskApp = fg.Process
			s.recipeRanInJob = false
			s.mu.Unlock()
		}
		if m := s.activeMonitor(); m.ID != 0 {
			s.d.Overlay.Show(m, platform.OverlayControlling)
		}

		// Search for matching recipes and return suggestions.
		f := s.controlStatus()
		suggestions := s.suggestRecipes(in.Task)
		f["suggested_recipes"] = suggestions
		if len(suggestions) > 0 {
			if score, ok := suggestions[0]["score"].(float64); ok && score >= 0.5 {
				slug, _ := suggestions[0]["slug"].(string)
				f["hint"] = fmt.Sprintf("Best match: %s (score %.2f) — run it with recipe{action:\"run\",slug:\"%s\"} before doing anything by hand.", slug, score, slug)
			}
		}
		return okResult(f, nil), nil, nil

	case "release":
		// Auto-record before releasing.
		s.autoRecord()

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

// suggestRecipes searches the recipe store for matches, returning a slice for the response.
func (s *Session) suggestRecipes(task string) []map[string]any {
	if s.d.Recipes == nil || task == "" {
		return []map[string]any{}
	}
	s.mu.Lock()
	app := s.taskApp
	s.mu.Unlock()
	matches, err := s.d.Recipes.Search(task, app, 3)
	if err != nil || len(matches) == 0 {
		return []map[string]any{}
	}
	out := make([]map[string]any, len(matches))
	for i, m := range matches {
		out[i] = map[string]any{
			"slug":      m.Recipe.Slug,
			"name":      m.Recipe.Name,
			"score":     m.Score,
			"params":    m.Recipe.Params,
			"runs":      m.Recipe.Runs,
			"successes": m.Recipe.Successes,
			"auto":      m.Recipe.Auto,
		}
	}
	return out
}

// autoRecord saves a draft recipe automatically if the conditions are met:
// >= 4 action steps, no recipe run in this job, and a task caption was set.
func (s *Session) autoRecord() {
	if s.d.Recipes == nil {
		return
	}
	s.mu.Lock()
	caption := s.taskCaption
	app := s.taskApp
	ranRecipe := s.recipeRanInJob
	s.mu.Unlock()

	if caption == "" || ranRecipe {
		return
	}

	draft := recipes.Draft(s.traceToRecipeEntries(), caption, "", app)
	if len(draft.Steps) < 4 {
		return
	}

	_, _ = s.d.Recipes.Save(draft)
}

// OnRelease performs auto-recording. Called by external hooks (e.g., guard idle transition).
func (s *Session) OnRelease() {
	s.autoRecord()
}
