//go:build windows

package overlay

import (
	"image"
	"math"
	"sync"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/uithread"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

type layeredWin struct {
	hwnd    uintptr
	surf    *win.LayeredSurface
	x, y    int
	w, h    int
	visible bool
}

func newLayeredWin(x, y, w, h int) (*layeredWin, error) {
	hwnd, err := win.CreateOverlayWindow(x, y, w, h)
	if err != nil {
		return nil, err
	}
	surf, err := win.NewLayeredSurface(w, h)
	if err != nil {
		win.DestroyWindow(hwnd)
		return nil, err
	}
	return &layeredWin{hwnd: hwnd, surf: surf, x: x, y: y, w: w, h: h}, nil
}

func (l *layeredWin) update(img *image.RGBA) {
	_ = l.surf.Update(l.hwnd, l.x, l.y, img)
	if !l.visible {
		win.ShowNoActivate(l.hwnd)
		l.visible = true
	}
}

func (l *layeredWin) hide() {
	if l.visible {
		win.HideWindow(l.hwnd)
		l.visible = false
	}
}

func (l *layeredWin) destroy() {
	l.hide()
	l.surf.Close()
	win.DestroyWindow(l.hwnd)
}

// Overlay implements platform.Overlay. All window work happens on the UI thread via t.Do.
type Overlay struct {
	cfg Config
	t   *uithread.Thread

	mu     sync.Mutex // guards the fields below (read on the UI thread, written from anywhere)
	state  platform.OverlayState
	mon    platform.Monitor
	title  string
	action string

	// UI-thread-only state
	strips   [4]*layeredWin
	set      *stripSet
	anim     *time.Ticker
	stopAnim chan struct{}
	phase    float64
	hideAt   *time.Timer
}

func New(t *uithread.Thread, cfg Config) (*Overlay, error) {
	if cfg.Thick <= 0 {
		cfg.Thick = 32
	}
	return &Overlay{cfg: cfg, t: t}, nil
}

func (o *Overlay) Show(m platform.Monitor, s platform.OverlayState) {
	o.mu.Lock()
	same := o.state == s && o.mon.ID == m.ID && o.mon.Rect == m.Rect
	o.state, o.mon = s, m
	o.mu.Unlock()
	if same {
		return
	}
	o.t.Do(func() { o.rebuild(m, s) })
}

func (o *Overlay) Hide() {
	o.mu.Lock()
	o.state = platform.OverlayHidden
	o.mu.Unlock()
	o.t.Do(o.hideAll)
}

func (o *Overlay) SetTitle(title string) {
	o.mu.Lock()
	o.title = title
	o.mu.Unlock()
	o.t.Do(o.refreshHUD)
}
func (o *Overlay) SetAction(action string) {
	o.mu.Lock()
	o.action = action
	o.mu.Unlock()
	o.t.Do(o.refreshHUD)
}
func (o *Overlay) Ripple(p geom.Point) { o.t.Do(func() { o.ripple(p) }) }

func (o *Overlay) Close() {
	o.t.DoSync(func() {
		o.hideAll()
		for i, s := range o.strips {
			if s != nil {
				s.destroy()
				o.strips[i] = nil
			}
		}
		o.destroyHUD()
	})
}

// HideForCapture hides the overlay windows without changing state (fallback capture exclusion).
func (o *Overlay) HideForCapture() {
	for _, s := range o.strips {
		if s != nil {
			s.hide()
		}
	}
	o.hideHUD()
}

// ShowAfterCapture restores the overlay windows after a capture (fallback capture exclusion).
func (o *Overlay) ShowAfterCapture() {
	for _, s := range o.strips {
		if s != nil && !s.visible {
			// Only re-show if we were showing before
			if o.set != nil {
				s.update(o.set.images()[0]) // just re-show; next frame() will paint correctly
			}
		}
	}
	// Re-show strips with their current images
	o.frame()
}

// ---- UI thread ----

func (o *Overlay) rebuild(m platform.Monitor, s platform.OverlayState) {
	o.hideAll()
	if s == platform.OverlayHidden {
		return
	}
	th := int(math.Round(float64(o.cfg.Thick) * m.ScaleFactor))
	r := m.Rect
	rects := [4]geom.Rect{
		{X: r.X, Y: r.Y, W: r.W, H: th},
		{X: r.X, Y: r.Bottom() - th, W: r.W, H: th},
		{X: r.X, Y: r.Y + th, W: th, H: r.H - 2*th},
		{X: r.Right() - th, Y: r.Y + th, W: th, H: r.H - 2*th},
	}
	for i, rr := range rects {
		if o.strips[i] == nil || o.strips[i].w != rr.W || o.strips[i].h != rr.H {
			if o.strips[i] != nil {
				o.strips[i].destroy()
			}
			lw, err := newLayeredWin(rr.X, rr.Y, rr.W, rr.H)
			if err != nil {
				return
			}
			o.strips[i] = lw
		}
		o.strips[i].x, o.strips[i].y = rr.X, rr.Y
	}
	col, peak := o.cfg.Accent, 0.6
	if s == platform.OverlayPaused {
		col, peak = pausedColor, 0.45
	}
	o.set = renderStrips(borderSpec{W: r.W, H: r.H, Thick: th, Color: col, Peak: peak})
	o.frame()
	o.showHUD(m, s)
	o.startAnim()
	if s == platform.OverlayPaused {
		o.hideAt = time.AfterFunc(2500*time.Millisecond, func() { o.t.Do(o.hideAll) })
	}
}

func (o *Overlay) frame() {
	if o.set == nil {
		return
	}
	breath := 0.72 + 0.28*math.Sin(o.phase)
	o.set.apply(breath)
	for i, img := range o.set.images() {
		if o.strips[i] != nil {
			o.strips[i].update(img)
		}
	}
	o.pulseHUD()
}

func (o *Overlay) startAnim() {
	o.stopAnimLoop()
	o.anim = time.NewTicker(time.Second / 30)
	o.stopAnim = make(chan struct{})
	stop, tick := o.stopAnim, o.anim
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				o.t.Do(func() {
					o.phase += 2 * math.Pi / (2.4 * 30)
					o.frame()
				})
			}
		}
	}()
}

func (o *Overlay) stopAnimLoop() {
	if o.anim != nil {
		o.anim.Stop()
		close(o.stopAnim)
		o.anim = nil
	}
}

func (o *Overlay) hideAll() {
	o.stopAnimLoop()
	if o.hideAt != nil {
		o.hideAt.Stop()
		o.hideAt = nil
	}
	for _, s := range o.strips {
		if s != nil {
			s.hide()
		}
	}
	o.hideHUD()
}

// HUD and ripple are implemented in Task 13; keep these stubs compiling until then.
func (o *Overlay) showHUD(platform.Monitor, platform.OverlayState) {}
func (o *Overlay) refreshHUD()                                     {}
func (o *Overlay) pulseHUD()                                       {}
func (o *Overlay) hideHUD()                                        {}
func (o *Overlay) destroyHUD()                                     {}
func (o *Overlay) ripple(geom.Point)                               {}

var _ platform.Overlay = (*Overlay)(nil)
