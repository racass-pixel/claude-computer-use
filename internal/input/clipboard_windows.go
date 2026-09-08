//go:build windows

package input

import (
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

type Clipboard struct{}

func NewClipboard() *Clipboard              { return &Clipboard{} }
func (*Clipboard) GetText() (string, error) { return win.GetClipboardText() }
func (*Clipboard) SetText(s string) error   { return win.SetClipboardText(s) }

var _ platform.Clipboard = (*Clipboard)(nil)
