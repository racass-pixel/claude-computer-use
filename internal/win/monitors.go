//go:build windows

package win

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
)

type monitorInfoEx struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
	SzDevice  [32]uint16
}

type MonitorInfo struct {
	Handle  uintptr
	Rect    RECT
	Work    RECT
	Primary bool
	Device  string
	DPI     uint32
}

// One callback for the process lifetime: windows.NewCallback must not be called repeatedly.
var (
	enumMu       sync.Mutex
	enumOut      []MonitorInfo
	enumCallback = windows.NewCallback(func(hMonitor, hdc uintptr, rc *RECT, lparam uintptr) uintptr {
		var mi monitorInfoEx
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		if r, _, _ := procGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&mi))); r == 0 {
			return 1
		}
		enumOut = append(enumOut, MonitorInfo{
			Handle:  hMonitor,
			Rect:    mi.RcMonitor,
			Work:    mi.RcWork,
			Primary: mi.DwFlags&1 != 0, // MONITORINFOF_PRIMARY
			Device:  windows.UTF16ToString(mi.SzDevice[:]),
			DPI:     MonitorDPI(hMonitor),
		})
		return 1
	})
)

// EnumMonitors lists all display monitors in physical pixels.
func EnumMonitors() ([]MonitorInfo, error) {
	enumMu.Lock()
	defer enumMu.Unlock()
	enumOut = nil
	r, _, e := procEnumDisplayMonitors.Call(0, 0, enumCallback, 0)
	if err := callErr("EnumDisplayMonitors", r, e); err != nil {
		return nil, err
	}
	out := make([]MonitorInfo, len(enumOut))
	copy(out, enumOut)
	return out, nil
}

func GetCursorPos() (POINT, error) {
	var p POINT
	r, _, e := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p, callErr("GetCursorPos", r, e)
}

// VirtualScreenRect returns the bounding box of all monitors (SM_*VIRTUALSCREEN).
func VirtualScreenRect() RECT {
	x, _, _ := procGetSystemMetrics.Call(76)
	y, _, _ := procGetSystemMetrics.Call(77)
	w, _, _ := procGetSystemMetrics.Call(78)
	h, _, _ := procGetSystemMetrics.Call(79)
	return RECT{int32(x), int32(y), int32(x) + int32(w), int32(y) + int32(h)}
}
