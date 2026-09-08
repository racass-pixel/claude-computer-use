package server

import (
	"context"
	"image"
	"regexp"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/racass-pixel/claude-computer-use/internal/screen"
	"github.com/racass-pixel/claude-computer-use/internal/window"
)

type WaitIn struct {
	Ms         int     `json:"ms,omitempty" jsonschema:"plain sleep in milliseconds"`
	Window     string  `json:"window,omitempty" jsonschema:"wait until a window matching this regexp (title or process) exists"`
	Stable     *bool   `json:"stable,omitempty" jsonschema:"wait until the active monitor stops changing (animations, page loads)"`
	Element    *FindIn `json:"element,omitempty" jsonschema:"wait until find returns at least one element for this query"`
	TimeoutMs  int     `json:"timeout_ms,omitempty" jsonschema:"give up after this long (default 10000)"`
	Screenshot *bool   `json:"screenshot,omitempty" jsonschema:"return a screenshot when done (default true)"`
}

// frameDiff is the mean absolute RGB difference (0..255) between two equally sized frames.
func frameDiff(a, b *image.RGBA) float64 {
	if len(a.Pix) != len(b.Pix) || len(a.Pix) == 0 {
		return 255
	}
	var sum int64
	n := 0
	for i := 0; i+3 < len(a.Pix); i += 4 {
		for k := 0; k < 3; k++ {
			d := int(a.Pix[i+k]) - int(b.Pix[i+k])
			if d < 0 {
				d = -d
			}
			sum += int64(d)
		}
		n += 3
	}
	return float64(sum) / float64(n)
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func (s *Session) toolWait(ctx context.Context, req *mcp.CallToolRequest, in WaitIn) (*mcp.CallToolResult, any, error) {
	t0 := time.Now()
	timeout := time.Duration(in.TimeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	deadline := time.Now().Add(timeout)
	cond := "ms"
	switch {
	case in.Window != "":
		cond = "window"
		re, err := regexp.Compile("(?i)" + in.Window)
		if err != nil {
			return errResult("bad_args", "window: "+err.Error()), nil, nil
		}
		for {
			list, _ := s.d.Wins.List()
			if w, err := window.Match(list, in.Window); err == nil && re != nil {
				return s.finish("wait", t0, map[string]any{"condition": cond, "matched": map[string]any{"id": w.ID, "title": w.Title, "process": w.Process}}, s.wantShot(in.Screenshot), 50*time.Millisecond), nil, nil
			}
			if time.Now().After(deadline) || !sleepCtx(ctx, 150*time.Millisecond) {
				break
			}
		}
	case in.Element != nil:
		cond = "element"
		for {
			out, _, err := s.findElements(*in.Element)
			if err == nil && len(out) > 0 {
				return s.finish("wait", t0, map[string]any{"condition": cond, "elements": out, "count": len(out)}, s.wantShot(in.Screenshot), 50*time.Millisecond), nil, nil
			}
			if time.Now().After(deadline) || !sleepCtx(ctx, 200*time.Millisecond) {
				break
			}
		}
	case in.Stable != nil && *in.Stable:
		cond = "stable"
		m := s.activeMonitor()
		var prev *image.RGBA
		quiet := 0
		for {
			img, err := s.d.Screen.Capture(m.Rect)
			if err != nil {
				return errResult("capture_failed", err.Error()), nil, nil
			}
			small := screen.Scale(img, 0.2)
			if prev != nil && frameDiff(prev, small) < 1.0 {
				quiet++
				if quiet >= 2 {
					return s.finish("wait", t0, map[string]any{"condition": cond, "stable_after_ms": time.Since(t0).Milliseconds()}, s.wantShot(in.Screenshot), 0), nil, nil
				}
			} else {
				quiet = 0
			}
			prev = small
			if time.Now().After(deadline) || !sleepCtx(ctx, 150*time.Millisecond) {
				break
			}
		}
	default:
		d := time.Duration(in.Ms) * time.Millisecond
		if d <= 0 {
			d = 500 * time.Millisecond
		}
		sleepCtx(ctx, min(d, timeout))
		return s.finish("wait", t0, map[string]any{"condition": cond, "waited_ms": time.Since(t0).Milliseconds()}, s.wantShot(in.Screenshot), 0), nil, nil
	}
	f := map[string]any{"ok": false, "timeout": true, "condition": cond, "waited_ms": time.Since(t0).Milliseconds()}
	return s.finish("wait", t0, f, s.wantShot(in.Screenshot), 0), nil, nil
}
