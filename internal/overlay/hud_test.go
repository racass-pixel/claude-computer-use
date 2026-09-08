package overlay

import (
	"image/color"
	"testing"
)

func TestCaptionForRu(t *testing.T) {
	v, d := captionFor("ru", "click|640,412")
	if v != "Кликаю" || d != "640,412" {
		t.Fatalf("ru click: got %q / %q", v, d)
	}
}

func TestCaptionForEn(t *testing.T) {
	v, d := captionFor("en", "type|\"hello\"")
	if v != "Typing" || d != "\"hello\"" {
		t.Fatalf("en type: got %q / %q", v, d)
	}
}

func TestCaptionForUnknownTool(t *testing.T) {
	v, d := captionFor("en", "newtool|stuff")
	if v != "newtool|stuff" || d != "" {
		t.Fatalf("unknown tool: got %q / %q", v, d)
	}
}

func TestCaptionForLegacy(t *testing.T) {
	v, d := captionFor("en", "click 640,412")
	if v != "click 640,412" || d != "" {
		t.Fatalf("legacy: got %q / %q", v, d)
	}
}

func TestCaptionForEmpty(t *testing.T) {
	v, d := captionFor("en", "")
	if v != "" || d != "" {
		t.Fatal("empty should return empty")
	}
}

func TestCaptionForControl(t *testing.T) {
	v, d := captionFor("en", "control|acquire")
	if v != "" || d != "" {
		t.Fatal("control should be silent")
	}
}

func TestCaptionForFallbackLang(t *testing.T) {
	v, _ := captionFor("xx", "click|here")
	if v != "Clicking" {
		t.Fatalf("fallback lang should use en: got %q", v)
	}
}

func TestHudTextLocalization(t *testing.T) {
	l1, l2 := hudText("ru", "Esc Esc", "", "click|640,412", false)
	if l1 != "Claude" {
		t.Fatalf("expected default title Claude, got %q", l1)
	}
	if l2 != "Кликаю · 640,412" {
		t.Fatalf("ru action: %q", l2)
	}
	l1, l2 = hudText("en", "Ctrl+Alt+Esc", "Filling the order form", "", false)
	if l1 != "Filling the order form" || l2 != "" {
		t.Fatalf("en custom title: %q / %q", l1, l2)
	}
	l1, l2 = hudText("en", "Esc Esc", "ignored while paused", "ignored", true)
	if l1 != "You are in control" {
		t.Fatalf("paused title: %q", l1)
	}
	if l1, _ := hudText("xx", "Esc Esc", "", "", false); l1 != "Claude" {
		t.Fatalf("unknown lang must fall back: %q", l1)
	}
}

func TestRenderHUDProducesAPill(t *testing.T) {
	img := renderHUD(hudSpec{
		Title:       "Demo: Claude Computer Use",
		Sub:         "Clicking · 640,412",
		HotkeyLabel: "Esc Esc",
		Lang:        "en",
		Scale:       1,
		Accent:      color.RGBA{R: 217, G: 119, B: 87, A: 255},
		SparkScale:  1,
	})
	b := img.Bounds()
	if b.Dx() < 200 || b.Dy() < 40 {
		t.Fatalf("unexpected HUD size %v", b)
	}
	if img.RGBAAt(0, 0).A != 0 {
		t.Fatal("pill corners must be transparent")
	}
	if img.RGBAAt(b.Dx()/2, b.Dy()/2).A < 200 {
		t.Fatal("pill body must be nearly opaque")
	}
}

func TestRenderHUDAlphaMultiplier(t *testing.T) {
	full := renderHUD(hudSpec{
		Title:       "Test",
		Sub:         "Clicking · here",
		HotkeyLabel: "Esc Esc",
		Lang:        "en",
		Scale:       1,
		Accent:      color.RGBA{R: 217, G: 119, B: 87, A: 255},
	})
	half := renderHUD(hudSpec{
		Title:       "Test",
		Sub:         "Clicking · here",
		HotkeyLabel: "Esc Esc",
		Lang:        "en",
		Scale:       1,
		Accent:      color.RGBA{R: 217, G: 119, B: 87, A: 255},
		Alpha:       0.5,
	})
	var maxFull, maxHalf uint8
	for i := 3; i < len(full.Pix); i += 4 {
		if full.Pix[i] > maxFull {
			maxFull = full.Pix[i]
		}
	}
	for i := 3; i < len(half.Pix); i += 4 {
		if half.Pix[i] > maxHalf {
			maxHalf = half.Pix[i]
		}
	}
	ratio := float64(maxHalf) / float64(maxFull)
	if ratio < 0.4 || ratio > 0.6 {
		t.Fatalf("alpha 0.5 ratio = %.2f (full=%d half=%d)", ratio, maxFull, maxHalf)
	}
}

func TestRenderHUDKeyCaps(t *testing.T) {
	img := renderHUD(hudSpec{
		Title:       "Test",
		Sub:         "",
		HotkeyLabel: "Esc Esc",
		Lang:        "en",
		Scale:       1,
		Accent:      color.RGBA{R: 217, G: 119, B: 87, A: 255},
	})
	b := img.Bounds()
	found := false
	for y := b.Dy() / 2; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := img.RGBAAt(x, y)
			if c.A > 200 && c.R >= 0x35 && c.R <= 0x40 && c.G >= 0x33 && c.G <= 0x3E {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("no key-cap background pixel found in bottom half of HUD")
	}
}

func TestRippleFadesOut(t *testing.T) {
	a := renderRipple(120, 0.1, color.RGBA{R: 255, A: 255})
	z := renderRipple(120, 1.0, color.RGBA{R: 255, A: 255})
	var sumA, sumZ int
	for i := 3; i < len(a.Pix); i += 4 {
		sumA += int(a.Pix[i])
		sumZ += int(z.Pix[i])
	}
	if sumA == 0 || sumZ != 0 {
		t.Fatalf("ripple alpha early=%d late=%d", sumA, sumZ)
	}
}
