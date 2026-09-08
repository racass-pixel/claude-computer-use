package config

import "testing"

func TestDefaults(t *testing.T) {
	c := Default()
	if c.Hotkey != "esc esc" || !c.AutoPause || c.MouseThresholdPx != 12 || c.ScreenshotLongEdge != 1366 ||
		c.ScreenshotFormat != "png" || c.JPEGQuality != 85 || c.Lang != "auto" || c.Accent != "#D97757" ||
		!c.Overlay || c.IdleReleaseMs != 120000 || c.PauseWaitMs != 20000 || c.PasteThreshold != 200 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestFileThenEnvOverride(t *testing.T) {
	c := Default()
	if err := applyFile(&c, []byte(`{"hotkey":"ctrl+alt+esc","screenshot_long_edge":1024,"overlay":false}`)); err != nil {
		t.Fatal(err)
	}
	if c.Hotkey != "ctrl+alt+esc" || c.ScreenshotLongEdge != 1024 || c.Overlay || c.AutoPause != true {
		t.Fatalf("file merge wrong: %+v", c)
	}
	env := map[string]string{"CU_SCREENSHOT_LONG_EDGE": "1568", "CU_AUTO_PAUSE": "false", "CU_LANG": "ru", "CU_ACCENT": "#3366FF"}
	if err := applyEnv(&c, func(k string) string { return env[k] }); err != nil {
		t.Fatal(err)
	}
	if c.ScreenshotLongEdge != 1568 || c.AutoPause || c.Lang != "ru" || c.Accent != "#3366FF" {
		t.Fatalf("env override wrong: %+v", c)
	}
	r, g, b, err := c.AccentRGB()
	if err != nil || r != 0x33 || g != 0x66 || b != 0xFF {
		t.Fatalf("AccentRGB = %d %d %d %v", r, g, b, err)
	}
}

func TestInvalidValues(t *testing.T) {
	c := Default()
	if err := applyEnv(&c, func(k string) string {
		if k == "CU_SCREENSHOT_LONG_EDGE" {
			return "abc"
		}
		return ""
	}); err == nil {
		t.Fatal("non-numeric int must error")
	}
	c.Accent = "red"
	if _, _, _, err := c.AccentRGB(); err == nil {
		t.Fatal("accent must be #RRGGBB")
	}
	if err := applyFile(&c, []byte(`{"screenshot_format":"bmp"}`)); err == nil {
		t.Fatal("unknown format must error")
	}
}
