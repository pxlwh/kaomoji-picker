BIN      := kaomoji-picker
APPIMAGE := KaomojiPicker-x86_64.AppImage
CACHE    := $(HOME)/.cache/kaomoji/kaomoji.json

# gtk-layer-shell, built from source and linked **dynamically** (no system package needed).
# Dynamic on purpose: the library is LGPLv3, and shipping the .so lets users replace it.
LS_VER    := 0.10.1
LS_URL    := https://github.com/wmww/gtk-layer-shell/archive/refs/tags/v$(LS_VER).tar.gz
LS_SHA256 := 88c3a3e0a5300532f3d368d5df64838a87f1fb85273f22d41df0a6b8d0ec59c6
LS_TAR    := build/gtk-layer-shell-$(LS_VER).tar.gz
LS_SRC    := build/gtk-layer-shell-$(LS_VER)
LS_PREFIX := $(CURDIR)/build/ls-prefix
LS_PC     := $(LS_PREFIX)/lib/pkgconfig

# Use the vendored static build when `make deps` has run, else a system gtk-layer-shell if
# pkg-config finds one, else build without it (ui.c falls back to a floating window).
ifneq ($(wildcard $(LS_PC)/gtk-layer-shell-0.pc),)
export PKG_CONFIG_PATH := $(LS_PC)$(if $(PKG_CONFIG_PATH),:$(PKG_CONFIG_PATH))
endif
LS_FOUND := $(shell PKG_CONFIG_PATH="$(PKG_CONFIG_PATH)" pkg-config --exists gtk-layer-shell-0 && echo yes)
ifeq ($(LS_FOUND),yes)
GO_ENV := CGO_CFLAGS="-O2 -g -DHAVE_LAYER_SHELL $(shell PKG_CONFIG_PATH="$(PKG_CONFIG_PATH)" pkg-config --cflags gtk-layer-shell-0)" \
          CGO_LDFLAGS="$(shell PKG_CONFIG_PATH="$(PKG_CONFIG_PATH)" pkg-config --libs gtk-layer-shell-0) -Wl,-rpath,\$$ORIGIN/../lib -Wl,-rpath,\$$ORIGIN/build/ls-prefix/lib"
endif

.PHONY: build deps data test appimage clean

build:
	@echo "gtk-layer-shell: $(if $(GO_ENV),yes ($(or $(PKG_CONFIG_PATH),system)),no, floating window only)"
	$(GO_ENV) CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o $(BIN) .

deps: $(LS_PC)/gtk-layer-shell-0.pc

$(LS_TAR):
	mkdir -p build
	curl -fL -o $@.part $(LS_URL)
	echo "$(LS_SHA256)  $@.part" | sha256sum -c -
	mv $@.part $@

$(LS_PC)/gtk-layer-shell-0.pc: $(LS_TAR)
	rm -rf $(LS_SRC) build/ls-build
	tar -xzf $(LS_TAR) -C build
	meson setup build/ls-build $(LS_SRC) --prefix=$(LS_PREFIX) --libdir=lib --buildtype=release \
		--default-library=shared -Dintrospection=false -Dvapi=false -Dexamples=false -Ddocs=false -Dtests=false
	ninja -C build/ls-build install

# Rebuild data/kaomoji.json from the cache written by kaomoji-fetch.sh
# (the project's kaomoji-fetch.sh).
data:
	jq -cj '[.[] | {t: .text, c: .category, g: (.tags // []), l: (.labels // [])}]' $(CACHE) > data/kaomoji.json.tmp
	mv data/kaomoji.json.tmp data/kaomoji.json

test: build
	$(GO_ENV) go vet ./...
	$(GO_ENV) go test ./...
	env -u DISPLAY -u WAYLAND_DISPLAY ./$(BIN) --selftest --out /tmp/kao-selftest.json
	./$(BIN) --version

# The release bundle always carries the overlay: deps first, then a fresh make so the vendored lib is seen.
# The LGPL library is bundled as a shared object (usr/lib) and found through AppRun's LD_LIBRARY_PATH,
# so it stays replaceable instead of being baked into the binary.
appimage: deps
	$(MAKE) build
	rm -rf AppDir
	install -Dm755 $(BIN) AppDir/usr/bin/$(BIN)
	install -d AppDir/usr/lib
	cp -a $(LS_PREFIX)/lib/libgtk-layer-shell.so* AppDir/usr/lib/
	install -Dm755 packaging/AppRun AppDir/AppRun
	install -Dm644 packaging/kaomoji-picker.desktop AppDir/kaomoji-picker.desktop
	install -Dm644 packaging/kaomoji-picker.desktop AppDir/usr/share/applications/kaomoji-picker.desktop
	install -Dm644 assets/heart256.png AppDir/usr/share/icons/hicolor/256x256/apps/kaomoji-picker.png
	install -Dm644 assets/heart256.png AppDir/kaomoji-picker.png
	install -Dm644 THIRD-PARTY-NOTICES.md AppDir/usr/share/doc/kaomoji-picker/THIRD-PARTY-NOTICES.md
	install -Dm644 LICENSE-LGPL-3.0.txt AppDir/usr/share/doc/kaomoji-picker/LICENSE-LGPL-3.0.txt
	@LD_LIBRARY_PATH=$(CURDIR)/AppDir/usr/lib ldd AppDir/usr/bin/$(BIN) | grep -q "libgtk-layer-shell.so" || (echo "layer-shell lib not resolved from the bundle" && exit 1)
	@if LD_LIBRARY_PATH=$(CURDIR)/AppDir/usr/lib ldd AppDir/usr/bin/$(BIN) | grep -q "not found"; then echo "unresolved libraries in the bundle"; exit 1; fi
	ARCH=x86_64 appimagetool --no-appstream AppDir $(APPIMAGE)

clean:
	rm -rf AppDir $(BIN) $(APPIMAGE)
