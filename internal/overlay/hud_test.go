package overlay

import (
	"image/color"
	"testing"
)

func TestHudTextLocalization(t *testing.T) {
	l1, l2 := hudText("ru", "Esc Esc", "", "click 1,2", false)
	if l1 != "Claude управляет компьютером" || l2 != "Esc Esc — забрать управление  ·  click 1,2" {
		t.Fatalf("ru: %q / %q", l1, l2)
	}
	l1, l2 = hudText("en", "Ctrl+Alt+Esc", "Filling the order form", "", false)
	if l1 != "Filling the order form" || l2 != "Ctrl+Alt+Esc — take control" {
		t.Fatalf("en custom title: %q / %q", l1, l2)
	}
	l1, l2 = hudText("en", "Esc Esc", "ignored while paused", "ignored", true)
	if l1 != "You are in control" || l2 != "Esc Esc — hand back to Claude" {
		t.Fatalf("paused: %q / %q", l1, l2)
	}
	if l1, _ := hudText("xx", "Esc Esc", "", "", false); l1 != "Claude is controlling the computer" {
		t.Fatalf("unknown lang must fall back to en: %q", l1)
	}
}

func TestRenderHUDProducesAPill(t *testing.T) {
	img := renderHUD(hudSpec{Title: "Claude управляет компьютером", Sub: "Esc Esc — забрать управление", Scale: 1, Accent: color.RGBA{R: 217, G: 119, B: 87, A: 255}, Pulse: 1})
	b := img.Bounds()
	if b.Dx() < 200 || b.Dy() < 40 || b.Dx() < b.Dy()*3 {
		t.Fatalf("unexpected HUD size %v", b)
	}
	if img.RGBAAt(0, 0).A != 0 {
		t.Fatal("pill corners must be transparent")
	}
	if img.RGBAAt(b.Dx()/2, b.Dy()/2).A < 200 {
		t.Fatal("pill body must be nearly opaque")
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
