package overlay

import "image/color"

// Config is what the overlay needs from the app config.
type Config struct {
	Accent      color.RGBA
	Lang        string // "ru" | "en"
	HotkeyLabel string // "Esc Esc"
	Thick       int    // logical px, default 40
}

var pausedColor = color.RGBA{R: 154, G: 154, B: 154, A: 255}
