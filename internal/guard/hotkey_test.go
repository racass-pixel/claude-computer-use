package guard

import (
	"testing"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/input"
)

func TestParseHotkey(t *testing.T) {
	h, err := ParseHotkey("esc esc")
	if err != nil || h.Taps != 2 || h.Chord.Key != input.VK_ESCAPE || h.String() != "Esc Esc" {
		t.Fatalf("esc esc: %+v %v %q", h, err, h.String())
	}
	h, err = ParseHotkey("ctrl+alt+esc")
	if err != nil || h.Taps != 1 || len(h.Chord.Mods) != 2 || h.String() != "Ctrl+Alt+Esc" {
		t.Fatalf("chord: %+v %v", h, err)
	}
	for _, bad := range []string{"", "esc enter", "bogus bogus"} {
		if _, err := ParseHotkey(bad); err == nil {
			t.Fatalf("%q must fail", bad)
		}
	}
}

func at(ms int) time.Time { return time.Unix(0, 0).Add(time.Duration(ms) * time.Millisecond) }

func TestDoubleTapWithinWindow(t *testing.T) {
	h, _ := ParseHotkey("esc esc")
	m := NewTapMatcher(h, 400*time.Millisecond)
	if m.Feed(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(0)}) {
		t.Fatal("first tap must not complete")
	}
	m.Feed(Event{Kind: KeyUp, VK: input.VK_ESCAPE, At: at(50)})
	if !m.Feed(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(300)}) {
		t.Fatal("second tap within 400ms must complete")
	}
	// gesture resets after completing
	if m.Feed(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(500)}) {
		t.Fatal("a new sequence must start after completion")
	}
	if m.Feed(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(1200)}) {
		t.Fatal("taps 700ms apart must not complete")
	}
}

func TestChordNeedsAllModifiersHeld(t *testing.T) {
	h, _ := ParseHotkey("ctrl+alt+esc")
	m := NewTapMatcher(h, 400*time.Millisecond)
	if m.Feed(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(0)}) {
		t.Fatal("esc alone must not match")
	}
	m.Feed(Event{Kind: KeyDown, VK: input.VK_LCONTROL, At: at(10)})
	m.Feed(Event{Kind: KeyDown, VK: input.VK_RMENU, At: at(20)})
	if !m.Feed(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(30)}) {
		t.Fatal("ctrl+alt+esc must match with left/right variants held")
	}
	m.Feed(Event{Kind: KeyUp, VK: input.VK_LCONTROL, At: at(40)})
	if m.Feed(Event{Kind: KeyDown, VK: input.VK_ESCAPE, At: at(50)}) {
		t.Fatal("after releasing ctrl the chord must not match")
	}
	if !m.Involves(input.VK_RCONTROL) || m.Involves(0x41) {
		t.Fatal("Involves must cover the key and its modifiers")
	}
}
