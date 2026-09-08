//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/racass-pixel/claude-computer-use/internal/screen"
	"github.com/racass-pixel/claude-computer-use/internal/win"
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
	return nil
}
