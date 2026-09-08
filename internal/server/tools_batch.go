package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type BatchAction struct {
	Tool string         `json:"tool" jsonschema:"click, move, drag, scroll, type, key, wait, window, clipboard, find, pixel or click_until"`
	Args map[string]any `json:"args,omitempty" jsonschema:"the tool's arguments; screenshot is forced off for steps"`
}

type BatchIn struct {
	Actions          []BatchAction `json:"actions" jsonschema:"steps executed in order"`
	StopOnError      *bool         `json:"stop_on_error,omitempty" jsonschema:"stop at the first failing step (default true)"`
	ScreenshotRegion *RegionIn     `json:"screenshot_region,omitempty" jsonschema:"after all steps, return a zoomed screenshot of this rectangle (last-screenshot pixels) instead of the whole monitor; the coordinate space switches to that region"`
	Screenshot       *bool         `json:"screenshot,omitempty" jsonschema:"one screenshot after the last step (default true)"`
}

type batchFn func(ctx context.Context, args json.RawMessage) (*mcp.CallToolResult, error)

// wrap adapts a typed handler into a batchFn that forces screenshot=false and strips screenshot_region.
func wrap[In any](h func(context.Context, *mcp.CallToolRequest, In) (*mcp.CallToolResult, any, error)) batchFn {
	return func(ctx context.Context, args json.RawMessage) (*mcp.CallToolResult, error) {
		var m map[string]any
		if len(args) > 0 {
			if err := json.Unmarshal(args, &m); err != nil {
				return nil, err
			}
		}
		if m == nil {
			m = map[string]any{}
		}
		m["screenshot"] = false
		delete(m, "screenshot_region")
		b, _ := json.Marshal(m)
		var in In
		if err := json.Unmarshal(b, &in); err != nil {
			return nil, err
		}
		res, _, err := h(ctx, nil, in)
		return res, err
	}
}

func (s *Session) batchable() map[string]batchFn {
	return map[string]batchFn{
		"click":       wrap(s.toolClick),
		"move":        wrap(s.toolMove),
		"drag":        wrap(s.toolDrag),
		"scroll":      wrap(s.toolScroll),
		"type":        wrap(s.toolType),
		"key":         wrap(s.toolKey),
		"wait":        wrap(s.toolWait),
		"window":      wrap(s.toolWindow),
		"clipboard":   wrap(s.toolClipboard),
		"find":        wrap(s.toolFind),
		"pixel":       wrap(s.toolPixel),
		"click_until": wrap(s.toolClickUntil),
	}
}

// runSteps executes a sequence of BatchActions through the batchable registry.
// It returns step results and whether all succeeded. Shared by batch and recipe run.
func (s *Session) runSteps(ctx context.Context, actions []BatchAction, stopOnError bool) (steps []map[string]any, allOK bool) {
	reg := s.batchable()
	steps = make([]map[string]any, 0, len(actions))
	allOK = true
	for i, a := range actions {
		step := map[string]any{"i": i, "tool": a.Tool}
		fn, ok := reg[a.Tool]
		var res *mcp.CallToolResult
		var err error
		if !ok {
			err = fmt.Errorf("tool %q cannot be used in batch", a.Tool)
		} else {
			args, _ := json.Marshal(a.Args)
			res, err = fn(ctx, args)
		}
		if err == nil && res != nil && res.IsError {
			if tc, ok := res.Content[0].(*mcp.TextContent); ok {
				err = fmt.Errorf("%s", tc.Text)
			} else {
				err = fmt.Errorf("step failed")
			}
		}
		if err != nil {
			step["ok"] = false
			step["error"] = err.Error()
			steps = append(steps, step)
			allOK = false
			if stopOnError {
				break
			}
			continue
		}
		step["ok"] = true
		if tc, ok := res.Content[0].(*mcp.TextContent); ok {
			var f map[string]any
			if json.Unmarshal([]byte(tc.Text), &f) == nil {
				if r, ok := f["resumed"]; ok && r == true { // user handed control back mid-batch: stop, re-observe
					step["resumed"] = true
					steps = append(steps, step)
					allOK = false
					break
				}
			}
		}
		steps = append(steps, step)
	}
	return steps, allOK
}

func (s *Session) toolBatch(ctx context.Context, req *mcp.CallToolRequest, in BatchIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	if len(in.Actions) == 0 {
		return errResult("bad_args", "actions is empty"), nil, nil
	}
	shot := s.shotFor(s.wantShot(in.Screenshot), in.ScreenshotRegion)
	stop := in.StopOnError == nil || *in.StopOnError
	steps, allOK := s.runSteps(ctx, in.Actions, stop)
	f := map[string]any{"ok": allOK, "steps": steps, "completed": len(steps)}
	settle := time.Duration(0)
	if shot.Want {
		settle = 120 * time.Millisecond
	}
	return s.finish("batch", t0, f, shot, settle), nil, nil
}
