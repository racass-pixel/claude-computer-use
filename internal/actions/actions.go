// Package actions composes primitive platform.Input calls into human-like actions.
package actions

import (
	"fmt"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/input"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

type Actor struct {
	In             platform.Input
	Clip           platform.Clipboard // may be nil: paste mode then falls back to unicode
	Sleep          func(time.Duration)
	PasteThreshold int // "auto" typing pastes when the text is longer than this (runes)
}

func (a *Actor) sleep(d time.Duration) {
	if a.Sleep != nil {
		a.Sleep(d)
	} else {
		time.Sleep(d)
	}
}

func (a *Actor) holdMods(mods []uint16) error {
	for _, m := range mods {
		if err := a.In.KeyDown(m); err != nil {
			return err
		}
	}
	return nil
}

func (a *Actor) releaseMods(mods []uint16) error {
	for i := len(mods) - 1; i >= 0; i-- {
		if err := a.In.KeyUp(mods[i]); err != nil {
			return err
		}
	}
	return nil
}

// Click moves to p and presses btn count times (2 = double click) while holding mods.
func (a *Actor) Click(p geom.Point, btn platform.MouseButton, count int, mods []uint16) error {
	if count < 1 {
		count = 1
	}
	if btn == "" {
		btn = platform.ButtonLeft
	}
	if err := a.holdMods(mods); err != nil {
		return err
	}
	defer a.releaseMods(mods)
	if err := a.In.MouseMove(p); err != nil {
		return err
	}
	a.sleep(15 * time.Millisecond)
	for i := 0; i < count; i++ {
		if err := a.In.MouseDown(btn); err != nil {
			return err
		}
		a.sleep(12 * time.Millisecond)
		if err := a.In.MouseUp(btn); err != nil {
			return err
		}
		if i < count-1 {
			a.sleep(60 * time.Millisecond) // well inside the system double-click time
		}
	}
	return nil
}

// Drag presses at from, moves in steps over dur, pauses, and releases at to.
func (a *Actor) Drag(from, to geom.Point, btn platform.MouseButton, dur time.Duration) error {
	if btn == "" {
		btn = platform.ButtonLeft
	}
	if dur <= 0 {
		dur = 250 * time.Millisecond
	}
	if err := a.In.MouseMove(from); err != nil {
		return err
	}
	a.sleep(30 * time.Millisecond)
	if err := a.In.MouseDown(btn); err != nil {
		return err
	}
	a.sleep(80 * time.Millisecond) // Explorer and browsers start a drag only after the button is held
	steps := max(8, int(dur/(12*time.Millisecond)))
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		p := geom.Point{
			X: from.X + int(float64(to.X-from.X)*t+0.5),
			Y: from.Y + int(float64(to.Y-from.Y)*t+0.5),
		}
		if i == steps {
			p = to
		}
		if err := a.In.MouseMove(p); err != nil {
			return err
		}
		a.sleep(dur / time.Duration(steps))
	}
	a.sleep(80 * time.Millisecond) // let drop targets highlight before releasing
	return a.In.MouseUp(btn)
}

// Scroll optionally moves to p, then scrolls by ticks (dy>0 down, dx>0 right).
func (a *Actor) Scroll(p *geom.Point, dx, dy int) error {
	if p != nil {
		if err := a.In.MouseMove(*p); err != nil {
			return err
		}
		a.sleep(10 * time.Millisecond)
	}
	return a.In.Scroll(dx, dy)
}

// Chord presses modifiers, the key, holds, and releases in reverse order.
func (a *Actor) Chord(c input.Chord, hold time.Duration) error {
	if err := a.holdMods(c.Mods); err != nil {
		return err
	}
	if err := a.In.KeyDown(c.Key); err != nil {
		return err
	}
	if hold <= 0 {
		hold = 10 * time.Millisecond
	}
	a.sleep(hold)
	if err := a.In.KeyUp(c.Key); err != nil {
		return err
	}
	return a.releaseMods(c.Mods)
}

// Type enters text. mode: auto (unicode, paste when long), unicode, paste, keys (per-char with delay).
func (a *Actor) Type(text, mode string, delay time.Duration) error {
	runes := []rune(text)
	switch mode {
	case "", "auto":
		if a.Clip != nil && a.PasteThreshold > 0 && len(runes) > a.PasteThreshold {
			return a.paste(text)
		}
		return a.typeUnicode(runes, delay)
	case "unicode":
		return a.typeUnicode(runes, delay)
	case "paste":
		if a.Clip == nil {
			return a.typeUnicode(runes, delay)
		}
		return a.paste(text)
	case "keys":
		if delay < 15*time.Millisecond {
			delay = 15 * time.Millisecond
		}
		return a.typeUnicode(runes, delay)
	}
	return fmt.Errorf("unknown type mode %q (auto|unicode|paste|keys)", mode)
}

func (a *Actor) typeUnicode(runes []rune, delay time.Duration) error {
	if delay <= 0 {
		return a.In.TypeUnicode(string(runes))
	}
	for _, r := range runes {
		if err := a.In.TypeUnicode(string(r)); err != nil {
			return err
		}
		a.sleep(delay)
	}
	return nil
}

func (a *Actor) paste(text string) error {
	old, _ := a.Clip.GetText()
	if err := a.Clip.SetText(text); err != nil {
		return err
	}
	if err := a.Chord(input.Chord{Mods: []uint16{input.VK_CONTROL}, Key: input.VK_V}, 0); err != nil {
		return err
	}
	a.sleep(300 * time.Millisecond) // most apps read the clipboard synchronously on Ctrl+V; give slow ones time
	return a.Clip.SetText(old)
}
