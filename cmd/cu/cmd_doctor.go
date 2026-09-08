//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/config"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/screen"
	"github.com/racass-pixel/claude-computer-use/internal/uia"
	"github.com/racass-pixel/claude-computer-use/internal/win"
	"github.com/racass-pixel/claude-computer-use/internal/window"
)

func runDoctor(args []string) error {
	fmt.Printf("cu %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)

	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		fmt.Printf("config: FAIL %v\n", cfgErr)
	} else {
		fmt.Printf("hotkey: %s\n", cfg.Hotkey)
		fmt.Printf("auto_pause: %v\n", cfg.AutoPause)
	}

	if err := win.CheckRequiredProcs(); err != nil {
		fmt.Println("required procs: FAIL", err)
	} else {
		fmt.Println("required procs: OK")
	}

	if err := win.SetPerMonitorDPIAwareV2(); err != nil {
		fmt.Println("dpi awareness: FAIL", err)
	} else {
		fmt.Println("dpi awareness: per-monitor v2")
	}
	mons, err := screen.ListMonitors()
	if err != nil {
		return err
	}
	fmt.Printf("monitors: %d\n", len(mons))
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(mons); err != nil {
		return err
	}
	if p, err := win.GetCursorPos(); err == nil {
		fmt.Printf("cursor: %d,%d\n", p.X, p.Y)
	}

	// Capture benchmark: 3x screen.Grab of monitor 1 through the real screenshot
	// path (auto-scale to screenshot_long_edge, configured format/quality), print min ms.
	if len(mons) > 0 {
		s := &screen.Screen{}
		longEdge, format, quality := 1366, "jpeg", 90
		if cfgErr == nil {
			longEdge, format, quality = cfg.ScreenshotLongEdge, cfg.ScreenshotFormat, cfg.JPEGQuality
		}
		scale := screen.AutoScale(mons[0].Rect, longEdge)
		var minMs int64 = 1<<63 - 1
		for i := 0; i < 3; i++ {
			t0 := time.Now()
			_, grabErr := screen.Grab(s, mons[0].Rect, scale, format, quality)
			ms := time.Since(t0).Milliseconds()
			if grabErr != nil {
				fmt.Printf("capture benchmark: FAIL %v\n", grabErr)
				break
			}
			if ms < minMs {
				minMs = ms
			}
		}
		if minMs < 1<<62 {
			fmt.Printf("capture benchmark: %d ms (best of 3, monitor 1, %s, scale %.2f)\n", minMs, format, scale)
		}
	}

	// Overlay capture exclusion: probe by creating a tiny hidden overlay window.
	if hwnd, cerr := win.CreateOverlayWindow(-100, -100, 8, 8); cerr != nil {
		fmt.Printf("overlay capture exclusion: unknown (%v)\n", cerr)
	} else {
		win.DestroyWindow(hwnd)
		if win.CaptureExclusionSupported {
			fmt.Println("overlay capture exclusion: native")
		} else {
			fmt.Println("overlay capture exclusion: hidden-during-capture (fallback: overlay is hidden while capturing)")
		}
	}

	// Recipes directory.
	if cfgDir, cerr := os.UserConfigDir(); cerr == nil {
		recDir := filepath.Join(cfgDir, "claude-computer-use", "recipes")
		count := 0
		if entries, rerr := os.ReadDir(recDir); rerr == nil {
			for _, e := range entries {
				if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
					count++
				}
			}
		}
		fmt.Printf("recipes dir: %s (%d recipes)\n", recDir, count)
	}

	wl, err := window.New().List()
	if err != nil {
		return err
	}
	fmt.Printf("windows: %d\n", len(wl))
	for _, w := range wl {
		mark := " "
		if w.Foreground {
			mark = "*"
		}
		fmt.Printf(" %s %d %-20s %-9s %v %q\n", mark, w.ID, w.Process, w.State, w.Rect, w.Title)
	}
	if u, err := uia.New(); err != nil {
		fmt.Println("ui automation: FAIL", err)
	} else {
		fg := win.ForegroundWindow()
		t0 := time.Now()
		els, err := u.Find(platform.FindQuery{Window: fg, Limit: 8})
		fmt.Printf("ui automation: %d elements in foreground window in %dms (err=%v)\n", len(els), time.Since(t0).Milliseconds(), err)
		for _, e := range els {
			fmt.Printf("  %-10s %q %v\n", e.Role, e.Name, e.Rect)
		}
		u.Release(nil)
		u.Close()
	}
	return nil
}
