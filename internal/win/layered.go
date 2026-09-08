//go:build windows

package win

import (
	"fmt"
	"image"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procUpdateLayeredWindow      = user32.NewProc("UpdateLayeredWindow")
	procSetWindowDisplayAffinity = user32.NewProc("SetWindowDisplayAffinity")
	procDestroyWindow            = user32.NewProc("DestroyWindow")
)

const (
	OverlayClass = "CuOverlay"

	wsPopup         = 0x80000000
	wsExLayered     = 0x00080000
	wsExTransparent = 0x00000020
	wsExTopmost     = 0x00000008
	wsExNoActivate  = 0x08000000
	ulwAlpha        = 0x00000002
	acSrcAlpha      = 0x01

	wdaExcludeFromCapture = 0x00000011
)

// CaptureExclusionSupported is false if SetWindowDisplayAffinity failed on this machine.
var CaptureExclusionSupported = true

// OverlayVisibleInCapture skips SetWindowDisplayAffinity when true (debug flag for cu demo -show-in-capture).
var OverlayVisibleInCapture bool

var overlayWndProc = windows.NewCallback(func(hwnd, msg, wparam, lparam uintptr) uintptr {
	return DefWindowProc(hwnd, msg, wparam, lparam)
})

type blendFunction struct{ BlendOp, BlendFlags, SourceConstantAlpha, AlphaFormat byte }
type size struct{ Cx, Cy int32 }

// CreateOverlayWindow creates a hidden, click-through, always-on-top layered window excluded from capture.
func CreateOverlayWindow(x, y, w, h int) (uintptr, error) {
	if err := RegisterClass(OverlayClass, overlayWndProc); err != nil {
		return 0, err
	}
	cn, _ := windows.UTF16PtrFromString(OverlayClass)
	hwnd, _, e := procCreateWindowExW.Call(
		wsExLayered|wsExTransparent|wsExTopmost|wsExToolWindow|wsExNoActivate,
		uintptr(unsafe.Pointer(cn)), 0, wsPopup,
		uintptr(int32(x)), uintptr(int32(y)), uintptr(int32(w)), uintptr(int32(h)),
		0, 0, moduleHandle(), 0)
	if hwnd == 0 {
		return 0, callErr("CreateWindowExW(overlay)", hwnd, e)
	}
	if !OverlayVisibleInCapture {
		if r, _, _ := procSetWindowDisplayAffinity.Call(hwnd, wdaExcludeFromCapture); r == 0 {
			CaptureExclusionSupported = false
		}
	}
	return hwnd, nil
}

// LayeredSurface caches a DIB so 30 fps updates do not allocate.
type LayeredSurface struct {
	w, h int
	hdc  uintptr
	hbm  uintptr
	old  uintptr
	bits []byte
}

func NewLayeredSurface(w, h int) (*LayeredSurface, error) {
	screen, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, screen)
	hdc, _, _ := procCreateCompatibleDC.Call(screen)
	if hdc == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	bi := bitmapInfo{Header: bitmapInfoHeader{Size: 40, Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}}
	var bits unsafe.Pointer
	hbm, _, e := procCreateDIBSection.Call(hdc, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbm == 0 || bits == nil {
		procDeleteDC.Call(hdc)
		return nil, callErr("CreateDIBSection", hbm, e)
	}
	old, _, _ := procSelectObject.Call(hdc, hbm)
	return &LayeredSurface{w: w, h: h, hdc: hdc, hbm: hbm, old: old, bits: unsafe.Slice((*byte)(bits), w*h*4)}, nil
}

// Update pushes img (straight alpha, same size as the surface) to hwnd at screen position x,y.
func (s *LayeredSurface) Update(hwnd uintptr, x, y int, img *image.RGBA) error {
	if img.Bounds().Dx() != s.w || img.Bounds().Dy() != s.h {
		return fmt.Errorf("image %v does not match surface %dx%d", img.Bounds(), s.w, s.h)
	}
	premultiplyInto(s.bits, img)
	pt := POINT{int32(x), int32(y)}
	src := POINT{}
	sz := size{int32(s.w), int32(s.h)}
	bf := blendFunction{SourceConstantAlpha: 255, AlphaFormat: acSrcAlpha}
	r, _, e := procUpdateLayeredWindow.Call(hwnd, 0, uintptr(unsafe.Pointer(&pt)), uintptr(unsafe.Pointer(&sz)),
		s.hdc, uintptr(unsafe.Pointer(&src)), 0, uintptr(unsafe.Pointer(&bf)), ulwAlpha)
	return callErr("UpdateLayeredWindow", r, e)
}

func premultiplyInto(dst []byte, src *image.RGBA) {
	p := src.Pix
	for i := 0; i+3 < len(p) && i+3 < len(dst); i += 4 {
		a := uint32(p[i+3])
		dst[i+0] = byte(uint32(p[i+2]) * a / 255)
		dst[i+1] = byte(uint32(p[i+1]) * a / 255)
		dst[i+2] = byte(uint32(p[i+0]) * a / 255)
		dst[i+3] = byte(a)
	}
}

func (s *LayeredSurface) Close() {
	procSelectObject.Call(s.hdc, s.old)
	procDeleteObject.Call(s.hbm)
	procDeleteDC.Call(s.hdc)
}

func ShowNoActivate(hwnd uintptr) { procShowWindow.Call(hwnd, SW_SHOWNOACTIVATE) }
func HideWindow(hwnd uintptr)     { procShowWindow.Call(hwnd, 0 /* SW_HIDE */) }
func DestroyWindow(hwnd uintptr)  { procDestroyWindow.Call(hwnd) }
