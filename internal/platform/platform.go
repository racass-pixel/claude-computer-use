package platform

import (
	"image"

	"github.com/racass-pixel/claude-computer-use/internal/geom"
)

type Monitor struct {
	ID          int       `json:"id"`           // 1-based; primary first, then left-to-right, top-to-bottom
	Name        string    `json:"name"`         // e.g. `\\.\DISPLAY1`
	Rect        geom.Rect `json:"rect"`         // physical pixels, virtual-screen coordinates
	Work        geom.Rect `json:"work"`         // minus taskbar
	ScaleFactor float64   `json:"scale_factor"` // DPI / 96
	Primary     bool      `json:"primary"`
}

type WindowState string

const (
	WindowNormal    WindowState = "normal"
	WindowMinimized WindowState = "minimized"
	WindowMaximized WindowState = "maximized"
)

type WindowInfo struct {
	ID         uintptr     `json:"id"` // HWND
	Title      string      `json:"title"`
	Process    string      `json:"process"` // "notepad.exe"
	PID        uint32      `json:"pid"`
	Rect       geom.Rect   `json:"rect"` // visible frame bounds
	State      WindowState `json:"state"`
	Foreground bool        `json:"is_foreground"`
}

type MouseButton string

const (
	ButtonLeft   MouseButton = "left"
	ButtonRight  MouseButton = "right"
	ButtonMiddle MouseButton = "middle"
)

// Screen reads monitors and pixels.
type Screen interface {
	Monitors() ([]Monitor, error)
	Capture(r geom.Rect) (*image.RGBA, error) // r in screen coordinates
	CursorPos() (geom.Point, error)
}

// Input injects mouse and keyboard events. vk are Windows virtual-key codes.
type Input interface {
	MouseMove(p geom.Point) error
	MouseDown(b MouseButton) error
	MouseUp(b MouseButton) error
	Scroll(dx, dy int) error // wheel ticks; dy>0 scrolls down, dx>0 scrolls right
	KeyDown(vk uint16) error
	KeyUp(vk uint16) error
	TypeUnicode(s string) error // types s as unicode key events; \n → Enter, \t → Tab
}

type Clipboard interface {
	GetText() (string, error)
	SetText(s string) error
}

type Windows interface {
	List() ([]WindowInfo, error) // visible, titled, non-tool top-level windows
	Foreground() (WindowInfo, error)
	Focus(id uintptr) error
	SetState(id uintptr, s WindowState) error
	Close(id uintptr) error
	Move(id uintptr, r geom.Rect) error
}

// Element is a UI Automation element snapshot. Ref is an opaque handle valid until Release.
type Element struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Role         string    `json:"role"` // "Button", "Edit", "MenuItem", ...
	AutomationID string    `json:"automation_id,omitempty"`
	Value        string    `json:"value,omitempty"`
	Rect         geom.Rect `json:"-"` // screen coordinates
	Enabled      bool      `json:"enabled"`
	Focused      bool      `json:"focused"`
	Offscreen    bool      `json:"offscreen,omitempty"`
	Ref          any       `json:"-"`
}

type FindQuery struct {
	Name         string  // case-insensitive regexp on Name; empty = any
	Role         string  // control type name; empty = any
	AutomationID string  // exact; empty = any
	Window       uintptr // HWND to search under; 0 = whole desktop
	Limit        int     // max results (default 25)
}

type Accessibility interface {
	Find(q FindQuery) ([]Element, error)
	Rect(ref any) (geom.Rect, error) // fresh bounding rect for a Ref from Find
	Release(refs []any)
}

type OverlayState int

const (
	OverlayHidden OverlayState = iota
	OverlayControlling
	OverlayPaused
)

type Overlay interface {
	Show(m Monitor, s OverlayState)
	Hide()
	SetTitle(title string)   // task title line; empty = default localized title
	SetAction(action string) // "click 640,412"; shown after the hotkey hint
	Ripple(p geom.Point)     // screen coordinates
	Close()
}

// NopOverlay is used when the overlay is disabled or not yet implemented.
type NopOverlay struct{}

func (NopOverlay) Show(Monitor, OverlayState) {}
func (NopOverlay) Hide()                      {}
func (NopOverlay) SetTitle(string)            {}
func (NopOverlay) SetAction(string)           {}
func (NopOverlay) Ripple(geom.Point)          {}
func (NopOverlay) Close()                     {}
