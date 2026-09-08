package window

import (
	"testing"

	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

var wins = []platform.WindowInfo{
	{ID: 100, Title: "Untitled - Notepad", Process: "notepad.exe"},
	{ID: 200, Title: "Downloads - File Explorer", Process: "explorer.exe", Foreground: true},
	{ID: 300, Title: "GitHub - Google Chrome", Process: "chrome.exe"},
}

func TestMatchByIDForegroundAndRegex(t *testing.T) {
	if w, _ := Match(wins, "300"); w.ID != 300 {
		t.Fatalf("id match failed: %+v", w)
	}
	if w, _ := Match(wins, "foreground"); w.ID != 200 {
		t.Fatalf("foreground match failed: %+v", w)
	}
	if w, _ := Match(wins, "notepad"); w.ID != 100 {
		t.Fatalf("regex on title/process failed: %+v", w)
	}
	if w, _ := Match(wins, "(?i)chrome$"); w.ID != 300 {
		t.Fatalf("regex with flags failed: %+v", w)
	}
	if _, err := Match(wins, "nothing-here"); err == nil {
		t.Fatalf("no match must error")
	}
	if w, _ := Match(wins, "e"); w.ID != 200 {
		t.Fatalf("ambiguous match must prefer the foreground window: %+v", w)
	}
}
