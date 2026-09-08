// Package fake provides in-memory platform implementations for tests.
package fake

import (
	"fmt"
	"image"
	"image/color"
	"regexp"
	"sync"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

// Input records every call as a string like "move 10,20", "down left", "key_down 17", "type hello".
type Input struct {
	mu     sync.Mutex
	Calls  []string
	Cursor geom.Point
}

func (f *Input) record(s string) { f.mu.Lock(); f.Calls = append(f.Calls, s); f.mu.Unlock() }
func (f *Input) MouseMove(p geom.Point) error {
	f.Cursor = p
	f.record(fmt.Sprintf("move %d,%d", p.X, p.Y))
	return nil
}
func (f *Input) MouseDown(b platform.MouseButton) error { f.record("down " + string(b)); return nil }
func (f *Input) MouseUp(b platform.MouseButton) error   { f.record("up " + string(b)); return nil }
func (f *Input) Scroll(dx, dy int) error                { f.record(fmt.Sprintf("scroll %d,%d", dx, dy)); return nil }
func (f *Input) KeyDown(vk uint16) error                { f.record(fmt.Sprintf("key_down %d", vk)); return nil }
func (f *Input) KeyUp(vk uint16) error                  { f.record(fmt.Sprintf("key_up %d", vk)); return nil }
func (f *Input) TypeUnicode(s string) error             { f.record("type " + s); return nil }

// Screen serves a fixed monitor list and a solid-color image for any capture.
type Screen struct {
	Mons   []platform.Monitor
	Cursor geom.Point
	Fill   color.RGBA
	Frames [][]byte // optional: successive Capture calls return these raw RGBA pix if set
	n      int
}

func (s *Screen) Monitors() ([]platform.Monitor, error) { return s.Mons, nil }
func (s *Screen) CursorPos() (geom.Point, error)        { return s.Cursor, nil }
func (s *Screen) Capture(r geom.Rect) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, r.W, r.H))
	if s.n < len(s.Frames) {
		copy(img.Pix, s.Frames[s.n])
		s.n++
		return img, nil
	}
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = s.Fill.R, s.Fill.G, s.Fill.B, 255
	}
	return img, nil
}

type Clipboard struct{ Text string }

func (c *Clipboard) GetText() (string, error) { return c.Text, nil }
func (c *Clipboard) SetText(s string) error   { c.Text = s; return nil }

type Windows struct {
	Wins  []platform.WindowInfo
	Calls []string
}

func (w *Windows) List() ([]platform.WindowInfo, error) { return w.Wins, nil }
func (w *Windows) Foreground() (platform.WindowInfo, error) {
	for _, x := range w.Wins {
		if x.Foreground {
			return x, nil
		}
	}
	return platform.WindowInfo{}, fmt.Errorf("no foreground window")
}
func (w *Windows) Focus(id uintptr) error {
	for i := range w.Wins {
		w.Wins[i].Foreground = w.Wins[i].ID == id
	}
	w.Calls = append(w.Calls, fmt.Sprintf("focus %d", id))
	return nil
}
func (w *Windows) SetState(id uintptr, s platform.WindowState) error {
	w.Calls = append(w.Calls, fmt.Sprintf("state %d %s", id, s))
	return nil
}
func (w *Windows) Close(id uintptr) error {
	w.Calls = append(w.Calls, fmt.Sprintf("close %d", id))
	return nil
}
func (w *Windows) Move(id uintptr, r geom.Rect) error {
	w.Calls = append(w.Calls, fmt.Sprintf("move %d %v", id, r))
	return nil
}

type Accessibility struct{ Elems []platform.Element }

func (a *Accessibility) Find(q platform.FindQuery) ([]platform.Element, error) {
	var re *regexp.Regexp
	if q.Name != "" {
		re = regexp.MustCompile("(?i)" + q.Name)
	}
	var out []platform.Element
	for _, e := range a.Elems {
		if re != nil && !re.MatchString(e.Name) {
			continue
		}
		if q.Role != "" && q.Role != e.Role {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}
func (a *Accessibility) Rect(ref any) (geom.Rect, error) { return ref.(geom.Rect), nil }
func (a *Accessibility) Release([]any)                   {}

// Overlay records calls.
type Overlay struct{ Calls []string }

func (o *Overlay) Show(m platform.Monitor, s platform.OverlayState) {
	o.Calls = append(o.Calls, fmt.Sprintf("show %d %d", m.ID, s))
}
func (o *Overlay) Hide()              { o.Calls = append(o.Calls, "hide") }
func (o *Overlay) SetTitle(t string)  { o.Calls = append(o.Calls, "title "+t) }
func (o *Overlay) SetAction(a string) { o.Calls = append(o.Calls, "action "+a) }
func (o *Overlay) Ripple(p geom.Point) {
	o.Calls = append(o.Calls, fmt.Sprintf("ripple %d,%d", p.X, p.Y))
}
func (o *Overlay) Close() { o.Calls = append(o.Calls, "close") }

var (
	_ platform.Input         = (*Input)(nil)
	_ platform.Screen        = (*Screen)(nil)
	_ platform.Clipboard     = (*Clipboard)(nil)
	_ platform.Windows       = (*Windows)(nil)
	_ platform.Accessibility = (*Accessibility)(nil)
	_ platform.Overlay       = (*Overlay)(nil)
)
