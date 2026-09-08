package recipes

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSlugifyASCII(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Open Notepad", "open-notepad"},
		{"  Hello   World  ", "hello-world"},
		{"ctrl+s save!", "ctrl-s-save"},
		{"", ""}, // empty → hash fallback tested below
	}
	for _, tc := range tests {
		got := Slugify(tc.in)
		if tc.in == "" {
			if len(got) == 0 {
				t.Fatal("empty name must produce a fallback slug")
			}
			if got[:7] != "recipe-" {
				t.Fatalf("empty slug = %q, want recipe-<hex>", got)
			}
			continue
		}
		if got != tc.want {
			t.Fatalf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSlugifyCyrillic(t *testing.T) {
	got := Slugify("Сохранить файл в Блокноте")
	if got == "" {
		t.Fatal("Cyrillic slug is empty")
	}
	// Must be ASCII-only
	for _, r := range got {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			t.Fatalf("slug %q contains non-ASCII rune %q", got, string(r))
		}
	}
	if got != "sokhranit-fayl-v-bloknote" {
		t.Fatalf("Cyrillic slug = %q, want sokhranit-fayl-v-bloknote", got)
	}
}

func TestSlugifyMaxLength(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += "word "
	}
	got := Slugify(long)
	if len(got) > 60 {
		t.Fatalf("slug len %d > 60: %q", len(got), got)
	}
}

func TestSaveGetRoundTrip(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := Recipe{
		Name:        "Open Notepad",
		Description: "Opens notepad via Win+R",
		Steps: []Step{
			{Tool: "key", Args: map[string]any{"key": "win+r"}},
			{Tool: "type", Args: map[string]any{"text": "notepad"}},
			{Tool: "key", Args: map[string]any{"key": "enter"}},
		},
	}
	saved, err := s.Save(r)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Slug == "" {
		t.Fatal("slug is empty")
	}
	if saved.CreatedAt.IsZero() {
		t.Fatal("created_at not set")
	}

	got, err := s.Get(saved.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Open Notepad" || len(got.Steps) != 3 {
		t.Fatalf("Get returned wrong data: %+v", got)
	}
}

func TestSaveRejectsEmptySteps(t *testing.T) {
	s, _ := Open(t.TempDir())
	_, err := s.Save(Recipe{Name: "empty"})
	if err == nil {
		t.Fatal("expected error for empty steps")
	}
}

func TestSaveRejectsInvalidTool(t *testing.T) {
	s, _ := Open(t.TempDir())
	_, err := s.Save(Recipe{
		Name:  "bad",
		Steps: []Step{{Tool: "screenshot"}},
	})
	if err == nil {
		t.Fatal("expected error for non-batchable tool")
	}
}

func TestList(t *testing.T) {
	s, _ := Open(t.TempDir())
	s.Save(Recipe{Name: "A", Steps: []Step{{Tool: "click"}}})
	s.Save(Recipe{Name: "B", Steps: []Step{{Tool: "type", Args: map[string]any{"text": "hi"}}}})
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("List returned %d recipes", len(list))
	}
}

func TestDelete(t *testing.T) {
	s, _ := Open(t.TempDir())
	saved, _ := s.Save(Recipe{Name: "Del", Steps: []Step{{Tool: "key", Args: map[string]any{"key": "enter"}}}})
	if err := s.Delete(saved.Slug); err != nil {
		t.Fatal(err)
	}
	_, err := s.Get(saved.Slug)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestSearchRanksExactNameAbovePartial(t *testing.T) {
	s, _ := Open(t.TempDir())
	s.Save(Recipe{
		Name:        "Open Notepad via Run",
		Description: "opens notepad",
		Steps:       []Step{{Tool: "key"}},
	})
	s.Save(Recipe{
		Name:        "Open Calculator",
		Description: "opens calculator via run dialog",
		Steps:       []Step{{Tool: "key"}},
	})
	matches, err := s.Search("open notepad", "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no matches")
	}
	if matches[0].Recipe.Name != "Open Notepad via Run" {
		t.Fatalf("first match = %q, want Open Notepad via Run", matches[0].Recipe.Name)
	}
	// The notepad recipe should have a higher score because all query tokens appear in the name
	if len(matches) > 1 && matches[0].Score <= matches[1].Score {
		t.Fatalf("exact name match (%f) should score higher than partial (%f)", matches[0].Score, matches[1].Score)
	}
}

func TestSearchDropsLowScores(t *testing.T) {
	s, _ := Open(t.TempDir())
	s.Save(Recipe{
		Name:        "Format Disk",
		Description: "formats a disk",
		Steps:       []Step{{Tool: "key"}},
	})
	matches, _ := s.Search("open notepad calculator", "", 5)
	// "Format Disk" should not match "open notepad calculator" at all
	if len(matches) != 0 {
		t.Fatalf("expected no matches for unrelated query, got %d (score=%f)", len(matches), matches[0].Score)
	}
}

func TestSearchAppBonus(t *testing.T) {
	s, _ := Open(t.TempDir())
	s.Save(Recipe{
		Name:  "Save file in Notepad",
		App:   "notepad.exe",
		Steps: []Step{{Tool: "key"}},
	})
	s.Save(Recipe{
		Name:  "Save file in Excel",
		App:   "excel.exe",
		Steps: []Step{{Tool: "key"}},
	})
	matches, _ := s.Search("save file", "notepad.exe", 5)
	if len(matches) < 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].Recipe.App != "notepad.exe" {
		t.Fatal("notepad.exe should rank first with app filter")
	}
}

func TestRenderSubstitution(t *testing.T) {
	r := Recipe{
		Params: []string{"text", "path"},
		Steps: []Step{
			{Tool: "type", Args: map[string]any{"text": "{{text}}"}},
			{Tool: "key", Args: map[string]any{"key": "ctrl+s"}},
			{Tool: "type", Args: map[string]any{"text": "{{path}}"}},
		},
	}
	steps, err := Render(r, map[string]string{"text": "hello", "path": "C:\\test.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].Args["text"] != "hello" {
		t.Fatalf("step 0 text = %v", steps[0].Args["text"])
	}
	if steps[2].Args["text"] != "C:\\test.txt" {
		t.Fatalf("step 2 text = %v", steps[2].Args["text"])
	}
}

func TestRenderMissingParam(t *testing.T) {
	r := Recipe{
		Params: []string{"text"},
		Steps:  []Step{{Tool: "type", Args: map[string]any{"text": "{{text}}"}}},
	}
	_, err := Render(r, nil)
	if err == nil {
		t.Fatal("expected error for missing param")
	}
}

func TestBump(t *testing.T) {
	s, _ := Open(t.TempDir())
	saved, _ := s.Save(Recipe{Name: "Bump", Steps: []Step{{Tool: "click"}}})
	if err := s.Bump(saved.Slug, true, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Bump(saved.Slug, false, "step 2 failed"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(saved.Slug)
	if got.Runs != 2 || got.Successes != 1 {
		t.Fatalf("runs=%d successes=%d, want 2/1", got.Runs, got.Successes)
	}
}

func TestSaveUpdatesExisting(t *testing.T) {
	s, _ := Open(t.TempDir())
	r1, _ := s.Save(Recipe{Name: "Test", Steps: []Step{{Tool: "click"}}})
	time.Sleep(time.Millisecond) // ensure UpdatedAt changes
	r2, _ := s.Save(Recipe{Name: "Test", Description: "updated", Steps: []Step{{Tool: "key"}}})
	if r2.Slug != r1.Slug {
		t.Fatalf("slug changed: %q -> %q", r1.Slug, r2.Slug)
	}
	if r2.UpdatedAt.Equal(r1.CreatedAt) {
		t.Fatal("UpdatedAt not bumped")
	}
	got, _ := s.Get(r1.Slug)
	if got.Description != "updated" || len(got.Steps) != 1 || got.Steps[0].Tool != "key" {
		t.Fatalf("update not persisted: %+v", got)
	}
}

func TestPathTraversal(t *testing.T) {
	s, _ := Open(t.TempDir())
	_, err := s.Get("../../../etc/passwd")
	if err == nil {
		t.Fatal("path traversal should fail")
	}
	err = s.Delete("../evil")
	if err == nil {
		t.Fatal("path traversal delete should fail")
	}
}

func TestDraftCollapsesWaitsAndParametrises(t *testing.T) {
	trace := []TraceEntry{
		{Tool: "screenshot", Args: map[string]any{}, OK: true},        // non-action → dropped
		{Tool: "key", Args: map[string]any{"key": "win+r"}, OK: true}, // action
		{Tool: "wait", Args: map[string]any{"stable": true}, OK: true},
		{Tool: "wait", Args: map[string]any{"ms": float64(500)}, OK: true},                     // consecutive → collapsed
		{Tool: "type", Args: map[string]any{"text": "this is a long sentence here"}, OK: true}, // > 3 words → param
		{Tool: "key", Args: map[string]any{"key": "enter"}, OK: true},
		{Tool: "control", Args: map[string]any{"action": "release"}, OK: true}, // non-action → dropped
		{Tool: "click", Args: map[string]any{"x": float64(10), "y": float64(20)}, OK: true},
	}
	draft := Draft(trace, "Test Draft", "", "notepad.exe")
	if !draft.Auto {
		t.Fatal("draft must be Auto:true")
	}
	if draft.App != "notepad.exe" {
		t.Fatalf("app = %q", draft.App)
	}
	// Should have: key, wait, type(param), key, wait(auto-inserted after win+r... already covered), click
	// Let me count: key(win+r) → wait{stable} inserted? No, next is already wait. So: key, wait, type, key, click
	// Actually the sequence: key(win+r) has next entry = wait, so needWait is true but nextIsWait is true → no insert.
	// Then wait(stable) kept, wait(ms:500) collapsed (prev is wait), type → param, key(enter), click.
	// So: key, wait, type(param), key, click = 5 steps.

	// Check non-action tools were dropped
	for _, s := range draft.Steps {
		if s.Tool == "screenshot" || s.Tool == "control" {
			t.Fatalf("non-action tool %q should be dropped", s.Tool)
		}
	}
	// Check consecutive waits collapsed
	prevWait := false
	for _, s := range draft.Steps {
		if s.Tool == "wait" {
			if prevWait {
				t.Fatal("consecutive waits should be collapsed")
			}
			prevWait = true
		} else {
			prevWait = false
		}
	}
	// Check long type text was parameterised
	if len(draft.Params) != 1 || draft.Params[0] != "text1" {
		t.Fatalf("params = %v, want [text1]", draft.Params)
	}
	found := false
	for _, s := range draft.Steps {
		if s.Tool == "type" {
			if s.Args["text"] != "{{text1}}" {
				t.Fatalf("type text = %v, want {{text1}}", s.Args["text"])
			}
			found = true
		}
	}
	if !found {
		t.Fatal("no type step found")
	}
}

func TestDraftInsertsWaitAfterWinKey(t *testing.T) {
	trace := []TraceEntry{
		{Tool: "key", Args: map[string]any{"key": "win+r"}, OK: true},
		{Tool: "type", Args: map[string]any{"text": "hi"}, OK: true},
	}
	draft := Draft(trace, "Test", "", "")
	// Should insert wait{stable:true} after the win+r key
	if len(draft.Steps) < 3 {
		t.Fatalf("expected at least 3 steps (key, wait, type), got %d", len(draft.Steps))
	}
	if draft.Steps[1].Tool != "wait" {
		t.Fatalf("step 1 should be wait, got %s", draft.Steps[1].Tool)
	}
	if draft.Steps[1].Args["stable"] != true {
		t.Fatal("inserted wait should have stable:true")
	}
}

func TestSearchPrefersCuratedOverAuto(t *testing.T) {
	s, _ := Open(t.TempDir())
	s.Save(Recipe{
		Name:        "Open Notepad",
		Description: "open notepad via run dialog",
		Auto:        true,
		Steps:       []Step{{Tool: "key"}},
	})
	s.Save(Recipe{
		Name:        "Open Notepad curated",
		Description: "open notepad via run dialog",
		Steps:       []Step{{Tool: "key"}},
	})
	matches, _ := s.Search("open notepad", "", 5)
	if len(matches) < 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	// Curated should rank first (it has +0.1 bonus)
	if matches[0].Recipe.Auto {
		t.Fatal("curated recipe should rank above auto")
	}
}

func TestSearchDropsRepeatedFailures(t *testing.T) {
	s, _ := Open(t.TempDir())
	saved, _ := s.Save(Recipe{
		Name:        "Failing recipe",
		Description: "this recipe always fails",
		Steps:       []Step{{Tool: "key"}},
	})
	// Bump 2 failures
	s.Bump(saved.Slug, false, "err1")
	s.Bump(saved.Slug, false, "err2")

	matches, _ := s.Search("failing recipe", "", 5)
	if len(matches) != 0 {
		t.Fatalf("expected failing recipe (runs>=2, successes=0) to be excluded, got %d matches", len(matches))
	}
}

func TestSaveResetsCountersOnCuratedOverwrite(t *testing.T) {
	s, _ := Open(t.TempDir())
	saved, _ := s.Save(Recipe{Name: "Counter", Steps: []Step{{Tool: "click"}}})
	s.Bump(saved.Slug, true, "")
	s.Bump(saved.Slug, true, "")

	got, _ := s.Get(saved.Slug)
	if got.Runs != 2 {
		t.Fatalf("runs before resave = %d, want 2", got.Runs)
	}

	// Resave (curated, not auto) should reset counters
	resaved, _ := s.Save(Recipe{Name: "Counter", Description: "updated", Steps: []Step{{Tool: "key"}}})
	if resaved.Runs != 0 || resaved.Successes != 0 {
		t.Fatalf("after curated resave: runs=%d successes=%d, want 0/0", resaved.Runs, resaved.Successes)
	}
}

func TestAutoSaveDoesNotOverwriteCurated(t *testing.T) {
	s, _ := Open(t.TempDir())
	// Save a curated recipe
	s.Save(Recipe{Name: "My Task", Steps: []Step{{Tool: "click"}}})

	// Auto-save with same name should get a different slug
	auto, _ := s.Save(Recipe{Name: "My Task", Auto: true, Steps: []Step{{Tool: "key"}}})
	if auto.Slug == "my-task" {
		t.Fatal("auto recipe should not overwrite curated slug")
	}
	if auto.Slug != "my-task-2" {
		t.Fatalf("auto slug = %q, want my-task-2", auto.Slug)
	}

	// Original curated recipe still intact
	curated, _ := s.Get("my-task")
	if curated.Steps[0].Tool != "click" {
		t.Fatal("curated recipe was overwritten")
	}
}

func TestBumpStoresLastError(t *testing.T) {
	s, _ := Open(t.TempDir())
	saved, _ := s.Save(Recipe{Name: "Err", Steps: []Step{{Tool: "click"}}})
	s.Bump(saved.Slug, false, "step 3: click failed")
	got, _ := s.Get(saved.Slug)
	if got.LastError != "step 3: click failed" {
		t.Fatalf("LastError = %q", got.LastError)
	}
	// Success clears LastError
	s.Bump(saved.Slug, true, "")
	got, _ = s.Get(saved.Slug)
	if got.LastError != "" {
		t.Fatalf("LastError after success = %q, want empty", got.LastError)
	}
}

func TestOpenCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sub", "recipes")
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(s.Dir)
	if err != nil || !fi.IsDir() {
		t.Fatal("dir not created")
	}
}
