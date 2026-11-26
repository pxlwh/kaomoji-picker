BIN      := kaomoji-picker
APPIMAGE := KaomojiPicker-x86_64.AppImage
CACHE    := $(HOME)/.cache/kaomoji/kaomoji.json

.PHONY: build data test appimage clean

build:
	CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o $(BIN) .

# Rebuild data/kaomoji.json from the cache written by kaomoji-fetch.sh
# (the project's kaomoji-fetch.sh).
data:
	jq -cj '[.[] | {t: .text, c: .category, g: (.tags // []), l: (.labels // [])}]' $(CACHE) > data/kaomoji.json.tmp
	mv data/kaomoji.json.tmp data/kaomoji.json

test: build
	go vet ./...
	go test ./...
	env -u DISPLAY -u WAYLAND_DISPLAY ./$(BIN) --selftest --out /tmp/kao-selftest.json
	./$(BIN) --version

appimage: build
	rm -rf AppDir
	install -Dm755 $(BIN) AppDir/usr/bin/$(BIN)
	install -Dm755 packaging/AppRun AppDir/AppRun
	install -Dm644 packaging/kaomoji-picker.desktop AppDir/kaomoji-picker.desktop
	install -Dm644 packaging/kaomoji-picker.desktop AppDir/usr/share/applications/kaomoji-picker.desktop
	install -Dm644 assets/heart256.png AppDir/usr/share/icons/hicolor/256x256/apps/kaomoji-picker.png
	install -Dm644 assets/heart256.png AppDir/kaomoji-picker.png
	ARCH=x86_64 appimagetool --no-appstream AppDir $(APPIMAGE)

clean:
	rm -rf AppDir $(BIN) $(APPIMAGE)
