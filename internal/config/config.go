// Package config loads cu settings: defaults ← %APPDATA%\claude-computer-use\config.json ← CU_* env.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Hotkey             string  `json:"hotkey"`
	AutoPause          bool    `json:"auto_pause"`
	MouseThresholdPx   int     `json:"mouse_threshold_px"`
	ScreenshotLongEdge int     `json:"screenshot_long_edge"`
	ScreenshotFormat   string  `json:"screenshot_format"`
	JPEGQuality        int     `json:"jpeg_quality"`
	Lang               string  `json:"lang"`
	Accent             string  `json:"accent"`
	Overlay            bool    `json:"overlay"`
	IdleReleaseMs      int     `json:"idle_release_ms"`
	PauseWaitMs        int     `json:"pause_wait_ms"`
	PasteThreshold     int     `json:"paste_threshold"`
	BorderThickness    int     `json:"border_thickness"`
	BorderIntensity    float64 `json:"border_intensity"`
	MouseGlideMs       int     `json:"mouse_glide_ms"`
	LogFile            string  `json:"log_file"`
}

func Default() Config {
	return Config{
		Hotkey: "esc esc", AutoPause: false, MouseThresholdPx: 12,
		ScreenshotLongEdge: 1366, ScreenshotFormat: "png", JPEGQuality: 85,
		Lang: "auto", Accent: "#D97757", Overlay: true,
		IdleReleaseMs: 120000, PauseWaitMs: 20000, PasteThreshold: 200,
		BorderThickness: 56, BorderIntensity: 0.85, MouseGlideMs: 220,
	}
}

func Path() (string, error) {
	dir, err := os.UserConfigDir() // %APPDATA% on Windows
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "claude-computer-use", "config.json"), nil
}

// Load never fails on a missing file; a malformed file or env value is an error.
func Load() (Config, error) {
	c := Default()
	p, err := Path()
	if err == nil {
		if data, rerr := os.ReadFile(p); rerr == nil {
			if err := applyFile(&c, data); err != nil {
				return c, fmt.Errorf("%s: %w", p, err)
			}
		} else if !errors.Is(rerr, os.ErrNotExist) {
			return c, rerr
		}
	}
	if err := applyEnv(&c, os.Getenv); err != nil {
		return c, err
	}
	return c, c.validate()
}

func applyFile(c *Config, data []byte) error {
	if err := json.Unmarshal(data, c); err != nil {
		return err
	}
	return c.validate()
}

func applyEnv(c *Config, getenv func(string) string) error {
	str := func(key string, dst *string) {
		if v := getenv(key); v != "" {
			*dst = v
		}
	}
	num := func(key string, dst *int) error {
		v := getenv(key)
		if v == "" {
			return nil
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s: %q is not a number", key, v)
		}
		*dst = n
		return nil
	}
	boolean := func(key string, dst *bool) error {
		v := getenv(key)
		if v == "" {
			return nil
		}
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("%s: %q is not a boolean", key, v)
		}
		*dst = b
		return nil
	}
	float := func(key string, dst *float64) error {
		v := getenv(key)
		if v == "" {
			return nil
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("%s: %q is not a number", key, v)
		}
		*dst = f
		return nil
	}
	str("CU_HOTKEY", &c.Hotkey)
	str("CU_SCREENSHOT_FORMAT", &c.ScreenshotFormat)
	str("CU_LANG", &c.Lang)
	str("CU_ACCENT", &c.Accent)
	str("CU_LOG_FILE", &c.LogFile)
	for _, e := range []error{
		boolean("CU_AUTO_PAUSE", &c.AutoPause),
		boolean("CU_OVERLAY", &c.Overlay),
		num("CU_MOUSE_THRESHOLD_PX", &c.MouseThresholdPx),
		num("CU_SCREENSHOT_LONG_EDGE", &c.ScreenshotLongEdge),
		num("CU_JPEG_QUALITY", &c.JPEGQuality),
		num("CU_IDLE_RELEASE_MS", &c.IdleReleaseMs),
		num("CU_PAUSE_WAIT_MS", &c.PauseWaitMs),
		num("CU_PASTE_THRESHOLD", &c.PasteThreshold),
		num("CU_BORDER_THICKNESS", &c.BorderThickness),
		num("CU_MOUSE_GLIDE_MS", &c.MouseGlideMs),
		float("CU_BORDER_INTENSITY", &c.BorderIntensity),
	} {
		if e != nil {
			return e
		}
	}
	return c.validate()
}

func (c Config) validate() error {
	switch c.ScreenshotFormat {
	case "png", "jpeg", "jpg":
	default:
		return fmt.Errorf("screenshot_format must be png or jpeg, got %q", c.ScreenshotFormat)
	}
	switch c.Lang {
	case "auto", "ru", "en":
	default:
		return fmt.Errorf("lang must be auto, ru or en, got %q", c.Lang)
	}
	return nil
}

// AccentRGB parses "#RRGGBB".
func (c Config) AccentRGB() (r, g, b uint8, err error) {
	s := strings.TrimPrefix(c.Accent, "#")
	if len(s) != 6 {
		return 0, 0, 0, fmt.Errorf("accent must be #RRGGBB, got %q", c.Accent)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("accent must be #RRGGBB, got %q", c.Accent)
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v), nil
}
