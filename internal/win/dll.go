//go:build windows

// Package win contains thin, dependency-free bindings to the Win32 APIs used by cu.
package win

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	shcore   = windows.NewLazySystemDLL("shcore.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
)

type RECT struct{ Left, Top, Right, Bottom int32 }
type POINT struct{ X, Y int32 }

func (r RECT) Width() int32  { return r.Right - r.Left }
func (r RECT) Height() int32 { return r.Bottom - r.Top }

// callErr converts a zero return + errno into a Go error carrying the API name.
func callErr(name string, r uintptr, e error) error {
	if r != 0 {
		return nil
	}
	if en, ok := e.(syscall.Errno); ok && en == 0 {
		return fmt.Errorf("%s failed", name)
	}
	return fmt.Errorf("%s: %w", name, e)
}
