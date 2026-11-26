package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const (
	uiFace  = "Segoe UI"
	symFace = "Segoe UI Symbol" // has the ★ glyph
)

// Colours are 0xRRGGBB, all derived from the accent by derivePalette (see theme.go).
var (
	colPanel, colBg, colAccent, colAccentDim, colText, colSecondary, colSelected, colHover, colChip uint32

	faceFont = "MS Gothic" // replaced by resolveFaceFont when missing
	brushes  = map[uint32]uintptr{}
	fonts    = map[string]uintptr{}
)

func init() {
	p, _ := derivePalette(defaultAccent)
	setPalette(p)
}

func rgb(c uint32) uintptr {
	return uintptr(c>>16&0xFF | c&0xFF00 | (c&0xFF)<<16)
}

// blend composites fg at alpha a (0-255) over bg, standing in for the translucent fills.
func blend(fg, bg, a uint32) uint32 {
	var out uint32
	for sh := 0; sh <= 16; sh += 8 {
		f, b := fg>>sh&0xFF, bg>>sh&0xFF
		out |= (f*a + b*(255-a)) / 255 << sh
	}
	return out
}

func brush(c uint32) uintptr {
	if b, ok := brushes[c]; ok {
		return b
	}
	b, _, _ := createSolidBrush.Call(rgb(c))
	brushes[c] = b
	return b
}

// font returns a cached ClearType font; px is the pixel height already scaled for DPI.
// Fonts are freed by resetGDI on theme change; between changes the set is bounded by sizes x monitor DPIs.
func font(face string, px int, bold bool) uintptr {
	key := fmt.Sprint(face, px, bold)
	if f, ok := fonts[key]; ok {
		return f
	}
	weight := 400
	if bold {
		weight = 600
	}
	f, _, _ := createFontW.Call(uintptr(-px), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(wstr(face))))
	fonts[key] = f
	return f
}

// resolveFaceFont falls back to Yu Gothic when MS Gothic is not installed.
func resolveFaceFont() {
	dc, _, _ := getDC.Call(0)
	defer releaseDC.Call(0, dc)
	old, _, _ := selectObject.Call(dc, font(faceFont, 22, false))
	buf := make([]uint16, 64)
	getTextFaceW.Call(dc, 64, uintptr(unsafe.Pointer(&buf[0])))
	selectObject.Call(dc, old)
	if !strings.EqualFold(syscall.UTF16ToString(buf), faceFont) {
		faceFont = "Yu Gothic"
	}
}

func fillRect(dc uintptr, r rect, c uint32) {
	fillRectProc.Call(dc, uintptr(unsafe.Pointer(&r)), brush(c))
}

func frameRect(dc uintptr, r rect, c uint32) {
	frameRectProc.Call(dc, uintptr(unsafe.Pointer(&r)), brush(c))
}

func drawText(dc uintptr, s string, r rect, c uint32, f uintptr, flags uintptr) {
	selectObject.Call(dc, f)
	setTextColor.Call(dc, rgb(c))
	drawTextW.Call(dc, uintptr(unsafe.Pointer(wstr(s))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags)
}

func textWidth(dc, f uintptr, s string) int {
	u, _ := syscall.UTF16FromString(s)
	if len(u) < 2 {
		return 0
	}
	selectObject.Call(dc, f)
	var sz sizeT
	getTextExtentPoint32W.Call(dc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&sz)))
	return int(sz.CX)
}

// paint draws the whole picker into an offscreen bitmap to avoid flicker.
func (p *picker) paint() {
	var ps paintStruct
	hdc, _, _ := beginPaint.Call(p.hwnd, uintptr(unsafe.Pointer(&ps)))
	mdc, _, _ := createCompatibleDC.Call(hdc)
	bmp, _, _ := createCompatibleBitmap.Call(hdc, uintptr(p.w), uintptr(p.h))
	old, _, _ := selectObject.Call(mdc, bmp)
	p.draw(mdc)
	bitBlt.Call(hdc, 0, 0, uintptr(p.w), uintptr(p.h), mdc, 0, 0, srcCopy)
	selectObject.Call(mdc, old)
	deleteObject.Call(bmp)
	deleteDC.Call(mdc)
	endPaint.Call(p.hwnd, uintptr(unsafe.Pointer(&ps)))
}

func clip(dc uintptr, r rect) {
	saveDC.Call(dc)
	intersectClipRect.Call(dc, uintptr(r.L), uintptr(r.T), uintptr(r.R), uintptr(r.B))
}

func (p *picker) draw(dc uintptr) {
	setBkMode.Call(dc, transparent)
	l := p.layout()
	fillRect(dc, rc(0, 0, p.w, p.h), colBg)
	panel := rc(0, 0, p.w, p.h).inset(p.s(6))
	fillRect(dc, panel, colPanel)
	frameRect(dc, panel, colAccentDim)

	// Header.
	drawText(dc, appName, l.header, colText, font(uiFace, p.s(18), true), dtLine)
	n, shown := len(p.matches), p.shown()
	count := fmt.Sprintf("%d / %d", shown, len(idx.Entries))
	if p.cat >= 0 || p.favOnly || strings.TrimSpace(p.query) != "" {
		count = fmt.Sprintf("%d matches · showing %d", n, shown)
	}
	drawText(dc, count, l.header, colSecondary, font(uiFace, p.s(13), false), dtLine|dtRight)

	// Search box frame; the EDIT control sits inside it.
	fillRect(dc, l.search, colBg)
	frameRect(dc, l.search, colAccent)

	// Category chips.
	clip(dc, l.chips)
	for i := range p.chipX {
		cf := font(uiFace, p.s(13), false)
		if i == 0 {
			cf = font(symFace, p.s(13), false)
		}
		x := int(l.chips.L) + p.chipX[i] - p.chipScroll
		if x >= int(l.chips.R) {
			break
		}
		if x+p.chipW[i] < int(l.chips.L) {
			continue
		}
		r := rc(x, int(l.chips.T), x+p.chipW[i], int(l.chips.B))
		fill, border, text := colChip, uint32(colAccentDim), uint32(colSecondary)
		if (i == 0 && p.favOnly) || (i > 0 && i-1 == p.cat) {
			fill, border, text = colSelected, colAccent, colText
		}
		fillRect(dc, r, fill)
		frameRect(dc, r, border)
		drawText(dc, p.chipLabel(i), r, text, cf, dtLine|dtCenter)
	}
	restoreDC.Call(dc, ^uintptr(0))

	// Grid, virtualised to the visible rows.
	clip(dc, l.grid)
	if shown == 0 {
		drawText(dc, "No matches", l.grid, colSecondary, font(uiFace, p.s(16), false), dtLine|dtCenter)
	}
	tagF := font(uiFace, p.s(11), false)
	for i := p.scrollY / l.cellH * l.cols; i < shown; i++ {
		row, col := i/l.cols, i%l.cols
		y := int(l.grid.T) + row*l.cellH - p.scrollY
		if y >= int(l.grid.B) {
			break
		}
		tile := rc(int(l.grid.L)+col*l.cellW, y, int(l.grid.L)+(col+1)*l.cellW, y+l.cellH).inset(p.s(2))
		switch {
		case i == p.sel:
			fillRect(dc, tile, colSelected)
			frameRect(dc, tile, colAccent)
		case i == p.hover:
			fillRect(dc, tile, colHover)
		}
		e := idx.Entries[p.matches[i]]
		faceR := tile
		if cfg.ShowTags {
			faceR.B -= int32(p.s(18))
		}
		px := p.s(22)
		f := font(faceFont, px, false)
		if tw, avail := textWidth(dc, f, e.T), int(tile.R-tile.L)-p.s(12); tw > avail && avail > 0 {
			f = font(faceFont, max(p.s(9), px*avail/tw), false)
		}
		drawText(dc, e.T, faceR, colText, f, dtLine|dtCenter)
		if favSet[e.T] {
			drawText(dc, "★", rc(int(tile.R)-p.s(22), int(tile.T)+p.s(2), int(tile.R)-p.s(4), int(tile.T)+p.s(22)), colAccent, font(symFace, p.s(14), false), dtLine|dtRight)
		}
		if cfg.ShowTags {
			tagR := rc(int(tile.L)+p.s(6), int(faceR.B)-p.s(4), int(tile.R)-p.s(6), int(tile.B)-p.s(4))
			drawText(dc, strings.Join(append([]string{e.C}, e.G...), " · "), tagR, colAccentDim, tagF, dtLine|dtCenter|dtEndEllipsis)
		}
	}
	restoreDC.Call(dc, ^uintptr(0))

	// Scroll thumb in the right margin.
	gridH := int(l.grid.B - l.grid.T)
	if contentH := p.contentH(l); contentH > gridH {
		thumbH := max(p.s(24), gridH*gridH/contentH)
		y := int(l.grid.T) + p.scrollY*(gridH-thumbH)/(contentH-gridH)
		fillRect(dc, rc(int(l.grid.R)+p.s(5), y, int(l.grid.R)+p.s(9), y+thumbH), colAccentDim)
	}
}
