//go:build !windows

package overlay

import (
	"github.com/racass-pixel/claude-computer-use/internal/platform"
)

// Overlay is a no-op on non-Windows platforms.
type Overlay = platform.NopOverlay

// New returns a NopOverlay on non-Windows.
func New(_ any, _ Config) (*Overlay, error) { return &Overlay{}, nil }
