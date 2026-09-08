// Package window implements platform.Windows and window target resolution.
package window

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

// ExcludeClassPrefix hides cu's own overlay windows from List.
const ExcludeClassPrefix = "CuOverlay"

// Match resolves "foreground", a decimal HWND, or a case-insensitive regexp over title and process.
func Match(wins []platform.WindowInfo, target string) (platform.WindowInfo, error) {
	if target == "" || target == "foreground" {
		for _, w := range wins {
			if w.Foreground {
				return w, nil
			}
		}
		return platform.WindowInfo{}, fmt.Errorf("no foreground window")
	}
	if id, err := strconv.ParseUint(target, 10, 64); err == nil {
		for _, w := range wins {
			if w.ID == uintptr(id) {
				return w, nil
			}
		}
		return platform.WindowInfo{}, fmt.Errorf("no window with id %d (call windows to list them)", id)
	}
	re, err := regexp.Compile("(?i)" + target)
	if err != nil {
		return platform.WindowInfo{}, fmt.Errorf("bad window pattern %q: %v", target, err)
	}
	var first *platform.WindowInfo
	for i := range wins {
		w := &wins[i]
		if re.MatchString(w.Title) || re.MatchString(w.Process) {
			if w.Foreground {
				return *w, nil
			}
			if first == nil {
				first = w
			}
		}
	}
	if first == nil {
		return platform.WindowInfo{}, fmt.Errorf("no window matches %q (call windows to list them)", target)
	}
	return *first, nil
}
