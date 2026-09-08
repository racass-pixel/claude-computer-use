//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/screen"
	"github.com/racass-pixel/claude-computer-use/internal/uia"
	"github.com/racass-pixel/claude-computer-use/internal/win"
	"github.com/racass-pixel/claude-computer-use/internal/window"
)

func runDoctor(args []string) error {
	fmt.Printf("cu %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
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
