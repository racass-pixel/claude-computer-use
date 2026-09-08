package server

import (
	"context"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/recipes"
)

type RecipeIn struct {
	Action      string            `json:"action" jsonschema:"search, get, save, run, delete, trace or list"`
	Query       string            `json:"query,omitempty"`
	App         string            `json:"app,omitempty"`
	Slug        string            `json:"slug,omitempty"`
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Params      []string          `json:"params,omitempty"`
	Steps       []recipes.Step    `json:"steps,omitempty"`
	Values      map[string]string `json:"values,omitempty"`
	Limit       int               `json:"limit,omitempty"`
	StopOnError *bool             `json:"stop_on_error,omitempty"`
	Screenshot  *bool             `json:"screenshot,omitempty"`
}

func (s *Session) toolRecipe(ctx context.Context, req *mcp.CallToolRequest, in RecipeIn) (*mcp.CallToolResult, any, error) {
	switch in.Action {
	case "trace":
		return s.recipeTrace()
	case "search":
		return s.recipeSearch(in)
	case "get":
		return s.recipeGet(in)
	case "save":
		return s.recipeSave(in)
	case "run":
		return s.recipeRun(ctx, in)
	case "delete":
		return s.recipeDelete(in)
	case "list":
		return s.recipeList()
	default:
		return errResult("bad_args", "action must be search, get, save, run, delete, trace or list"), nil, nil
	}
}

func (s *Session) recipeTrace() (*mcp.CallToolResult, any, error) {
	entries := s.traceEntries()
	out := make([]map[string]any, len(entries))
	for i, e := range entries {
		out[i] = map[string]any{
			"tool":    e.Tool,
			"summary": e.Summary,
			"ok":      e.OK,
			"ms":      e.Ms,
			"at":      e.At.Format(time.RFC3339),
		}
	}
	return okResult(map[string]any{"trace": out, "count": len(out)}, nil), nil, nil
}

func (s *Session) requireStore() *mcp.CallToolResult {
	if s.d.Recipes == nil {
		return errResult("unsupported", "recipe store is not available")
	}
	return nil
}

func (s *Session) recipeSearch(in RecipeIn) (*mcp.CallToolResult, any, error) {
	if r := s.requireStore(); r != nil {
		return r, nil, nil
	}
	if in.Query == "" {
		return errResult("bad_args", "query is required"), nil, nil
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 5
	}
	matches, err := s.d.Recipes.Search(in.Query, in.App, limit)
	if err != nil {
		return errResult("search_failed", err.Error()), nil, nil
	}
	out := make([]map[string]any, len(matches))
	for i, m := range matches {
		out[i] = map[string]any{
			"slug":        m.Recipe.Slug,
			"name":        m.Recipe.Name,
			"description": m.Recipe.Description,
			"app":         m.Recipe.App,
			"params":      m.Recipe.Params,
			"score":       m.Score,
			"runs":        m.Recipe.Runs,
			"successes":   m.Recipe.Successes,
		}
	}
	return okResult(map[string]any{"matches": out, "count": len(out)}, nil), nil, nil
}

func (s *Session) recipeGet(in RecipeIn) (*mcp.CallToolResult, any, error) {
	if r := s.requireStore(); r != nil {
		return r, nil, nil
	}
	if in.Slug == "" {
		return errResult("bad_args", "slug is required"), nil, nil
	}
	rec, err := s.d.Recipes.Get(in.Slug)
	if err != nil {
		return errResult("not_found", err.Error()), nil, nil
	}
	return okResult(map[string]any{"recipe": rec}, nil), nil, nil
}

func (s *Session) recipeSave(in RecipeIn) (*mcp.CallToolResult, any, error) {
	if r := s.requireStore(); r != nil {
		return r, nil, nil
	}
	if in.Name == "" {
		return errResult("bad_args", "name is required"), nil, nil
	}
	if len(in.Steps) == 0 {
		return errResult("bad_args", "steps is required"), nil, nil
	}
	rec := recipes.Recipe{
		Name:        in.Name,
		Description: in.Description,
		App:         in.App,
		Tags:        in.Tags,
		Params:      in.Params,
		Steps:       in.Steps,
	}
	saved, err := s.d.Recipes.Save(rec)
	if err != nil {
		return errResult("save_failed", err.Error()), nil, nil
	}
	return okResult(map[string]any{"recipe": saved}, nil), nil, nil
}

func (s *Session) recipeRun(ctx context.Context, in RecipeIn) (*mcp.CallToolResult, any, error) {
	if r := s.requireStore(); r != nil {
		return r, nil, nil
	}
	if in.Slug == "" {
		return errResult("bad_args", "slug is required"), nil, nil
	}
	t0 := time.Now()

	rec, err := s.d.Recipes.Get(in.Slug)
	if err != nil {
		return errResult("not_found", err.Error()), nil, nil
	}

	rendered, err := recipes.Render(rec, in.Values)
	if err != nil {
		return errResult("render_failed", err.Error()), nil, nil
	}

	// Convert recipe steps to BatchActions
	actions := make([]BatchAction, len(rendered))
	for i, st := range rendered {
		actions[i] = BatchAction{Tool: st.Tool, Args: st.Args}
	}

	stop := in.StopOnError == nil || *in.StopOnError
	steps, allOK := s.runSteps(ctx, actions, stop)

	// Bump counters
	_ = s.d.Recipes.Bump(in.Slug, allOK)

	// Re-read for updated counts
	updated, _ := s.d.Recipes.Get(in.Slug)

	f := map[string]any{
		"ok":        allOK,
		"recipe":    in.Slug,
		"steps":     steps,
		"completed": len(steps),
		"runs":      updated.Runs,
		"successes": updated.Successes,
	}
	if s.wantShot(in.Screenshot) {
		return s.finish("recipe", t0, f, true, 120*time.Millisecond), nil, nil
	}
	return s.finish("recipe", t0, f, false, 0), nil, nil
}

func (s *Session) recipeDelete(in RecipeIn) (*mcp.CallToolResult, any, error) {
	if r := s.requireStore(); r != nil {
		return r, nil, nil
	}
	if in.Slug == "" {
		return errResult("bad_args", "slug is required"), nil, nil
	}
	if err := s.d.Recipes.Delete(in.Slug); err != nil {
		return errResult("not_found", err.Error()), nil, nil
	}
	return okResult(map[string]any{"deleted": in.Slug}, nil), nil, nil
}

func (s *Session) recipeList() (*mcp.CallToolResult, any, error) {
	if r := s.requireStore(); r != nil {
		return r, nil, nil
	}
	all, err := s.d.Recipes.List()
	if err != nil {
		return errResult("list_failed", err.Error()), nil, nil
	}
	out := make([]map[string]any, len(all))
	for i, r := range all {
		out[i] = map[string]any{
			"name":        r.Name,
			"slug":        r.Slug,
			"description": r.Description,
			"app":         r.App,
			"runs":        r.Runs,
			"successes":   r.Successes,
		}
	}
	return okResult(map[string]any{"recipes": out, "count": len(out)}, nil), nil, nil
}
