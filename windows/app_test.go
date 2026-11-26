package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

func TestParseHotkey(t *testing.T) {
	ok := map[string]string{
		"Ctrl+Alt+K":           "Ctrl+Alt+K",
		"ctrl + shift + space": "Ctrl+Shift+Space",
		"Win+Alt+K":            "Alt+Win+K",
		"Alt+Ctrl+F24":         "Ctrl+Alt+F24",
		"Control+7":            "Ctrl+7",
	}
	for in, want := range ok {
		if _, _, got, err := parseHotkey(in); err != nil || got != want {
			t.Errorf("parseHotkey(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "K", "Ctrl", "Ctrl+K+J", "Ctrl+F25", "Ctrl+F0", "Ctrl+Tab", "Ctrl+Alt+KK"} {
		if _, _, _, err := parseHotkey(in); err == nil {
			t.Errorf("parseHotkey(%q) accepted", in)
		}
	}
}

func TestFilter(t *testing.T) {
	ix, err := loadIndex(rawData)
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Entries) != 6582 || len(ix.Cats) != 105 {
		t.Fatalf("entries=%d cats=%d", len(ix.Entries), len(ix.Cats))
	}
	for i := 1; i < len(ix.Cats); i++ {
		if ix.Cats[i].Count > ix.Cats[i-1].Count {
			t.Fatal("categories not biggest-first")
		}
	}
	all := len(ix.Filter("", ""))
	if all != 6582 || len(ix.Filter("  SHRUG  ", "")) != len(ix.Filter("shrug", "")) {
		t.Fatal("filter not case/space insensitive")
	}
	both := ix.Filter("cat happy", "")
	for _, i := range both {
		h := ix.hay[i]
		if !strings.Contains(h, "cat") || !strings.Contains(h, "happy") {
			t.Fatalf("entry %d does not match every word", i)
		}
	}
	c := ix.Cats[0].Name
	if n := len(ix.Filter("", c)); n != ix.Cats[0].Count {
		t.Fatalf("category filter %q = %d, want %d", c, n, ix.Cats[0].Count)
	}
}

func TestBlend(t *testing.T) {
	ix, err := loadIndex(rawData)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := selftestFavorites(ix); err != nil {
		t.Fatal(err)
	}
	if got := loadFavorites(filepath.Join(t.TempDir(), "missing.json")); len(got) != 0 {
		t.Fatal("missing file not empty")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(bad, []byte("{not json"), 0o644)
	if got := loadFavorites(bad); len(got) != 0 {
		t.Fatal("corrupt file not empty")
	}
	if blend(0xff6fae, 0x1c0a12, 255) != 0xff6fae || blend(0xff6fae, 0x1c0a12, 0) != 0x1c0a12 {
		t.Fatal("blend endpoints wrong")
	}
	if rgb(0x112233) != 0x332211 {
		t.Fatal("rgb byte order wrong")
	}
}

// Sizes from the Windows SDK headers for amd64.
func TestStructSizes(t *testing.T) {
	for name, got := range map[string]uintptr{
		"MSG": unsafe.Sizeof(msgT{}), "WNDCLASSEXW": unsafe.Sizeof(wndClassEx{}), "PAINTSTRUCT": unsafe.Sizeof(paintStruct{}),
		"MONITORINFO": unsafe.Sizeof(monitorInfo{}), "NOTIFYICONDATAW": unsafe.Sizeof(notifyIconData{}), "INPUT": unsafe.Sizeof(keyInput{}),
		"DRAWITEMSTRUCT": unsafe.Sizeof(drawItemStruct{}), "TRACKMOUSEEVENT": unsafe.Sizeof(trackMouseEvent{}),
	} {
		want := map[string]uintptr{"MSG": 48, "WNDCLASSEXW": 80, "PAINTSTRUCT": 72, "MONITORINFO": 40, "NOTIFYICONDATAW": 976, "INPUT": 40, "DRAWITEMSTRUCT": 64, "TRACKMOUSEEVENT": 24}[name]
		if got != want {
			t.Errorf("%s size %d, want %d", name, got, want)
		}
	}
}

func TestParseHex(t *testing.T) {
	for in, want := range map[string]uint32{"#ff6fae": 0xff6fae, "FF6FAE": 0xff6fae, "#f0a": 0xff00aa, " 3FA9f5 ": 0x3fa9f5} {
		if got, err := parseHex(in); err != nil || got != want {
			t.Errorf("parseHex(%q) = %x, %v; want %x", in, got, err, want)
		}
	}
	for _, in := range []string{"", "#", "#ff", "#ffff", "#fffffff", "#gggggg", "pink", "#+f0a0b0", "0x1234", "#ff 6fae"} {
		if _, err := parseHex(in); err == nil {
			t.Errorf("parseHex(%q) accepted", in)
		}
	}
	if _, err := derivePalette("zzz"); err == nil {
		t.Error("derivePalette accepted garbage")
	}
}

func TestDerivePalette(t *testing.T) {
	d, err := derivePalette("#ff6fae")
	if err != nil {
		t.Fatal(err)
	}
	if d.Accent != 0xff6fae || d.Text != 0xffe9f2 || d.AccentDim != mix(0xff6fae, 0, 0.55) || d.Selected != blend(d.Accent, d.Panel, 0x33) {
		t.Errorf("default palette wrong: %+v", d)
	}
	// panel keeps the hue in near-black: red-dominant, blue above green, all dark
	if r, g, b := d.Panel>>16&0xFF, d.Panel>>8&0xFF, d.Panel&0xFF; !(r > b && b > g && r < 0x30) {
		t.Errorf("panel %06x not a dark pink", d.Panel)
	}
	if hexStr(d.Bg) >= hexStr(d.Panel) {
		t.Errorf("bg %06x not darker than panel %06x", d.Bg, d.Panel)
	}
	bl, _ := derivePalette("3fa9f5")
	if bl.Accent != 0x3fa9f5 || bl.Text != 0xe2f2fd || bl.Panel&0xFF <= bl.Panel>>16&0xFF {
		t.Errorf("blue palette wrong: %+v", bl)
	}
	if g, _ := derivePalette("#808080"); g.Panel != 0x121212 {
		t.Errorf("grey accent panel %06x, want neutral 121212", g.Panel)
	}
	for _, hex := range []string{"#ff6fae", "#3fa9f5", "#f5f5f5", "#000000", "#ffffff", "#0000ff"} {
		p, _ := derivePalette(hex)
		if c := contrast(p.Text, p.Panel); c < minContrast {
			t.Errorf("%s text/panel contrast %.2f", hex, c)
		}
	}
	if _, ok, err := selftestTheme(); err != nil || !ok {
		t.Errorf("selftestTheme: %v %v", ok, err)
	}
}

func TestIconTint(t *testing.T) {
	for _, n := range []int{16, 32} {
		px, err := iconBGRA(n, 0x3fa9f5)
		if err != nil || len(px) != n*n*4 {
			t.Fatal(n, err)
		}
		opaque := 0
		for i := 0; i < len(px); i += 4 {
			if px[i] != 0xf5 || px[i+1] != 0xa9 || px[i+2] != 0x3f {
				t.Fatal("pixel not tinted")
			}
			if px[i+3] == 255 {
				opaque++
			}
		}
		if opaque == 0 || px[3] != 0 {
			t.Errorf("size %d: opaque=%d corner alpha=%d", n, opaque, px[3])
		}
	}
}

func TestAccentFallback(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir) // os.UserConfigDir on Windows
	t.Setenv("XDG_CONFIG_HOME", dir)
	d, _ := configDir()
	os.MkdirAll(d, 0o755)
	p := filepath.Join(d, "settings.json")
	for in, want := range map[string]string{`{"accent":"not a colour"}`: defaultAccent, `{"accent":"3FA9F5"}`: "#3fa9f5", `{broken`: defaultAccent, `{}`: defaultAccent} {
		os.WriteFile(p, []byte(in), 0o644)
		if got := loadSettings().Accent; got != want {
			t.Errorf("settings %s -> accent %q, want %q", in, got, want)
		}
	}
	os.Remove(p)
	if loadSettings().Accent != defaultAccent {
		t.Error("missing settings not default")
	}
}
