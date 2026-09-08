// cu is the Claude Computer Use binary: MCP server, control CLI and dev helpers.
package main

import (
	"fmt"
	"os"
)

// version is injected at build time: -ldflags "-X main.version=0.1.0".
var version = "dev"

func usage() {
	fmt.Fprintln(os.Stderr, `usage: cu <command> [args]

commands:
  serve                      run the MCP server over stdio (used by Claude Code)
  ctl status|resume|release|pause [--quiet]
  doctor                     report monitors, DPI, privileges, capture speed
  demo [-seconds N]          show the overlay for N seconds (default 5)
  screenshot [-m N] [-o file.png]
  input move X Y | click X Y [button] | type TEXT | key CHORD
  version`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	args := os.Args[2:]
	var err error
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Println(version)
	case "serve":
		err = runServe(args)
	case "ctl":
		err = runCtl(args)
	case "doctor":
		err = runDoctor(args)
	case "demo":
		err = runDemo(args)
	case "screenshot":
		err = runScreenshot(args)
	case "input":
		err = runInput(args)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "cu:", err)
		os.Exit(1)
	}
}
