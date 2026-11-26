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
	appVersion = "1.0.0"
)

// GTK and every Go callback from it must stay on the main OS thread.
func init() { runtime.LockOSThread() }

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	selftest := flag.Bool("selftest", false, "run the headless self-test and exit (no display needed)")
	out := flag.String("out", "", "self-test report path (default stdout)")
	toggleFlag := flag.Bool("toggle", false, "open or close the picker in the running instance")
	flag.Parse()

	switch {
	case *showVersion:
		fmt.Printf("%s %s\n", appName, appVersion)
	case *selftest:
		if err := runSelftest(*out); err != nil {
			fmt.Fprintln(os.Stderr, "selftest failed:", err)
			os.Exit(1)
		}
	case *toggleFlag:
		if err := sendCmd("toggle"); err != nil {
			fmt.Fprintln(os.Stderr, "kaomoji-picker is not running (start it first):", err)
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
