package overlay

import (
	"image"
	"image/color"
	"math"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type hudStrings struct{ Title, Sub, PausedTitle, PausedSub string }

var texts = map[string]hudStrings{
	"en": {"Claude is controlling the computer", "%s — take control", "You are in control", "%s — hand back to Claude"},
	"ru": {"Claude управляет компьютером", "%s — забрать управление", "Управление у вас", "%s — вернуть Claude"},
}

func hudText(lang, hotkeyLabel, title, action string, paused bool) (string, string) {
	s, ok := texts[lang]
	if !ok {
		s = texts["en"]
	}
	if paused {
		return s.PausedTitle, sprintf(s.PausedSub, hotkeyLabel)
	}
	l1 := s.Title
	if title != "" {
		l1 = title
	}
	l2 := sprintf(s.Sub, hotkeyLabel)
	if action != "" {
		l2 += "  ·  " + action
	}
	return l1, l2
}

func sprintf(format, a string) string { // avoid fmt for the hot path; format has exactly one %s
	out := make([]byte, 0, len(format)+len(a))
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) && format[i+1] == 's' {
			out = append(out, a...)
			i++
			continue
		}
		out = append(out, format[i])
	}
	return string(out)
}

type hudSpec struct {
	Title, Sub string
	Scale      float64
	Accent     color.RGBA
	Paused     bool
	Pulse      float64
}

var (
	fontOnce sync.Once
	fontMed  *opentype.Font
	fontReg  *opentype.Font
)

func loadFonts() {
	fontMed, _ = opentype.Parse(gomedium.TTF)
	fontReg, _ = opentype.Parse(goregular.TTF)
}

func face(f *opentype.Font, px float64) font.Face {
	fc, _ := opentype.NewFace(f, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingFull})
	return fc
}

func textWidth(fc font.Face, s string) int { return font.MeasureString(fc, s).Ceil() }

func drawText(img *image.RGBA, fc font.Face, x, baseline int, s string, c color.RGBA) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: fc, Dot: fixed.P(x, baseline)}
	d.DrawString(s)
}

// renderHUD draws the pill: [dot] Title / Sub, dark translucent background, 1px light border.
func renderHUD(spec hudSpec) *image.RGBA {
	fontOnce.Do(loadFonts)
	sc := spec.Scale
	if sc <= 0 {
		sc = 1
	}
	titleFace := face(fontMed, 15*sc)
	subFace := face(fontReg, 12.5*sc)
	defer titleFace.Close()
	defer subFace.Close()

	padX, padY := int(16*sc), int(10*sc)
	dot := int(8 * sc)
	gap := int(9 * sc)
	lineGap := int(4 * sc)
	tw := max(textWidth(titleFace, spec.Title), textWidth(subFace, spec.Sub))
	titleH := titleFace.Metrics().Height.Ceil()
	subH := subFace.Metrics().Height.Ceil()
	w := padX*2 + dot + gap + tw
	h := padY*2 + titleH + lineGap + subH
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	radius := 14 * sc
	bg := color.RGBA{R: 22, G: 22, B: 24, A: 222}
	if spec.Paused {
		bg = color.RGBA{R: 44, G: 44, B: 46, A: 222}
	}
	fillRoundedRect(img, img.Bounds(), radius, color.RGBA{R: 255, G: 255, B: 255, A: 28}) // hairline border
	fillRoundedRect(img, image.Rect(1, 1, w-1, h-1), radius-1, bg)

	dotC := spec.Accent
	if spec.Paused {
		dotC = pausedColor
	}
	dotC.A = uint8(120 + 135*math.Max(0, math.Min(1, spec.Pulse)))
	cy := float64(padY) + float64(titleH)/2
	strokeRing(img, float64(padX)+float64(dot)/2, cy, float64(dot)/2, float64(dot), dotC) // a filled disc: width == diameter

	x := padX + dot + gap
	drawText(img, titleFace, x, padY+titleFace.Metrics().Ascent.Ceil(), spec.Title, color.RGBA{R: 245, G: 245, B: 245, A: 255})
	drawText(img, subFace, x, padY+titleH+lineGap+subFace.Metrics().Ascent.Ceil(), spec.Sub, color.RGBA{R: 200, G: 200, B: 205, A: 255})
	return img
}
