// Kaomoji Picker: a tray app with a global hotkey that opens a searchable kaomoji grid.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	appName       = "Kaomoji Picker"
	appVersion    = "1.0.0"
	msgClass      = "KaomojiPickerMessageWindow" // found by FindWindowW from a second instance
	settingsClass = "KaomojiPickerSettings"
	hotkeyClass   = "KaomojiPickerHotkeyBox"
	mutexName     = `Local\KaomojiPicker.SingleInstance`

	hotkeyID     = 1
	timerPaste   = 2
	wmTray       = wmApp + 1
	wmShowPicker = wmApp + 2
	menuOpen     = 1
	menuSettings = 2
	menuReload   = 3
	menuExit     = 4
)

var (
	hinst, msgHwnd, trayIcon uintptr
	idx                      *Index
	cfg                      Settings
	hotkeyOK                 bool
	taskbarCreated           uintptr
)

// The Win32 message loop and all windows must stay on one OS thread.
func init() { runtime.LockOSThread() }

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	selftest := flag.Bool("selftest", false, "run the headless data self-test and exit")
	out := flag.String("out", "", "self-test report path (default stdout)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("%s %s\n", appName, appVersion)
		return
	}
	if *selftest {
		if err := runSelftest(*out); err != nil {
			fmt.Fprintln(os.Stderr, "selftest failed:", err)
			os.Exit(1)
		}
		return
	}

	setDPIAware()
	if _, _, err := createMutexW.Call(0, 0, uintptr(unsafe.Pointer(wstr(mutexName)))); err == syscall.Errno(errAlreadyExist) {
		signalFirstInstance()
		return
	}

	var err error
	if idx, err = readData(); err != nil {
		messageBox("Could not load kaomoji data: " + err.Error())
		os.Exit(1)
	}
	cfg = loadSettings()
	setFavs(loadFavorites(favoritesPath()))
	hinst, _, _ = getModuleHandleW.Call(0)
	resolveFaceFont()

	if p, err := derivePalette(cfg.Accent); err == nil {
		setPalette(p)
	}
	trayIcon = trayIconFor(colAccent)
	// Class icons stay 0: trayIcon is replaced on theme change, so windows get it via WM_SETICON.
	registerClass(msgClass, syscall.NewCallback(msgProc), 0, 0)
	registerClass(settingsClass, syscall.NewCallback(settingsProc), 0, 0)
	registerClass(hotkeyClass, syscall.NewCallback(hotkeyBoxProc), 0, 0)
	// A hidden top-level window rather than HWND_MESSAGE, because FindWindowW skips message-only windows.
	msgHwnd = createWindow(0, msgClass, appName, wsPopup, 0, 0, 0, 0, 0, 0)
	taskbarCreated, _, _ = registerWindowMessageW.Call(uintptr(unsafe.Pointer(wstr("TaskbarCreated"))))
	pk.create()
	trayAdd()
	applyHotkey()

	var m msgT
	for {
		r, _, _ := getMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if sd.hwnd != 0 {
			if r, _, _ := isDialogMessageW.Call(sd.hwnd, uintptr(unsafe.Pointer(&m))); r != 0 {
				continue
			}
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// signalFirstInstance asks the running instance to open its picker.
func signalFirstInstance() {
	h, _, _ := findWindowW.Call(uintptr(unsafe.Pointer(wstr(msgClass))), 0)
	if h == 0 {
		return
	}
	var pid uint32
	getWindowThreadProcessId.Call(h, uintptr(unsafe.Pointer(&pid)))
	allowSetForegroundWindow.Call(uintptr(pid))
	postMessageW.Call(h, wmShowPicker, 0, 0)
}

func applyHotkey() {
	unregisterHotKey.Call(msgHwnd, hotkeyID)
	hotkeyOK = false
	if mods, vk, _, err := parseHotkey(cfg.Hotkey); err == nil {
		r, _, _ := registerHotKey.Call(msgHwnd, hotkeyID, uintptr(mods|modNoRepeat), uintptr(vk))
		hotkeyOK = r != 0
	}
	trayTip()
	if !hotkeyOK {
		balloon("Hotkey unavailable", "Could not register "+cfg.Hotkey+". It may be in use by another app; pick another in Settings.", niifWarn)
	}
}

func setW(dst []uint16, s string) {
	u, _ := syscall.UTF16FromString(s)
	if len(u) > len(dst) {
		u = append(u[:len(dst)-1], 0)
	}
	copy(dst, u)
}

func trayData() notifyIconData {
	n := notifyIconData{Wnd: msgHwnd, ID: 1, CallbackMessage: wmTray, Icon: trayIcon}
	n.Size = uint32(unsafe.Sizeof(n))
	return n
}

func trayAdd() {
	n := trayData()
	n.Flags = nifMessage | nifIcon | nifTip
	setW(n.Tip[:], appName)
	shellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&n)))
	trayTip()
}

func trayTip() {
	n := trayData()
	n.Flags = nifTip
	setW(n.Tip[:], appName+" ("+cfg.Hotkey+")")
	shellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&n)))
}

func balloon(title, text string, flags uint32) {
	n := trayData()
	n.Flags = nifInfo
	n.InfoFlags = flags
	setW(n.InfoTitle[:], title)
	setW(n.Info[:], text)
	shellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&n)))
}

func trayMenu() {
	m, _, _ := createPopupMenu.Call()
	defer destroyMenu.Call(m)
	for _, it := range []struct {
		id   uintptr
		text string
	}{{menuOpen, "Open picker"}, {menuSettings, "Settings…"}, {menuReload, "Reload data"}, {0, ""}, {menuExit, "Exit"}} {
		if it.id == 0 {
			appendMenuW.Call(m, mfSeparator, 0, 0)
		} else {
			appendMenuW.Call(m, mfString, it.id, uintptr(unsafe.Pointer(wstr(it.text))))
		}
	}
	pt := cursorPos()
	setForegroundWindow.Call(msgHwnd) // required so the menu closes when clicking elsewhere
	cmd, _, _ := trackPopupMenu.Call(m, tpmReturnCm|tpmRightBtn, uintptr(pt.X), uintptr(pt.Y), 0, msgHwnd, 0)
	postMessageW.Call(msgHwnd, wmNull, 0, 0)
	switch cmd {
	case menuOpen:
		pk.show()
	case menuSettings:
		openSettings()
	case menuReload:
		reloadData()
	case menuExit:
		destroyWindow.Call(msgHwnd)
	}
}

func reloadData() {
	ix, err := readData()
	if err != nil {
		balloon("Reload failed", err.Error(), niifWarn)
		return
	}
	idx = ix
	pk.cat, pk.chipScroll = -1, 0
	pk.measureChips()
	pk.refilter()
	balloon("Data reloaded", fmt.Sprintf("%d kaomoji in %d categories", len(ix.Entries), len(ix.Cats)), niifInfo)
}

func msgProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch msg {
	case wmHotkey:
		pk.toggle()
		return 0
	case wmTray:
		switch lp & 0xFFFF {
		case wmLButtonUp:
			pk.toggle()
		case wmRButtonUp, wmContextMenu:
			trayMenu()
		}
		return 0
	case wmShowPicker:
		pk.show()
		return 0
	case wmTimer:
		if wp == timerPaste {
			killTimer.Call(hwnd, timerPaste)
			sendCtrlV()
		}
		return 0
	case wmDestroy:
		n := trayData()
		shellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&n)))
		postQuitMessage.Call(0)
		return 0
	}
	if taskbarCreated != 0 && msg == taskbarCreated {
		trayAdd() // Explorer restarted
		return 0
	}
	return defProc(hwnd, msg, wp, lp)
}
