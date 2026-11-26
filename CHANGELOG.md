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

## v1.0.1

The Linux build becomes an overlay, and the settings dialog stops looking like GTK.

### Added

- **Layer-shell overlay** on compositors that support `wlr-layer-shell` (Hyprland, Sway,
  KWin): full-screen transparent surface on the overlay layer, panel centred, input region
  clipped to the panel so the margins click through. Falls back to a floating undecorated
  window on X11 and GNOME. `gtk-layer-shell` 0.10.1 is built from a pinned, sha256-checked
  tarball and shipped as a shared object inside the AppImage (LGPLv3, see
  `linux/THIRD-PARTY-NOTICES.md`).
- **Settings is a pane** drawn on the picker's own canvas — hex field with live validation
  and an error line, preset swatches, hand-drawn checkboxes, Cancel/Save. No GTK dialog.
- `--toggle` starts the app when nothing is running instead of exiting 1.
- `--render`, `--render-query`, `--render-settings`, `--render-settings-hex` and
  `--render-screen` for headless screenshots of the real paint path.

### Fixed

- The AppImage now ships the full LGPL-3.0 text for the bundled `gtk-layer-shell`
  (`usr/share/doc/kaomoji-picker/LICENSE-LGPL-3.0.txt`), and both binaries report their own
  version instead of a hard-coded 1.0.0.
- The picker closed the moment the pointer left it (focus-follows-mouse counts as focus-out).
  It now hides only on a real focus loss after a grace period, and never while settings is open.
- The panel was an opaque slab; it is now 0.93 alpha like the Quickshell overlay, so a
  blurring compositor shows through.
- Typing into the accent field did nothing: the pane now takes keyboard focus on open.
- The first open after starting the app closed instantly, because a phantom focus-out arrives
  before the surface ever holds focus.
