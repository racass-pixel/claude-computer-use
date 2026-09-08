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

func TestAcquireReturnsSuggestedRecipes(t *testing.T) {
	h := newRecipeHarness(t)
	ctx := context.Background()

	// Save a matching recipe
	h.s.toolRecipe(ctx, nil, RecipeIn{
		Action:      "save",
		Name:        "Open Notepad",
		Description: "open notepad via win+r run dialog",
		Steps:       []recipes.Step{{Tool: "key", Args: map[string]any{"key": "win+r"}}},
	})

	// Acquire with a task that matches
	res, _, _ := h.s.toolControl(ctx, nil, ControlIn{Action: "acquire", Task: "Open Notepad via Run"})
	fields, _ := decode(t, res)

	suggestions, ok := fields["suggested_recipes"].([]any)
	if !ok {
		t.Fatalf("suggested_recipes not in response: %v", fields)
	}
	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggested recipe")
	}
	first := suggestions[0].(map[string]any)
	if first["slug"] != "open-notepad" {
		t.Fatalf("first suggestion slug = %v", first["slug"])
	}
	if first["score"].(float64) < 0.5 {
		t.Fatalf("score = %v, expected >= 0.5", first["score"])
	}
}

func TestAcquireEmptyStoreReturnEmptyList(t *testing.T) {
	h := newRecipeHarness(t)
	ctx := context.Background()

	res, _, _ := h.s.toolControl(ctx, nil, ControlIn{Action: "acquire", Task: "anything"})
	fields, _ := decode(t, res)
	suggestions, ok := fields["suggested_recipes"].([]any)
	if !ok {
		t.Fatal("suggested_recipes not in response")
	}
	if len(suggestions) != 0 {
		t.Fatalf("expected empty suggestions, got %d", len(suggestions))
	}
}

func TestReleaseAutoRecordsRecipe(t *testing.T) {
	h := newRecipeHarness(t)
	ctx := context.Background()

	// Acquire with a task
	h.s.toolControl(ctx, nil, ControlIn{Action: "acquire", Task: "Test auto record"})

	// Perform >= 4 action tool calls
	h.s.toolClick(ctx, nil, ClickIn{X: intPtr(100), Y: intPtr(100)})
	h.s.toolClick(ctx, nil, ClickIn{X: intPtr(200), Y: intPtr(200)})
	h.s.toolType(ctx, nil, TypeIn{Text: "hello"})
	h.s.toolKey(ctx, nil, KeyIn{Key: "enter"})

	// Release should auto-record
	h.s.toolControl(ctx, nil, ControlIn{Action: "release"})

	// Check recipe was saved
	all, err := h.s.d.Recipes.List()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range all {
		if r.Auto && r.Name == "Test auto record" {
			found = true
			if len(r.Steps) < 4 {
				t.Fatalf("auto-recorded recipe has only %d steps", len(r.Steps))
			}
		}
	}
	if !found {
		t.Fatal("no auto-recorded recipe found after release")
	}
}

func TestReleaseNoAutoRecordWhenRecipeRan(t *testing.T) {
	h := newRecipeHarness(t)
	ctx := context.Background()

	// Save a recipe to run
	h.s.toolRecipe(ctx, nil, RecipeIn{
		Action: "save",
		Name:   "Quick click",
		Steps:  []recipes.Step{{Tool: "click"}},
	})

	// Acquire
	h.s.toolControl(ctx, nil, ControlIn{Action: "acquire", Task: "Quick click test"})

	// Run the recipe
	h.s.toolRecipe(ctx, nil, RecipeIn{Action: "run", Slug: "quick-click"})

	// Do extra actions
	h.s.toolClick(ctx, nil, ClickIn{X: intPtr(100), Y: intPtr(100)})
	h.s.toolClick(ctx, nil, ClickIn{X: intPtr(200), Y: intPtr(200)})
	h.s.toolClick(ctx, nil, ClickIn{X: intPtr(300), Y: intPtr(300)})
	h.s.toolClick(ctx, nil, ClickIn{X: intPtr(400), Y: intPtr(400)})

	// Release
	h.s.toolControl(ctx, nil, ControlIn{Action: "release"})

	// Should NOT auto-record because a recipe was run
	all, _ := h.s.d.Recipes.List()
	for _, r := range all {
		if r.Auto {
			t.Fatal("should not auto-record when a recipe was run in the job")
		}
	}
}

func TestRecipeRunReportsFailedSteps(t *testing.T) {
	h := newRecipeHarness(t)
	ctx := context.Background()

	// Save a recipe with a step that will fail (drag needs from/to)
	h.s.toolRecipe(ctx, nil, RecipeIn{
		Action: "save",
		Name:   "Failing steps test",
		Steps: []recipes.Step{
			{Tool: "type", Args: map[string]any{"text": "ok"}},
			{Tool: "drag"}, // will fail: no from/to
		},
	})

	stopFalse := false
	res, _, _ := h.s.toolRecipe(ctx, nil, RecipeIn{
		Action:      "run",
		Slug:        "failing-steps-test",
		StopOnError: &stopFalse,
	})
	fields, _ := decode(t, res)
	if fields["ok"] != false {
		t.Fatal("expected ok=false for failing recipe")
	}
	failed, ok := fields["failed"].([]any)
	if !ok || len(failed) == 0 {
		t.Fatalf("expected failed steps in result, got %v", fields["failed"])
	}
	// Check that successes was not incremented
	if fields["successes"] != float64(0) {
		t.Fatalf("successes = %v, want 0", fields["successes"])
	}
	if fields["runs"] != float64(1) {
		t.Fatalf("runs = %v, want 1", fields["runs"])
	}
}

func TestRecipeDraftAction(t *testing.T) {
	h := newRecipeHarness(t)
	ctx := context.Background()

	// Acquire to set caption
	h.s.toolControl(ctx, nil, ControlIn{Action: "acquire", Task: "Draft test"})

	// Perform actions
	h.s.toolClick(ctx, nil, ClickIn{X: intPtr(100), Y: intPtr(100)})
	h.s.toolType(ctx, nil, TypeIn{Text: "hello"})
	h.s.toolKey(ctx, nil, KeyIn{Key: "enter"})

	// Call draft
	res, _, _ := h.s.toolRecipe(ctx, nil, RecipeIn{Action: "draft"})
	fields, _ := decode(t, res)
	draft, ok := fields["draft"].(map[string]any)
	if !ok {
		t.Fatalf("draft not in result: %v", fields)
	}
	if draft["name"] != "Draft test" {
		t.Fatalf("draft name = %v", draft["name"])
	}
	if draft["auto"] != true {
		t.Fatal("draft should have auto:true")
	}
}

func TestTraceContainsArgs(t *testing.T) {
	h := newRecipeHarness(t)
	ctx := context.Background()

	h.s.toolKey(ctx, nil, KeyIn{Key: "enter"})

	res, _, _ := h.s.toolRecipe(ctx, nil, RecipeIn{Action: "trace"})
	fields, _ := decode(t, res)
	trace := fields["trace"].([]any)
	if len(trace) == 0 {
		t.Fatal("trace empty")
	}
	entry := trace[len(trace)-1].(map[string]any)
	args, ok := entry["args"].(map[string]any)
	if !ok {
		t.Fatalf("trace entry has no args: %v", entry)
	}
	if args["key"] != "enter" {
		t.Fatalf("args key = %v", args["key"])
	}
}

func TestOnReleaseAutoRecordsLikeControlRelease(t *testing.T) {
	h := newRecipeHarness(t)
	ctx := context.Background()

	// Acquire with a task
	h.s.toolControl(ctx, nil, ControlIn{Action: "acquire", Task: "Idle auto record"})

	// Perform >= 4 action tool calls
	h.s.toolClick(ctx, nil, ClickIn{X: intPtr(100), Y: intPtr(100)})
	h.s.toolClick(ctx, nil, ClickIn{X: intPtr(200), Y: intPtr(200)})
	h.s.toolType(ctx, nil, TypeIn{Text: "test"})
	h.s.toolKey(ctx, nil, KeyIn{Key: "enter"})

	// Call OnRelease externally (simulates idle release from guard callback)
	h.s.OnRelease()

	// Check recipe was saved
	all, err := h.s.d.Recipes.List()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range all {
		if r.Auto && r.Name == "Idle auto record" {
			found = true
		}
	}
	if !found {
		t.Fatal("OnRelease did not auto-record a recipe")
	}
}

func intPtr(i int) *int { return &i }
