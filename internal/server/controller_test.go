package server

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeController implements Controller for testing pause/latch semantics.
type fakeController struct {
	mu       sync.Mutex
	state    string // "idle", "controlling", "paused"
	userHold bool
	resumed  chan struct{} // closed on Resume
	onStatus func(n int)   // optional: called (under the lock) on every Status() with the call count
	statusN  int
}

func newFakeController(state string) *fakeController {
	return &fakeController{state: state, resumed: make(chan struct{})}
}

func (c *fakeController) IsPaused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state == "paused"
}

func (c *fakeController) Acquire(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.userHold {
		return false
	}
	if c.state == "idle" {
		c.state = "controlling"
	}
	return true
}

func (c *fakeController) Touch(now time.Time) {}

func (c *fakeController) Release(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = "idle"
	// userHold is NOT cleared by Release.
}

func (c *fakeController) WaitResume(ctx context.Context) bool {
	c.mu.Lock()
	ch := c.resumed
	c.mu.Unlock()
	select {
	case <-ch:
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.state == "controlling"
	case <-ctx.Done():
		return false
	}
}

func (c *fakeController) Status() ControllerStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.statusN++
	if c.onStatus != nil {
		c.onStatus(c.statusN)
	}
	return ControllerStatus{State: c.state, Hotkey: "Esc Esc", UserHold: c.userHold}
}

// resume simulates user handing control back.
func (c *fakeController) resume() {
	c.mu.Lock()
	c.state = "controlling"
	c.userHold = false
	ch := c.resumed
	c.resumed = make(chan struct{})
	c.mu.Unlock()
	close(ch)
}

// setUserHold sets the latch.
func (c *fakeController) setUserHold(v bool) {
	c.mu.Lock()
	c.userHold = v
	c.mu.Unlock()
}

// TestBeginWithLatchReturnsUserTookControlImmediately verifies that begin()
// returns user_took_control immediately when the userHold latch is set (C2).
func TestBeginWithLatchReturnsUserTookControlImmediately(t *testing.T) {
	h := newHarness(t)
	ctrl := newFakeController("idle")
	ctrl.setUserHold(true)
	h.s.d.Controller = ctrl

	x, y := 100, 100
	off := false
	t0 := time.Now()
	res, _, _ := h.s.toolClick(context.Background(), nil, ClickIn{X: &x, Y: &y, Screenshot: &off})
	elapsed := time.Since(t0)
	if !res.IsError {
		t.Fatal("expected error when latch is set")
	}
	fields, _ := decode(t, res)
	if fields["error"] != "user_took_control" {
		t.Fatalf("expected user_took_control error, got %v", fields)
	}
	// Must return immediately, not wait 20s.
	if elapsed > 2*time.Second {
		t.Fatalf("begin() with latch should return immediately, took %v", elapsed)
	}
}

// TestBeginPausedThenResumed verifies resumed:true path.
func TestBeginPausedThenResumed(t *testing.T) {
	h := newHarness(t)
	ctrl := newFakeController("paused")
	h.s.d.Controller = ctrl

	go func() {
		time.Sleep(50 * time.Millisecond)
		ctrl.resume()
	}()

	x, y := 100, 100
	res, _, _ := h.s.toolClick(context.Background(), nil, ClickIn{X: &x, Y: &y})
	fields, _ := decode(t, res)
	if fields["resumed"] != true {
		t.Fatalf("expected resumed:true, got %v", fields)
	}
	// Verify no click was performed (resumed returns without executing).
	for _, call := range h.in.Calls {
		if strings.HasPrefix(call, "down") {
			t.Fatalf("click should not be performed on resume, got calls: %v", h.in.Calls)
		}
	}
}

// TestBatchAbortsOnUserTookControl verifies I6: batch stops on user_took_control
// even with stop_on_error:false.
func TestBatchAbortsOnUserTookControl(t *testing.T) {
	h := newHarness(t)
	ctrl := newFakeController("idle")
	h.s.d.Controller = ctrl

	off := false
	soe := false // stop_on_error = false
	// The first action succeeds; the user takes control (latch) before the
	// second step's begin() checks the controller. Deterministic: no timers.
	ctrl.onStatus = func(n int) {
		if n == 2 {
			ctrl.state = "idle"
			ctrl.userHold = true
		}
	}

	res, _, _ := h.s.toolBatch(context.Background(), nil, BatchIn{
		Actions: []BatchAction{
			{Tool: "type", Args: map[string]any{"text": "a"}},
			{Tool: "type", Args: map[string]any{"text": "b"}},
			{Tool: "type", Args: map[string]any{"text": "c"}},
		},
		StopOnError: &soe,
		Screenshot:  &off,
	})
	fields, _ := decode(t, res)
	if fields["ok"] != false {
		t.Fatalf("batch should fail, got %v", fields)
	}
	// "c" should not have been typed if batch stopped on user_took_control.
	steps := fields["steps"].([]any)
	typed := 0
	for _, s := range steps {
		sm := s.(map[string]any)
		if sm["tool"] == "type" && sm["ok"] == true {
			typed++
		}
	}
	if typed > 1 {
		t.Fatalf("expected at most 1 successful type, got %d", typed)
	}
}

// TestPanicRecoverWrapper verifies that safeHandler catches panics (I10).
func TestPanicRecoverWrapper(t *testing.T) {
	logger := log.New(os.Stderr, "test: ", 0)
	panicker := func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		panic("test panic in tool handler")
	}
	safe := safeHandler(logger, panicker)
	res, _, err := safe(context.Background(), nil, struct{}{})
	if err != nil {
		t.Fatalf("safeHandler should not return an error, got %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected an error result from panic recovery")
	}
	fields, _ := decode(t, res)
	if fields["error"] != "internal_error" {
		t.Fatalf("expected internal_error, got %v", fields)
	}
}
