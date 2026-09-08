//go:build windows

package main

import (
	"context"
	"io"
	"log"
	"os"
	"os/signal"

	"github.com/racass-pixel/claude-computer-use/internal/config"
	"github.com/racass-pixel/claude-computer-use/internal/input"
	"github.com/racass-pixel/claude-computer-use/internal/screen"
	"github.com/racass-pixel/claude-computer-use/internal/server"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	deps := server.Deps{
		Screen:  screen.New(),
		Input:   input.New(),
		Clip:    input.NewClipboard(),
		Wins:    window.New(),
		Version: version,
	}
	return server.Run(ctx, deps, cfg, logger)
}
