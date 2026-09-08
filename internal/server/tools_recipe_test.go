package server

import (
	"context"
	"strings"
	"testing"

	"github.com/racass-pixel/claude-computer-use/internal/recipes"
)

func newRecipeHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	store, err := recipes.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h.s.d.Recipes = store
	return h
}

func TestRecipeSaveThenRunExecutesSteps(t *testing.T) {
	h := newRecipeHarness(t)
	ctx := context.Background()

	// Save a recipe
	res, _, _ := h.s.toolRecipe(ctx, nil, RecipeIn{
		Action: "save",
		Name:   "Type and Enter",
		Steps: []recipes.Step{
			{Tool: "type", Args: map[string]any{"text": "hello"}},
			{Tool: "key", Args: map[string]any{"key": "enter"}},
		},
	})
	fields, _ := decode(t, res)
	if res.IsError {
		t.Fatalf("save failed: %v", fields)
	}
	rec := fields["recipe"].(map[string]any)
	slug := rec["slug"].(string)
	if slug == "" {
		t.Fatal("no slug returned")
	}

	// Clear fake input calls
	h.in.Calls = nil

	// Run the recipe
	res, _, _ = h.s.toolRecipe(ctx, nil, RecipeIn{
		Action: "run",
		Slug:   slug,
	})
	fields, img := decode(t, res)
	if res.IsError {
		t.Fatalf("run failed: %v", fields)
	}
	if fields["ok"] != true {
		t.Fatalf("run ok = %v", fields["ok"])
	}
	if fields["recipe"] != slug {
		t.Fatalf("recipe = %v", fields["recipe"])
	}
	if fields["runs"] != float64(1) {
		t.Fatalf("runs = %v", fields["runs"])
	}
	if img == nil {
		t.Fatal("expected screenshot after run")
	}

	// Verify the fake input received the actions
	got := strings.Join(h.in.Calls, "|")
	if !strings.Contains(got, "type hello") || !strings.Contains(got, "key_down 13") {
		t.Fatalf("input calls = %q", got)
	}
}

func TestRecipeRunUnknownSlug(t *testing.T) {
	h := newRecipeHarness(t)
	res, _, _ := h.s.toolRecipe(context.Background(), nil, RecipeIn{
		Action: "run",
		Slug:   "nonexistent",
	})
	if !res.IsError {
		t.Fatal("expected error for unknown slug")
	}
}

func TestRecipeRunWithParams(t *testing.T) {
	h := newRecipeHarness(t)
	ctx := context.Background()

	res, _, _ := h.s.toolRecipe(ctx, nil, RecipeIn{
		Action: "save",
		Name:   "Type text",
		Params: []string{"msg"},
		Steps: []recipes.Step{
			{Tool: "type", Args: map[string]any{"text": "{{msg}}"}},
		},
	})
	fields, _ := decode(t, res)
	slug := fields["recipe"].(map[string]any)["slug"].(string)

	h.in.Calls = nil
	res, _, _ = h.s.toolRecipe(ctx, nil, RecipeIn{
		Action: "run",
		Slug:   slug,
		Values: map[string]string{"msg": "world"},
	})
	fields, _ = decode(t, res)
	if fields["ok"] != true {
		t.Fatalf("run failed: %v", fields)
	}
	if !strings.Contains(strings.Join(h.in.Calls, "|"), "type world") {
		t.Fatalf("calls = %v, expected type world", h.in.Calls)
	}
}

func TestRecipeTraceListsActions(t *testing.T) {
	h := newRecipeHarness(t)
	ctx := context.Background()

	// Perform a click to generate trace
	h.s.toolClick(ctx, nil, ClickIn{X: intPtr(100), Y: intPtr(100)})

	// Get trace
	res, _, _ := h.s.toolRecipe(ctx, nil, RecipeIn{Action: "trace"})
	fields, _ := decode(t, res)
	trace := fields["trace"].([]any)
	if len(trace) == 0 {
		t.Fatal("trace is empty after click")
	}
	entry := trace[len(trace)-1].(map[string]any)
	if entry["tool"] != "click" {
		t.Fatalf("trace entry tool = %v", entry["tool"])
	}
}

func TestRecipeTraceClaredOnAcquire(t *testing.T) {
	h := newRecipeHarness(t)
	ctx := context.Background()

	// Perform a click
	h.s.toolClick(ctx, nil, ClickIn{X: intPtr(100), Y: intPtr(100)})

	// Acquire with task clears trace
	h.s.toolControl(ctx, nil, ControlIn{Action: "acquire", Task: "new task"})

	// Trace should be empty
	res, _, _ := h.s.toolRecipe(ctx, nil, RecipeIn{Action: "trace"})
	fields, _ := decode(t, res)
	trace := fields["trace"].([]any)
	if len(trace) != 0 {
		t.Fatalf("trace should be empty after acquire, got %d entries", len(trace))
	}
}

func TestRecipeNilStoreReturnsUnsupported(t *testing.T) {
	h := newHarness(t) // no recipe store
	res, _, _ := h.s.toolRecipe(context.Background(), nil, RecipeIn{Action: "search", Query: "test"})
	if !res.IsError {
		t.Fatal("expected error when store is nil")
	}
}

func TestRecipeSearchAndList(t *testing.T) {
	h := newRecipeHarness(t)
	ctx := context.Background()

	h.s.toolRecipe(ctx, nil, RecipeIn{
		Action:      "save",
		Name:        "Open Notepad",
		Description: "opens notepad via run dialog",
		Steps:       []recipes.Step{{Tool: "key", Args: map[string]any{"key": "win+r"}}},
	})

	// Search
	res, _, _ := h.s.toolRecipe(ctx, nil, RecipeIn{Action: "search", Query: "open notepad"})
	fields, _ := decode(t, res)
	matches := fields["matches"].([]any)
	if len(matches) == 0 {
		t.Fatal("no search results")
	}

	// List
	res, _, _ = h.s.toolRecipe(ctx, nil, RecipeIn{Action: "list"})
	fields, _ = decode(t, res)
	list := fields["recipes"].([]any)
	if len(list) != 1 {
		t.Fatalf("list returned %d", len(list))
	}
}

func TestRecipeDelete(t *testing.T) {
	h := newRecipeHarness(t)
	ctx := context.Background()

	res, _, _ := h.s.toolRecipe(ctx, nil, RecipeIn{
		Action: "save",
		Name:   "To Delete",
		Steps:  []recipes.Step{{Tool: "click"}},
	})
	fields, _ := decode(t, res)
	slug := fields["recipe"].(map[string]any)["slug"].(string)

	res, _, _ = h.s.toolRecipe(ctx, nil, RecipeIn{Action: "delete", Slug: slug})
	if res.IsError {
		t.Fatal("delete failed")
	}

	res, _, _ = h.s.toolRecipe(ctx, nil, RecipeIn{Action: "get", Slug: slug})
	if !res.IsError {
		t.Fatal("get should fail after delete")
	}
}

func intPtr(i int) *int { return &i }
