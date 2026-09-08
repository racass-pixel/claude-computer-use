package main

import "fmt"

// parseCtlArgs parses the arguments to `cu ctl`. --quiet is accepted in any
// position. err is non-nil for a missing or unknown command, but quiet still
// reports correctly in that case so callers can suppress the error even when
// parsing itself failed (hooks must not fail Claude Code when misconfigured).
func parseCtlArgs(args []string) (cmd string, quiet bool, err error) {
	var rest []string
	for _, a := range args {
		if a == "--quiet" {
			quiet = true
			continue
		}
		rest = append(rest, a)
	}
	if len(rest) < 1 {
		return "", quiet, fmt.Errorf("usage: cu ctl status|resume|release|pause [--quiet]")
	}
	cmd = rest[0]
	switch cmd {
	case "status", "resume", "release", "pause":
	default:
		return cmd, quiet, fmt.Errorf("unknown ctl command %q", cmd)
	}
	return cmd, quiet, nil
}
