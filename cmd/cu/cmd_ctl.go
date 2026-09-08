//go:build windows

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/racass-pixel/claude-computer-use/internal/ipc"
)

// runCtl never returns an error in --quiet mode: hooks must not fail Claude Code when no server runs.
func runCtl(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cu ctl status|resume|release|pause [--quiet]")
	}
	cmd := args[0]
	quiet := len(args) > 1 && args[1] == "--quiet"
	switch cmd {
	case "status", "resume", "release", "pause":
	default:
		return fmt.Errorf("unknown ctl command %q", cmd)
	}
	replies := ipc.Broadcast(cmd, 2*time.Second)
	if quiet {
		return nil
	}
	if len(replies) == 0 {
		fmt.Fprintln(os.Stderr, "no running cu server found")
		return nil
	}
	for _, r := range replies {
		if r.Err != nil {
			fmt.Printf("%s: error: %v\n", r.Pipe, r.Err)
			continue
		}
		fmt.Printf("%s: %s\n", r.Pipe, r.Body)
	}
	return nil
}
