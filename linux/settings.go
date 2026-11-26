package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	Accent         string `json:"accent"`
	PasteAfterPick bool   `json:"paste_after_pick"`
	AlwaysOnTop    bool   `json:"always_on_top"`
	ShowTags       bool   `json:"show_tags"`
}

func defaultSettings() Settings {
	return Settings{Accent: defaultAccent, PasteAfterPick: true, AlwaysOnTop: true, ShowTags: true}
}

func settingsPath() string {
	if dir, err := configDir(); err == nil {
		return filepath.Join(dir, "settings.json")
	}
	return ""
}

// loadSettings starts from defaults so missing keys keep their default; a bad accent is reset.
func loadSettings(path string) Settings {
	s := defaultSettings()
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, &s)
	}
	if _, err := parseHex(s.Accent); err != nil {
		s.Accent = defaultAccent
	}
	return s
}

func saveSettings(path string, s Settings) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
