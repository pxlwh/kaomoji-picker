// Kaomoji Picker (Linux): a GTK3 tray app with a searchable kaomoji grid, toggled over a unix socket.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
)

const (
	appName    = "Kaomoji Picker (Linux)"
	appVersion = "1.0.1"
)

// GTK and every Go callback from it must stay on the main OS thread.
func init() { runtime.LockOSThread() }

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	selftest := flag.Bool("selftest", false, "run the headless self-test and exit (no display needed)")
	out := flag.String("out", "", "self-test report path (default stdout)")
	toggleFlag := flag.Bool("toggle", false, "open or close the picker in the running instance, starting one if none runs")
	diagFlag := flag.Bool("diag", false, "print display capabilities (visual, layer-shell) and exit")
	render := flag.String("render", "", "paint the picker at 1100x700 into this PNG and exit (no display needed)")
	renderQuery := flag.String("render-query", "", "search text for --render")
	renderSettings := flag.String("render-settings", "", "paint the settings pane at 1100x700 into this PNG and exit (no display needed)")
	renderHex := flag.String("render-settings-hex", "", "accent field text for --render-settings (default: the saved accent)")
	renderScreen := flag.String("render-screen", "", "with --render/--render-settings: paint as a WxH layer-shell overlay, panel centred")
	flag.Parse()

	w, h, screen := 1100, 700, *renderScreen != ""
	if _, err := fmt.Sscanf(*renderScreen, "%dx%d", &w, &h); screen && (err != nil || w < 1 || h < 1) {
		fmt.Fprintln(os.Stderr, "kaomoji-picker: --render-screen wants WxH, e.g. 1920x1080")
		os.Exit(2)
	}
	switch {
	case *showVersion:
		fmt.Printf("%s %s\n", appName, appVersion)
	case *selftest:
		if err := runSelftest(*out); err != nil {
			fmt.Fprintln(os.Stderr, "selftest failed:", err)
			os.Exit(1)
		}
	case *diagFlag:
		os.Exit(diag())
	case *render != "":
		if err := runRender(*render, *renderQuery, false, "", screen, w, h); err != nil {
			fmt.Fprintln(os.Stderr, "kaomoji-picker:", err)
			os.Exit(1)
		}
	case *renderSettings != "":
		if err := runRender(*renderSettings, "", true, *renderHex, screen, w, h); err != nil {
			fmt.Fprintln(os.Stderr, "kaomoji-picker:", err)
			os.Exit(1)
		}
	case *toggleFlag:
		if err := toggleOrStart(startDetached); err != nil {
			fmt.Fprintln(os.Stderr, "kaomoji-picker --toggle:", err)
			os.Exit(1)
		}
	default:
		ln, err := listen()
		if errors.Is(err, errRunning) {
			return // the running instance was asked to show its picker
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "kaomoji-picker: socket:", err)
			os.Exit(1)
		}
		os.Exit(runApp(ln))
	}
}
