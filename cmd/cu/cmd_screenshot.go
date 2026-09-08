//go:build windows

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/screen"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

func runScreenshot(args []string) error {
	fs := flag.NewFlagSet("screenshot", flag.ContinueOnError)
	mon := fs.Int("m", 1, "monitor id (0 = whole virtual screen)")
	out := fs.String("o", "screenshot.png", "output file (.png or .jpg)")
	longEdge := fs.Int("long-edge", 1366, "scale so the long edge is at most this many px")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := win.SetPerMonitorDPIAwareV2(); err != nil {
		return err
	}
	s := screen.New()
	mons, err := s.Monitors()
	if err != nil {
		return err
	}
	var rect geom.Rect
	if *mon == 0 {
		rect = screen.VirtualScreen(mons)
	} else {
		m, ok := screen.MonitorByID(mons, *mon)
		if !ok {
			return fmt.Errorf("no monitor %d", *mon)
		}
		rect = m.Rect
	}
	format := "png"
	if len(*out) > 4 && (*out)[len(*out)-4:] == ".jpg" {
		format = "jpeg"
	}
	shot, err := screen.Grab(s, rect, screen.AutoScale(rect, *longEdge), format, 85)
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, shot.Data, 0o644); err != nil {
		return err
	}
	fmt.Printf("%s: %dx%d (scale %.3f) capture=%dms scale=%dms encode=%dms bytes=%d\n",
		*out, shot.Size.W, shot.Size.H, shot.Scale, shot.CaptureMs, shot.ScaleMs, shot.EncodeMs, len(shot.Data))
	return nil
}
