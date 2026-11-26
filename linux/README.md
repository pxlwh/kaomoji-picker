# Kaomoji Picker for Linux

A tray app that opens a searchable grid of 6582 kaomoji, copies the chosen face to the
clipboard and pastes it into the window you were using. Works on any desktop with a
StatusNotifier tray (KDE, GNOME with the AppIndicator extension, XFCE, Cinnamon, MATE,
waybar/Quickshell trays) on X11 or Wayland.

Native GTK3 + Ayatana AppIndicator through small hand-written cgo bindings (`ui.c`,
`ui.h`, `ui.go`). No third-party Go modules. The data (`data/kaomoji.json`) and the heart
icon are embedded in the binary.

Same search, shorthand, favourites and layout as the Windows build in `../windows`.

## Why not just bundle Quickshell?

A Quickshell QML overlay picker (`qs ipc call kaomoji toggle`) cannot be shipped as a
portable AppImage: Quickshell links against Qt's private ABI, so it only runs against the
exact Qt it was built with, and it draws through `wlr-layer-shell`, which GNOME and X11
sessions do not have. This build is a separate, small GTK3 program instead, and is the
portable one for X11 and non-wlroots desktops.

## Build

Needs Go 1.27+, gcc, `gtk+-3.0` and `ayatana-appindicator3-0.1` development files, and
`appimagetool` for the AppImage.

    make build      # CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o kaomoji-picker .
    make test       # go vet, go test, headless --selftest, --version
    make appimage   # KaomojiPicker-x86_64.AppImage in the repo root
    make data       # regenerate data/kaomoji.json from ~/.cache/kaomoji/kaomoji.json

`make data` converts the cache written by `kaomoji-fetch.sh`
(the project's fetch script, run with
`--force` to refetch) into the compact `{"t","c","g","l"}` rows with `jq`. After
regenerating, update the pinned counts in `app_test.go`.

The AppImage holds only the binary, `AppRun`, the desktop file and the icon. GTK,
AppIndicator and glibc (2.34 or newer) come from the host system and are not bundled.

## Run

    ./kaomoji-picker                 # or ./KaomojiPicker-x86_64.AppImage

A heart appears in the tray. Its menu has `Open picker`, `Settings…`, `Reload data`,
`Exit`. AppIndicator does not tell the app about left clicks: most trays open the menu on
left click, and middle click opens the picker directly. Starting the app a second time
opens the picker in the running copy.

Flags that never open a window:

    kaomoji-picker --version                         prints "Kaomoji Picker (Linux) 1.0.0"
    kaomoji-picker --selftest --out report.json      headless data/search/favourites/palette check
    kaomoji-picker --toggle                          open or close the picker in the running copy

`--selftest` runs before GTK is initialised, so it works without a display (CI, ssh).
It exits non-zero if a check fails. For the AppImage without FUSE add
`--appimage-extract-and-run` or set `APPIMAGE_EXTRACT_AND_RUN=1`.

## Hotkey

There is no compositor-specific key grab. The running app listens on
`$XDG_RUNTIME_DIR/kaomoji-picker.sock`, and `kaomoji-picker --toggle` sends it one line
and exits. Start the app at login, then bind `--toggle` in your desktop's shortcut editor
(use the full path to the binary or AppImage):

- KDE Plasma: System Settings > Keyboard > Shortcuts > Add New > Command.
- GNOME: Settings > Keyboard > View and Customize Shortcuts > Custom Shortcuts.
- XFCE: Settings > Keyboard > Application Shortcuts > Add.
- Cinnamon / MATE: Keyboard > Shortcuts > Custom Shortcuts.
- Hyprland: `bind = SUPER, period, exec, /path/to/kaomoji-picker --toggle`
- Sway / i3: `bindsym $mod+period exec /path/to/kaomoji-picker --toggle`

Autostart: copy `packaging/kaomoji-picker.desktop` to `~/.config/autostart/` and point its
`Exec=` at the binary or AppImage.

## Using the picker

- Type to search. Every word must appear in the face, its category, tags or labels.
  Shorthand expands to alternatives: `gf` finds girlfriend/love/heart/kiss, likewise `idk`,
  `flip`, `ily`, `yawn`, `gg` and others; a trailing `s` also matches the singular.
- Chips: `★ favorites N` first, then categories biggest first. Click to filter, click
  again to clear. Scroll the strip with the mouse wheel.
- Right-click a tile to add or remove a favourite. Favourites get a `★` and are pinned
  first, newest first.
- Enter picks the selected tile, Down moves from the search box into the grid, Esc closes.
  Clicking a tile picks it. The picker hides when it loses focus.
- 600 tiles render up front; 400 more are added as you scroll near the bottom.

Picking copies the face to the clipboard, hides the picker and, if `Paste after picking` is
on, sends Ctrl+V after 250 ms: `wtype -M ctrl -k v -m ctrl` on Wayland if `wtype` is
installed, otherwise `xdotool key ctrl+v` on X11, otherwise the face just stays on the
clipboard. Terminals usually want Ctrl+Shift+V, so paste there yourself.

The window is undecorated, opens at the mouse on X11 and is capped to 1100x700 within the
monitor's work area. Wayland compositors choose where it goes.

## Settings and files

All under `$XDG_CONFIG_HOME/kaomoji-picker/` (default `~/.config/kaomoji-picker/`):

- `settings.json`: `accent` (`#rrggbb`), `paste_after_pick`, `always_on_top`, `show_tags`.
  Edited from the tray's `Settings…` dialog (hex entry plus preset swatches).
- `favorites.json`: favourite faces, newest first.
- `kaomoji.json`: optional; if present it replaces the embedded data (`Reload data` rereads it).

## Theming

Everything comes from the one `accent` hex (default `#ff6fae`):

| Role | Derivation |
|------|------------|
| accent | H |
| accent dim | H mixed 55% toward black |
| panel | H's hue at HSL lightness 0.07, saturation × 0.35 |
| background | lightness 0.04, saturation × 0.30 |
| text | H mixed 85% toward white |
| secondary | H mixed 45% toward panel |
| selection | H at 20% over panel |
| hover | H at 10% over panel |

The tray heart is recoloured to the accent using its alpha channel as the mask, written
to `$XDG_RUNTIME_DIR/kaomoji-picker/heart-RRGGBB.png`. The self-test prints the derived
palette and WCAG contrast ratios for `#ff6fae` and `#3fa9f5`. The minimums it checks are
text 7:1, accent 4.5:1 and secondary 2.5:1; with this formula the secondary colour comes
out at about 3:1.
