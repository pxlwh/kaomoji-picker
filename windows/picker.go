package main

import (
	"math"
	"strconv"
	"syscall"
	"unsafe"
)

const (
	pickerClass  = "KaomojiPickerPopup"
	idEdit       = 100
	timerFilter  = 1
	filterDelay  = 60  // ms of typing debounce
	windowAlpha  = 235 // ~92% opaque backdrop
	hideDebounce = 300 // ms: a tray click that just hid the picker must not reopen it
)

type picker struct {
	hwnd, edit, editProc uintptr
	w, h, dpi            int
	query                string
	cat                  int // index into idx.Cats, -1 for all
	favOnly              bool
	matches              []int
	limit, sel, hover    int
	scrollY, chipScroll  int
	chipX, chipW         []int
	prevFg               uintptr
	lastHide             uint32
	tracking             bool
}

var pk = picker{cat: -1, hover: -1}

type pickerLayout struct {
	header, search, chips, grid rect
	cols, cellW, cellH          int
}

func (p *picker) s(v int) int {
	d := p.dpi
	if d == 0 {
		d = 96
	}
	return v * d / 96
}

func (p *picker) layout() pickerLayout {
	m := p.s(18)
	var l pickerLayout
	l.header = rc(m, m, p.w-m, m+p.s(26))
	l.search = rc(m, int(l.header.B)+p.s(6), p.w-m, int(l.header.B)+p.s(40))
	l.chips = rc(m, int(l.search.B)+p.s(10), p.w-m, int(l.search.B)+p.s(40))
	l.grid = rc(m, int(l.chips.B)+p.s(10), p.w-m, max(int(l.chips.B)+p.s(10), p.h-m))
	gw := int(l.grid.R - l.grid.L)
	l.cols = max(3, gw/p.s(140))
	l.cellW = max(1, gw/l.cols)
	l.cellH = max(1, int(math.Round(float64(l.cellW)*0.7)))
	return l
}

func (p *picker) shown() int { return min(len(p.matches), p.limit) }

func (p *picker) contentH(l pickerLayout) int {
	return (p.shown() + l.cols - 1) / l.cols * l.cellH
}

// settle clamps scrolling and appends the next page when the view nears the end of what is rendered.
func (p *picker) settle(l pickerLayout) {
	gridH := int(l.grid.B - l.grid.T)
	if p.limit < len(p.matches) && p.scrollY+gridH >= p.contentH(l)-2*l.cellH {
		p.limit = min(len(p.matches), p.limit+nextPage)
	}
	p.scrollY = max(0, min(p.scrollY, p.contentH(l)-gridH))
	maxChip := 0
	if n := len(p.chipX); n > 0 {
		maxChip = max(0, p.chipX[n-1]+p.chipW[n-1]-int(l.chips.R-l.chips.L))
	}
	p.chipScroll = max(0, min(p.chipScroll, maxChip))
}

func (p *picker) create() {
	registerClass(pickerClass, syscall.NewCallback(pickerProc), 0, 0)
	p.hwnd = createWindow(wsExToolWindow|wsExTopmost|wsExLayered, pickerClass, appName, wsPopup|wsClipChildren, 0, 0, 400, 300, 0, 0)
	setLayeredWindowAttributes.Call(p.hwnd, 0, windowAlpha, lwaAlpha)
	p.edit = createWindow(0, "EDIT", "", wsChild|wsVisible|esAutoHScroll, 0, 0, 10, 10, p.hwnd, idEdit)
	p.editProc, _, _ = setWindowLongPtrW.Call(p.edit, gwlpWndProc, syscall.NewCallback(editProc))
	p.measureChips()
	p.refilter()
}

func (p *picker) measureChips() {
	dc, _, _ := getDC.Call(0)
	defer releaseDC.Call(0, dc)
	f := font(uiFace, p.s(13), false)
	p.chipX, p.chipW = make([]int, len(idx.Cats)+1), make([]int, len(idx.Cats)+1)
	x := 0
	for i := range p.chipX {
		p.chipX[i] = x
		p.chipW[i] = textWidth(dc, f, p.chipLabel(i)) + p.s(20)
		x += p.chipW[i] + p.s(6)
	}
}

// chipLabel: chip 0 is the synthetic favourites chip, chip i>0 is idx.Cats[i-1].
func (p *picker) chipLabel(i int) string {
	if i == 0 {
		return "★ favorites  " + strconv.Itoa(idx.favCount(favs))
	}
	return idx.Cats[i-1].Name + " " + strconv.Itoa(idx.Cats[i-1].Count)
}

// toggleFavAt stars or unstars the tile at i, saves, and re-sorts without losing the scroll position.
func (p *picker) toggleFavAt(i int) {
	setFavs(toggleFav(favs, idx.Entries[p.matches[i]].T))
	if err := saveFavorites(favoritesPath(), favs); err != nil {
		balloon("Could not save favorites", err.Error(), niifWarn)
	}
	scroll, sel := p.scrollY, p.sel
	p.measureChips()
	p.refilter()
	p.scrollY, p.sel = scroll, max(0, min(sel, len(p.matches)-1))
	p.settle(p.layout())
}

func (p *picker) placeEdit() {
	l := p.layout()
	eh := p.s(22)
	moveWindow.Call(p.edit, uintptr(int(l.search.L)+p.s(10)), uintptr(int(l.search.T)+(int(l.search.B-l.search.T)-eh)/2),
		uintptr(int(l.search.R-l.search.L)-p.s(20)), uintptr(eh), 1)
}

func (p *picker) refilter() {
	p.query = windowText(p.edit)
	cat := ""
	if p.cat >= 0 && p.cat < len(idx.Cats) {
		cat = idx.Cats[p.cat].Name
	} else {
		p.cat = -1
	}
	p.matches = idx.FilterFav(p.query, cat, favs, p.favOnly)
	p.limit, p.sel, p.hover, p.scrollY = firstPage, 0, -1, 0
	invalidate(p.hwnd)
}

func (p *picker) show() {
	if fg, _, _ := getForegroundWindow.Call(); fg != p.hwnd && fg != msgHwnd && fg != sd.hwnd {
		p.prevFg = fg
	}
	mon, _, _ := monitorFromWindow.Call(p.prevFg, monDefNearest)
	if cfg.OpenOnCursorMonitor {
		mon = monitorAt(cursorPos())
	}
	work := workArea(mon)
	if dpi := monitorDPI(mon); dpi != p.dpi {
		p.dpi = dpi
		sendMsg(p.edit, wmSetFont, font(uiFace, p.s(16), false), 0)
		p.measureChips()
	}
	ww, wh := int(work.R-work.L), int(work.B-work.T)
	w, h := min(p.s(1100), ww-p.s(80)), min(p.s(700), wh-p.s(80))
	after := hwndNoTopmst
	if cfg.AlwaysOnTop {
		after = hwndTopmost
	}
	setWindowPos.Call(p.hwnd, after, uintptr(int(work.L)+(ww-w)/2), uintptr(int(work.T)+(wh-h)/2), uintptr(w), uintptr(h), swpShowWin)
	setForegroundWindow.Call(p.hwnd)
	setFocus.Call(p.edit)
	sendMsg(p.edit, emSetSel, 0, ^uintptr(0))
	invalidate(p.hwnd)
}

func (p *picker) hide() {
	if visible(p.hwnd) {
		showWindow.Call(p.hwnd, swHide)
		p.lastHide = tickCount()
	}
}

func (p *picker) toggle() {
	switch {
	case visible(p.hwnd):
		p.hide()
	case tickCount()-p.lastHide > hideDebounce:
		p.show()
	}
}

// pick copies the face, returns focus to the previous window and optionally pastes.
func (p *picker) pick(i int) {
	if i < 0 || i >= len(p.matches) {
		return
	}
	ok := setClipboard(idx.Entries[p.matches[i]].T)
	p.hide()
	if p.prevFg != 0 {
		setForegroundWindow.Call(p.prevFg)
	}
	if ok && cfg.PasteAfterPick {
		setTimer.Call(msgHwnd, timerPaste, 250, 0)
	}
}

func (p *picker) move(d int) {
	if len(p.matches) == 0 {
		return
	}
	p.sel = max(0, min(len(p.matches)-1, p.sel+d))
	if p.sel >= p.limit {
		p.limit = min(len(p.matches), p.sel+nextPage)
	}
	l := p.layout()
	gridH := int(l.grid.B - l.grid.T)
	row := p.sel / l.cols
	if top := row * l.cellH; top < p.scrollY {
		p.scrollY = top
	} else if top+l.cellH > p.scrollY+gridH {
		p.scrollY = top + l.cellH - gridH
	}
	p.settle(l)
	invalidate(p.hwnd)
}

// key handles navigation keys and reports whether it consumed the key.
func (p *picker) key(vk uintptr) bool {
	l := p.layout()
	page := max(1, int(l.grid.B-l.grid.T)/l.cellH) * l.cols
	switch vk {
	case vkEscape:
		p.hide()
	case vkReturn:
		p.pick(p.sel)
	case vkUp:
		p.move(-l.cols)
	case vkDown:
		p.move(l.cols)
	case vkLeft:
		p.move(-1)
	case vkRight:
		p.move(1)
	case vkPrior:
		p.move(-page)
	case vkNext:
		p.move(page)
	default:
		return false
	}
	return true
}

func (p *picker) tileAt(l pickerLayout, x, y int) int {
	if !l.grid.in(x, y) {
		return -1
	}
	col := (x - int(l.grid.L)) / l.cellW
	i := (y-int(l.grid.T)+p.scrollY)/l.cellH*l.cols + col
	if col >= l.cols || i >= p.shown() {
		return -1
	}
	return i
}

func (p *picker) chipAt(l pickerLayout, x, y int) int {
	if !l.chips.in(x, y) {
		return -1
	}
	for i := range p.chipX {
		cx := int(l.chips.L) + p.chipX[i] - p.chipScroll
		if x >= cx && x < cx+p.chipW[i] {
			return i
		}
	}
	return -1
}

func pickerProc(hwnd, msg, wp, lp uintptr) uintptr {
	p := &pk
	switch msg {
	case wmPaint:
		p.paint()
		return 0
	case wmEraseBkgnd:
		return 1
	case wmSize:
		p.w, p.h = lo16(lp)&0xFFFF, hi16(lp)&0xFFFF
		p.placeEdit()
		p.settle(p.layout())
		return 0
	case wmDpiChanged:
		return 0 // show() sizes the window for the target monitor itself
	case wmActivate:
		if wp&0xFFFF == 0 {
			p.hide()
		}
		return 0
	case wmCtlColorEdit:
		setTextColor.Call(wp, rgb(colText))
		setBkColor.Call(wp, rgb(colBg))
		return brush(colBg)
	case wmCommand:
		if wp>>16&0xFFFF == enChange && lp == p.edit {
			setTimer.Call(hwnd, timerFilter, filterDelay, 0)
		}
		return 0
	case wmTimer:
		if wp == timerFilter {
			killTimer.Call(hwnd, timerFilter)
			p.refilter()
		}
		return 0
	case wmKeyDown:
		if p.key(wp) {
			return 0
		}
	case wmMouseMove:
		if !p.tracking {
			t := trackMouseEvent{Flags: tmeLeave, Track: hwnd}
			t.Size = uint32(unsafe.Sizeof(t))
			trackMouseEventProc.Call(uintptr(unsafe.Pointer(&t)))
			p.tracking = true
		}
		if i := p.tileAt(p.layout(), lo16(lp), hi16(lp)); i != p.hover {
			p.hover = i
			if i >= 0 {
				p.sel = i
			}
			invalidate(hwnd)
		}
		return 0
	case wmMouseLeave:
		p.tracking, p.hover = false, -1
		invalidate(hwnd)
		return 0
	case wmLButtonDown:
		x, y := lo16(lp), hi16(lp)
		l := p.layout()
		if !rc(0, 0, p.w, p.h).inset(p.s(6)).in(x, y) {
			p.hide()
		} else if c := p.chipAt(l, x, y); c >= 0 {
			if c == 0 {
				p.favOnly = !p.favOnly
			} else if c-1 == p.cat {
				p.cat = -1
			} else {
				p.cat = c - 1
			}
			p.refilter()
		} else if i := p.tileAt(l, x, y); i >= 0 {
			p.pick(i)
		}
		return 0
	case wmRButtonUp:
		if i := p.tileAt(p.layout(), lo16(lp), hi16(lp)); i >= 0 {
			p.toggleFavAt(i)
		}
		return 0
	case wmMouseWheel:
		pt := point{int32(lo16(lp)), int32(hi16(lp))}
		screenToClient.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
		delta := hi16(wp)
		l := p.layout()
		if l.chips.in(int(pt.X), int(pt.Y)) {
			p.chipScroll -= delta * p.s(80) / 120
		} else {
			p.scrollY -= delta * l.cellH / 120
		}
		p.settle(l)
		invalidate(hwnd)
		return 0
	}
	return defProc(hwnd, msg, wp, lp)
}

// editProc subclasses the search box so navigation keys reach the grid while typing stays in the box.
func editProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch msg {
	case wmKeyDown:
		if pk.key(wp) {
			return 0
		}
	case wmChar:
		if wp == '\r' || wp == vkEscape {
			return 0 // already handled on key down; avoids the edit control's beep
		}
	case wmMouseWheel:
		return sendMsg(pk.hwnd, msg, wp, lp)
	}
	r, _, _ := callWindowProcW.Call(pk.editProc, hwnd, msg, wp, lp)
	return r
}
