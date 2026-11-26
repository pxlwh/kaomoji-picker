package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

type Settings struct {
	Hotkey              string `json:"hotkey"`
	StartWithWindows    bool   `json:"start_with_windows"`
	PasteAfterPick      bool   `json:"paste_after_pick"`
	AlwaysOnTop         bool   `json:"always_on_top"`
	OpenOnCursorMonitor bool   `json:"open_on_cursor_monitor"`
	ShowTags            bool   `json:"show_tags"`
	Accent              string `json:"accent"`
}

const (
	defaultHotkey = "Ctrl+Alt+K"
	runKey        = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue      = "KaomojiPicker"
)

func defaultSettings() Settings {
	return Settings{Hotkey: defaultHotkey, PasteAfterPick: true, AlwaysOnTop: true, OpenOnCursorMonitor: true, ShowTags: true, Accent: defaultAccent}
}

// configDir is %APPDATA%\KaomojiPicker.
func configDir() (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "KaomojiPicker"), nil
}

// loadSettings falls back to defaults for a missing or malformed file and for an unparsable hotkey.
func loadSettings() Settings {
	s := defaultSettings()
	dir, err := configDir()
	if err != nil {
		return s
	}
	b, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil || json.Unmarshal(b, &s) != nil {
		return defaultSettings()
	}
	if _, _, canon, err := parseHotkey(s.Hotkey); err == nil {
		s.Hotkey = canon
	} else {
		s.Hotkey = defaultHotkey
	}
	if c, err := parseHex(s.Accent); err == nil {
		s.Accent = hexStr(c)
	} else {
		s.Accent = defaultAccent
	}
	return s
}

func saveSettings(s Settings) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "settings.json"), b, 0o644)
}

func keyName(vk uint32) string {
	switch {
	case vk >= 'A' && vk <= 'Z', vk >= '0' && vk <= '9':
		return string(rune(vk))
	case vk >= 0x70 && vk <= 0x87:
		return "F" + strconv.Itoa(int(vk-0x70+1))
	case vk == 0x20:
		return "Space"
	}
	return ""
}

// hotkeyName formats a combination canonically, or returns "" if it is not a valid hotkey.
func hotkeyName(mods, vk uint32) string {
	k := keyName(vk)
	if k == "" || mods&(modControl|modAlt|modShift|modWin) == 0 {
		return ""
	}
	var parts []string
	for _, m := range []struct {
		bit  uint32
		name string
	}{{modControl, "Ctrl"}, {modAlt, "Alt"}, {modShift, "Shift"}, {modWin, "Win"}} {
		if mods&m.bit != 0 {
			parts = append(parts, m.name)
		}
	}
	return strings.Join(append(parts, k), "+")
}

// parseHotkey accepts text like "Ctrl+Alt+K", "Ctrl+Shift+Space" or "Win+Alt+F5".
func parseHotkey(s string) (mods, vk uint32, canon string, err error) {
	for _, p := range strings.Split(s, "+") {
		p = strings.ToUpper(strings.TrimSpace(p))
		switch p {
		case "CTRL", "CONTROL":
			mods |= modControl
		case "ALT":
			mods |= modAlt
		case "SHIFT":
			mods |= modShift
		case "WIN", "WINDOWS":
			mods |= modWin
		default:
			if vk != 0 {
				return 0, 0, "", fmt.Errorf("hotkey %q has more than one key", s)
			}
			switch n, convErr := strconv.Atoi(strings.TrimPrefix(p, "F")); {
			case p == "SPACE":
				vk = 0x20
			case len(p) == 1 && (p[0] >= 'A' && p[0] <= 'Z' || p[0] >= '0' && p[0] <= '9'):
				vk = uint32(p[0])
			case len(p) > 1 && p[0] == 'F' && convErr == nil && n >= 1 && n <= 24:
				vk = uint32(0x70 + n - 1)
			default:
				return 0, 0, "", fmt.Errorf("unknown key %q in hotkey %q", p, s)
			}
		}
	}
	if canon = hotkeyName(mods, vk); canon == "" {
		return 0, 0, "", fmt.Errorf("hotkey %q needs a modifier and a key", s)
	}
	return mods, vk, canon, nil
}

func setStartWithWindows(on bool) error {
	var key uintptr
	if r, _, _ := regOpenKeyExW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(wstr(runKey))), 0, keySetValue, uintptr(unsafe.Pointer(&key))); r != 0 {
		return syscall.Errno(r)
	}
	defer regCloseKey.Call(key)
	name := wstr(runValue)
	if !on {
		if r, _, _ := regDeleteValueW.Call(key, uintptr(unsafe.Pointer(name))); r != 0 && r != uintptr(syscall.ERROR_FILE_NOT_FOUND) {
			return syscall.Errno(r)
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	v, err := syscall.UTF16FromString(`"` + exe + `"`)
	if err != nil {
		return err
	}
	if r, _, _ := regSetValueExW.Call(key, uintptr(unsafe.Pointer(name)), 0, regSZ, uintptr(unsafe.Pointer(&v[0])), uintptr(len(v)*2)); r != 0 {
		return syscall.Errno(r)
	}
	return nil
}

// Settings dialog.

const (
	idSave       = 1 // IDOK, so Enter saves via IsDialogMessage
	idCancel     = 2 // IDCANCEL, so Esc cancels
	idHotkeyBox  = 10
	idCurrentLbl = 11
	idHexBox     = 12
	idThemeErr   = 13
	idPreview    = 14
	idFirstCheck = 20
	idSwatch     = 30
)

var swatches = [...]string{"#ff6fae", "#3fa9f5", "#4ade80", "#f5a623", "#a78bfa", "#f5f5f5"}

var sd struct {
	hwnd, box, current, hex, themeErr, preview uintptr
	checks                                     [5]uintptr
	captured                                   string
	prev                                       Palette
	dpi                                        int
}

func sds(v int) int { return v * sd.dpi / 96 }

func openSettings() {
	if sd.hwnd != 0 {
		setForegroundWindow.Call(sd.hwnd)
		return
	}
	pk.hide()
	unregisterHotKey.Call(msgHwnd, hotkeyID) // so the capture box can receive the current combination
	sd.captured = cfg.Hotkey
	work := workArea(monitorAt(cursorPos()))
	sd.dpi = monitorDPI(monitorAt(cursorPos()))

	const style = wsCaption | wsSysMenu
	cw, ch := sds(380), sds(450)
	r := rc(0, 0, cw, ch)
	adjustWindowRectEx.Call(uintptr(unsafe.Pointer(&r)), style, 0, wsExTopmost)
	w, h := int(r.R-r.L), int(r.B-r.T)
	x := int(work.L) + (int(work.R-work.L)-w)/2
	y := int(work.T) + (int(work.B-work.T)-h)/2
	sd.hwnd = createWindow(wsExTopmost, settingsClass, appName+" Settings", style, x, y, w, h, 0, 0)
	if sd.hwnd == 0 {
		applyHotkey()
		return
	}
	f := font(uiFace, sds(14), false)
	child := func(class, text string, style uintptr, x, y, w, h int, id uintptr) uintptr {
		c := createWindow(0, class, text, wsChild|wsVisible|style, x, y, w, h, sd.hwnd, id)
		sendMsg(c, wmSetFont, f, 0)
		return c
	}
	child("STATIC", "Hotkey (click the box, then press a combination)", 0, sds(20), sds(16), sds(340), sds(20), 0)
	sd.box = child(hotkeyClass, "", wsTabStop, sds(20), sds(40), sds(340), sds(32), idHotkeyBox)
	cur := "Current hotkey: " + cfg.Hotkey
	if !hotkeyOK {
		cur += " (not registered)"
	}
	sd.current = child("STATIC", cur, 0, sds(20), sds(78), sds(340), sds(20), idCurrentLbl)
	labels := []string{"Start with Windows", "Paste after picking", "Always on top", "Open on monitor with cursor", "Show tags under faces"}
	vals := []bool{cfg.StartWithWindows, cfg.PasteAfterPick, cfg.AlwaysOnTop, cfg.OpenOnCursorMonitor, cfg.ShowTags}
	for i, l := range labels {
		sd.checks[i] = child("BUTTON", l, bsAutoCheckbox|wsTabStop, sds(20), sds(112+i*30), sds(340), sds(24), uintptr(idFirstCheck+i))
		if vals[i] {
			sendMsg(sd.checks[i], bmSetCheck, 1, 0)
		}
	}
	child("STATIC", "Accent colour (hex)", 0, sds(20), sds(268), sds(340), sds(20), 0)
	sd.prev, _ = derivePalette(cfg.Accent)
	sd.hex = child("EDIT", cfg.Accent, wsTabStop|wsBorder|esAutoHScroll, sds(20), sds(292), sds(100), sds(28), idHexBox)
	for i := range swatches {
		child("BUTTON", swatches[i], bsOwnerDraw|wsTabStop, sds(134+i*38), sds(292), sds(32), sds(28), uintptr(idSwatch+i))
	}
	sd.preview = child("STATIC", "", ssOwnerDraw, sds(20), sds(328), sds(340), sds(32), idPreview)
	sd.themeErr = child("STATIC", "", 0, sds(20), sds(366), sds(340), sds(20), idThemeErr)
	child("BUTTON", "Save", bsOwnerDraw|wsTabStop, cw-sds(200), ch-sds(50), sds(85), sds(32), idSave)
	child("BUTTON", "Cancel", bsOwnerDraw|wsTabStop, cw-sds(105), ch-sds(50), sds(85), sds(32), idCancel)
	sendMsg(sd.hwnd, wmSetIcon, 0, trayIcon)
	sendMsg(sd.hwnd, wmSetIcon, 1, trayIcon)
	showWindow.Call(sd.hwnd, swShow)
	setForegroundWindow.Call(sd.hwnd)
	setFocus.Call(sd.box)
}

// saveFromDialog stores the dialog's values; it returns false (keeping the dialog open) on an invalid accent.
func saveFromDialog() bool {
	n := cfg
	c, err := parseHex(windowText(sd.hex))
	if err != nil {
		setWindowTextW.Call(sd.themeErr, uintptr(unsafe.Pointer(wstr(err.Error()))))
		return false
	}
	n.Accent = hexStr(c)
	if _, _, canon, err := parseHotkey(sd.captured); err == nil {
		n.Hotkey = canon
	}
	checked := func(i int) bool { return sendMsg(sd.checks[i], bmGetCheck, 0, 0) == 1 }
	n.StartWithWindows, n.PasteAfterPick, n.AlwaysOnTop, n.OpenOnCursorMonitor, n.ShowTags = checked(0), checked(1), checked(2), checked(3), checked(4)
	if err := setStartWithWindows(n.StartWithWindows); err != nil {
		balloon("Start with Windows failed", err.Error(), niifWarn)
	}
	cfg = n
	if err := saveSettings(cfg); err != nil {
		balloon("Could not save settings", err.Error(), niifWarn)
	}
	return true
}

func settingsProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch msg {
	case wmEraseBkgnd:
		var r rect
		getClientRectProc.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		fillRect(wp, r, colPanel)
		return 1
	case wmCtlColorEdit:
		setTextColor.Call(wp, rgb(colText))
		setBkColor.Call(wp, rgb(colBg))
		return brush(colBg)
	case wmCtlColorStatic, wmCtlColorBtn:
		c := uint32(colText)
		if lp == sd.current {
			c = colSecondary
		}
		if lp == sd.themeErr {
			c = colAccent
		}
		setTextColor.Call(wp, rgb(c))
		setBkColor.Call(wp, rgb(colPanel))
		return brush(colPanel)
	case wmDrawItem:
		d := ptrAt[drawItemStruct](lp)
		switch id := int(d.CtlID); {
		case id == idPreview: // accent / panel / text of the hex being typed
			w := int(d.Item.R-d.Item.L) / 3
			for i, c := range []uint32{sd.prev.Accent, sd.prev.Panel, sd.prev.Text} {
				r := d.Item
				r.L, r.R = d.Item.L+int32(i*w), d.Item.L+int32((i+1)*w)
				fillRect(d.Hdc, r, c)
			}
			setBkMode.Call(d.Hdc, transparent)
			mid := d.Item
			mid.L, mid.R = d.Item.L+int32(w), d.Item.L+int32(2*w)
			drawText(d.Hdc, "Aa kaomoji", mid, sd.prev.Text, font(uiFace, sds(14), false), dtLine|dtCenter)
			return 1
		case id >= idSwatch && id < idSwatch+len(swatches):
			c, _ := parseHex(swatches[id-idSwatch])
			fillRect(d.Hdc, d.Item, c)
			frameRect(d.Hdc, d.Item, colAccentDim)
			if d.ItemState&odsFocus != 0 {
				frameRect(d.Hdc, d.Item.inset(sds(2)), colText)
			}
			return 1
		}
		fill := uint32(colAccentDim)
		if d.ItemState&odsSelected != 0 {
			fill = colAccent
		}
		fillRect(d.Hdc, d.Item, fill)
		if d.ItemState&odsFocus != 0 {
			frameRect(d.Hdc, d.Item, colAccent)
		}
		setBkMode.Call(d.Hdc, transparent)
		drawText(d.Hdc, windowText(d.HwndItem), d.Item, colText, font(uiFace, sds(14), false), dtLine|dtCenter)
		return 1
	case wmCommand:
		id := int(wp & 0xFFFF)
		switch {
		case id == idSave:
			// Fonts and brushes are rebuilt after the dialog is gone so no live child holds a deleted font.
			if prev := cfg.Accent; saveFromDialog() {
				destroyWindow.Call(hwnd)
				if cfg.Accent != prev {
					applyTheme(cfg.Accent)
				}
			}
		case id == idCancel:
			destroyWindow.Call(hwnd)
		case id == idHexBox && wp>>16&0xFFFF == enChange:
			msg := ""
			if c, err := parseHex(windowText(sd.hex)); err != nil {
				msg = err.Error()
			} else {
				sd.prev, _ = derivePalette(hexStr(c))
			}
			setWindowTextW.Call(sd.themeErr, uintptr(unsafe.Pointer(wstr(msg))))
			invalidate(sd.preview)
		case id >= idSwatch && id < idSwatch+len(swatches):
			setWindowTextW.Call(sd.hex, uintptr(unsafe.Pointer(wstr(swatches[id-idSwatch]))))
		}
		return 0
	case wmClose:
		destroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		sd.hwnd = 0
		applyHotkey()
		return 0
	}
	return defProc(hwnd, msg, wp, lp)
}

func currentMods() uint32 {
	down := func(vk uintptr) bool { r, _, _ := getKeyState.Call(vk); return int16(r) < 0 }
	var m uint32
	if down(vkControl) {
		m |= modControl
	}
	if down(vkMenu) {
		m |= modAlt
	}
	if down(vkShift) {
		m |= modShift
	}
	if down(vkLWin) || down(vkRWin) {
		m |= modWin
	}
	return m
}

// hotkeyBoxProc is the capture box: it records the first valid combination pressed while focused.
func hotkeyBoxProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch msg {
	case wmGetDlgCode:
		return dlgcWantAll | dlgcWantChars
	case wmLButtonDown:
		setFocus.Call(hwnd)
		return 0
	case wmSetFocus, wmKillFocus:
		invalidate(hwnd)
		return 0
	case wmKeyDown, wmSysKeyDown:
		if name := hotkeyName(currentMods(), uint32(wp)); name != "" {
			sd.captured = name
			invalidate(hwnd)
		}
		return 0
	case wmKeyUp, wmSysKeyUp, wmChar, wmSysChar:
		return 0 // keep Alt from opening the system menu and Enter/Esc from beeping
	case wmEraseBkgnd:
		return 1
	case wmPaint:
		var ps paintStruct
		dc, _, _ := beginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		var r rect
		getClientRectProc.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		fillRect(dc, r, colBg)
		border := uint32(colAccentDim)
		focus, _, _ := getFocus.Call()
		if focus == hwnd {
			border = colAccent
		}
		frameRect(dc, r, border)
		setBkMode.Call(dc, transparent)
		drawText(dc, sd.captured, r, colText, font(uiFace, sds(15), true), dtLine|dtCenter)
		endPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	}
	return defProc(hwnd, msg, wp, lp)
}
