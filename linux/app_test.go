package main

import (
	"os"
	"path/filepath"
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
