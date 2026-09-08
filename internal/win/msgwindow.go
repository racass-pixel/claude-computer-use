//go:build windows

package win

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

const WM_APP = 0x8000

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

var (
	classMu   sync.Mutex
	classDone = map[string]bool{}
)

func moduleHandle() uintptr {
	h, _, _ := procGetModuleHandleW.Call(0)
	return h
}

// RegisterClass registers a window class once. wndProc comes from windows.NewCallback.
func RegisterClass(name string, wndProc uintptr) error {
	classMu.Lock()
	defer classMu.Unlock()
	if classDone[name] {
		return nil
	}
	cn, _ := windows.UTF16PtrFromString(name)
	wc := wndClassEx{WndProc: wndProc, Instance: moduleHandle(), ClassName: cn}
	wc.Size = uint32(unsafe.Sizeof(wc))
	r, _, e := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if err := callErr("RegisterClassExW", r, e); err != nil {
		return err
	}
	classDone[name] = true
	return nil
}

// CreateMessageWindow creates an invisible message-only window (parent HWND_MESSAGE).
func CreateMessageWindow(class string) (uintptr, error) {
	cn, _ := windows.UTF16PtrFromString(class)
	const hwndMessage = ^uintptr(2) // (HWND)-3
	h, _, e := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cn)), 0, 0, 0, 0, 0, 0, hwndMessage, 0, moduleHandle(), 0)
	return h, callErr("CreateWindowExW", h, e)
}

func DefWindowProc(hwnd, m, wparam, lparam uintptr) uintptr {
	r, _, _ := procDefWindowProcW.Call(hwnd, m, wparam, lparam)
	return r
}

func PostMessage(hwnd uintptr, m uint32, wparam, lparam uintptr) error {
	r, _, e := procPostMessageW.Call(hwnd, uintptr(m), wparam, lparam)
	return callErr("PostMessageW", r, e)
}

// RunMessageLoop pumps messages for the calling thread until WM_QUIT.
func RunMessageLoop() {
	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func PostQuitMessage() { procPostQuitMessage.Call(0) }

func CurrentThreadID() uint32 {
	r, _, _ := procGetCurrentThreadId.Call()
	return uint32(r)
}
