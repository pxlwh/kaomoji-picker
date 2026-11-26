# Kaomoji Picker for Windows

A tray app that opens a searchable grid of 6582 kaomoji on a global hotkey, copies the
chosen face to the clipboard and pastes it into the window you were using.

Single `.exe`, pure Go standard library, Win32 through `syscall.NewLazyDLL`. The kaomoji
data (`data/kaomoji.json`) and tray icon (`assets/heart.ico`, `assets/heart.png`) are embedded; nothing is
fetched at runtime.

## Build

Needs Go 1.27 or newer. Builds from Linux or Windows:

    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H windowsgui -s -w" -o kaomoji-picker.exe .

Checks:

    GOOS=windows GOARCH=amd64 go vet ./...
    GOOS=windows GOARCH=amd64 go test -c -o kao-test.exe . && wine kao-test.exe -test.v

## Run

Double-click `kaomoji-picker.exe`. A pink heart appears in the notification area.

- Left click the heart: open or close the picker.
- Right click: `Open picker`, `Settings…`, `Reload data`, `Exit`.
- Launching the exe again while it runs opens the picker in the running copy.

Command line flags (no window is created for these):

    kaomoji-picker.exe --version                      prints "Kaomoji Picker 1.0.0"
    kaomoji-picker.exe --selftest --out report.json   loads the data, runs sample searches, writes a JSON report

The self-test also checks favourites (toggle, pinned-first order, chip count, save and
reload) against a temporary file, never your real `%APPDATA%`, and exits non-zero if a
check fails.

The exe is a GUI-subsystem binary, so `--version` output may not appear in `cmd.exe`;
the self-test report file is the reliable output.

## Hotkey

Default `Ctrl+Alt+K`. It can be changed in Settings: click the capture box and press a
combination of Ctrl / Alt / Shift / Win with one key A-Z, 0-9, F1-F24 or Space. If the
hotkey cannot be registered (another app owns it), a tray notification says so and the
app keeps running; the tray icon still works.

## Using the picker

The picker opens centred on the monitor under the mouse cursor.

- Type to search. Every word must appear in the face, its category or its tags.
- Click a category chip to filter by it, click it again to clear. Scroll the chip strip with the mouse wheel.
- Right-click a tile to add or remove it from your favourites. Favourites show a pink `★` in the tile's top-right corner and are pinned first in every result list, newest favourite first.
- The first chip, `★ favorites N`, shows only favourites; click it again to clear. It combines with the search text and with a category chip.
- Arrow keys, PgUp/PgDn move the selection, Enter picks, Esc closes.
- Hovering a tile selects it, clicking picks it. Clicking outside the picker closes it.

Picking copies the face to the clipboard, returns focus to the previous window and, if
`Paste after picking` is on, sends Ctrl+V after about 250 ms.

## Settings

Tray menu, `Settings…`:

| Setting | Default | Effect |
|---|---|---|
| Hotkey | `Ctrl+Alt+K` | Global shortcut to open the picker |
| Start with Windows | off | Adds or removes `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\KaomojiPicker` |
| Paste after picking | on | Sends Ctrl+V to the previous window after copying |
| Always on top | on | Keeps the picker above other windows |
| Open on monitor with cursor | on | Otherwise opens on the monitor of the active window |
| Show tags under faces | on | Category and tags line under each face |
| Accent colour | `#ff6fae` | One hex colour that themes the whole UI and the tray icon, see Theming |

## Theming

One hex accent (`accent` in `settings.json`, default `#ff6fae`) drives every colour.
`derivePalette` in `theme.go` computes, from accent H:

| Colour | Derivation |
|---|---|
| accent | H (star glyph, selection border, chip accents) |
| accentDim | H mixed 55% toward black (borders, tags) |
| panel | H hue, HSL lightness 0.07, H's saturation x 0.35 |
| bg | H hue, HSL lightness 0.04, H's saturation x 0.30 |
| text | H mixed 85% toward white |
| secondary | H mixed 45% toward panel (counts, chip text) |
| tile selected / hover | H at 20% / 10% opacity pre-blended over panel |

Saturation is scaled, not set, so a grey accent gives a neutral grey UI. Hex input accepts
`#rgb` / `#rrggbb`, with or without `#`, any case; anything else is rejected with a message
in the settings dialog and the dialog stays open. A malformed `accent` in `settings.json`
falls back to `#ff6fae`.

In Settings, type a hex or click one of six swatches; the strip shows accent / panel / text
live. Save repaints immediately: the brush and font caches are deleted and rebuilt, and the
tray icon is re-tinted. The icon is `assets/heart.png`'s alpha channel box-filtered to
`SM_CXSMICON`, filled with the accent, built with `CreateIconIndirect` from two 32-bit DIBs
and swapped in with `Shell_NotifyIcon(NIM_MODIFY)`; the old `HICON` is destroyed. If that
fails the embedded `heart.ico` is used.

`--selftest` reports the derived palettes for the default and `#3fa9f5` under `theme`, and
`contrast_ok`. It fails (non-zero exit) if text on panel is below 4.5:1.

## Where config lives

    %APPDATA%\KaomojiPicker\settings.json
    %APPDATA%\KaomojiPicker\favorites.json

The folder is created on first save. A missing or malformed `settings.json` falls back to
the defaults above. `favorites.json` is a JSON array of face strings, newest first; it is
rewritten on every right-click, and a missing or malformed file means no favourites.

`Reload data` reads `%APPDATA%\KaomojiPicker\kaomoji.json` if it exists (same format as
`data/kaomoji.json`: an array of `{"t": face, "c": category, "g": [tags]}`), otherwise the
embedded data. That file is also used at startup.

## Source layout

| File | Contents |
|---|---|
| `main.go` | Flags, lifecycle, single instance, tray, hotkey registration |
| `win32.go` | DLL procs, structs, constants, small Win32 helpers |
| `picker.go` | Picker window, layout, keyboard, mouse, picking |
| `draw.go` | Colour vars, brush/font cache, painting |
| `theme.go` | Palette derivation, contrast, tray icon tinting, theme switch |
| `settings.go` | Config file, hotkey parsing, Run key, settings dialog |
| `data.go` | Embedded data, search index, filter, favourites, self-test |
| `app_test.go` | Hotkey parser, filter, colour and theme tests |
