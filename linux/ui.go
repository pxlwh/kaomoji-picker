package main

/*
#cgo pkg-config: gtk+-3.0 ayatana-appindicator3-0.1
#cgo CFLAGS: -Wno-deprecated-declarations
#cgo LDFLAGS: -lm
#include <stdlib.h>
#include "ui.h"
*/
import "C"

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	cmdToggle = 1
	cmdShow   = 2
)

var (
	idx *Index
	cfg Settings
	st  = struct {
		query   string
		cat     int // index into idx.Cats, -1 for all
		favOnly bool
		matches []int
		limit   int
	}{cat: -1}
)

func cs(s string) (*C.char, func()) {
	c := C.CString(s)
	return c, func() { C.free(unsafe.Pointer(c)) }
}

func sockPath() string { return runtimeDir() + "/kaomoji-picker.sock" }

func sendCmd(cmd string) error {
	c, err := net.DialTimeout("unix", sockPath(), time.Second)
	if err != nil {
		return err
	}
	defer c.Close()
	_, err = c.Write([]byte(cmd + "\n"))
	return err
}

var errRunning = errors.New("already running")

// toggleOrStart toggles the running instance; with none listening it start()s one and retries
// until its socket answers, so the toggle lands and opens the fresh instance's picker.
func toggleOrStart(start func() error) error {
	if sendCmd("toggle") == nil {
		return nil
	}
	if err := start(); err != nil {
		return err
	}
	var err error
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
		if err = sendCmd("toggle"); err == nil {
			return nil
		}
	}
	return fmt.Errorf("started, but the socket never answered: %w", err)
}

// startDetached runs this program with no flags in its own session, outliving the caller.
// Inside an AppImage it relaunches the AppImage, because our mount goes away when we exit.
func startDetached() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if a, d := os.Getenv("APPIMAGE"), os.Getenv("APPDIR"); a != "" && d != "" && strings.HasPrefix(exe, d) {
		exe = a
	}
	cmd := exec.Command(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// listen claims the socket; a live instance gets "show" instead, a stale socket file is replaced.
func listen() (net.Listener, error) {
	if sendCmd("show") == nil {
		return nil, errRunning
	}
	os.Remove(sockPath())
	return net.Listen("unix", sockPath())
}

func serve(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer c.Close()
			c.SetReadDeadline(time.Now().Add(2 * time.Second))
			line, _ := bufio.NewReader(c).ReadString('\n')
			switch strings.TrimSpace(line) {
			case "toggle":
				C.kp_post(cmdToggle)
			case "show":
				C.kp_post(cmdShow)
			}
		}()
	}
}

// diag prints the display capabilities and exits; used to explain an opaque overlay.
func diag() int {
	if C.kp_init() == 0 {
		fmt.Fprintln(os.Stderr, "kaomoji-picker: cannot open a display")
		return 1
	}
	fmt.Print(C.GoString(C.kp_diag()))
	fmt.Print(C.GoString(C.kp_diag_window()))
	return 0
}

func runApp(ln net.Listener) int {
	if C.kp_init() == 0 {
		fmt.Fprintln(os.Stderr, "kaomoji-picker: cannot open a display")
		return 1
	}
	var err error
	if idx, err = readData(); err != nil {
		fmt.Fprintln(os.Stderr, "kaomoji-picker: could not load data:", err)
		return 1
	}
	cfg = loadSettings(settingsPath())
	setFavs(loadFavorites(favoritesPath()))
	C.kp_build()
	icon := applyTheme()
	ic, free := cs(icon)
	C.kp_tray(ic)
	free()
	rebuildChips()
	refilter(firstPage, 0)
	go serve(ln)
	C.kp_main()
	ln.Close() // unlinks the socket
	return 0
}

// layerCompiled reports whether ui.c was built with gtk-layer-shell (HAVE_LAYER_SHELL).
func layerCompiled() bool { return C.kp_layer_compiled() != 0 }

// applyTheme pushes the derived CSS and returns the tinted tray icon path.
func applyTheme() string {
	accent := setPalette()
	c, free := cs(derivePalette(accent).css())
	C.kp_set_css(c)
	free()
	icon, err := writeTrayIcon(accent)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kaomoji-picker: tray icon:", err)
		icon = "face-smile" // themed fallback so the indicator still shows
	}
	return icon
}

// setPalette hands the canvas its colours and returns the accent.
func setPalette() uint32 {
	accent, _ := parseHex(cfg.Accent)
	p := derivePalette(accent)
	C.kp_set_palette(C.uint(p.Accent), C.uint(p.AccentDim), C.uint(p.Bg), C.uint(p.Text), C.uint(p.Secondary))
	return accent
}

// runRender paints the picker (or, with settings, the settings pane with hexText in the accent
// field, "" for the saved accent) exactly as on screen into a PNG, without GTK or a display.
// With screen, w x h is an output and the panel is centred in it as on a layer-shell surface.
func runRender(path, query string, settings bool, hexText string, screen bool, w, h int) error {
	var err error
	if idx, err = readData(); err != nil {
		return err
	}
	cfg = loadSettings(settingsPath())
	setFavs(loadFavorites(favoritesPath()))
	setPalette()
	rebuildChips()
	st.query = query
	refilter(firstPage, 0)
	if settings {
		if hexText == "" {
			hexText = cfg.Accent
		}
		openSettings(hexText)
	}
	p, free1 := cs(path)
	defer free1()
	q, free2 := cs(query)
	defer free2()
	if s := C.kp_render(p, q, C.int(b2i(settings)), C.int(b2i(screen)), C.int(w), C.int(h)); s != 0 {
		return fmt.Errorf("render %s: cairo status %d", path, int(s))
	}
	return nil
}

func chipLabel(i int) string {
	if i == 0 {
		return "★ favorites  " + strconv.Itoa(idx.favCount(favs))
	}
	return idx.Cats[i-1].Name + " " + strconv.Itoa(idx.Cats[i-1].Count)
}

func rebuildChips() {
	C.kp_clear_chips()
	for i := 0; i <= len(idx.Cats); i++ {
		c, free := cs(chipLabel(i))
		C.kp_add_chip(c, C.int(i))
		free()
	}
	chipStates()
}

func chipStates() {
	for i := 0; i <= len(idx.Cats); i++ {
		on := (i == 0 && st.favOnly) || (i > 0 && i-1 == st.cat)
		c, free := cs(chipLabel(i))
		C.kp_chip_state(C.int(i), c, C.int(b2i(on)))
		free()
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// refilter recomputes matches and renders the first n tiles, then restores scroll.
func refilter(n int, scroll float64) {
	cat := ""
	if st.cat >= 0 && st.cat < len(idx.Cats) {
		cat = idx.Cats[st.cat].Name
	} else {
		st.cat = -1
	}
	st.matches = idx.FilterFav(st.query, cat, favs, st.favOnly)
	st.limit = 0
	C.kp_reset_tiles()
	appendTiles(n)
	C.kp_select_first()
	if scroll > 0 {
		C.kp_set_scroll(C.double(scroll))
	}
}

func appendTiles(n int) {
	end := min(len(st.matches), st.limit+n)
	for _, i := range st.matches[st.limit:end] {
		e := idx.Entries[i]
		tags := ""
		if cfg.ShowTags {
			tags = strings.Join(append([]string{e.C}, e.G...), " · ")
		}
		f, free1 := cs(e.T)
		t, free2 := cs(tags)
		C.kp_add_tile(f, t, C.int(b2i(favSet[e.T])))
		free1()
		free2()
	}
	st.limit = end
	// same header text as windows/draw.go
	count := fmt.Sprintf("%d / %d", st.limit, len(idx.Entries))
	if st.cat >= 0 || st.favOnly || strings.TrimSpace(st.query) != "" {
		count = fmt.Sprintf("%d matches · showing %d", len(st.matches), st.limit)
	}
	c, free := cs(count)
	C.kp_set_count(c)
	free()
}

func toggle() {
	if C.kp_visible() != 0 {
		C.kp_hide()
	} else {
		C.kp_show(C.int(b2i(cfg.AlwaysOnTop)))
	}
}

//export goCommand
func goCommand(cmd C.int) {
	switch cmd {
	case cmdToggle:
		toggle()
	case cmdShow:
		C.kp_show(C.int(b2i(cfg.AlwaysOnTop)))
	}
}

//export goSearch
func goSearch(q *C.char) {
	st.query = C.GoString(q)
	refilter(firstPage, 0)
}

//export goChip
func goChip(i C.int) {
	switch {
	case i == 0:
		st.favOnly = !st.favOnly
	case int(i)-1 == st.cat:
		st.cat = -1
	default:
		st.cat = int(i) - 1
	}
	chipStates()
	refilter(firstPage, 0)
}

//export goMore
func goMore() {
	if st.limit < len(st.matches) {
		appendTiles(nextPage)
	}
}

//export goFav
func goFav(i C.int) {
	if int(i) < 0 || int(i) >= st.limit {
		return
	}
	setFavs(toggleFav(favs, idx.Entries[st.matches[i]].T))
	if err := saveFavorites(favoritesPath(), favs); err != nil {
		fmt.Fprintln(os.Stderr, "kaomoji-picker: could not save favorites:", err)
	}
	chipStates()
	refilter(st.limit, float64(C.kp_get_scroll()))
}

//export goPick
func goPick(i C.int) {
	if int(i) < 0 || int(i) >= st.limit {
		return
	}
	c, free := cs(idx.Entries[st.matches[i]].T)
	C.kp_copy(c)
	free()
	C.kp_hide()
	if cfg.PasteAfterPick {
		go paste()
	}
}

// paste sends Ctrl+V to whatever regains focus; without a tool the face stays on the clipboard.
func paste() {
	time.Sleep(250 * time.Millisecond) // let the compositor hand focus back first
	var cmd *exec.Cmd
	if p, err := exec.LookPath("wtype"); err == nil && os.Getenv("WAYLAND_DISPLAY") != "" {
		cmd = exec.Command(p, "-M", "ctrl", "-k", "v", "-m", "ctrl")
	} else if p, err := exec.LookPath("xdotool"); err == nil && os.Getenv("DISPLAY") != "" {
		cmd = exec.Command(p, "key", "--clearmodifiers", "ctrl+v")
	} else {
		return
	}
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "kaomoji-picker: paste:", err)
	}
}

//export goMenu
func goMenu(id C.int) {
	switch id {
	case 1:
		C.kp_show(C.int(b2i(cfg.AlwaysOnTop)))
	case 2:
		openSettings(cfg.Accent)
		C.kp_show(C.int(b2i(cfg.AlwaysOnTop)))
	case 3:
		ix, err := readData()
		if err != nil {
			fmt.Fprintln(os.Stderr, "kaomoji-picker: reload failed:", err)
			return
		}
		idx, st.cat = ix, -1
		rebuildChips()
		refilter(firstPage, 0)
		fmt.Fprintf(os.Stderr, "kaomoji-picker: reloaded %d kaomoji in %d categories\n", len(ix.Entries), len(ix.Cats))
	case 4:
		C.kp_quit()
	}
}

// openSettings switches the canvas to the settings pane, seeded from cfg with hexText in the field.
func openSettings(hexText string) {
	h, free := cs(hexText)
	C.kp_settings(h, C.int(b2i(cfg.PasteAfterPick)), C.int(b2i(cfg.AlwaysOnTop)), C.int(b2i(cfg.ShowTags)))
	free()
}

// goSaveSettings returns NULL on success or a malloc'd error message the C side frees.
//
//export goSaveSettings
func goSaveSettings(h *C.char, paste, top, tags C.int) *C.char {
	accent, err := parseHex(C.GoString(h))
	if err != nil {
		return C.CString(err.Error())
	}
	next := Settings{Accent: hex(accent), PasteAfterPick: paste != 0, AlwaysOnTop: top != 0, ShowTags: tags != 0}
	if err := saveSettings(settingsPath(), next); err != nil {
		return C.CString("could not save: " + err.Error())
	}
	cfg = next
	ic, free := cs(applyTheme())
	C.kp_set_icon(ic)
	free()
	C.kp_set_keep_above(C.int(b2i(cfg.AlwaysOnTop)))
	refilter(firstPage, 0)
	return nil
}
