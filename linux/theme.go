package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed assets/heart64.png
var heartPNG []byte

const defaultAccent = "#ff6fae"

// Palette colours are 0xRRGGBB, all derived from one accent hex.
type Palette struct {
	Accent, AccentDim, Panel, Bg, Text, Secondary, Selected, Hover uint32
}

func parseHex(s string) (uint32, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return 0, fmt.Errorf("accent %q is not #rrggbb", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("accent %q is not #rrggbb", s)
	}
	return uint32(v), nil
}

func hex(c uint32) string { return fmt.Sprintf("#%06x", c) }

// mix moves a toward b by t (0..1) per channel.
func mix(a, b uint32, t float64) uint32 {
	var out uint32
	for sh := 0; sh <= 16; sh += 8 {
		x, y := float64(a>>sh&0xFF), float64(b>>sh&0xFF)
		out |= uint32(math.Round(x+(y-x)*t)) << sh
	}
	return out
}

func toHSL(c uint32) (h, s, l float64) {
	r, g, b := float64(c>>16&0xFF)/255, float64(c>>8&0xFF)/255, float64(c&0xFF)/255
	mx, mn := max(r, g, b), min(r, g, b)
	l = (mx + mn) / 2
	d := mx - mn
	if d == 0 {
		return 0, 0, l
	}
	s = d / (1 - math.Abs(2*l-1))
	switch mx {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return math.Mod(h*60+360, 360), s, l
}

func fromHSL(h, s, l float64) uint32 {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g = c, x
	case h < 120:
		r, g = x, c
	case h < 180:
		g, b = c, x
	case h < 240:
		g, b = x, c
	case h < 300:
		r, b = x, c
	default:
		r, b = c, x
	}
	ch := func(v float64) uint32 { return uint32(math.Round(max(0, min(1, v+m)) * 255)) }
	return ch(r)<<16 | ch(g)<<8 | ch(b)
}

func derivePalette(accent uint32) Palette {
	h, s, _ := toHSL(accent)
	p := Palette{
		Accent:    accent,
		AccentDim: mix(accent, 0x000000, 0.55),
		Panel:     fromHSL(h, s*0.35, 0.07),
		Bg:        fromHSL(h, s*0.30, 0.04),
		Text:      mix(accent, 0xffffff, 0.85),
	}
	// secondary text (chip labels, counts, idle tags at 11 px) must stay legible. The original
	// 45%-toward-panel mix measured 2.99:1; blending 40% of the way from the accent toward the
	// body text keeps the hue and clears 4.5:1 for both test accents.
	p.Secondary = mix(accent, p.Text, 0.40)
	p.Selected = mix(p.Panel, accent, 0.20)
	p.Hover = mix(p.Panel, accent, 0.10)
	return p
}

// luminance is the WCAG relative luminance.
func luminance(c uint32) float64 {
	lin := func(v uint32) float64 {
		x := float64(v) / 255
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c>>16&0xFF) + 0.7152*lin(c>>8&0xFF) + 0.0722*lin(c&0xFF)
}

func contrast(a, b uint32) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// css styles the picker and settings dialog; names are the widget names/classes set in ui.c.
func (p Palette) css() string {
	return fmt.Sprintf(`
#kp-root { background: %[2]s; }
#kp-panel { background: %[3]s; border: 1px solid %[4]s; border-radius: 10px; padding: 12px; }
#kp-panel label, #kp-settings label { color: %[5]s; }
#kp-title { font-weight: bold; font-size: 15pt; }
#kp-count, .kp-tags { color: %[6]s; }
.kp-tags { font-size: 8pt; }
#kp-panel entry, #kp-settings entry { background: %[2]s; color: %[5]s; caret-color: %[1]s; border: 1px solid %[1]s; border-radius: 6px; box-shadow: none; }
#kp-panel scrolledwindow, #kp-panel viewport, #kp-panel flowbox { background: transparent; border: none; }
button.kp-chip { background: %[8]s; color: %[6]s; border: 1px solid %[4]s; border-radius: 12px; padding: 2px 10px; box-shadow: none; text-shadow: none; -gtk-icon-shadow: none; }
button.kp-chip label { color: %[6]s; }
button.kp-chip.kp-on, button.kp-chip.kp-on:hover { background: %[7]s; border-color: %[1]s; }
button.kp-chip.kp-on label { color: %[5]s; }
flowboxchild { border-radius: 8px; padding: 0; border: 1px solid transparent; }
flowboxchild:hover { background: %[8]s; }
flowboxchild:selected { background: %[7]s; border-color: %[1]s; }
.kp-face { font-family: "Noto Sans Mono CJK JP", "Noto Sans CJK JP", monospace; font-size: 15pt; }
.kp-star { color: %[1]s; font-size: 11pt; margin: 4px 6px; }
#kp-settings { background: %[3]s; }
`, hex(p.Accent), hex(p.Bg), hex(p.Panel), hex(p.AccentDim), hex(p.Text), hex(p.Secondary), hex(p.Selected), hex(p.Hover))
}

// tintHeart repaints the heart in the accent colour, keeping its alpha channel as the mask.
func tintHeart(accent uint32) (*image.NRGBA, error) {
	src, err := png.Decode(bytes.NewReader(heartPNG))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	out := image.NewNRGBA(b)
	r, g, bl := uint8(accent>>16), uint8(accent>>8), uint8(accent)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			a := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA).A
			out.SetNRGBA(x, y, color.NRGBA{r, g, bl, a})
		}
	}
	return out, nil
}

// writeTrayIcon writes the tinted heart to $XDG_RUNTIME_DIR and returns its path. The name
// carries the hex so a colour change is a new path, which makes tray hosts reload the icon.
func writeTrayIcon(accent uint32) (string, error) {
	img, err := tintHeart(accent)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(runtimeDir(), "kaomoji-picker")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "heart-"+hex(accent)[1:]+".png")
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, buf.Bytes(), 0o644)
}

func runtimeDir() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return d
	}
	return os.TempDir()
}

type paletteReport struct {
	Colors   map[string]string  `json:"colors"`
	Contrast map[string]float64 `json:"contrast"`
	Floors   map[string]float64 `json:"floors"`
	OK       bool               `json:"ok"`
}

// Contrast floors: body text AAA, accent AA, secondary AA (it is small text).
var contrastFloors = map[string]float64{"text/panel": 7, "text/bg": 7, "accent/panel": 4.5, "secondary/panel": 4.5}

func checkPalette(accent uint32) (paletteReport, error) {
	p := derivePalette(accent)
	r := paletteReport{
		Colors: map[string]string{"accent": hex(p.Accent), "accent_dim": hex(p.AccentDim), "panel": hex(p.Panel), "bg": hex(p.Bg),
			"text": hex(p.Text), "secondary": hex(p.Secondary), "selected": hex(p.Selected), "hover": hex(p.Hover)},
		Contrast: map[string]float64{},
		Floors:   contrastFloors,
		OK:       true,
	}
	pairs := map[string][2]uint32{"text/panel": {p.Text, p.Panel}, "text/bg": {p.Text, p.Bg},
		"accent/panel": {p.Accent, p.Panel}, "secondary/panel": {p.Secondary, p.Panel}}
	var bad []string
	for k, c := range pairs {
		v := math.Round(contrast(c[0], c[1])*100) / 100
		r.Contrast[k] = v
		if v < contrastFloors[k] {
			bad = append(bad, fmt.Sprintf("%s %.2f < %.1f", k, v, contrastFloors[k]))
		}
	}
	// panel and bg must stay dark and ordered, or the derivation is broken
	if !(luminance(p.Bg) < luminance(p.Panel) && luminance(p.Panel) < 0.02 && p.Accent == accent) {
		bad = append(bad, "panel/bg not dark or not ordered")
	}
	if len(bad) > 0 {
		r.OK = false
		return r, fmt.Errorf("palette %s: %s", hex(accent), strings.Join(bad, "; "))
	}
	return r, nil
}
