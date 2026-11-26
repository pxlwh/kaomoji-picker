# Third-party notices

The Linux build links one third-party library: **gtk-layer-shell**. It is shipped as a
shared object inside the AppImage (`usr/lib/libgtk-layer-shell.so.0`), not compiled into
the binary, so it stays replaceable — which is what its licence asks for.

## gtk-layer-shell

- Version: 0.10.1 (pinned in `Makefile` by URL and SHA-256)
- Upstream: https://github.com/wmww/gtk-layer-shell
- Licence: **LGPL-3.0-or-later**, full text in `LICENSE_LGPL.txt` beside this file
- What it does here: lets the picker put itself on the Wayland *overlay* layer so it can
  appear above windows on compositors that support `wlr-layer-shell` (Hyprland, Sway, KWin).
  Without it, the app builds and runs as a normal floating window.
- To replace it with your own build: drop your `libgtk-layer-shell.so.0` into `usr/lib/` of
  an extracted AppDir (or beside the binary), or rebuild the AppImage with `make deps` after
  pointing `LS_VER`/`LS_URL`/`LS_SHA256` at your version. The app has no other link-time
  dependency on it beyond the published API.

The rest of this project is MIT licensed; see `LICENSE`.

## Data

Kaomoji text is scraped from kaomoji.you, emoticoncentral.com and japaneseemoticons.me by
`kaomoji-fetch.sh`. The faces are internet folk art, not ours to license; the per-face
labels used for search come from those sites' own page titles.
