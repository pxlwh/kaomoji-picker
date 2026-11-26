# Changelog

## v1.0.0

First public release (ﾉ◕ヮ◕)ﾉ*:･ﾟ✧

### Added

- Windows tray app (`kaomoji-picker.exe`): global hotkey `Ctrl+Alt+K`, mosaic grid of 6582
  faces, search with tags and shorthand, 105 category chips, favourites, paste into the
  previous window.
- Linux tray app (`KaomojiPicker-x86_64.AppImage`): native GTK3 + Ayatana AppIndicator,
  same grid, search, chips and favourites, socket `--toggle` for DE keybinds, clipboard
  plus `wtype`/`xdotool` paste.
- Single-hex theming on both platforms: one accent colour derives the panel, backdrop,
  text, secondary text and selection fills, and retints the tray heart. Default `#ff6fae`.
- Search built on the sources' own per-face labels plus a shorthand map (`gf`, `idk`,
  `flip`, `ily`, `yawn`, `gg`, plural fallback).
- Favourites pinned to the front, with a `★ favorites N` chip on both builds.

### Notes

- Data is embedded: 6582 faces from kaomoji.you, emoticoncentral.com and
  japaneseemoticons.me, duplicates merged so each face keeps every label it was given.
- Windows build is pure Go stdlib with Win32 through `syscall`; Linux uses cgo against
  system GTK3 and libayatana-appindicator3 (glibc ≥ 2.34).
