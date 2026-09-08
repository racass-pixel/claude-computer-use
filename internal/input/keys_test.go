package input

import "testing"

func TestParseChord(t *testing.T) {
	cases := map[string]Chord{
		"ctrl+shift+t": {Mods: []uint16{VK_CONTROL, VK_SHIFT}, Key: 0x54},
		"Enter":        {Key: VK_RETURN},
		"win+r":        {Mods: []uint16{VK_LWIN}, Key: 0x52},
		"alt+f4":       {Mods: []uint16{VK_MENU}, Key: 0x73},
		"f12":          {Key: 0x7B},
		"ctrl+1":       {Mods: []uint16{VK_CONTROL}, Key: 0x31},
		"escape":       {Key: VK_ESCAPE},
		"esc":          {Key: VK_ESCAPE},
		"pgdn":         {Key: 0x22},
		"cmd+space":    {Mods: []uint16{VK_LWIN}, Key: 0x20},
		"shift+ctrl+a": {Mods: []uint16{VK_CONTROL, VK_SHIFT}, Key: 0x41}, // canonical order ctrl, alt, shift, win
		"ctrl+plus":    {Mods: []uint16{VK_CONTROL}, Key: 0xBB},
		"arrowleft":    {Key: 0x25},
		"printscreen":  {Key: 0x2C},
	}
	for in, want := range cases {
		got, err := ParseChord(in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got.Key != want.Key || len(got.Mods) != len(want.Mods) {
			t.Fatalf("%q: got %+v want %+v", in, got, want)
		}
		for i := range want.Mods {
			if got.Mods[i] != want.Mods[i] {
				t.Fatalf("%q: mods %v want %v", in, got.Mods, want.Mods)
			}
		}
	}
	for _, bad := range []string{"", "ctrl+", "bogus", "ctrl+ctrl+a", "+", "ctrl+ü", "shift"} {
		if _, err := ParseChord(bad); err == nil {
			t.Fatalf("%q must fail", bad)
		}
	}
}

func TestChordString(t *testing.T) {
	c, _ := ParseChord("shift+ctrl+esc")
	if c.String() != "Ctrl+Shift+Esc" {
		t.Fatalf("String = %q", c.String())
	}
	if KeyName(0x0D) != "Enter" || KeyName(0x41) != "A" {
		t.Fatalf("KeyName wrong")
	}
	if NormalizeModifier(0xA2) != VK_CONTROL || NormalizeModifier(0x5C) != VK_LWIN || !IsModifier(0xA5) {
		t.Fatalf("modifier normalisation wrong")
	}
}
