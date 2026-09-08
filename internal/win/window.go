//go:build windows

package win

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procEnumWindows                = user32.NewProc("EnumWindows")
	procGetWindowTextW             = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW       = user32.NewProc("GetWindowTextLengthW")
	procGetClassNameW              = user32.NewProc("GetClassNameW")
	procIsWindowVisible            = user32.NewProc("IsWindowVisible")
	procGetWindowLongPtrW          = user32.NewProc("GetWindowLongPtrW")
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop           = user32.NewProc("BringWindowToTop")
	procGetWindowRect              = user32.NewProc("GetWindowRect")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procIsIconic                   = user32.NewProc("IsIconic")
	procIsZoomed                   = user32.NewProc("IsZoomed")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procPostMessageW               = user32.NewProc("PostMessageW")
	procAttachThreadInput          = user32.NewProc("AttachThreadInput")
	procGetCurrentThreadId         = kernel32.NewProc("GetCurrentThreadId")
	procDwmGetWindowAttribute      = dwmapi.NewProc("DwmGetWindowAttribute")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
)

const (
	gwlExStyle       = ^uintptr(19) // -20
	wsExToolWindow   = 0x00000080
	dwmwaCloaked     = 14
	dwmwaFrameBounds = 9
	wmClose          = 0x0010

	SW_MAXIMIZE       = 3
	SW_SHOWNOACTIVATE = 4
	SW_MINIMIZE       = 6
	SW_RESTORE        = 9

	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
)

type RawWindow struct {
	HWND      uintptr
	Title     string
	Class     string
	PID, TID  uint32
	Rect      RECT
	Minimized bool
	Maximized bool
}

func windowText(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf)
}

func className(hwnd uintptr) string {
	var buf [256]uint16
	procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return windows.UTF16ToString(buf[:])
}

func isCloaked(hwnd uintptr) bool {
	var cloaked uint32
	procDwmGetWindowAttribute.Call(hwnd, dwmwaCloaked, uintptr(unsafe.Pointer(&cloaked)), 4)
	return cloaked != 0
}

// WindowFrameRect returns the visible frame bounds (without the invisible resize borders).
func WindowFrameRect(hwnd uintptr) (RECT, bool) {
	var r RECT
	if hr, _, _ := procDwmGetWindowAttribute.Call(hwnd, dwmwaFrameBounds, uintptr(unsafe.Pointer(&r)), unsafe.Sizeof(r)); hr == 0 {
		return r, true
	}
	if ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); ok != 0 {
		return r, false
	}
	return RECT{}, false
}

var (
	enumWinMu  sync.Mutex
	enumWinOut []RawWindow
	enumWinCb  = windows.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		if v, _, _ := procIsWindowVisible.Call(hwnd); v == 0 {
			return 1
		}
		ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
		if ex&wsExToolWindow != 0 || isCloaked(hwnd) {
			return 1
		}
		title := windowText(hwnd)
		if title == "" {
			return 1
		}
		var pid uint32
		tid, _, _ := procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		rect, _ := WindowFrameRect(hwnd)
		iconic, _, _ := procIsIconic.Call(hwnd)
		zoomed, _, _ := procIsZoomed.Call(hwnd)
		enumWinOut = append(enumWinOut, RawWindow{
			HWND: hwnd, Title: title, Class: className(hwnd), PID: pid, TID: uint32(tid),
			Rect: rect, Minimized: iconic != 0, Maximized: zoomed != 0,
		})
		return 1
	})
)

// EnumTopLevelWindows lists visible, titled, non-tool, non-cloaked top-level windows in Z order.
func EnumTopLevelWindows() ([]RawWindow, error) {
	enumWinMu.Lock()
	defer enumWinMu.Unlock()
	enumWinOut = nil
	r, _, e := procEnumWindows.Call(enumWinCb, 0)
	if err := callErr("EnumWindows", r, e); err != nil {
		return nil, err
	}
	out := make([]RawWindow, len(enumWinOut))
	copy(out, enumWinOut)
	return out, nil
}

func ForegroundWindow() uintptr {
	h, _, _ := procGetForegroundWindow.Call()
	return h
}

// ProcessImageName returns the lower-case executable name for a PID ("notepad.exe"), or "".
func ProcessImageName(pid uint32) string {
	h, _, _ := procOpenProcess.Call(0x1000 /* PROCESS_QUERY_LIMITED_INFORMATION */, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)
	var buf [1024]uint16
	size := uint32(len(buf))
	if r, _, _ := procQueryFullProcessImageNameW.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r == 0 {
		return ""
	}
	return strings.ToLower(filepath.Base(windows.UTF16ToString(buf[:size])))
}

func ShowWindowCmd(hwnd uintptr, cmd int32) { procShowWindow.Call(hwnd, uintptr(cmd)) }

// FocusWindow brings hwnd to the foreground, working around SetForegroundWindow's restrictions.
func FocusWindow(hwnd uintptr) error {
	if iconic, _, _ := procIsIconic.Call(hwnd); iconic != 0 {
		ShowWindowCmd(hwnd, SW_RESTORE)
		time.Sleep(50 * time.Millisecond)
	}
	try := func() bool {
		procSetForegroundWindow.Call(hwnd)
		procBringWindowToTop.Call(hwnd)
		time.Sleep(30 * time.Millisecond)
		return ForegroundWindow() == hwnd
	}
	if try() {
		return nil
	}
	// Trick 1: a synthetic Alt press marks our process as "last input", unlocking SetForegroundWindow.
	KeyEvent(0x12, MapVirtualKeyToScan(0x12), 0)
	KeyEvent(0x12, MapVirtualKeyToScan(0x12), KeyEventfKeyUp)
	if try() {
		return nil
	}
	// Trick 2: attach our input queue to the current foreground thread.
	fg := ForegroundWindow()
	var pid uint32
	fgTid, _, _ := procGetWindowThreadProcessId.Call(fg, uintptr(unsafe.Pointer(&pid)))
	cur, _, _ := procGetCurrentThreadId.Call()
	if fgTid != 0 && fgTid != cur {
		procAttachThreadInput.Call(cur, fgTid, 1)
		ok := try()
		procAttachThreadInput.Call(cur, fgTid, 0)
		if ok {
			return nil
		}
	}
	return fmt.Errorf("could not bring window %d to the foreground (Windows only flashed it in the taskbar)", hwnd)
}

// SetWindowRect moves/resizes so the *visible frame* matches (x,y,w,h), compensating the DWM border.
func SetWindowRect(hwnd uintptr, x, y, w, h int) error {
	var wr RECT
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	fr, ok := WindowFrameRect(hwnd)
	dl, dt, dr, db := int32(0), int32(0), int32(0), int32(0)
	if ok {
		dl, dt, dr, db = fr.Left-wr.Left, fr.Top-wr.Top, wr.Right-fr.Right, wr.Bottom-fr.Bottom
	}
	r, _, e := procSetWindowPos.Call(hwnd, 0,
		uintptr(int32(x)-dl), uintptr(int32(y)-dt), uintptr(int32(w)+dl+dr), uintptr(int32(h)+dt+db),
		swpNoZOrder|swpNoActivate)
	return callErr("SetWindowPos", r, e)
}

func CloseWindow(hwnd uintptr) error {
	r, _, e := procPostMessageW.Call(hwnd, wmClose, 0, 0)
	return callErr("PostMessage(WM_CLOSE)", r, e)
}
