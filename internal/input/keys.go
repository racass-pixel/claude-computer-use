// Package input implements platform.Input and the key chord grammar ("ctrl+shift+t").
package input

import (
	"fmt"
	"strings"
)

const (
	VK_BACK     = 0x08
	VK_TAB      = 0x09
	VK_RETURN   = 0x0D
	VK_SHIFT    = 0x10
	VK_CONTROL  = 0x11
	VK_MENU     = 0x12
	VK_PAUSE    = 0x13
	VK_CAPITAL  = 0x14
	VK_ESCAPE   = 0x1B
	VK_SPACE    = 0x20
	VK_PRIOR    = 0x21
	VK_NEXT     = 0x22
	VK_END      = 0x23
	VK_HOME     = 0x24
	VK_LEFT     = 0x25
	VK_UP       = 0x26
	VK_RIGHT    = 0x27
	VK_DOWN     = 0x28
	VK_SNAPSHOT = 0x2C
	VK_INSERT   = 0x2D
	VK_DELETE   = 0x2E
	VK_LWIN     = 0x5B
	VK_RWIN     = 0x5C
	VK_APPS     = 0x5D
	VK_V        = 0x56
	VK_NUMLOCK  = 0x90
	VK_SCROLL   = 0x91
	VK_LSHIFT   = 0xA0
	VK_RSHIFT   = 0xA1
	VK_LCONTROL = 0xA2
	VK_RCONTROL = 0xA3
	VK_LMENU    = 0xA4
	VK_RMENU    = 0xA5
)

// Chord is one key with modifiers, e.g. Ctrl+Shift+T. Mods are in canonical order: ctrl, alt, shift, win.
type Chord struct {
	Mods []uint16
	Key  uint16
}

var modifierOrder = []uint16{VK_CONTROL, VK_MENU, VK_SHIFT, VK_LWIN}

var modifierNames = map[string]uint16{
	"ctrl": VK_CONTROL, "control": VK_CONTROL,
	"alt": VK_MENU, "option": VK_MENU,
	"shift": VK_SHIFT,
	"win":   VK_LWIN, "cmd": VK_LWIN, "super": VK_LWIN, "meta": VK_LWIN, "windows": VK_LWIN,
}

var keyNames = map[string]uint16{
	"enter": VK_RETURN, "return": VK_RETURN,
	"esc": VK_ESCAPE, "escape": VK_ESCAPE,
	"tab": VK_TAB, "space": VK_SPACE, "backspace": VK_BACK,
	"delete": VK_DELETE, "del": VK_DELETE, "insert": VK_INSERT, "ins": VK_INSERT,
	"home": VK_HOME, "end": VK_END,
	"pageup": VK_PRIOR, "pgup": VK_PRIOR, "pagedown": VK_NEXT, "pgdn": VK_NEXT,
	"left": VK_LEFT, "arrowleft": VK_LEFT, "up": VK_UP, "arrowup": VK_UP,
	"right": VK_RIGHT, "arrowright": VK_RIGHT, "down": VK_DOWN, "arrowdown": VK_DOWN,
	"printscreen": VK_SNAPSHOT, "prtsc": VK_SNAPSHOT, "pause": VK_PAUSE,
	"capslock": VK_CAPITAL, "numlock": VK_NUMLOCK, "scrolllock": VK_SCROLL,
	"apps": VK_APPS, "menu": VK_APPS, "contextmenu": VK_APPS,
	"plus": 0xBB, "equal": 0xBB, "minus": 0xBD, "comma": 0xBC, "period": 0xBE, "dot": 0xBE,
	"slash": 0xBF, "backslash": 0xDC, "semicolon": 0xBA, "quote": 0xDE,
	"lbracket": 0xDB, "rbracket": 0xDD, "grave": 0xC0, "backquote": 0xC0, "tilde": 0xC0,
	"numpad0": 0x60, "numpad1": 0x61, "numpad2": 0x62, "numpad3": 0x63, "numpad4": 0x64,
	"numpad5": 0x65, "numpad6": 0x66, "numpad7": 0x67, "numpad8": 0x68, "numpad9": 0x69,
	"multiply": 0x6A, "add": 0x6B, "subtract": 0x6D, "decimal": 0x6E, "divide": 0x6F,
	"volumeup": 0xAF, "volumedown": 0xAE, "volumemute": 0xAD,
	"mediaplaypause": 0xB3, "mediastop": 0xB2, "medianext": 0xB0, "mediaprev": 0xB1,
	"browserback": 0xA6, "browserforward": 0xA7,
}

// ExtendedKeys need KEYEVENTF_EXTENDEDKEY so apps see the "real" key, not the numpad twin.
var ExtendedKeys = map[uint16]bool{
	VK_INSERT: true, VK_DELETE: true, VK_HOME: true, VK_END: true, VK_PRIOR: true, VK_NEXT: true,
	VK_LEFT: true, VK_UP: true, VK_RIGHT: true, VK_DOWN: true, VK_LWIN: true, VK_RWIN: true,
	VK_APPS: true, VK_NUMLOCK: true, VK_SNAPSHOT: true, 0x6F: true, VK_RCONTROL: true, VK_RMENU: true,
}

func init() {
	for c := 'a'; c <= 'z'; c++ {
		keyNames[string(c)] = uint16(0x41 + c - 'a')
	}
	for c := '0'; c <= '9'; c++ {
		keyNames[string(c)] = uint16(0x30 + c - '0')
	}
	for i := 1; i <= 24; i++ {
		keyNames[fmt.Sprintf("f%d", i)] = uint16(0x70 + i - 1)
	}
}

// ModifierVK maps a modifier name ("ctrl", "control", "alt", "shift", "win", "cmd"...) to its VK.
func ModifierVK(name string) (uint16, bool) {
	vk, ok := modifierNames[strings.ToLower(strings.TrimSpace(name))]
	return vk, ok
}

// ParseChord parses "ctrl+shift+t", "Enter", "win+r". Case-insensitive; spaces around '+' allowed.
func ParseChord(s string) (Chord, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return Chord{}, fmt.Errorf("empty key chord")
	}
	parts := strings.Split(s, "+")
	// a trailing "+" (e.g. "ctrl++") means the plus key
	var tokens []string
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			if i == len(parts)-1 && len(parts) > 1 && tokens != nil && parts[i-1] == "" {
				tokens = append(tokens, "plus")
			}
			continue
		}
		tokens = append(tokens, p)
	}
	if len(tokens) == 0 {
		return Chord{}, fmt.Errorf("invalid key chord %q", s)
	}
	seen := map[uint16]bool{}
	var c Chord
	for i, tok := range tokens {
		last := i == len(tokens)-1
		if vk, ok := modifierNames[tok]; ok && !last {
			if seen[vk] {
				return Chord{}, fmt.Errorf("duplicate modifier %q in %q", tok, s)
			}
			seen[vk] = true
			continue
		}
		if !last {
			return Chord{}, fmt.Errorf("%q is not a modifier in %q", tok, s)
		}
		vk, ok := keyNames[tok]
		if !ok {
			return Chord{}, fmt.Errorf("unknown key %q in %q (use `type` for text)", tok, s)
		}
		c.Key = vk
	}
	for _, m := range modifierOrder {
		if seen[m] {
			c.Mods = append(c.Mods, m)
		}
	}
	if c.Key == 0 {
		return Chord{}, fmt.Errorf("chord %q has no key", s)
	}
	return c, nil
}

var displayNames = map[uint16]string{
	VK_CONTROL: "Ctrl", VK_MENU: "Alt", VK_SHIFT: "Shift", VK_LWIN: "Win",
	VK_RETURN: "Enter", VK_ESCAPE: "Esc", VK_TAB: "Tab", VK_SPACE: "Space", VK_BACK: "Backspace",
	VK_DELETE: "Delete", VK_INSERT: "Insert", VK_HOME: "Home", VK_END: "End",
	VK_PRIOR: "PageUp", VK_NEXT: "PageDown", VK_LEFT: "Left", VK_UP: "Up", VK_RIGHT: "Right", VK_DOWN: "Down",
	VK_SNAPSHOT: "PrintScreen", VK_APPS: "Menu", 0xBB: "+", 0xBD: "-",
}

// KeyName is the display name of a virtual key ("Ctrl", "Enter", "A", "F5").
func KeyName(vk uint16) string {
	if n, ok := displayNames[vk]; ok {
		return n
	}
	switch {
	case vk >= 0x41 && vk <= 0x5A, vk >= 0x30 && vk <= 0x39:
		return string(rune(vk))
	case vk >= 0x70 && vk <= 0x87:
		return fmt.Sprintf("F%d", vk-0x70+1)
	}
	return fmt.Sprintf("VK%02X", vk)
}

func (c Chord) String() string {
	parts := make([]string, 0, len(c.Mods)+1)
	for _, m := range c.Mods {
		parts = append(parts, KeyName(m))
	}
	parts = append(parts, KeyName(c.Key))
	return strings.Join(parts, "+")
}

// NormalizeModifier maps left/right variants to the generic modifier code.
func NormalizeModifier(vk uint16) uint16 {
	switch vk {
	case VK_LSHIFT, VK_RSHIFT:
		return VK_SHIFT
	case VK_LCONTROL, VK_RCONTROL:
		return VK_CONTROL
	case VK_LMENU, VK_RMENU:
		return VK_MENU
	case VK_RWIN:
		return VK_LWIN
	}
	return vk
}

func IsModifier(vk uint16) bool {
	switch NormalizeModifier(vk) {
	case VK_SHIFT, VK_CONTROL, VK_MENU, VK_LWIN:
		return true
	}
	return false
}
