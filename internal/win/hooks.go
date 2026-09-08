//go:build windows

package win

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
)

const (
	whKeyboardLL  = 13
	whMouseLL     = 14
	llkhfInjected = 0x10
	llmhfInjected = 0x01

	wmKeyDown     = 0x0100
	wmKeyUp       = 0x0101
	wmSysKeyDown  = 0x0104
	wmSysKeyUp    = 0x0105
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmRButtonDown = 0x0204
	wmMButtonDown = 0x0207
	wmMouseWheel  = 0x020A
	wmMouseHWheel = 0x020E
	wmXButtonDown = 0x020B
)

type kbdLLHookStruct struct {
	VkCode, ScanCode, Flags, Time uint32
	ExtraInfo                     uintptr
}

type msLLHookStruct struct {
	Pt                     POINT
	MouseData, Flags, Time uint32
	ExtraInfo              uintptr
}

type HookKind int

const (
	HookKeyDown HookKind = iota
	HookKeyUp
	HookMouseMove
	HookMouseDown
	HookWheel
)

type HookEvent struct {
	Kind     HookKind
	VK       uint16
	X, Y     int32
	Injected bool
}

var (
	hookMu   sync.RWMutex
	hookSink func(HookEvent)

	kbdHookProc = windows.NewCallback(func(nCode int32, wparam, lparam uintptr) uintptr {
		if nCode >= 0 {
			k := (*kbdLLHookStruct)(unsafe.Pointer(lparam))
			ev := HookEvent{VK: uint16(k.VkCode), Injected: k.Flags&llkhfInjected != 0 || k.ExtraInfo == InputTag}
			switch wparam {
			case wmKeyDown, wmSysKeyDown:
				ev.Kind = HookKeyDown
			default:
				ev.Kind = HookKeyUp
			}
			hookMu.RLock()
			s := hookSink
			hookMu.RUnlock()
			if s != nil {
				s(ev)
			}
		}
		r, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wparam, lparam)
		return r
	})

	mouseHookProc = windows.NewCallback(func(nCode int32, wparam, lparam uintptr) uintptr {
		if nCode >= 0 {
			m := (*msLLHookStruct)(unsafe.Pointer(lparam))
			ev := HookEvent{X: m.Pt.X, Y: m.Pt.Y, Injected: m.Flags&llmhfInjected != 0 || m.ExtraInfo == InputTag}
			deliver := true
			switch wparam {
			case wmMouseMove:
				ev.Kind = HookMouseMove
			case wmLButtonDown, wmRButtonDown, wmMButtonDown, wmXButtonDown:
				ev.Kind = HookMouseDown
			case wmMouseWheel, wmMouseHWheel:
				ev.Kind = HookWheel
			default:
				deliver = false
			}
			if deliver {
				hookMu.RLock()
				s := hookSink
				hookMu.RUnlock()
				if s != nil {
					s(ev)
				}
			}
		}
		r, _, _ := procCallNextHookEx.Call(0, uintptr(nCode), wparam, lparam)
		return r
	})
)

// InstallLLHooks installs global keyboard and mouse hooks on the calling thread, which must pump messages.
// sink must return quickly (Windows silently removes hooks that take longer than a few hundred ms).
func InstallLLHooks(sink func(HookEvent)) (func(), error) {
	hookMu.Lock()
	hookSink = sink
	hookMu.Unlock()
	hk, _, e := procSetWindowsHookExW.Call(whKeyboardLL, kbdHookProc, 0, 0)
	if err := callErr("SetWindowsHookExW(keyboard)", hk, e); err != nil {
		return nil, err
	}
	hm, _, e := procSetWindowsHookExW.Call(whMouseLL, mouseHookProc, 0, 0)
	if err := callErr("SetWindowsHookExW(mouse)", hm, e); err != nil {
		procUnhookWindowsHookEx.Call(hk)
		return nil, err
	}
	return func() {
		procUnhookWindowsHookEx.Call(hk)
		procUnhookWindowsHookEx.Call(hm)
		hookMu.Lock()
		hookSink = nil
		hookMu.Unlock()
	}, nil
}
