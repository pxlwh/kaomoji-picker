package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/png"
	"math"
	"strconv"
	"strings"
	"unsafe"
)

const defaultAccent = "#ff6fae"

// heart.png is a flat colour on transparent; only its alpha channel is used as the tray icon mask.
//
//go:embed assets/heart.png
var heartPNG []byte

// Palette holds 0xRRGGBB colours derived from one accent.
type Palette struct {
	Accent, AccentDim, Panel, Bg, Text, Secondary, Selected, Hover, Chip uint32
}

func hexStr(c uint32) string { return fmt.Sprintf("#%06x", c) }

// parseHex accepts "#rgb", "rgb", "#rrggbb" or "rrggbb", any case.
func parseHex(s string) (uint32, error) {
	h := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return 0, fmt.Errorf("%q is not a hex colour (use #rgb or #rrggbb)", s)
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("%q is not a hex colour (use #rgb or #rrggbb)", s)
	}
	return uint32(v), nil
}

// mix moves c toward to by t (0-1), per channel, truncating.
func mix(c, to uint32, t float64) uint32 {
	var out uint32
	for sh := 0; sh <= 16; sh += 8 {
		a, b := float64(c>>sh&0xFF), float64(to>>sh&0xFF)
		out |= uint32(a+(b-a)*t) << sh
	}
	return out
}

// hslDark keeps the hue of c and sets lightness l and saturation s*sat.
func hslDark(c uint32, l, sat float64) uint32 {
	r, g, b := float64(c>>16&0xFF)/255, float64(c>>8&0xFF)/255, float64(c&0xFF)/255
	mx, mn := max(r, g, b), min(r, g, b)
	var h, s float64
	if d := mx - mn; d > 0 {
		s = d / (1 - math.Abs(mx+mn-1))
		switch mx {
		case r:
			h = math.Mod((g-b)/d, 6)
		case g:
			h = (b-r)/d + 2
		default:
			h = (r-g)/d + 4
		}
		if h < 0 {
			h += 6
		}
	}
	ch := (1 - math.Abs(2*l-1)) * s * sat
	x := ch * (1 - math.Abs(math.Mod(h, 2)-1))
	m := l - ch/2
	rgb := [6][3]float64{{ch, x, 0}, {x, ch, 0}, {0, ch, x}, {0, x, ch}, {x, 0, ch}, {ch, 0, x}}[int(h)%6]
	return uint32((rgb[0]+m)*255+0.5)<<16 | uint32((rgb[1]+m)*255+0.5)<<8 | uint32((rgb[2]+m)*255+0.5)
}

// derivePalette builds every colour of the UI from one accent hex string.
func derivePalette(hex string) (Palette, error) {
	a, err := parseHex(hex)
	if err != nil {
		return Palette{}, err
	}
	p := Palette{Accent: a, AccentDim: mix(a, 0, 0.55), Panel: hslDark(a, 0.07, 0.35), Bg: hslDark(a, 0.04, 0.30), Text: mix(a, 0xffffff, 0.85)}
	// secondary text (chip labels, tag lines at 11 px) has to stay legible: 45% toward the
	// panel measured 2.99:1, so blend 40% of the way from the accent toward the body text
	// instead, which keeps the hue and clears 4.5:1 for both test accents
	p.Secondary = mix(a, p.Text, 0.40)
	p.Selected = blend(a, p.Panel, 0x33) // 20% opacity
	p.Hover = blend(a, p.Panel, 0x19)    // 10% opacity
	p.Chip = blend(p.AccentDim, p.Panel, 0x40)
	return p, nil
}

func luminance(c uint32) float64 {
	lin := func(v uint32) float64 {
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c>>16&0xFF) + 0.7152*lin(c>>8&0xFF) + 0.0722*lin(c&0xFF)
}

// contrast is the WCAG contrast ratio, 1 to 21.
func contrast(a, b uint32) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + 0.05) / (min(la, lb) + 0.05)
}

// iconBGRA box-filters the heart's alpha down to n x n and paints it in colour c (straight alpha, BGRA).
func iconBGRA(n int, c uint32) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(heartPNG))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	if n <= 0 || b.Dx() < n || b.Dy() < n {
		return nil, fmt.Errorf("icon size %d unsupported", n)
	}
	out := make([]byte, n*n*4)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			x0, x1, y0, y1 := x*b.Dx()/n, (x+1)*b.Dx()/n, y*b.Dy()/n, (y+1)*b.Dy()/n
			var sum, cnt uint32
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					_, _, _, a := src.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
					sum += a >> 8
					cnt++
				}
			}
			o := (y*n + x) * 4
			out[o], out[o+1], out[o+2], out[o+3] = byte(c), byte(c>>8), byte(c>>16), byte(sum/cnt)
		}
	}
	return out, nil
}

type bitmapInfoHeader struct {
	Size                   uint32
	Width, Height          int32
	Planes, BitCount       uint16
	Compression, SizeImage uint32
	XPels, YPels           int32
	ClrUsed, ClrImportant  uint32
}

type iconInfo struct {
	Icon         uint32
	XHot, YHot   uint32
	Mask, Colour uintptr
}

// dib32 makes an n x n top-down 32-bit DIB section holding px.
func dib32(n int, px []byte) uintptr {
	bi := bitmapInfoHeader{Width: int32(n), Height: -int32(n), Planes: 1, BitCount: 32}
	bi.Size = uint32(unsafe.Sizeof(bi))
	var bits uintptr
	h, _, _ := createDIBSection.Call(0, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if h != 0 && bits != 0 {
		copy(unsafe.Slice(ptrAt[byte](bits), n*n*4), px)
	}
	return h
}

// tintedIcon builds an HICON from the heart mask in colour c, or 0 on failure.
func tintedIcon(n int, c uint32) uintptr {
	px, err := iconBGRA(n, c)
	if err != nil {
		return 0
	}
	colour, mask := dib32(n, px), dib32(n, make([]byte, len(px)))
	defer deleteObject.Call(colour)
	defer deleteObject.Call(mask)
	if colour == 0 || mask == 0 {
		return 0
	}
	ii := iconInfo{Icon: 1, Mask: mask, Colour: colour}
	h, _, _ := createIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	return h
}

// trayIconFor returns a tinted icon at the tray size, falling back to the embedded .ico.
func trayIconFor(c uint32) uintptr {
	cx, _, _ := getSystemMetrics.Call(smCxSmIcon)
	if h := tintedIcon(int(cx), c); h != 0 {
		return h
	}
	return loadIcon(int(cx))
}

// resetGDI deletes every cached brush and font; callers re-create them lazily.
func resetGDI() {
	for k, b := range brushes {
		deleteObject.Call(b)
		delete(brushes, k)
	}
	for k, f := range fonts {
		deleteObject.Call(f)
		delete(fonts, k)
	}
}

// setPalette installs p as the live colours (no GDI work).
func setPalette(p Palette) {
	colAccent, colAccentDim, colPanel, colBg, colText, colSecondary = p.Accent, p.AccentDim, p.Panel, p.Bg, p.Text, p.Secondary
	colSelected, colHover, colChip = p.Selected, p.Hover, p.Chip
}

// applyTheme switches the running app to accent: palette, GDI caches, picker repaint and tray icon.
func applyTheme(accent string) {
	p, err := derivePalette(accent)
	if err != nil {
		return
	}
	setPalette(p)
	resetGDI()
	if pk.hwnd != 0 {
		invalidate(pk.hwnd)
	}
	if h := trayIconFor(p.Accent); h != 0 {
		old := trayIcon
		trayIcon = h
		n := trayData()
		n.Flags = nifIcon
		shellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&n)))
		if old != 0 {
			destroyIcon.Call(old)
		}
	}
}

type themeEntry struct {
	Accent    string  `json:"accent"`
	AccentDim string  `json:"accent_dim"`
	Panel     string  `json:"panel"`
	Bg        string  `json:"bg"`
	Text      string  `json:"text"`
	Secondary string  `json:"secondary"`
	Selected  string  `json:"selected"`
	Hover     string  `json:"hover"`
	Contrast  float64 `json:"contrast"`
}

const minContrast = 4.5

// selftestTheme derives the default and a blue accent and checks text-on-panel contrast.
func selftestTheme() (map[string]themeEntry, bool, error) {
	out := map[string]themeEntry{}
	for name, hex := range map[string]string{"default": defaultAccent, "blue": "#3fa9f5"} {
		p, err := derivePalette(hex)
		if err != nil {
			return nil, false, fmt.Errorf("theme %s: %w", name, err)
		}
		c := math.Round(contrast(p.Text, p.Panel)*100) / 100
		if c < minContrast {
			return nil, false, fmt.Errorf("theme %s: text/panel contrast %.2f below %.1f", name, c, minContrast)
		}
		sc := math.Round(contrast(p.Secondary, p.Panel)*100) / 100
		if sc < minContrast {
			return nil, false, fmt.Errorf("theme %s: secondary/panel contrast %.2f below %.1f", name, sc, minContrast)
		}
		out[name] = themeEntry{hexStr(p.Accent), hexStr(p.AccentDim), hexStr(p.Panel), hexStr(p.Bg), hexStr(p.Text), hexStr(p.Secondary), hexStr(p.Selected), hexStr(p.Hover), c}
	}
	return out, true, nil
}
