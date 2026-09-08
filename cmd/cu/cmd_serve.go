//go:build windows

package main

import (
	"context"
	"fmt"
	"image/color"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"time"

	"golang.org/x/sys/windows"

	"github.com/racass-pixel/claude-computer-use/internal/config"
	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/guard"
	"github.com/racass-pixel/claude-computer-use/internal/input"
	"github.com/racass-pixel/claude-computer-use/internal/ipc"
	"github.com/racass-pixel/claude-computer-use/internal/overlay"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/recipes"
	"github.com/racass-pixel/claude-computer-use/internal/screen"
	"github.com/racass-pixel/claude-computer-use/internal/server"
	"github.com/racass-pixel/claude-computer-use/internal/uia"
	"github.com/racass-pixel/claude-computer-use/internal/uithread"
	"github.com/racass-pixel/claude-computer-use/internal/win"
	"github.com/racass-pixel/claude-computer-use/internal/window"
)

func runServe(args []string) error {
	if err := win.SetPerMonitorDPIAwareV2(); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	var w io.Writer = os.Stderr
	if cfg.LogFile != "" {
		if f, ferr := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); ferr == nil {
			defer f.Close()
			w = io.MultiWriter(os.Stderr, f)
		}
	}
	logger := log.New(w, "cu: ", log.Ltime|log.Lmicroseconds)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { // exit with Claude Code even if stdio is not closed cleanly
		ppid, err := win.ParentPID()
		if err != nil {
			logger.Printf("parent watch disabled: %v", err)
			return
		}
		_ = win.WaitForProcessExit(ppid)
		logger.Printf("parent %d exited", ppid)
		cancel()
		// server.Run may not return if stdin is a console; force exit.
		time.AfterFunc(500*time.Millisecond, func() {
			logger.Printf("forcing exit after parent death")
			os.Exit(0)
		})
	}()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() { <-sig; cancel() }()

	hotkey, err := guard.ParseHotkey(cfg.Hotkey)
	if err != nil {
		return err
	}
	ui, err := uithread.New()
	if err != nil {
		return err
	}
	defer ui.Close()

	r, g, b, err := cfg.AccentRGB()
	if err != nil {
		return err
	}
	lang := cfg.Lang
	if lang == "auto" {
		lang = win.UserUILanguage()
	}
	if os.Getenv("CU_OVERLAY_VISIBLE_IN_CAPTURE") == "1" {
		win.OverlayVisibleInCapture = true
	}
	var ov platform.Overlay = platform.NopOverlay{}
	if cfg.Overlay {
		o, oerr := overlay.New(ui, overlay.Config{Accent: color.RGBA{R: r, G: g, B: b, A: 255}, Lang: lang, HotkeyLabel: hotkey.String(), Thick: cfg.BorderThickness, Intensity: cfg.BorderIntensity, ShimmerOff: !cfg.BorderShimmerEnabled()})
		if oerr != nil {
			return oerr
		}
		ov = o
	}
	defer ov.Close()

	activeMon := func() platform.Monitor {
		mons, _ := screen.ListMonitors()
		p, _ := win.GetCursorPos()
		fg := window.New()
		w, _ := fg.Foreground()
		return screen.ActiveMonitor(mons, w.Rect, geom.Point{X: int(p.X), Y: int(p.Y)})
	}

	// sessionRef is set by OnSessionReady; the guard callback uses it for idle auto-record.
	var sessionRef atomic.Pointer[server.Session]

	machine := guard.New(guard.Config{
		AutoPause: cfg.AutoPause, MouseThresholdPx: cfg.MouseThresholdPx,
		IdleRelease: time.Duration(cfg.IdleReleaseMs) * time.Millisecond, Hotkey: hotkey,
	}, func(tr guard.Transition) {
		logger.Printf("guard: %s -> %s (%s)", tr.From, tr.To, tr.Reason)
		switch tr.To {
		case guard.Paused:
			ov.Show(activeMon(), platform.OverlayPaused)
		case guard.Controlling:
			ov.Show(activeMon(), platform.OverlayControlling)
		case guard.Idle:
			ov.Hide()
			// Auto-record only on idle timeout, not on explicit control release
			// (which already calls autoRecord itself).
			if tr.Reason == guard.ReasonIdle {
				if s := sessionRef.Load(); s != nil {
					s.OnRelease()
				}
			}
		}
	})
	runner, err := guard.Start(machine, ui)
	if err != nil {
		return err
	}
	defer runner.Stop()

	go func() {
		err := ipc.Serve(ctx, windows.GetCurrentProcessId(), func(cmd string) (any, error) {
			now := time.Now()
			switch cmd {
			case "status":
				return machine.Status(), nil
			case "resume":
				machine.Resume(now, guard.ReasonPrompt)
				return machine.Status(), nil
			case "release":
				machine.Release(now)
				return machine.Status(), nil
			case "pause":
				machine.Pause(now, guard.ReasonCtl)
				return machine.Status(), nil
			}
			return nil, fmt.Errorf("unknown command %q", cmd)
		})
		if err != nil {
			logger.Printf("ipc: %v", err)
		}
	}()

	var access *uia.UIA
	if a, aerr := uia.New(); aerr != nil {
		logger.Printf("ui automation: %v (find tool will be unavailable)", aerr)
	} else {
		access = a
	}
	defer func() {
		if access != nil {
			access.Close()
		}
	}()

	var recipeStore *recipes.Store
	if cfgDir, cerr := os.UserConfigDir(); cerr == nil {
		recDir := filepath.Join(cfgDir, "claude-computer-use", "recipes")
		if rs, rerr := recipes.Open(recDir); rerr != nil {
			logger.Printf("recipes: %v (recipe tool will be unavailable)", rerr)
		} else {
			recipeStore = rs
		}
	} else {
		logger.Printf("recipes: config dir: %v (recipe tool will be unavailable)", cerr)
	}

	deps := server.Deps{
		Screen:     screen.New(),
		Input:      input.New(),
		Clip:       input.NewClipboard(),
		Wins:       window.New(),
		Access:     access,
		Controller: controller{machine},
		Overlay:    ov,
		Recipes:    recipeStore,
		Version:    version,
		OnSessionReady: func(s *server.Session) {
			sessionRef.Store(s)
			// On shutdown, release any held mouse button.
			go func() {
				<-ctx.Done()
				s.OnRelease()
			}()
		},
	}
	return server.Run(ctx, deps, cfg, logger)
}

// controller adapts guard.Machine to server.Controller.
type controller struct{ m *guard.Machine }

func (c controller) IsPaused() bool                      { return c.m.IsPaused() }
func (c controller) Acquire(now time.Time) bool          { return c.m.Acquire(now) }
func (c controller) Touch(now time.Time)                 { c.m.Touch(now) }
func (c controller) Release(now time.Time)               { c.m.Release(now) }
func (c controller) WaitResume(ctx context.Context) bool { return c.m.WaitResume(ctx) }
func (c controller) Status() server.ControllerStatus {
	st := c.m.Status()
	return server.ControllerStatus{State: st.State, Hotkey: st.Hotkey, IdleMs: st.IdleMs, UserHold: st.UserHold}
}
