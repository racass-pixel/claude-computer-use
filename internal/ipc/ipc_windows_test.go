//go:build windows

package ipc

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestRoundTrip starts a server on a synthetic pid, finds it through the pipe
// listing and exchanges one command through Broadcast (the `cu ctl` path).
func TestRoundTrip(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pid := uint32(900000 + time.Now().UnixNano()%100000)
	errCh := make(chan error, 1)
	go func() {
		errCh <- Serve(ctx, pid, func(cmd string) (any, error) { return map[string]any{"echo": cmd}, nil })
	}()
	name := pipeName(pid)
	deadline := time.Now().Add(3 * time.Second)
	for {
		found := false
		for _, s := range listServers() {
			if s == name {
				found = true
			}
		}
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pipe %s not listed after 3s", name)
		}
		time.Sleep(20 * time.Millisecond)
	}

	var mine *Reply
	for _, r := range Broadcast("status", 2*time.Second) {
		if r.Pipe == name {
			rr := r
			mine = &rr
		}
	}
	if mine == nil {
		t.Fatalf("no reply from %s", name)
	}
	if mine.Err != nil {
		t.Fatalf("reply error: %v", mine.Err)
	}
	var body struct {
		OK     bool           `json:"ok"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(mine.Body, &body); err != nil {
		t.Fatalf("bad body %s: %v", mine.Body, err)
	}
	if !body.OK || body.Result["echo"] != "status" {
		t.Fatalf("unexpected body %s", mine.Body)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Serve did not return after cancel")
	}
}
