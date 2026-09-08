//go:build windows

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/actions"
	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/input"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

func runInput(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cu input move X Y | click X Y [left|right|middle] | drag X1 Y1 X2 Y2 | type TEXT | key CHORD")
	}
	if err := win.SetPerMonitorDPIAwareV2(); err != nil {
		return err
	}
	a := &actions.Actor{In: input.New(), Clip: input.NewClipboard(), PasteThreshold: 200}
	atoi := func(s string) int { n, _ := strconv.Atoi(s); return n }
	switch args[0] {
	case "move":
		return a.In.MouseMove(geom.Point{X: atoi(args[1]), Y: atoi(args[2])})
	case "click":
		btn := platform.ButtonLeft
		if len(args) > 3 {
			btn = platform.MouseButton(args[3])
		}
		return a.Click(geom.Point{X: atoi(args[1]), Y: atoi(args[2])}, btn, 1, nil)
	case "drag":
		return a.Drag(geom.Point{X: atoi(args[1]), Y: atoi(args[2])}, geom.Point{X: atoi(args[3]), Y: atoi(args[4])}, platform.ButtonLeft, 300*time.Millisecond)
	case "type":
		time.Sleep(2 * time.Second) // time to focus a text field
		return a.Type(strings.Join(args[1:], " "), "auto", 0)
	case "key":
		c, err := input.ParseChord(args[1])
		if err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
		return a.Chord(c, 0)
	}
	return fmt.Errorf("unknown input subcommand %q", args[0])
}
