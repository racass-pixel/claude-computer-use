//go:build windows

// Package ipc lets `cu ctl` (run by Claude Code hooks or the user) talk to running cu servers.
package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
)

const Prefix = "claude-computer-use-"

type Handler func(cmd string) (any, error)

func pipeName(pid uint32) string { return fmt.Sprintf(`\\.\pipe\%s%d`, Prefix, pid) }

func Serve(ctx context.Context, pid uint32, h Handler) error {
	l, err := winio.ListenPipe(pipeName(pid), nil)
	if err != nil {
		return err
	}
	go func() { <-ctx.Done(); l.Close() }()
	for {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go serveConn(conn, h)
	}
}

func serveConn(conn net.Conn, h Handler) {
	defer conn.Close()
	// Guard against a client that connects and never sends a newline: without
	// this, a slow/misbehaving client would hang this goroutine forever, since
	// cancelling ctx only closes the listener, not accepted connections.
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil && line == "" {
		return
	}
	cmd := strings.TrimSpace(line)
	res, err := h(cmd)
	var out []byte
	if err != nil {
		out, _ = json.Marshal(map[string]any{"ok": false, "error": err.Error()})
	} else {
		out, _ = json.Marshal(map[string]any{"ok": true, "result": res})
	}
	conn.Write(append(out, '\n'))
}
