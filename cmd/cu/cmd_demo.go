//go:build windows

package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/config"
	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/overlay"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/screen"
	"github.com/racass-pixel/claude-computer-use/internal/uithread"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

func runDemo(args []string) error {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	secs := fs.Int("seconds", 5, "how long to show the overlay")
	out := fs.String("o", "", "also save a screenshot (to prove the overlay is excluded)")
	pausedOut := fs.String("paused-o", "", "save a screenshot during the paused state")
	mon := fs.Int("m", 1, "monitor id")
	showInCapture := fs.Bool("show-in-capture", false, "skip SetWindowDisplayAffinity so the overlay appears in screenshots")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showInCapture {
		win.OverlayVisibleInCapture = true
	}
	if err := win.SetPerMonitorDPIAwareV2(); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	r, g, b, err := cfg.AccentRGB()
	if err != nil {
		return err
	}
	ui, err := uithread.New()
	if err != nil {
		return err
	}
	defer ui.Close()
	lang := cfg.Lang
	if lang == "auto" {
		lang = win.UserUILanguage()
	}
	ov, err := overlay.New(ui, overlay.Config{Accent: color.RGBA{R: r, G: g, B: b, A: 255}, Lang: lang, HotkeyLabel: "Esc Esc", Thick: cfg.BorderThickness})
	if err != nil {
		return err
	}
	defer ov.Close()
	s := screen.New()
	mons, err := s.Monitors()
	if err != nil {
		return err
	}
	m, ok := screen.MonitorByID(mons, *mon)
	if !ok {
		return fmt.Errorf("no monitor %d", *mon)
	}
	ov.SetTitle("Demo: Claude Computer Use")
	ov.Show(m, platform.OverlayControlling)
	for i := 0; i < *secs*2; i++ {
		ov.SetAction(fmt.Sprintf("click %d,%d", 100+i*40, 200))
		ov.Ripple(geom.Point{X: m.Rect.X + 200 + i*60, Y: m.Rect.Y + 300})
		time.Sleep(500 * time.Millisecond)
		if *out != "" && i == *secs {
			shot, err := screen.Grab(s, m.Rect, screen.AutoScale(m.Rect, 1366), "png", 85)
			if err != nil {
				return err
			}
			if err := os.WriteFile(*out, shot.Data, 0o644); err != nil {
				return err
			}
			fmt.Printf("saved %s (capture exclusion supported: %v)\n", *out, win.CaptureExclusionSupported)
		}
	}
	ov.Show(m, platform.OverlayPaused)
	if *pausedOut != "" {
		time.Sleep(500 * time.Millisecond)
		shot, err := screen.Grab(s, m.Rect, screen.AutoScale(m.Rect, 1366), "png", 85)
		if err != nil {
			return err
		}
		if err := os.WriteFile(*pausedOut, shot.Data, 0o644); err != nil {
			return err
		}
		fmt.Printf("saved paused %s\n", *pausedOut)
	}
	time.Sleep(3 * time.Second)
	return nil
}
