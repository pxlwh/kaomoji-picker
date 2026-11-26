package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type queryResult struct {
	Matches int `json:"matches"`
	Shown   int `json:"shown"`
}

type selftestReport struct {
	Version    string                   `json:"version"`
	Total      int                      `json:"total"`
	Categories int                      `json:"categories"`
	Queries    map[string]queryResult   `json:"queries"`
	Favorites  favReport                `json:"favorites"`
	Palettes   map[string]paletteReport `json:"palettes"`
	TrayIconOK bool                     `json:"tray_icon_ok"`
	SettingsOK bool                     `json:"settings_ok"`
}

// runSelftest touches no display, no real config and no GTK; it runs before gtk_init.
func runSelftest(out string) error {
	ix, err := loadIndex(rawData)
	if err != nil {
		return err
	}
	rep := selftestReport{Version: appVersion, Total: len(ix.Entries), Categories: len(ix.Cats),
		Queries: map[string]queryResult{}, Palettes: map[string]paletteReport{}}
	var errs []error
	for _, q := range []string{"", "e", "cat", "sad", "shrug", "gf", "creepy smile"} {
		n := len(ix.Filter(q, ""))
		rep.Queries[q] = queryResult{n, min(n, firstPage)}
	}
	if rep.Queries[""].Matches != rep.Total {
		errs = append(errs, errors.New("empty query does not match everything"))
	}
	// AND across groups: a two-word query can never beat either word alone
	if c := rep.Queries["creepy smile"].Matches; c == 0 || c > len(ix.Filter("creepy", "")) || c > len(ix.Filter("smile", "")) {
		errs = append(errs, fmt.Errorf("creepy smile = %d, AND semantics broken", c))
	}
	if rep.Queries["gf"].Matches <= len(ix.Filter("girlfriend", "")) {
		errs = append(errs, errors.New("gf shorthand not expanding"))
	}
	var ferr error
	if rep.Favorites, ferr = selftestFavorites(ix); ferr != nil {
		errs = append(errs, ferr)
	}
	for _, h := range []string{"#ff6fae", "#3fa9f5"} {
		c, _ := parseHex(h)
		r, perr := checkPalette(c)
		rep.Palettes[h] = r
		if perr != nil {
			errs = append(errs, perr)
		}
	}
	if img, err := tintHeart(0x3fa9f5); err == nil {
		px := img.NRGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
		corner := img.NRGBAAt(0, 0)
		rep.TrayIconOK = px.R == 0x3f && px.G == 0xa9 && px.B == 0xf5 && px.A == 255 && corner.A == 0
	}
	if !rep.TrayIconOK {
		errs = append(errs, errors.New("tray icon tint wrong"))
	}
	rep.SettingsOK = selftestSettings()
	if !rep.SettingsOK {
		errs = append(errs, errors.New("settings save/reload mismatch"))
	}
	b, err := json.Marshal(rep)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if out == "" {
		_, err = os.Stdout.Write(b)
	} else {
		err = os.WriteFile(out, b, 0o644)
	}
	return errors.Join(append(errs, err)...)
}

func selftestSettings() bool {
	dir, err := os.MkdirTemp("", "kao-set")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	p := filepath.Join(dir, "settings.json")
	want := Settings{Accent: "#3fa9f5", PasteAfterPick: false, AlwaysOnTop: true, ShowTags: false}
	if saveSettings(p, want) != nil || loadSettings(p) != want {
		return false
	}
	os.WriteFile(p, []byte(`{"accent":"nope"}`), 0o644)
	return loadSettings(p).Accent == defaultAccent && loadSettings(filepath.Join(dir, "missing")) == defaultSettings()
}
