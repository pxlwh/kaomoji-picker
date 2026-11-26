package main

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	shcore   = syscall.NewLazyDLL("shcore.dll")

	adjustWindowRectEx            = user32.NewProc("AdjustWindowRectEx")
	allowSetForegroundWindow      = user32.NewProc("AllowSetForegroundWindow")
	appendMenuW                   = user32.NewProc("AppendMenuW")
	beginPaint                    = user32.NewProc("BeginPaint")
	callWindowProcW               = user32.NewProc("CallWindowProcW")
	closeClipboard                = user32.NewProc("CloseClipboard")
	createIconFromResourceEx      = user32.NewProc("CreateIconFromResourceEx")
	createIconIndirect            = user32.NewProc("CreateIconIndirect")
	destroyIcon                   = user32.NewProc("DestroyIcon")
	setWindowTextW                = user32.NewProc("SetWindowTextW")
	createPopupMenu               = user32.NewProc("CreatePopupMenu")
	createWindowExW               = user32.NewProc("CreateWindowExW")
	defWindowProcW                = user32.NewProc("DefWindowProcW")
	destroyMenu                   = user32.NewProc("DestroyMenu")
	destroyWindow                 = user32.NewProc("DestroyWindow")
	dispatchMessageW              = user32.NewProc("DispatchMessageW")
	drawTextW                     = user32.NewProc("DrawTextW")
	emptyClipboard                = user32.NewProc("EmptyClipboard")
	endPaint                      = user32.NewProc("EndPaint")
	fillRectProc                  = user32.NewProc("FillRect")
	findWindowW                   = user32.NewProc("FindWindowW")
	frameRectProc                 = user32.NewProc("FrameRect")
	getCursorPos                  = user32.NewProc("GetCursorPos")
	getClientRectProc             = user32.NewProc("GetClientRect")
	getDC                         = user32.NewProc("GetDC")
	getFocus                      = user32.NewProc("GetFocus")
	getForegroundWindow           = user32.NewProc("GetForegroundWindow")
	getKeyState                   = user32.NewProc("GetKeyState")
	getMessageW                   = user32.NewProc("GetMessageW")
	getMonitorInfoW               = user32.NewProc("GetMonitorInfoW")
	getSystemMetrics              = user32.NewProc("GetSystemMetrics")
	getWindowTextLengthW          = user32.NewProc("GetWindowTextLengthW")
	getWindowTextW                = user32.NewProc("GetWindowTextW")
	getWindowThreadProcessId      = user32.NewProc("GetWindowThreadProcessId")
	invalidateRect                = user32.NewProc("InvalidateRect")
	isDialogMessageW              = user32.NewProc("IsDialogMessageW")
	isWindowVisible               = user32.NewProc("IsWindowVisible")
	killTimer                     = user32.NewProc("KillTimer")
	loadCursorW                   = user32.NewProc("LoadCursorW")
	lookupIconIdFromDirectoryEx   = user32.NewProc("LookupIconIdFromDirectoryEx")
	messageBoxW                   = user32.NewProc("MessageBoxW")
	monitorFromPoint              = user32.NewProc("MonitorFromPoint")
	monitorFromWindow             = user32.NewProc("MonitorFromWindow")
	moveWindow                    = user32.NewProc("MoveWindow")
	openClipboard                 = user32.NewProc("OpenClipboard")
	postMessageW                  = user32.NewProc("PostMessageW")
	postQuitMessage               = user32.NewProc("PostQuitMessage")
	registerClassExW              = user32.NewProc("RegisterClassExW")
	registerHotKey                = user32.NewProc("RegisterHotKey")
	registerWindowMessageW        = user32.NewProc("RegisterWindowMessageW")
	releaseDC                     = user32.NewProc("ReleaseDC")
	screenToClient                = user32.NewProc("ScreenToClient")
	sendInput                     = user32.NewProc("SendInput")
	sendMessageW                  = user32.NewProc("SendMessageW")
	setClipboardData              = user32.NewProc("SetClipboardData")
	setFocus                      = user32.NewProc("SetFocus")
	setForegroundWindow           = user32.NewProc("SetForegroundWindow")
	setLayeredWindowAttributes    = user32.NewProc("SetLayeredWindowAttributes")
	setProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	setProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	setTimer                      = user32.NewProc("SetTimer")
	setWindowLongPtrW             = user32.NewProc("SetWindowLongPtrW")
	setWindowPos                  = user32.NewProc("SetWindowPos")
	showWindow                    = user32.NewProc("ShowWindow")
	trackMouseEventProc           = user32.NewProc("TrackMouseEvent")
	trackPopupMenu                = user32.NewProc("TrackPopupMenu")
	translateMessage              = user32.NewProc("TranslateMessage")
	unregisterHotKey              = user32.NewProc("UnregisterHotKey")
	createMutexW                  = kernel32.NewProc("CreateMutexW")
	getModuleHandleW              = kernel32.NewProc("GetModuleHandleW")
	getTickCount                  = kernel32.NewProc("GetTickCount")
	globalAlloc                   = kernel32.NewProc("GlobalAlloc")
	globalFree                    = kernel32.NewProc("GlobalFree")
	globalLock                    = kernel32.NewProc("GlobalLock")
	globalUnlock                  = kernel32.NewProc("GlobalUnlock")
	bitBlt                        = gdi32.NewProc("BitBlt")
	createCompatibleBitmap        = gdi32.NewProc("CreateCompatibleBitmap")
	createCompatibleDC            = gdi32.NewProc("CreateCompatibleDC")
	createDIBSection              = gdi32.NewProc("CreateDIBSection")
	createFontW                   = gdi32.NewProc("CreateFontW")
	createSolidBrush              = gdi32.NewProc("CreateSolidBrush")
	deleteDC                      = gdi32.NewProc("DeleteDC")
	deleteObject                  = gdi32.NewProc("DeleteObject")
	getDeviceCaps                 = gdi32.NewProc("GetDeviceCaps")
	getTextExtentPoint32W         = gdi32.NewProc("GetTextExtentPoint32W")
	getTextFaceW                  = gdi32.NewProc("GetTextFaceW")
	intersectClipRect             = gdi32.NewProc("IntersectClipRect")
	restoreDC                     = gdi32.NewProc("RestoreDC")
	saveDC                        = gdi32.NewProc("SaveDC")
	selectObject                  = gdi32.NewProc("SelectObject")
	setBkColor                    = gdi32.NewProc("SetBkColor")
	setBkMode                     = gdi32.NewProc("SetBkMode")
	setTextColor                  = gdi32.NewProc("SetTextColor")
	shellNotifyIconW              = shell32.NewProc("Shell_NotifyIconW")
	regCloseKey                   = advapi32.NewProc("RegCloseKey")
	regDeleteValueW               = advapi32.NewProc("RegDeleteValueW")
	regOpenKeyExW                 = advapi32.NewProc("RegOpenKeyExW")
	regSetValueExW                = advapi32.NewProc("RegSetValueExW")
	getDpiForMonitor              = shcore.NewProc("GetDpiForMonitor")
)

const (
	wsPopup        = 0x80000000
	wsChild        = 0x40000000
	wsVisible      = 0x10000000
	wsClipChildren = 0x02000000
	wsCaption      = 0x00C00000
	wsSysMenu      = 0x00080000
	wsTabStop      = 0x00010000
	wsBorder       = 0x00800000
	wsExToolWindow = 0x00000080
	wsExTopmost    = 0x00000008
	wsExLayered    = 0x00080000
	esAutoHScroll  = 0x0080
	bsAutoCheckbox = 0x0003
	bsOwnerDraw    = 0x000B

	wmNull           = 0x0000
	wmDestroy        = 0x0002
	wmSize           = 0x0005
	wmActivate       = 0x0006
	wmSetFocus       = 0x0007
	wmKillFocus      = 0x0008
	wmPaint          = 0x000F
	wmClose          = 0x0010
	wmEraseBkgnd     = 0x0014
	wmSetFont        = 0x0030
	wmSetIcon        = 0x0080
	wmDrawItem       = 0x002B
	wmGetDlgCode     = 0x0087
	wmKeyDown        = 0x0100
	wmKeyUp          = 0x0101
	wmChar           = 0x0102
	wmSysKeyDown     = 0x0104
	wmSysKeyUp       = 0x0105
	wmSysChar        = 0x0106
	wmCommand        = 0x0111
	wmTimer          = 0x0113
	wmCtlColorEdit   = 0x0133
	wmCtlColorBtn    = 0x0135
	wmCtlColorStatic = 0x0138
	wmMouseMove      = 0x0200
	wmLButtonDown    = 0x0201
	wmLButtonUp      = 0x0202
	wmRButtonUp      = 0x0205
	wmMouseWheel     = 0x020A
	wmMouseLeave     = 0x02A3
	wmDpiChanged     = 0x02E0
	wmHotkey         = 0x0312
	wmContextMenu    = 0x007B
	wmApp            = 0x8000

	enChange      = 0x0300
	emSetSel      = 0x00B1
	bmGetCheck    = 0x00F0
	bmSetCheck    = 0x00F1
	odsSelected   = 0x0001
	odsFocus      = 0x0010
	dlgcWantAll   = 0x0004
	dlgcWantChars = 0x0080

	swHide       = 0
	swShow       = 5
	swpShowWin   = 0x0040
	hwndTopmost  = ^uintptr(0) // -1
	hwndNoTopmst = ^uintptr(1) // -2
	gwlpWndProc  = ^uintptr(3) // -4
	lwaAlpha     = 0x2
	tmeLeave     = 0x2

	mfString    = 0x0000
	mfSeparator = 0x0800
	tpmRightBtn = 0x0002
	tpmReturnCm = 0x0100

	nimAdd     = 0
	nimModify  = 1
	nimDelete  = 2
	nifMessage = 0x01
	nifIcon    = 0x02
	nifTip     = 0x04
	nifInfo    = 0x10
	niifWarn   = 0x02
	niifInfo   = 0x01

	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modWin      = 0x0008
	modNoRepeat = 0x4000

	vkReturn  = 0x0D
	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12
	vkEscape  = 0x1B
	vkPrior   = 0x21
	vkNext    = 0x22
	vkLeft    = 0x25
	vkUp      = 0x26
	vkRight   = 0x27
	vkDown    = 0x28
	vkLWin    = 0x5B
	vkRWin    = 0x5C

	dtCenter      = 0x0001
	dtRight       = 0x0002
	dtVCenter     = 0x0004
	dtSingleLine  = 0x0020
	dtNoPrefix    = 0x0800
	dtEndEllipsis = 0x8000
	dtLine        = dtSingleLine | dtVCenter | dtNoPrefix

	cfUnicodeText   = 13
	gmemMoveable    = 0x0002
	inputKeyboard   = 1
	keyeventfKeyUp  = 0x0002
	hkeyCurrentUser = 0x80000001
	keySetValue     = 0x0002
	regSZ           = 1
	errAlreadyExist = 183
	srcCopy         = 0x00CC0020
	transparent     = 1
	logPixelsX      = 88
	smCxSmIcon      = 49
	ssOwnerDraw     = 0x0D
	idcArrow        = 32512
	monDefNearest   = 2
)

type point struct{ X, Y int32 }

type rect struct{ L, T, R, B int32 }

type msgT struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	private uint32
}

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type paintStruct struct {
	Hdc       uintptr
	Erase     int32
	Paint     rect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

type monitorInfo struct {
	Size    uint32
	Monitor rect
	Work    rect
	Flags   uint32
}

type notifyIconData struct {
	Size            uint32
	Wnd             uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUID            [16]byte
	BalloonIcon     uintptr
}

// keyInput is INPUT with the KEYBDINPUT union member, padded to sizeof(INPUT) == 40 on amd64.
type keyInput struct {
	Type  uint32
	_     uint32
	Vk    uint16
	Scan  uint16
	Flags uint32
	Time  uint32
	_     uint32
	Extra uintptr
	_     [8]byte
}

type drawItemStruct struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   uintptr
	Hdc        uintptr
	Item       rect
	ItemData   uintptr
}

type trackMouseEvent struct {
	Size      uint32
	Flags     uint32
	Track     uintptr
	HoverTime uint32
}

type sizeT struct{ CX, CY int32 }

func wstr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

// ptrAt turns a pointer carried in a WPARAM/LPARAM or returned handle into a typed pointer.
func ptrAt[T any](p uintptr) *T { return *(**T)(unsafe.Pointer(&p)) }

func lo16(v uintptr) int { return int(int16(v & 0xFFFF)) }
func hi16(v uintptr) int { return int(int16((v >> 16) & 0xFFFF)) }

func rc(l, t, r, b int) rect { return rect{int32(l), int32(t), int32(r), int32(b)} }

func (r rect) in(x, y int) bool {
	return x >= int(r.L) && x < int(r.R) && y >= int(r.T) && y < int(r.B)
}

func (r rect) inset(d int) rect {
	return rect{r.L + int32(d), r.T + int32(d), r.R - int32(d), r.B - int32(d)}
}

func sendMsg(h, m, w, l uintptr) uintptr {
	r, _, _ := sendMessageW.Call(h, m, w, l)
	return r
}

func defProc(h, m, w, l uintptr) uintptr {
	r, _, _ := defWindowProcW.Call(h, m, w, l)
	return r
}

func invalidate(h uintptr) { invalidateRect.Call(h, 0, 0) }

func visible(h uintptr) bool {
	r, _, _ := isWindowVisible.Call(h)
	return r != 0
}

func tickCount() uint32 {
	r, _, _ := getTickCount.Call()
	return uint32(r)
}

func registerClass(name string, proc, bg, icon uintptr) {
	cur, _, _ := loadCursorW.Call(0, idcArrow)
	wc := wndClassEx{WndProc: proc, Instance: hinst, Cursor: cur, Background: bg, ClassName: wstr(name), Icon: icon, IconSm: icon}
	wc.Size = uint32(unsafe.Sizeof(wc))
	registerClassExW.Call(uintptr(unsafe.Pointer(&wc)))
}

func createWindow(ex uintptr, class, title string, style uintptr, x, y, w, h int, parent, id uintptr) uintptr {
	r, _, _ := createWindowExW.Call(ex, uintptr(unsafe.Pointer(wstr(class))), uintptr(unsafe.Pointer(wstr(title))),
		style, uintptr(x), uintptr(y), uintptr(w), uintptr(h), parent, id, hinst, 0)
	return r
}

func windowText(h uintptr) string {
	n, _, _ := getWindowTextLengthW.Call(h)
	buf := make([]uint16, n+1)
	getWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return syscall.UTF16ToString(buf)
}

func cursorPos() point {
	var pt point
	getCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	return pt
}

func monitorAt(pt point) uintptr {
	r, _, _ := monitorFromPoint.Call(uintptr(uint32(pt.X))|uintptr(uint32(pt.Y))<<32, monDefNearest)
	return r
}

func workArea(mon uintptr) rect {
	mi := monitorInfo{}
	mi.Size = uint32(unsafe.Sizeof(mi))
	getMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))
	return mi.Work
}

// monitorDPI returns the effective DPI of a monitor, falling back to the system DPI before Windows 8.1.
func monitorDPI(mon uintptr) int {
	if getDpiForMonitor.Find() == nil {
		var x, y uint32
		r, _, _ := getDpiForMonitor.Call(mon, 0, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)))
		if r == 0 && x > 0 {
			return int(x)
		}
	}
	dc, _, _ := getDC.Call(0)
	d, _, _ := getDeviceCaps.Call(dc, logPixelsX)
	releaseDC.Call(0, dc)
	if d == 0 {
		return 96
	}
	return int(d)
}

func setDPIAware() {
	if setProcessDpiAwarenessContext.Find() == nil {
		if r, _, _ := setProcessDpiAwarenessContext.Call(^uintptr(3)); r != 0 { // -4: PER_MONITOR_AWARE_V2
			return
		}
	}
	if setProcessDPIAware.Find() == nil {
		setProcessDPIAware.Call()
	}
}

func messageBox(text string) {
	messageBoxW.Call(0, uintptr(unsafe.Pointer(wstr(text))), uintptr(unsafe.Pointer(wstr(appName))), 0x10)
}

func setClipboard(s string) bool {
	u, err := syscall.UTF16FromString(s)
	if err != nil {
		return false
	}
	opened := false
	for i := 0; i < 10 && !opened; i++ {
		r, _, _ := openClipboard.Call(msgHwnd)
		if opened = r != 0; !opened {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if !opened {
		return false
	}
	defer closeClipboard.Call()
	emptyClipboard.Call()
	h, _, _ := globalAlloc.Call(gmemMoveable, uintptr(len(u)*2))
	if h == 0 {
		return false
	}
	p, _, _ := globalLock.Call(h)
	if p == 0 {
		globalFree.Call(h)
		return false
	}
	copy(unsafe.Slice(ptrAt[uint16](p), len(u)), u)
	globalUnlock.Call(h)
	if r, _, _ := setClipboardData.Call(cfUnicodeText, h); r == 0 {
		globalFree.Call(h)
		return false
	}
	return true
}

func sendCtrlV() {
	in := []keyInput{
		{Type: inputKeyboard, Vk: vkControl},
		{Type: inputKeyboard, Vk: 'V'},
		{Type: inputKeyboard, Vk: 'V', Flags: keyeventfKeyUp},
		{Type: inputKeyboard, Vk: vkControl, Flags: keyeventfKeyUp},
	}
	sendInput.Call(uintptr(len(in)), uintptr(unsafe.Pointer(&in[0])), unsafe.Sizeof(in[0]))
}

// loadIcon picks the best size from the embedded .ico directory.
func loadIcon(cx int) uintptr {
	off, _, _ := lookupIconIdFromDirectoryEx.Call(uintptr(unsafe.Pointer(&iconData[0])), 1, uintptr(cx), uintptr(cx), 0)
	if off == 0 || int(off) >= len(iconData) {
		return 0
	}
	h, _, _ := createIconFromResourceEx.Call(uintptr(unsafe.Pointer(&iconData[off])), uintptr(len(iconData))-off, 1, 0x00030000, uintptr(cx), uintptr(cx), 0)
	return h
}
