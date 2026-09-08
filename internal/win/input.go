//go:build windows

package win

import (
	"fmt"
	"math"
	"unsafe"
)

var (
	procSendInput      = user32.NewProc("SendInput")
	procSetCursorPos   = user32.NewProc("SetCursorPos")
	procMapVirtualKeyW = user32.NewProc("MapVirtualKeyW")
)

const (
	inputTypeMouse    = 0
	inputTypeKeyboard = 1

	MouseEventfMove        = 0x0001
	MouseEventfLeftDown    = 0x0002
	MouseEventfLeftUp      = 0x0004
	MouseEventfRightDown   = 0x0008
	MouseEventfRightUp     = 0x0010
	MouseEventfMiddleDown  = 0x0020
	MouseEventfMiddleUp    = 0x0040
	MouseEventfWheel       = 0x0800
	MouseEventfHWheel      = 0x1000
	MouseEventfVirtualDesk = 0x4000
	MouseEventfAbsolute    = 0x8000

	KeyEventfExtendedKey = 0x0001
	KeyEventfKeyUp       = 0x0002
	KeyEventfUnicode     = 0x0004

	// InputTag marks input injected by cu (dwExtraInfo) so our hooks can recognise it.
	InputTag = 0x0C1A0DE5
)

type mouseInput struct {
	Dx, Dy    int32
	MouseData uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

type keybdInput struct {
	Vk, Scan  uint16
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
	_         [8]byte // pad the union to MOUSEINPUT's 32 bytes
}

type inputMouseRec struct {
	Type uint32
	_    uint32
	Mi   mouseInput
}

type inputKeybdRec struct {
	Type uint32
	_    uint32
	Ki   keybdInput
}

func sendInput(ptr unsafe.Pointer, n int, size uintptr) error {
	r, _, e := procSendInput.Call(uintptr(n), uintptr(ptr), size)
	if int(r) != n {
		return fmt.Errorf("SendInput sent %d of %d events: %v", r, n, e)
	}
	return nil
}

// MouseMoveAbs moves the cursor to virtual-screen coordinates and emits a real mouse-move event.
func MouseMoveAbs(x, y int) error {
	vs := VirtualScreenRect()
	nx := int32(math.Round(float64(x-int(vs.Left)) * 65535 / float64(max(1, int(vs.Width())-1))))
	ny := int32(math.Round(float64(y-int(vs.Top)) * 65535 / float64(max(1, int(vs.Height())-1))))
	rec := inputMouseRec{Type: inputTypeMouse, Mi: mouseInput{
		Dx: nx, Dy: ny, Flags: MouseEventfMove | MouseEventfAbsolute | MouseEventfVirtualDesk, ExtraInfo: InputTag,
	}}
	if err := sendInput(unsafe.Pointer(&rec), 1, unsafe.Sizeof(rec)); err != nil {
		return err
	}
	// normalisation may land one pixel off; snap exactly.
	if p, err := GetCursorPos(); err == nil && (int(p.X) != x || int(p.Y) != y) {
		procSetCursorPos.Call(uintptr(x), uintptr(y))
	}
	return nil
}

// MouseButtonEvent sends one button event (a MouseEventf* down/up flag) at the current position.
func MouseButtonEvent(flags uint32) error {
	rec := inputMouseRec{Type: inputTypeMouse, Mi: mouseInput{Flags: flags, ExtraInfo: InputTag}}
	return sendInput(unsafe.Pointer(&rec), 1, unsafe.Sizeof(rec))
}

// MouseWheel sends a wheel event; delta is in WHEEL_DELTA units (120 per tick), positive = up / right.
func MouseWheel(delta int32, horizontal bool) error {
	flags := uint32(MouseEventfWheel)
	if horizontal {
		flags = MouseEventfHWheel
	}
	rec := inputMouseRec{Type: inputTypeMouse, Mi: mouseInput{MouseData: uint32(delta), Flags: flags, ExtraInfo: InputTag}}
	return sendInput(unsafe.Pointer(&rec), 1, unsafe.Sizeof(rec))
}

func MapVirtualKeyToScan(vk uint16) uint16 {
	r, _, _ := procMapVirtualKeyW.Call(uintptr(vk), 0 /* MAPVK_VK_TO_VSC */)
	return uint16(r)
}

// KeyEvent sends one virtual-key event.
func KeyEvent(vk, scan uint16, flags uint32) error {
	rec := inputKeybdRec{Type: inputTypeKeyboard, Ki: keybdInput{Vk: vk, Scan: scan, Flags: flags, ExtraInfo: InputTag}}
	return sendInput(unsafe.Pointer(&rec), 1, unsafe.Sizeof(rec))
}

// UnicodeText types UTF-16 code units as KEYEVENTF_UNICODE down/up pairs, in one SendInput call per chunk.
func UnicodeText(units []uint16) error {
	const chunk = 48 // 96 events per call keeps SendInput well below its practical limits
	for start := 0; start < len(units); start += chunk {
		end := min(start+chunk, len(units))
		recs := make([]inputKeybdRec, 0, (end-start)*2)
		for _, u := range units[start:end] {
			recs = append(recs,
				inputKeybdRec{Type: inputTypeKeyboard, Ki: keybdInput{Scan: u, Flags: KeyEventfUnicode, ExtraInfo: InputTag}},
				inputKeybdRec{Type: inputTypeKeyboard, Ki: keybdInput{Scan: u, Flags: KeyEventfUnicode | KeyEventfKeyUp, ExtraInfo: InputTag}},
			)
		}
		if err := sendInput(unsafe.Pointer(&recs[0]), len(recs), unsafe.Sizeof(recs[0])); err != nil {
			return err
		}
	}
	return nil
}
