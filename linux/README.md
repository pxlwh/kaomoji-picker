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
`appimagetool` for the AppImage. The overlay mode also needs `meson`, `ninja`,
`wayland-client`, `wayland-protocols` and `wayland-scanner` to build gtk-layer-shell.

    make build      # CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o kaomoji-picker .
    make deps       # download, verify and build gtk-layer-shell into build/ls-prefix (static)
    make test       # go vet, go test, headless --selftest, --version
    make appimage   # runs deps, then KaomojiPicker-x86_64.AppImage in this directory
    make data       # regenerate data/kaomoji.json from ~/.cache/kaomoji/kaomoji.json

gtk-layer-shell is not a system dependency. `make deps` fetches the release tarball pinned
in the `Makefile` (URL and sha256), builds it with meson as a static library into
`build/ls-prefix` (`build/` is gitignored), and `make build` then compiles with
`-DHAVE_LAYER_SHELL` and links `libgtk-layer-shell.a` into the binary. Without `make deps`,
`make build` uses a system `gtk-layer-shell-0` if pkg-config finds one, otherwise it builds
without overlay support. `--selftest` reports which in `layer_shell.compiled`.

`make data` converts the cache written by `kaomoji-fetch.sh`
(the project's fetch script, run with
`--force` to refetch) into the compact `{"t","c","g","l"}` rows with `jq`. After
regenerating, update the pinned counts in `app_test.go`.

The AppImage holds only the binary, `AppRun`, the desktop file and the icon. GTK,
AppIndicator and glibc (2.34 or newer) come from the host system and are not bundled.
gtk-layer-shell is linked in statically, so there is no `libgtk-layer-shell.so` to ship
(`make appimage` fails if `ldd` finds one).

## Run

    ./kaomoji-picker                 # or ./KaomojiPicker-x86_64.AppImage

A heart appears in the tray. Its menu has `Open picker`, `Settings…`, `Reload data`,
`Exit`. AppIndicator does not tell the app about left clicks: most trays open the menu on
left click, and middle click opens the picker directly. Starting the app a second time
opens the picker in the running copy, and `--toggle` with no copy running starts one in
the background and opens its picker.

Flags that never open a window:

    kaomoji-picker --version                         prints "Kaomoji Picker (Linux) 1.0.0"
    kaomoji-picker --selftest --out report.json      headless data/search/favourites/palette check
    kaomoji-picker --render out.png                  paint the picker at 1100x700 into a PNG
    kaomoji-picker --render out.png --render-query sad
    kaomoji-picker --render-settings out.png         paint the settings pane
    kaomoji-picker --render-settings out.png --render-settings-hex '#12'
                                                     ...with that text in the accent field ('#12'
                                                     shows the red border and error line)
    kaomoji-picker --render out.png --render-screen 1920x1080
                                                     paint as the overlay would on a 1920x1080
                                                     output: panel centred, margins transparent

`--toggle` opens or closes the picker in the running copy and starts one if none runs, so
it opens a window.

`--selftest` and the `--render` flags run before GTK is initialised, so they work without a
display (CI, ssh). They read your settings and favourites but never write them.
It exits non-zero if a check fails. For the AppImage without FUSE add
`--appimage-extract-and-run` or set `APPIMAGE_EXTRACT_AND_RUN=1`.

## Overlay or window

On compositors with the `wlr-layer-shell` protocol (Hyprland, sway and other wlroots
compositors, KDE Plasma's KWin) the picker is an overlay: a screen-sized layer surface on
the overlay layer of the focused output, with the panel (up to 1100x700, 40 px from each
edge on smaller screens) centred in it. It sits above every window, including fullscreen ones,
and gets the keyboard on demand. Only the panel takes input, so the transparent area around it
passes clicks to the windows below.

On GNOME and on X11 there is no layer-shell. There, or in a build without gtk-layer-shell, it
is an undecorated floating window of the same size, opened at the mouse on X11 and placed by
the compositor on Wayland. `Keep picker on top` only applies to this window mode.

## Hotkey

There is no compositor-specific key grab. The running app listens on
`$XDG_RUNTIME_DIR/kaomoji-picker.sock`, and `kaomoji-picker --toggle` sends it one line
and exits; if nothing is listening it starts the app first. Bind it to a key (use the full
path if the binary or AppImage is not on `PATH`):

Hyprland (`hyprland.conf`; opens as an overlay):

    bind = SUPER, K, exec, kaomoji-picker --toggle

Hyprland with a Lua config (`hyprland.lua`):

    hl.bind("SUPER + K", hl.dsp.exec_cmd("kaomoji-picker --toggle"))

sway (`~/.config/sway/config`; overlay) and i3 (window):

    bindsym $mod+k exec kaomoji-picker --toggle

KDE Plasma (overlay on Wayland, window on X11): System Settings > Keyboard > Shortcuts >
Add New > Command or Script, command `kaomoji-picker --toggle`, then set the key.

GNOME (window; GNOME has no layer-shell). This replaces your list of custom shortcuts, so
if you already have some, append the new path to the list printed by
`gsettings get org.gnome.settings-daemon.plugins.media-keys custom-keybindings` instead:

    K=/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/kaomoji/
    gsettings set org.gnome.settings-daemon.plugins.media-keys custom-keybindings "['$K']"
    S=org.gnome.settings-daemon.plugins.media-keys.custom-keybinding:$K
    gsettings set $S name 'Kaomoji picker'
    gsettings set $S command 'kaomoji-picker --toggle'
    gsettings set $S binding '<Super>k'

XFCE: Settings > Keyboard > Application Shortcuts > Add. Cinnamon / MATE: Keyboard >
Shortcuts > Custom Shortcuts.

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
  Clicking a tile picks it. The picker hides 120 ms after it loses focus while the mouse
  is outside it (click another window to dismiss); it stays while the mouse is over it,
  and the settings pane only closes with Esc, Cancel, Save or the tray.
- 600 tiles render up front; 400 more are added as you scroll near the bottom.

Picking copies the face to the clipboard, hides the picker and, if `Paste after picking` is
on, sends Ctrl+V after 250 ms: `wtype -M ctrl -k v -m ctrl` on Wayland if `wtype` is
installed, otherwise `xdotool key ctrl+v` on X11, otherwise the face just stays on the
clipboard. Terminals usually want Ctrl+Shift+V, so paste there yourself.

See [Overlay or window](#overlay-or-window) for where it opens.

## Settings and files

All under `$XDG_CONFIG_HOME/kaomoji-picker/` (default `~/.config/kaomoji-picker/`):

- `settings.json`: `accent` (`#rrggbb`), `paste_after_pick`, `always_on_top`, `show_tags`.
  Edited from the tray's `Settings…` pane, drawn inside the picker window (hex entry plus
  preset swatches). The hex field has the keyboard while the pane is open; an invalid value
  turns its border red with `accent must be #rrggbb` under it, and Save refuses it. Esc or
  Cancel returns to the picker.
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
