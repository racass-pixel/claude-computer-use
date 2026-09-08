//go:build !windows

package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

const Prefix = "claude-computer-use-"

type Handler func(cmd string) (any, error)

type Reply struct {
	Pipe string
	Body json.RawMessage
	Err  error
}

func Serve(ctx context.Context, pid uint32, h Handler) error {
	return errors.New("ipc is Windows-only")
}

// Broadcast sends cmd to every running cu server and collects replies.
func Broadcast(cmd string, timeout time.Duration) []Reply {
	return nil
}
