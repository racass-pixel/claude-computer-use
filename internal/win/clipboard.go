//go:build windows

package win

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procEmptyClipboard             = user32.NewProc("EmptyClipboard")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procSetClipboardData           = user32.NewProc("SetClipboardData")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procGlobalAlloc                = kernel32.NewProc("GlobalAlloc")
	procGlobalLock                 = kernel32.NewProc("GlobalLock")
	procGlobalUnlock               = kernel32.NewProc("GlobalUnlock")
	procGlobalFree                 = kernel32.NewProc("GlobalFree")
	procGlobalSize                 = kernel32.NewProc("GlobalSize")
	procRtlMoveMemory              = kernel32.NewProc("RtlMoveMemory")
)

const (
	cfUnicodeText = 13
	gmemMoveable  = 0x0002
)

func openClipboard() error {
	for i := 0; i < 10; i++ {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("clipboard is busy")
}

// GetClipboardText reads CF_UNICODETEXT. The GlobalLock'd HGLOBAL address is never converted
// to unsafe.Pointer directly (that address is owned by the OS, not the Go runtime/GC); instead
// RtlMoveMemory copies it into a Go-owned buffer, whose address we may safely take.
func GetClipboardText() (string, error) {
	if err := openClipboard(); err != nil {
		return "", err
	}
	defer procCloseClipboard.Call()
	if r, _, _ := procIsClipboardFormatAvailable.Call(cfUnicodeText); r == 0 {
		return "", nil
	}
	h, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return "", nil
	}
	size, _, _ := procGlobalSize.Call(h)
	if size == 0 {
		return "", nil
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		return "", fmt.Errorf("GlobalLock failed")
	}
	defer procGlobalUnlock.Call(h)
	buf := make([]uint16, size/2)
	procRtlMoveMemory.Call(uintptr(unsafe.Pointer(&buf[0])), p, size)
	for i, u := range buf {
		if u == 0 {
			buf = buf[:i]
			break
		}
	}
	return windows.UTF16ToString(buf), nil
}

// SetClipboardText writes CF_UNICODETEXT via RtlMoveMemory, for the same reason as GetClipboardText.
func SetClipboardText(s string) error {
	units, err := windows.UTF16FromString(s)
	if err != nil {
		return err
	}
	size := uintptr(len(units) * 2)
	h, _, e := procGlobalAlloc.Call(gmemMoveable, size)
	if h == 0 {
		return callErr("GlobalAlloc", h, e)
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		procGlobalFree.Call(h)
		return fmt.Errorf("GlobalLock failed")
	}
	procRtlMoveMemory.Call(p, uintptr(unsafe.Pointer(&units[0])), size)
	procGlobalUnlock.Call(h)
	if err := openClipboard(); err != nil {
		procGlobalFree.Call(h)
		return err
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()
	if r, _, e := procSetClipboardData.Call(cfUnicodeText, h); r == 0 {
		procGlobalFree.Call(h)
		return callErr("SetClipboardData", r, e)
	}
	return nil // ownership of h moved to the system
}
