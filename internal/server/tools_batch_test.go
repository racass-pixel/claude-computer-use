package server

import (
	"context"
	"strings"
	"testing"
)

func TestBatchRunsInOrderAndTakesOneScreenshot(t *testing.T) {
	h := newHarness(t)
	res, _, _ := h.s.toolBatch(context.Background(), nil, BatchIn{Actions: []BatchAction{
		{Tool: "click", Args: map[string]any{"x": 10, "y": 10}},
		{Tool: "type", Args: map[string]any{"text": "hi"}},
		{Tool: "key", Args: map[string]any{"key": "enter"}},
	}})
	fields, img := decode(t, res)
	if res.IsError || img == nil {
		t.Fatalf("batch failed: %v", fields)
	}
	steps := fields["steps"].([]any)
	if len(steps) != 3 {
		t.Fatalf("steps = %v", steps)
	}
	got := strings.Join(h.in.Calls, "|")
	if !strings.HasPrefix(got, "move ") || !strings.Contains(got, "|type hi|key_down 13|key_up 13") {
		t.Fatalf("calls = %q", got)
	}
}

func TestBatchStopsOnError(t *testing.T) {
	h := newHarness(t)
	res, _, _ := h.s.toolBatch(context.Background(), nil, BatchIn{Actions: []BatchAction{
		{Tool: "nope", Args: nil},
		{Tool: "type", Args: map[string]any{"text": "never"}},
	}})
	fields, _ := decode(t, res)
	if fields["ok"] != false || len(h.in.Calls) != 0 {
		t.Fatalf("batch must stop at the first failing step: %v / %v", fields, h.in.Calls)
	}
	if steps := fields["steps"].([]any); len(steps) != 1 || steps[0].(map[string]any)["ok"] != false {
		t.Fatalf("steps = %v", steps)
	}
}
