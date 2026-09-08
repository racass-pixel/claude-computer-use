//go:build windows

package input

import (
	"fmt"
	"unicode/utf16"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
	"github.com/racass-pixel/claude-computer-use/internal/platform"
	"github.com/racass-pixel/claude-computer-use/internal/win"
)

// Input is the Windows platform.Input built on SendInput.
type Input struct{}

func New() *Input { return &Input{} }

func (*Input) MouseMove(p geom.Point) error { return win.MouseMoveAbs(p.X, p.Y) }

func buttonFlags(b platform.MouseButton) (down, up uint32, err error) {
	switch b {
	case platform.ButtonLeft, "":
		return win.MouseEventfLeftDown, win.MouseEventfLeftUp, nil
	case platform.ButtonRight:
		return win.MouseEventfRightDown, win.MouseEventfRightUp, nil
	case platform.ButtonMiddle:
		return win.MouseEventfMiddleDown, win.MouseEventfMiddleUp, nil
	}
	return 0, 0, fmt.Errorf("unknown mouse button %q", b)
}

func (*Input) MouseDown(b platform.MouseButton) error {
	d, _, err := buttonFlags(b)
	if err != nil {
		return err
	}
	return win.MouseButtonEvent(d)
}

func (*Input) MouseUp(b platform.MouseButton) error {
	_, u, err := buttonFlags(b)
	if err != nil {
		return err
	}
	return win.MouseButtonEvent(u)
}

func (*Input) Scroll(dx, dy int) error {
	if dy != 0 {
		if err := win.MouseWheel(int32(-dy*120), false); err != nil {
			return err
		}
	}
	if dx != 0 {
		if err := win.MouseWheel(int32(dx*120), true); err != nil {
			return err
		}
	}
	return nil
}

func keyFlags(vk uint16) uint32 {
	if ExtendedKeys[vk] {
		return win.KeyEventfExtendedKey
	}
	return 0
}

func (*Input) KeyDown(vk uint16) error {
	return win.KeyEvent(vk, win.MapVirtualKeyToScan(vk), keyFlags(vk))
}

func (*Input) KeyUp(vk uint16) error {
	return win.KeyEvent(vk, win.MapVirtualKeyToScan(vk), keyFlags(vk)|win.KeyEventfKeyUp)
}

// TypeUnicode types text; newlines become Enter and tabs become Tab so editors behave naturally.
func (in *Input) TypeUnicode(s string) error {
	var pending []uint16
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		err := win.UnicodeText(pending)
		pending = pending[:0]
		return err
	}
	for _, r := range s {
		switch r {
		case '\r':
			continue
		case '\n', '\t':
			if err := flush(); err != nil {
				return err
			}
			vk := uint16(VK_RETURN)
			if r == '\t' {
				vk = VK_TAB
			}
			if err := in.KeyDown(vk); err != nil {
				return err
			}
			if err := in.KeyUp(vk); err != nil {
				return err
			}
		default:
			pending = append(pending, utf16.Encode([]rune{r})...)
		}
	}
	return flush()
}

var _ platform.Input = (*Input)(nil)
