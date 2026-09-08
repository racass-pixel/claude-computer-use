// Package actions composes primitive platform.Input calls into human-like actions.
package actions

import (
	"fmt"
	"math"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/input"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

type Actor struct {
	In             platform.Input
	Clip           platform.Clipboard // may be nil: paste mode then falls back to unicode
	Sleep          func(time.Duration)
	PasteThreshold int                        // "auto" typing pastes when the text is longer than this (runes)
	GlideMs        int                        // mouse glide duration in ms (0 = teleport)
	Pos            func() (geom.Point, error) // current cursor position; nil = teleport
}

func (a *Actor) sleep(d time.Duration) {
	if a.Sleep != nil {
		a.Sleep(d)
	} else {
		time.Sleep(d)
	}
}

// MoveTo glides the cursor to p using an ease-in-out cubic curve, or teleports
// if glide is disabled (GlideMs == 0), the distance is tiny, or Pos is nil/errors.
func (a *Actor) MoveTo(p geom.Point) error {
	if a.GlideMs == 0 || a.Pos == nil {
		return a.In.MouseMove(p)
	}
	from, err := a.Pos()
	if err != nil {
		return a.In.MouseMove(p)
	}
	dx, dy := float64(p.X-from.X), float64(p.Y-from.Y)
	dist := math.Hypot(dx, dy)
	if dist < 4 {
		return a.In.MouseMove(p)
	}
	steps := int(dist / 8)
	if steps < 12 {
		steps = 12
	}
	if steps > 60 {
		steps = 60
	}
	dur := float64(a.GlideMs) * (0.6 + 0.4*math.Min(1, dist/800))
	stepDur := time.Duration(dur/float64(steps)*1e6) * time.Nanosecond
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		var e float64
		if t < 0.5 {
			e = 4 * t * t * t
		} else {
			e = 1 - math.Pow(-2*t+2, 3)/2
		}
		q := geom.Point{
			X: from.X + int(dx*e+0.5),
			Y: from.Y + int(dy*e+0.5),
		}
		if i == steps {
			q = p
		}
		if err := a.In.MouseMove(q); err != nil {
			return err
		}
		if i < steps {
			a.sleep(stepDur)
		}
	}
	return nil
}

// holdMods presses mods in order. If one fails partway, it releases (best-effort,
// ignoring errors) whatever it already pressed before returning the error, so a
// failed chord never leaves a modifier stuck down.
func (a *Actor) holdMods(mods []uint16) error {
	for i, m := range mods {
		if err := a.In.KeyDown(m); err != nil {
			a.releaseMods(mods[:i])
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
	if err := a.MoveTo(p); err != nil {
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

// Drag presses at from, moves in steps over dur, pauses holdMs, and releases at to.
func (a *Actor) Drag(from, to geom.Point, btn platform.MouseButton, dur, holdMs time.Duration) error {
	if btn == "" {
		btn = platform.ButtonLeft
	}
	if dur <= 0 {
		dur = 250 * time.Millisecond
	}
	if holdMs <= 0 {
		holdMs = 80 * time.Millisecond
	}
	if err := a.MoveTo(from); err != nil {
		return err
	}
	a.sleep(30 * time.Millisecond)
	if err := a.In.MouseDown(btn); err != nil {
		return err
	}
	released := false
	defer func() {
		if !released {
			a.In.MouseUp(btn) // best-effort: don't leave the button stuck down on an error path
		}
	}()
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
	a.sleep(holdMs) // let drop targets highlight before releasing
	released = true
	return a.In.MouseUp(btn)
}

// Waypoint is an intermediate point in a multi-leg drag with an optional pause.
type Waypoint struct {
	P      geom.Point
	WaitMs int
}

// Press moves to p and presses btn without releasing (for cross-call held drags).
func (a *Actor) Press(p geom.Point, btn platform.MouseButton) error {
	if btn == "" {
		btn = platform.ButtonLeft
	}
	if err := a.MoveTo(p); err != nil {
		return err
	}
	a.sleep(30 * time.Millisecond)
	return a.In.MouseDown(btn)
}

// Release optionally moves to p and releases btn.
func (a *Actor) Release(p *geom.Point, btn platform.MouseButton) error {
	if btn == "" {
		btn = platform.ButtonLeft
	}
	if p != nil {
		if err := a.MoveTo(*p); err != nil {
			return err
		}
	}
	a.sleep(80 * time.Millisecond)
	return a.In.MouseUp(btn)
}

// DragVia presses at from, interpolates through waypoints, and releases at to.
func (a *Actor) DragVia(from geom.Point, via []Waypoint, to geom.Point, btn platform.MouseButton, dur, holdMs time.Duration) error {
	if btn == "" {
		btn = platform.ButtonLeft
	}
	if dur <= 0 {
		dur = 250 * time.Millisecond
	}
	if holdMs <= 0 {
		holdMs = 80 * time.Millisecond
	}
	// Count total legs: from→wp1, wp1→wp2, ..., wpN→to
	legs := len(via) + 1
	legDur := dur / time.Duration(legs)

	if err := a.MoveTo(from); err != nil {
		return err
	}
	a.sleep(30 * time.Millisecond)
	if err := a.In.MouseDown(btn); err != nil {
		return err
	}
	released := false
	defer func() {
		if !released {
			a.In.MouseUp(btn)
		}
	}()
	a.sleep(80 * time.Millisecond)

	prev := from
	for _, wp := range via {
		if err := a.interpolateMove(prev, wp.P, legDur); err != nil {
			return err
		}
		if wp.WaitMs > 0 {
			a.sleep(time.Duration(wp.WaitMs) * time.Millisecond)
		}
		prev = wp.P
	}
	if err := a.interpolateMove(prev, to, legDur); err != nil {
		return err
	}
	a.sleep(holdMs)
	released = true
	return a.In.MouseUp(btn)
}

// interpolateMove moves from src to dst in linear steps over dur.
func (a *Actor) interpolateMove(src, dst geom.Point, dur time.Duration) error {
	steps := max(8, int(dur/(12*time.Millisecond)))
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		p := geom.Point{
			X: src.X + int(float64(dst.X-src.X)*t+0.5),
			Y: src.Y + int(float64(dst.Y-src.Y)*t+0.5),
		}
		if i == steps {
			p = dst
		}
		if err := a.In.MouseMove(p); err != nil {
			return err
		}
		if i < steps {
			a.sleep(dur / time.Duration(steps))
		}
	}
	return nil
}

// Scroll optionally moves to p, then scrolls by ticks (dy>0 down, dx>0 right).
func (a *Actor) Scroll(p *geom.Point, dx, dy int) error {
	if p != nil {
		if err := a.MoveTo(*p); err != nil {
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
	defer a.releaseMods(c.Mods) // run on every path after mods are held, not just the happy one
	if err := a.In.KeyDown(c.Key); err != nil {
		return err
	}
	if hold <= 0 {
		hold = 10 * time.Millisecond
	}
	a.sleep(hold)
	return a.In.KeyUp(c.Key)
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

// paste sets the clipboard, sends Ctrl+V, and restores the previous clipboard text.
// If reading the previous text failed, it never restores (that would clobber the
// user's clipboard with ""; better to leave the pasted text). The restore runs in a
// defer so it also happens when the Ctrl+V chord itself fails.
func (a *Actor) paste(text string) error {
	old, err := a.Clip.GetText()
	haveOld := err == nil
	if err := a.Clip.SetText(text); err != nil {
		return err
	}
	if haveOld {
		defer func() {
			a.sleep(300 * time.Millisecond) // most apps read the clipboard synchronously on Ctrl+V; give slow ones time
			a.Clip.SetText(old)
		}()
	}
	return a.Chord(input.Chord{Mods: []uint16{input.VK_CONTROL}, Key: input.VK_V}, 0)
}
