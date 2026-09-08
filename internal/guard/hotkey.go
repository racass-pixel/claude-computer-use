// Package guard decides when Claude may act: it owns the idle/controlling/paused state machine
// and recognises the user's take-over gesture.
package guard

import (
	"fmt"
	"strings"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/input"
)

type Hotkey struct {
	Chord input.Chord
	Taps  int
}

// ParseHotkey accepts "esc esc" (double tap) or a single chord like "ctrl+alt+esc".
func ParseHotkey(s string) (Hotkey, error) {
	tokens := strings.Fields(strings.ToLower(s))
	if len(tokens) == 0 {
		return Hotkey{}, fmt.Errorf("empty hotkey")
	}
	for _, tok := range tokens[1:] {
		if tok != tokens[0] {
			return Hotkey{}, fmt.Errorf("hotkey %q: repeated taps must use the same key (e.g. \"esc esc\")", s)
		}
	}
	c, err := input.ParseChord(tokens[0])
	if err != nil {
		return Hotkey{}, fmt.Errorf("hotkey %q: %w", s, err)
	}
	return Hotkey{Chord: c, Taps: len(tokens)}, nil
}

func (h Hotkey) String() string {
	parts := make([]string, h.Taps)
	for i := range parts {
		parts[i] = h.Chord.String()
	}
	return strings.Join(parts, " ")
}

type Kind int

const (
	KeyDown Kind = iota
	KeyUp
	MouseMove
	MouseDown
)

type Event struct {
	Kind     Kind
	VK       uint16
	Pos      geom.Point
	Injected bool
	At       time.Time
}

// TapMatcher recognises the hotkey gesture from a stream of key events.
type TapMatcher struct {
	h       Hotkey
	window  time.Duration
	held    map[uint16]bool // normalised modifiers currently down
	count   int
	lastTap time.Time
}

func NewTapMatcher(h Hotkey, window time.Duration) *TapMatcher {
	return &TapMatcher{h: h, window: window, held: map[uint16]bool{}}
}

func (m *TapMatcher) Involves(vk uint16) bool {
	n := input.NormalizeModifier(vk)
	if n == m.h.Chord.Key {
		return true
	}
	for _, mod := range m.h.Chord.Mods {
		if mod == n {
			return true
		}
	}
	return false
}

func (m *TapMatcher) modsHeld() bool {
	want := map[uint16]bool{}
	for _, mod := range m.h.Chord.Mods {
		want[mod] = true
	}
	for mod := range want {
		if !m.held[mod] {
			return false
		}
	}
	for mod := range m.held {
		if m.held[mod] && !want[mod] {
			return false // an extra modifier is held
		}
	}
	return true
}

func (m *TapMatcher) Feed(ev Event) bool {
	if input.IsModifier(ev.VK) {
		n := input.NormalizeModifier(ev.VK)
		switch ev.Kind {
		case KeyDown:
			m.held[n] = true
		case KeyUp:
			delete(m.held, n)
		}
		return false
	}
	if ev.Kind != KeyDown || ev.VK != m.h.Chord.Key || !m.modsHeld() {
		return false
	}
	if m.count > 0 && ev.At.Sub(m.lastTap) > m.window {
		m.count = 0
	}
	m.count++
	m.lastTap = ev.At
	if m.count >= m.h.Taps {
		m.count = 0
		return true
	}
	return false
}
