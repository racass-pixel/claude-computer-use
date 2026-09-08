//go:build windows

package win

import (
	"syscall"
	"unsafe"
)

var (
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procGetDpiForMonitor              = shcore.NewProc("GetDpiForMonitor")
)

// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 == (DPI_AWARENESS_CONTEXT)-4
const dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)

// SetPerMonitorDPIAwareV2 must run before any window or monitor API. Safe to call twice.
func SetPerMonitorDPIAwareV2() error {
	r, _, e := procSetProcessDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
	if r == 0 {
		if en, ok := e.(syscall.Errno); ok && en == syscall.ERROR_ACCESS_DENIED {
			return nil // already set for this process
		}
		return callErr("SetProcessDpiAwarenessContext", r, e)
	}
	return nil
}

// MonitorDPI returns the effective DPI of a monitor (96 = 100%).
func MonitorDPI(hMonitor uintptr) uint32 {
	var x, y uint32
	procGetDpiForMonitor.Call(hMonitor, 0 /* MDT_EFFECTIVE_DPI */, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)))
	if x == 0 {
		return 96
	}
	return x
}
