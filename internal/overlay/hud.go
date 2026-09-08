package overlay

import (
	"image"
	"image/color"
	"math"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// ---- Claude palette ----

var (
	hudBG          = color.RGBA{R: 0x1F, G: 0x1E, B: 0x1B, A: 235} // warm dark, alpha ~0.92
	hudBGPaused    = color.RGBA{R: 0x2A, G: 0x29, B: 0x26, A: 235}
	hudHairline    = color.RGBA{R: 0xD9, G: 0x77, B: 0x57, A: 89} // accent at alpha 0.35
	hudHairlinePsd = color.RGBA{R: 0x8A, G: 0x87, B: 0x80, A: 89}
	hudTitleColor  = color.RGBA{R: 0xF4, G: 0xF3, B: 0xEE, A: 255} // cream
	hudSubColor    = color.RGBA{R: 0xB1, G: 0xAD, B: 0xA1, A: 255} // warm gray
	hudDetailColor = color.RGBA{R: 0x8A, G: 0x87, B: 0x80, A: 255} // muted detail
	hudCapBG       = color.RGBA{R: 0x3A, G: 0x38, B: 0x35, A: 255}
	hudCapText     = color.RGBA{R: 0xF4, G: 0xF3, B: 0xEE, A: 255}
)

// ---- localisation ----

type hudStrings struct {
	PausedTitle, PausedSub string
	TakeControl            string // key-cap hint
	HandBack               string
}

var texts = map[string]hudStrings{
	"en": {"You are in control", "Press to hand back to Claude", "take control", "hand back to Claude"},
	"ru": {
		PausedTitle: "Управление у вас",
		PausedSub:   "Нажмите, чтобы вернуть Claude",
		TakeControl: "забрать управление",
		HandBack:    "вернуть Claude",
	},
}

// captionVerbs maps tool key to localized verb.
var captionVerbs = map[string]map[string]string{
	"en": {
		"click": "Clicking", "move": "Moving", "drag": "Dragging",
		"scroll": "Scrolling", "type": "Typing", "key": "Pressing",
		"window": "Switching window", "find": "Finding", "wait": "Waiting",
		"screenshot": "Looking", "batch": "Running a batch",
		"click_until": "Grinding a list", "recipe": "Running a recipe",
	},
	"ru": {
		"click":       "Кликаю",
		"move":        "Веду курсор",
		"drag":        "Перетаскиваю",
		"scroll":      "Прокручиваю",
		"type":        "Печатаю",
		"key":         "Нажимаю",
		"window":      "Переключаю окно",
		"find":        "Ищу элемент",
		"wait":        "Жду",
		"screenshot":  "Смотрю",
		"batch":       "Выполняю серию",
		"click_until": "Перебираю список",
		"recipe":      "Выполняю рецепт",
	},
}

// captionFor parses "tool|detail" into (verb, detail) using the lang verb table.
// Unknown tools or legacy free-form summaries are returned as-is.
func captionFor(lang, summary string) (verb, detail string) {
	if summary == "" {
		return "", ""
	}
	pipe := strings.IndexByte(summary, '|')
	if pipe < 0 {
		return summary, ""
	}
	tool := summary[:pipe]
	detail = summary[pipe+1:]
	if tool == "control" {
		return "", ""
	}
	verbs, ok := captionVerbs[lang]
	if !ok {
		verbs = captionVerbs["en"]
	}
	v, ok := verbs[tool]
	if !ok {
		return summary, ""
	}
	return v, detail
}

func hudText(lang, hotkeyLabel, title, action string, paused bool) (string, string) {
	s, ok := texts[lang]
	if !ok {
		s = texts["en"]
	}
	if paused {
		return s.PausedTitle, s.PausedSub
	}
	l1 := title
	if l1 == "" {
		l1 = "Claude"
	}
	verb, detail := captionFor(lang, action)
	l2 := verb
	if detail != "" && l2 != "" {
		l2 += " · " + detail
	}
	_ = hotkeyLabel
	return l1, l2
}

type hudSpec struct {
	Title, Sub  string
	HotkeyLabel string
	Lang        string
	Scale       float64
	Accent      color.RGBA
	Paused      bool
	SparkPhase  float64
	SparkScale  float64 // breathing multiplier 0.92..1.08; 0 means 1
	Alpha       float64 // overall alpha multiplier 0..1; 0 means 1
}

// ---- font cache (per-scale, avoids per-frame allocation) ----

var (
	fontOnce sync.Once
	fontMed  *opentype.Font
	fontReg  *opentype.Font

	faceMu    sync.Mutex
	faceCache map[faceKey]font.Face
)

type faceKey struct {
	font *opentype.Font
	px10 int // px * 10
}

func loadFonts() {
	fontMed, _ = opentype.Parse(gomedium.TTF)
	fontReg, _ = opentype.Parse(goregular.TTF)
	faceCache = make(map[faceKey]font.Face, 8)
}

func cachedFace(f *opentype.Font, px float64) font.Face {
	fontOnce.Do(loadFonts)
	key := faceKey{font: f, px10: int(math.Round(px * 10))}
	faceMu.Lock()
	defer faceMu.Unlock()
	if fc, ok := faceCache[key]; ok {
		return fc
	}
	fc, _ := opentype.NewFace(f, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingFull})
	faceCache[key] = fc
	return fc
}

func textWidth(fc font.Face, s string) int { return font.MeasureString(fc, s).Ceil() }

func drawText(img *image.RGBA, fc font.Face, x, baseline int, s string, c color.RGBA) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(c), Face: fc, Dot: fixed.P(x, baseline)}
	d.DrawString(s)
}

// renderHUD draws the Claude-style pill HUD.
// Layout: line 1 = [spark] [title]; line 2 = [action verb + detail] ... [Esc][Esc] hint
func renderHUD(spec hudSpec) *image.RGBA {
	fontOnce.Do(loadFonts)
	sc := spec.Scale
	if sc <= 0 {
		sc = 1
	}
	alpha := spec.Alpha
	if alpha <= 0 {
		alpha = 1
	}

	titleFace := cachedFace(fontMed, 15*sc)
	subFace := cachedFace(fontReg, 12.5*sc)
	capFace := cachedFace(fontReg, 11*sc)

	padX, padY := int(16*sc), int(10*sc)
	sparkSize := int(22 * sc)
	sparkGap := int(9 * sc)
	lineGap := int(5 * sc)

	tw := textWidth(titleFace, spec.Title)
	titleH := titleFace.Metrics().Height.Ceil()
	subH := subFace.Metrics().Height.Ceil()

	s, ok := texts[spec.Lang]
	if !ok {
		s = texts["en"]
	}
	hint := s.TakeControl
	if spec.Paused {
		hint = s.HandBack
	}

	keys := strings.Fields(spec.HotkeyLabel)
	if len(keys) == 0 {
		keys = []string{"Esc", "Esc"}
	}
	capPadX := int(6 * sc)
	capPadY := int(3 * sc)
	capH := capFace.Metrics().Height.Ceil() + capPadY*2
	capRadius := 4 * sc
	capGap := int(4 * sc)
	hintGap := int(6 * sc)

	capsWidth := 0
	var capWidths []int
	for _, k := range keys {
		kw := textWidth(capFace, k) + capPadX*2
		capWidths = append(capWidths, kw)
		capsWidth += kw + capGap
	}
	capsWidth += hintGap + textWidth(capFace, hint)

	subTW := 0
	if spec.Sub != "" {
		subTW = textWidth(subFace, spec.Sub)
	}
	subGapPx := int(20 * sc)
	line2W := subTW
	if subTW > 0 {
		line2W += subGapPx
	}
	line2W += capsWidth

	line1W := sparkSize + sparkGap + tw
	contentW := line1W
	if line2W > contentW {
		contentW = line2W
	}
	w := padX*2 + contentW
	h := padY*2 + titleH + lineGap + max(subH, capH)

	img := image.NewRGBA(image.Rect(0, 0, w, h))

	radius := 14 * sc
	bg := hudBG
	hairline := hudHairline
	if spec.Paused {
		bg = hudBGPaused
		hairline = hudHairlinePsd
	}

	fillRoundedRect(img, img.Bounds(), radius, hairline)
	fillRoundedRect(img, image.Rect(1, 1, w-1, h-1), radius-1, bg)

	// Spark
	sparkR := float64(sparkSize) / 2
	sparkCX := float64(padX) + sparkR
	sparkCY := float64(padY) + float64(titleH)/2
	sparkCol := spec.Accent
	sparkPhase := spec.SparkPhase
	sparkScale := spec.SparkScale
	if sparkScale <= 0 {
		sparkScale = 1
	}
	if spec.Paused {
		sparkCol = pausedColor
		sparkPhase = 0
		sparkScale = 1
	}
	renderSpark(img, sparkCX, sparkCY, sparkR*sparkScale, sparkPhase, sparkCol)

	// Title
	x := padX + sparkSize + sparkGap
	drawText(img, titleFace, x, padY+titleFace.Metrics().Ascent.Ceil(), spec.Title, hudTitleColor)

	// Line 2
	line2Y := padY + titleH + lineGap
	line2Baseline := line2Y + subFace.Metrics().Ascent.Ceil()

	if spec.Sub != "" {
		verb, detail := spec.Sub, ""
		if dot := strings.Index(spec.Sub, " · "); dot >= 0 {
			verb = spec.Sub[:dot]
			detail = spec.Sub[dot:]
		}
		drawText(img, subFace, padX, line2Baseline, verb, hudSubColor)
		if detail != "" {
			vw := textWidth(subFace, verb)
			drawText(img, subFace, padX+vw, line2Baseline, detail, hudDetailColor)
		}
	}

	// Key-caps
	capX := w - padX - capsWidth
	capBaseline := line2Y + (max(subH, capH)-capFace.Metrics().Height.Ceil())/2 + capFace.Metrics().Ascent.Ceil()
	for i, k := range keys {
		kw := capWidths[i]
		kh := capH
		ky := line2Y + (max(subH, capH)-kh)/2
		fillRoundedRect(img, image.Rect(capX, ky, capX+kw, ky+kh), capRadius, hudCapBG)
		drawText(img, capFace, capX+capPadX, capBaseline, k, hudCapText)
		capX += kw + capGap
	}
	capX += hintGap - capGap
	drawText(img, capFace, capX, capBaseline, hint, hudDetailColor)

	// Overall alpha
	if alpha < 1 {
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = uint8(float64(img.Pix[i]) * alpha)
		}
	}

	return img
}
