//go:build windows

package guard

import (
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/uithread"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

type Runner struct {
	m      *Machine
	remove func()
	stop   chan struct{}
	t      *uithread.Thread
}

// Start installs the low-level hooks on t and starts the idle ticker.
func Start(m *Machine, t *uithread.Thread) (*Runner, error) {
	r := &Runner{m: m, stop: make(chan struct{}), t: t}
	var err error
	t.DoSync(func() {
		r.remove, err = win.InstallLLHooks(func(h win.HookEvent) {
			ev := Event{VK: h.VK, Pos: geom.Point{X: int(h.X), Y: int(h.Y)}, Injected: h.Injected, At: time.Now()}
			switch h.Kind {
			case win.HookKeyDown:
				ev.Kind = KeyDown
			case win.HookKeyUp:
				ev.Kind = KeyUp
			case win.HookMouseMove:
				ev.Kind = MouseMove
			case win.HookMouseDown, win.HookWheel:
				ev.Kind = MouseDown
			}
			m.HandleEvent(ev)
		})
	})
	if err != nil {
		return nil, err
	}
	go func() {
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-r.stop:
				return
			case now := <-tick.C:
				m.CheckIdle(now)
			}
		}
	}()
	return r, nil
}

func (r *Runner) Stop() {
	close(r.stop)
	if r.remove != nil {
		r.t.DoSync(r.remove)
	}
}
