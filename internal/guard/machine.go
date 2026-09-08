package guard

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
)

type State int

const (
	Idle State = iota
	Controlling
	Paused
)

func (s State) String() string {
	switch s {
	case Controlling:
		return "controlling"
	case Paused:
		return "paused"
	}
	return "idle"
}

type Reason string

const (
	ReasonAction        Reason = "action"
	ReasonHotkey        Reason = "hotkey"
	ReasonPhysicalKey   Reason = "physical_key"
	ReasonPhysicalMouse Reason = "physical_mouse"
	ReasonIdle          Reason = "idle"
	ReasonRelease       Reason = "release"
	ReasonResume        Reason = "resume"
	ReasonPrompt        Reason = "prompt"
)

type Transition struct {
	From, To State
	Reason   Reason
	At       time.Time
}

type Config struct {
	AutoPause        bool
	MouseThresholdPx int
	MouseWindow      time.Duration // 300ms
	IdleRelease      time.Duration
	Hotkey           Hotkey
	TapWindow        time.Duration // 400ms
}

type Status struct {
	State  string
	Hotkey string
	IdleMs int64
}

type Machine struct {
	mu         sync.Mutex
	cfg        Config
	state      State
	lastAction time.Time
	taps       *TapMatcher
	onChange   func(Transition)

	mouseAcc      float64
	mouseWinStart time.Time
	lastMouse     geom.Point
	haveMouse     bool

	resumeCh chan struct{} // closed when leaving Paused
}

func New(cfg Config, onChange func(Transition)) *Machine {
	if cfg.MouseWindow <= 0 {
		cfg.MouseWindow = 300 * time.Millisecond
	}
	if cfg.TapWindow <= 0 {
		cfg.TapWindow = 400 * time.Millisecond
	}
	return &Machine{cfg: cfg, taps: NewTapMatcher(cfg.Hotkey, cfg.TapWindow), onChange: onChange, resumeCh: make(chan struct{})}
}

// set changes state under the lock and reports the transition after unlocking.
func (m *Machine) set(to State, r Reason, now time.Time) {
	from := m.state
	if from == to {
		return
	}
	m.state = to
	if from == Paused {
		close(m.resumeCh)
	}
	if to == Paused {
		m.resumeCh = make(chan struct{})
	}
	if to == Controlling {
		m.lastAction = now
		m.mouseAcc, m.haveMouse = 0, false
	}
	if m.onChange != nil {
		tr := Transition{From: from, To: to, Reason: r, At: now}
		m.mu.Unlock()
		m.onChange(tr)
		m.mu.Lock()
	}
}

func (m *Machine) State() State   { m.mu.Lock(); defer m.mu.Unlock(); return m.state }
func (m *Machine) IsPaused() bool { return m.State() == Paused }

func (m *Machine) Acquire(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == Idle {
		m.set(Controlling, ReasonAction, now)
	}
}

func (m *Machine) Touch(now time.Time) {
	m.mu.Lock()
	m.lastAction = now
	m.mu.Unlock()
}

func (m *Machine) Release(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.set(Idle, ReasonRelease, now)
}

func (m *Machine) Pause(now time.Time, r Reason) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == Controlling {
		m.set(Paused, r, now)
	}
}

func (m *Machine) Resume(now time.Time, r Reason) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == Paused {
		m.set(Controlling, r, now)
	}
}

func (m *Machine) CheckIdle(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == Controlling && m.cfg.IdleRelease > 0 && now.Sub(m.lastAction) > m.cfg.IdleRelease {
		m.set(Idle, ReasonIdle, now)
	}
}

// HandleEvent applies the take-over rules (spec §7). Injected events only refresh the mouse baseline.
func (m *Machine) HandleEvent(ev Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ev.Injected {
		if ev.Kind == MouseMove {
			m.lastMouse, m.haveMouse = ev.Pos, true
		}
		return
	}
	if ev.Kind == KeyDown || ev.Kind == KeyUp {
		if m.taps.Feed(ev) {
			switch m.state {
			case Controlling:
				m.set(Paused, ReasonHotkey, ev.At)
			case Paused:
				m.set(Controlling, ReasonHotkey, ev.At)
			}
			return
		}
	}
	if m.state != Controlling || !m.cfg.AutoPause {
		return
	}
	switch ev.Kind {
	case KeyDown:
		if m.taps.Involves(ev.VK) {
			return // reserved for the gesture
		}
		m.set(Paused, ReasonPhysicalKey, ev.At)
	case MouseDown:
		m.set(Paused, ReasonPhysicalMouse, ev.At)
	case MouseMove:
		if !m.haveMouse || ev.At.Sub(m.mouseWinStart) > m.cfg.MouseWindow {
			m.mouseAcc, m.mouseWinStart = 0, ev.At
			if !m.haveMouse {
				m.lastMouse, m.haveMouse = ev.Pos, true
				return
			}
		}
		dx, dy := float64(ev.Pos.X-m.lastMouse.X), float64(ev.Pos.Y-m.lastMouse.Y)
		m.mouseAcc += math.Hypot(dx, dy)
		m.lastMouse = ev.Pos
		if m.mouseAcc > float64(m.cfg.MouseThresholdPx) {
			m.set(Paused, ReasonPhysicalMouse, ev.At)
		}
	}
}

// WaitResume blocks until the machine is no longer Paused (true) or ctx ends (false).
func (m *Machine) WaitResume(ctx context.Context) bool {
	m.mu.Lock()
	if m.state != Paused {
		m.mu.Unlock()
		return true
	}
	ch := m.resumeCh
	m.mu.Unlock()
	select {
	case <-ch:
		return true
	case <-ctx.Done():
		return false
	}
}

func (m *Machine) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	idle := int64(0)
	if !m.lastAction.IsZero() {
		idle = time.Since(m.lastAction).Milliseconds()
	}
	return Status{State: m.state.String(), Hotkey: m.cfg.Hotkey.String(), IdleMs: idle}
}
