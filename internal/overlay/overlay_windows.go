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
	phase    float64 // border breathing phase (advances 2pi per 2.4s)
	hideAt   *time.Timer

	// HUD animation state (UI-thread-only)
	hud       *layeredWin
	hudImg    *image.RGBA
	hudAnimK  hudAnimKind // current HUD transition
	hudAnimT0 time.Time   // start of slide-in or fade-out
	hudRestY  int         // resting Y position
	hudRestX  int         // resting X position (centred)
	hudH      int         // current HUD image height (for slide offset)
	animStart time.Time   // when the animation loop started (for spark rotation)

	// Caption crossfade state (UI-thread-only)
	prevTitle       string
	prevSub         string
	cfadeT0         time.Time // crossfade start; zero = no crossfade
	cfadeTitle      string    // old title during crossfade
	cfadeSub        string    // old sub during crossfade
	cfadePrevPaused bool      // old paused state for palette blend

	// Ripple (UI-thread-only)
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
	prevState := o.state
	prevMon := o.mon
	o.state, o.mon = s, m
	o.mu.Unlock()
	if same {
		return
	}
	// If only the state changed (Controlling <-> Paused) on the same monitor
	// and the HUD is already visible, crossfade instead of rebuilding.
	sameMonitor := prevMon.ID == m.ID && prevMon.Rect == m.Rect
	stateTransition := sameMonitor &&
		prevState != platform.OverlayHidden && s != platform.OverlayHidden &&
		prevState != s
	if stateTransition {
		o.t.Do(func() { o.transitionState(m, s, prevState) })
		return
	}
	o.t.Do(func() { o.rebuild(m, s) })
}

func (o *Overlay) Hide() {
	o.mu.Lock()
	o.state = platform.OverlayHidden
	o.mu.Unlock()
	o.t.Do(o.startFadeOut)
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
		o.hideAllImmediate()
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
	o.hideHUDWin()
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
	o.hideAllImmediate()
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
	o.animStart = time.Now()
	o.frame()
	o.showHUD(m, s)
	o.startAnim()
	if s == platform.OverlayPaused {
		o.hideAt = time.AfterFunc(2500*time.Millisecond, func() {
			o.t.Do(o.startFadeOut)
		})
	}
}

// transitionState crossfades HUD text/palette when state changes on the same monitor.
// Rebuilds border strips but keeps the HUD window (no slide-in).
func (o *Overlay) transitionState(m platform.Monitor, s, prevState platform.OverlayState) {
	// Rebuild border strips with new color/peak.
	th := int(math.Round(float64(o.cfg.Thick) * m.ScaleFactor))
	peak := o.cfg.Intensity
	col := o.cfg.Accent
	if s == platform.OverlayPaused {
		col = pausedColor
		peak *= 0.7
	}
	o.set = renderStrips(borderSpec{W: m.Rect.W, H: m.Rect.H, Thick: th, Color: col, Peak: peak})

	// Snapshot old text for crossfade.
	o.mu.Lock()
	prevPaused := prevState == platform.OverlayPaused
	oldTitle, oldSub := hudText(o.cfg.Lang, o.cfg.HotkeyLabel, o.title, o.action, prevPaused)
	o.mu.Unlock()
	o.cfadeTitle = oldTitle
	o.cfadeSub = oldSub
	o.cfadePrevPaused = prevPaused
	o.cfadeT0 = time.Now()

	// Cancel any previous hide timer.
	if o.hideAt != nil {
		o.hideAt.Stop()
		o.hideAt = nil
	}
	// Make sure animation loop is running.
	if o.anim == nil {
		o.startAnim()
	}
	// Schedule fade-out if entering paused.
	if s == platform.OverlayPaused {
		o.hideAt = time.AfterFunc(2500*time.Millisecond, func() {
			o.t.Do(o.startFadeOut)
		})
	}
}

func (o *Overlay) frame() {
	if o.set == nil {
		return
	}
	breath := 0.85 + 0.15*math.Sin(o.phase)
	o.mu.Lock()
	paused := o.state == platform.OverlayPaused
	o.mu.Unlock()
	shimmerPos := -1.0
	if !o.cfg.ShimmerOff && !paused {
		// Shimmer travels around the frame once per 6 seconds.
		shimmerPos = math.Mod(o.phase/(2*math.Pi)*2.4/6.0, 1.0)
	}
	o.set.apply(breath, shimmerPos)
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

// hideAllImmediate stops everything and hides all windows instantly (no fade).
func (o *Overlay) hideAllImmediate() {
	o.stopAnimLoop()
	if o.hideAt != nil {
		o.hideAt.Stop()
		o.hideAt = nil
	}
	o.hudAnimK = hudAnimNone
	o.cfadeT0 = time.Time{}
	for _, s := range o.strips {
		if s != nil {
			s.hide()
		}
	}
	o.hideHUDWin()
}

// startFadeOut begins the 180ms fade-out. If a slide-in is running, it cancels.
func (o *Overlay) startFadeOut() {
	if o.hud == nil || !o.hud.visible {
		// Already hidden, just clean up.
		o.hideAllImmediate()
		return
	}
	// Cancel any running slide-in.
	o.hudAnimK = hudAnimFadeOut
	o.hudAnimT0 = time.Now()
	// Keep the animation loop running so pulseHUD drives the fade.
	// The border strips hide immediately.
	if o.hideAt != nil {
		o.hideAt.Stop()
		o.hideAt = nil
	}
	for _, s := range o.strips {
		if s != nil {
			s.hide()
		}
	}
}

func (o *Overlay) hudSpecNow() (hudSpec, platform.Monitor, platform.OverlayState) {
	o.mu.Lock()
	defer o.mu.Unlock()
	paused := o.state == platform.OverlayPaused
	l1, l2 := hudText(o.cfg.Lang, o.cfg.HotkeyLabel, o.title, o.action, paused)
	// Spark rotation: one revolution per 8 seconds.
	elapsed := time.Since(o.animStart).Seconds()
	sparkPhase := 2 * math.Pi * elapsed / 8.0
	// Spark breathing: synced with border phase.
	sparkScale := sparkBreathScale(o.phase)
	return hudSpec{
		Title:       l1,
		Sub:         l2,
		HotkeyLabel: o.cfg.HotkeyLabel,
		Lang:        o.cfg.Lang,
		Scale:       o.mon.ScaleFactor,
		Accent:      o.cfg.Accent,
		Paused:      paused,
		SparkPhase:  sparkPhase,
		SparkScale:  sparkScale,
	}, o.mon, o.state
}

func (o *Overlay) showHUD(m platform.Monitor, _ platform.OverlayState) {
	// Begin slide-in animation.
	o.hudAnimK = hudAnimSlideIn
	o.hudAnimT0 = time.Now()
	o.cfadeT0 = time.Time{}
	o.prevTitle = ""
	o.prevSub = ""
	o.refreshHUD()
}

func (o *Overlay) refreshHUD() {
	spec, m, st := o.hudSpecNow()
	if st == platform.OverlayHidden || m.ID == 0 {
		return
	}

	// Detect text change for crossfade.
	if o.prevTitle != "" || o.prevSub != "" {
		if spec.Title != o.prevTitle || spec.Sub != o.prevSub {
			// Only start a new crossfade if one is not already running from transitionState.
			if o.cfadeT0.IsZero() {
				o.cfadeTitle = o.prevTitle
				o.cfadeSub = o.prevSub
				o.cfadePrevPaused = spec.Paused // same palette for text-only crossfade
				o.cfadeT0 = time.Now()
			}
		}
	}
	o.prevTitle = spec.Title
	o.prevSub = spec.Sub

	// Compute HUD animation alpha and Y offset.
	now := time.Now()
	var yOff float64
	alpha := 1.0
	if o.hudAnimK != hudAnimNone {
		elapsed := float64(now.Sub(o.hudAnimT0).Milliseconds())
		// Use a dummy height for slide-in calculation; renderHUD will give us the real one.
		yOff, alpha = hudAnimState(o.hudAnimK, elapsed, float64(o.hudH))
		switch o.hudAnimK {
		case hudAnimSlideIn:
			if elapsed >= slideInMs {
				o.hudAnimK = hudAnimNone
				yOff, alpha = 0, 1
			}
		case hudAnimFadeOut:
			if elapsed >= fadeOutMs {
				o.hudAnimK = hudAnimNone
				o.hideAllImmediate()
				return
			}
		}
	}

	// Fill crossfade fields into the spec.
	if !o.cfadeT0.IsZero() {
		cfElapsed := float64(now.Sub(o.cfadeT0).Milliseconds())
		_, newA := crossFadeAlphas(cfElapsed)
		if newA >= 1 {
			o.cfadeT0 = time.Time{}
		} else {
			spec.PrevTitle = o.cfadeTitle
			spec.PrevSub = o.cfadeSub
			spec.PrevPaused = o.cfadePrevPaused
			spec.CrossT = newA
		}
	}

	spec.Alpha = alpha
	img := renderHUD(spec)
	// Update hudH for slide-in calculation on next frame.
	o.hudH = img.Bounds().Dy()
	o.updateHUDWindow(img, m, yOff)
}

func (o *Overlay) updateHUDWindow(img *image.RGBA, m platform.Monitor, yOff float64) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	x := m.Rect.X + (m.Rect.W-w)/2
	restY := m.Rect.Y + int(14*m.ScaleFactor)
	y := restY + int(yOff)

	o.hudRestX = x
	o.hudRestY = restY
	o.hudH = h

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

// hudNeedsRender returns true if the HUD should be re-rendered this frame.
func (o *Overlay) hudNeedsRender() bool {
	// Always render during animations.
	if o.hudAnimK != hudAnimNone {
		return true
	}
	// Always render during crossfade.
	if !o.cfadeT0.IsZero() {
		return true
	}
	// While controlling: spark rotates, so we need to re-render.
	o.mu.Lock()
	st := o.state
	o.mu.Unlock()
	if st == platform.OverlayControlling {
		return true
	}
	// Paused and static: no re-render needed.
	return false
}

// pulseHUD is called every frame.
func (o *Overlay) pulseHUD() {
	if o.hud != nil && o.hud.visible && o.hudNeedsRender() {
		o.refreshHUD()
	}
}

func (o *Overlay) hideHUDWin() {
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
