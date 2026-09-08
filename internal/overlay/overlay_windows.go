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
	if err := l.surf.Update(l.hwnd, l.x, l.y, img); err != nil {
		return
	}
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

	// HUD and ripple (UI-thread-only)
	hud     *layeredWin
	hudImg  *image.RGBA
	rip     *layeredWin
	ripStop chan struct{}
}

func New(t *uithread.Thread, cfg Config) (*Overlay, error) {
	if cfg.Thick <= 0 {
		cfg.Thick = 56
	}
	if cfg.Intensity <= 0 {
		cfg.Intensity = 0.85
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
			win.ShowNoActivate(s.hwnd)
			s.visible = true
		}
	}
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
	peak := o.cfg.Intensity
	col := o.cfg.Accent
	if s == platform.OverlayPaused {
		col = pausedColor
		peak *= 0.7
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
	breath := 0.85 + 0.15*math.Sin(o.phase)
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

func (o *Overlay) hudSpecNow() (hudSpec, platform.Monitor, platform.OverlayState) {
	o.mu.Lock()
	defer o.mu.Unlock()
	paused := o.state == platform.OverlayPaused
	l1, l2 := hudText(o.cfg.Lang, o.cfg.HotkeyLabel, o.title, o.action, paused)
	return hudSpec{Title: l1, Sub: l2, Scale: o.mon.ScaleFactor, Accent: o.cfg.Accent, Paused: paused, Pulse: 0.5 + 0.5*math.Sin(o.phase*2)}, o.mon, o.state
}

func (o *Overlay) showHUD(m platform.Monitor, s platform.OverlayState) { o.refreshHUD() }

func (o *Overlay) refreshHUD() {
	spec, m, st := o.hudSpecNow()
	if st == platform.OverlayHidden || m.ID == 0 {
		return
	}
	img := renderHUD(spec)
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	x := m.Rect.X + (m.Rect.W-w)/2
	y := m.Rect.Y + int(14*m.ScaleFactor)
	if o.hud == nil || o.hud.w != w || o.hud.h != h {
		if o.hud != nil {
			o.hud.destroy()
		}
		lw, err := newLayeredWin(x, y, w, h)
		if err != nil {
			return
		}
		o.hud = lw
	}
	o.hud.x, o.hud.y = x, y
	o.hudImg = img
	o.hud.update(img)
}

// pulseHUD is called every frame; re-rendering text at 30 fps is ~1 ms, acceptable.
func (o *Overlay) pulseHUD() {
	if o.hud != nil && o.hud.visible {
		o.refreshHUD()
	}
}

func (o *Overlay) hideHUD() {
	if o.hud != nil {
		o.hud.hide()
	}
	if o.rip != nil {
		o.rip.hide()
	}
}

func (o *Overlay) destroyHUD() {
	if o.hud != nil {
		o.hud.destroy()
		o.hud = nil
	}
	if o.rip != nil {
		o.rip.destroy()
		o.rip = nil
	}
}

func (o *Overlay) ripple(p geom.Point) {
	o.mu.Lock()
	sc, st := o.mon.ScaleFactor, o.state
	o.mu.Unlock()
	if st != platform.OverlayControlling {
		return
	}
	size := int(120 * math.Max(1, sc))
	if o.rip == nil || o.rip.w != size {
		if o.rip != nil {
			o.rip.destroy()
		}
		lw, err := newLayeredWin(p.X-size/2, p.Y-size/2, size, size)
		if err != nil {
			return
		}
		o.rip = lw
	}
	o.rip.x, o.rip.y = p.X-size/2, p.Y-size/2
	if o.ripStop != nil {
		close(o.ripStop)
	}
	stop := make(chan struct{})
	o.ripStop = stop
	start := time.Now()
	go func() {
		tick := time.NewTicker(time.Second / 60)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				t := float64(time.Since(start)) / float64(350*time.Millisecond)
				o.t.Do(func() {
					if o.rip == nil {
						return
					}
					if t >= 1 {
						o.rip.hide()
						return
					}
					o.rip.update(renderRipple(size, t, o.cfg.Accent))
				})
				if t >= 1 {
					return
				}
			}
		}
	}()
}

var _ platform.Overlay = (*Overlay)(nil)
