package main

import (
	"bufio"
	"image"
	"image/color"
	"image/png"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Counts pinned to the Windows picker on the same data/kaomoji.json; update both after `make data`.
func TestParityCounts(t *testing.T) {
	ix, err := loadIndex(rawData)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"": 6582, "e": 6289, "cat": 234, "sad": 502, "shrug": 213, "gf": 726, "creepy smile": 8}
	for q, n := range want {
		if got := len(ix.Filter(q, "")); got != n {
			t.Errorf("Filter(%q) = %d, want %d", q, got, n)
		}
	}
	for i := 1; i < len(ix.Cats); i++ {
		if ix.Cats[i].Count > ix.Cats[i-1].Count {
			t.Fatal("categories not biggest-first")
		}
	}
}

func TestSelftest(t *testing.T) {
	if err := runSelftest(filepath.Join(t.TempDir(), "r.json")); err != nil {
		t.Fatal(err)
	}
}

// TestRender paints both views headlessly and checks each panel came out: transparent rounded
// corner, accent border, and a filled body. The settings pane must also show its Save button,
// and the picker painted after it in the same process must not (set_open is static C state).
func TestRender(t *testing.T) {
	saveTop := func(img image.Image) uint32 { // top border of the Save button, bottom-right
		c := color.NRGBAModel.Convert(img.At(1100-12-45, 700-12-28)).(color.NRGBA)
		return uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
	}
	img := renderCheck(t, true) // loads cfg
	accent, _ := parseHex(cfg.Accent)
	dim := derivePalette(accent).AccentDim
	if c := saveTop(img); c != dim {
		t.Errorf("settings: save button border %06x, want %06x", c, dim)
	}
	if c := saveTop(renderCheck(t, false)); c == dim {
		t.Error("picker render still shows the settings pane")
	}
}

func renderCheck(t *testing.T, settings bool) image.Image {
	img := renderPNG(t, "sad", settings, "", false, 1100, 700)
	if b := img.Bounds(); b.Dx() != 1100 || b.Dy() != 700 {
		t.Fatalf("size %v", b)
	}
	accent, _ := parseHex(cfg.Accent)
	nrgba := func(x, y int) color.NRGBA { return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA) }
	if c := nrgba(0, 0); c.A != 0 {
		t.Errorf("corner not transparent: %v", c)
	}
	if c := nrgba(550, 0); c.A != 255 || uint32(c.R)<<16|uint32(c.G)<<8|uint32(c.B) != accent {
		t.Errorf("top border %v, want accent %06x", c, accent)
	}
	// the panel is intentionally translucent (0.93) so a blurring compositor shows through;
	// everything outside the panel must stay fully transparent
	if c := nrgba(550, 690); c.A < 200 || c.A > 250 {
		t.Errorf("panel body alpha %d, want ~236 (0.93)", c.A)
	}
	return img
}

func renderPNG(t *testing.T, query string, settings bool, hexText string, screen bool, w, h int) image.Image {
	path := filepath.Join(t.TempDir(), "r.png")
	if err := runRender(path, query, settings, hexText, screen, w, h); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// errorRed counts opaque pixels within ±10 per channel of the invalid-accent red #ff5f6d.
func errorRed(img image.Image) int {
	n := 0
	b := img.Bounds()
	near := func(v, want uint8) bool { return int(v)-int(want) <= 10 && int(want)-int(v) <= 10 }
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A == 255 && near(c.R, 0xff) && near(c.G, 0x5f) && near(c.B, 0x6d) {
				n++
			}
		}
	}
	return n
}

// TestRenderHexInvalid: "#12" in the accent field paints the red border and error line, a valid accent does not.
func TestRenderHexInvalid(t *testing.T) {
	if n := errorRed(renderPNG(t, "", true, "#3fa9f5", false, 1100, 700)); n != 0 {
		t.Errorf("valid accent shows %d error-red pixels", n)
	}
	if n := errorRed(renderPNG(t, "", true, "#12", false, 1100, 700)); n < 300 { // 160x32 box outline alone is ~380
		t.Errorf("#12 shows only %d error-red pixels", n)
	}
}

// TestRenderLayer: as a screen-sized layer surface the 1100x700 panel is centred with transparent margins.
func TestRenderLayer(t *testing.T) {
	for _, settings := range []bool{false, true} {
		img := renderPNG(t, "sad", settings, "", true, 1920, 1080)
		x0, y0, x1, y1 := opaqueBox(img)
		if x0 != 410 || y0 != 190 || x1 != 1509 || y1 != 889 {
			t.Errorf("settings=%v: panel box %d,%d..%d,%d, want 410,190..1509,889", settings, x0, y0, x1, y1)
		}
		if a := color.NRGBAModel.Convert(img.At(x0, y0)).(color.NRGBA).A; a != 0 {
			t.Errorf("settings=%v: panel corner not rounded (alpha %d)", settings, a)
		}
	}
}

// opaqueBox is the bounding box of every pixel with any alpha.
func opaqueBox(img image.Image) (x0, y0, x1, y1 int) {
	b := img.Bounds()
	x0, y0, x1, y1 = b.Max.X, b.Max.Y, -1, -1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA).A > 0 {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
			}
		}
	}
	return
}

// TestToggleOrStart: with nothing listening, --toggle starts an instance (here a fake one that
// only opens the real socket path) and the toggle then lands on it; with one listening it does not start.
func TestToggleOrStart(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	got := make(chan string, 2)
	starts := 0
	start := func() error {
		starts++
		ln, err := net.Listen("unix", sockPath())
		if err != nil {
			return err
		}
		t.Cleanup(func() { ln.Close() })
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				l, _ := bufio.NewReader(c).ReadString('\n')
				got <- strings.TrimSpace(l)
				c.Close()
			}
		}()
		return nil
	}
	for i := 0; i < 2; i++ {
		if err := toggleOrStart(start); err != nil {
			t.Fatal(err)
		}
		if l := <-got; l != "toggle" {
			t.Fatalf("instance got %q, want toggle", l)
		}
	}
	if starts != 1 {
		t.Fatalf("started %d times, want 1", starts)
	}
}

func TestPaletteMath(t *testing.T) {
	if mix(0xff6fae, 0, 0) != 0xff6fae || mix(0xff6fae, 0xffffff, 1) != 0xffffff {
		t.Fatal("mix endpoints wrong")
	}
	for _, c := range []uint32{0xff6fae, 0x3fa9f5, 0x7bd88f, 0x808080} {
		h, s, l := toHSL(c)
		if got := fromHSL(h, s, l); got != c {
			t.Errorf("hsl roundtrip %06x -> %06x", c, got)
		}
	}
	if _, err := parseHex("#12345"); err == nil {
		t.Fatal("short hex accepted")
	}
	if _, err := parseHex("zzzzzz"); err == nil {
		t.Fatal("bad hex accepted")
	}
}

func TestFavoritesFiles(t *testing.T) {
	if got := loadFavorites(filepath.Join(t.TempDir(), "missing.json")); len(got) != 0 {
		t.Fatal("missing file not empty")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte("{not json"), 0o644)
	if got := loadFavorites(bad); len(got) != 0 {
		t.Fatal("corrupt file not empty")
	}
}
