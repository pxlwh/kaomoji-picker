package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed data/kaomoji.json
var rawData []byte

//go:embed assets/heart.ico
var iconData []byte

const (
	firstPage = 600 // tiles rendered up front
	nextPage  = 400 // tiles appended when scrolling near the bottom
)

type Entry struct {
	T string   `json:"t"`
	C string   `json:"c"`
	G []string `json:"g"`
	L []string `json:"l"` // the sites' own names for the face, searchable as phrases
}

type Category struct {
	Name  string
	Count int
}

type Index struct {
	Entries []Entry
	Cats    []Category       // biggest first
	hay     []string         // lowercase text + category + tags, one per entry
	byText  map[string][]int // face text -> entry indices, for favourites
}

func loadIndex(b []byte) (*Index, error) {
	var e []Entry
	if err := json.Unmarshal(b, &e); err != nil {
		return nil, err
	}
	if len(e) == 0 {
		return nil, errors.New("kaomoji data is empty")
	}
	ix := &Index{Entries: e, hay: make([]string, len(e)), byText: map[string][]int{}}
	counts := map[string]int{}
	for i, x := range e {
		ix.hay[i] = strings.ToLower(x.T + " " + x.C + " " +
			strings.Join(x.G, " ") + " " + strings.Join(x.L, " "))
		counts[x.C]++
		ix.byText[x.T] = append(ix.byText[x.T], i)
	}
	for name, n := range counts {
		ix.Cats = append(ix.Cats, Category{name, n})
	}
	sort.Slice(ix.Cats, func(i, j int) bool {
		if ix.Cats[i].Count != ix.Cats[j].Count {
			return ix.Cats[i].Count > ix.Cats[j].Count
		}
		return ix.Cats[i].Name < ix.Cats[j].Name
	})
	return ix, nil
}

// shorthand expands a typed word into a group of alternatives. A face matches if ANY member of
// a group appears, and every group must match, so "gf" finds what the sites call love/heart/kiss
// without demanding the literal letters. Mirrors the same map in the Linux picker.
var shorthand = map[string]string{
	"gf": "girlfriend love heart kiss dating", "bf": "boyfriend love heart kiss dating",
	"idk": "shrug unsure dunno whatever", "dunno": "shrug unsure whatever",
	"omg": "shocked surprised wow", "lol": "laugh laughing haha joy",
	"lmao": "laugh laughing haha", "wtf": "confused shocked angry",
	"ily": "love heart", "ty": "thanks grateful thankful",
	"pls": "please begging apologize", "plz": "please begging",
	"brb": "wave bye leaving", "uwu": "cute happy blush", "owo": "cute curious happy",
	"nsfw": "lewd blush", "flip": "table flipping rage", "shrug": "shrug whatever meh",
	"cry": "crying tears sad", "hug": "hugging cuddle love", "kiss": "kissing love heart",
	"meh": "bored indifferent shrug", "yawn": "tired sleeping bored", "gg": "win triumph happy",
}

// queryGroups turns a query into one OR-group per typed word.
func queryGroups(q string) [][]string {
	fields := strings.Fields(strings.ToLower(q))
	groups := make([][]string, 0, len(fields))
	for _, w := range fields {
		g := []string{w}
		seen := map[string]bool{w: true}
		for _, x := range strings.Fields(shorthand[w]) {
			if !seen[x] {
				g = append(g, x)
				seen[x] = true
			}
		}
		if len(w) > 3 && strings.HasSuffix(w, "s") && !seen[w[:len(w)-1]] {
			g = append(g, w[:len(w)-1])
		}
		groups = append(groups, g)
	}
	return groups
}

// Filter returns entry indices matching every query group, restricted to category cat when set.
func (ix *Index) Filter(q, cat string) []int {
	groups := queryGroups(q)
	out := make([]int, 0, len(ix.Entries))
next:
	for i, h := range ix.hay {
		if cat != "" && ix.Entries[i].C != cat {
			continue
		}
		for _, g := range groups {
			hit := false
			for _, w := range g {
				if strings.Contains(h, w) {
					hit = true
					break
				}
			}
			if !hit {
				continue next
			}
		}
		out = append(out, i)
	}
	return out
}

// readData loads %APPDATA%\KaomojiPicker\kaomoji.json when present, otherwise the embedded data.
func readData() (*Index, error) {
	if dir, err := configDir(); err == nil {
		if b, err := os.ReadFile(filepath.Join(dir, "kaomoji.json")); err == nil {
			return loadIndex(b)
		}
	}
	return loadIndex(rawData)
}

// favs is the favourite faces, newest first; favSet mirrors it for painting.
var (
	favs   []string
	favSet = map[string]bool{}
)

func setFavs(l []string) {
	favs, favSet = l, map[string]bool{}
	for _, f := range l {
		favSet[f] = true
	}
}

// favoritesPath is %APPDATA%\KaomojiPicker\favorites.json, empty when the config dir is unknown.
func favoritesPath() string {
	if dir, err := configDir(); err == nil {
		return filepath.Join(dir, "favorites.json")
	}
	return ""
}

// loadFavorites returns an empty list for a missing or corrupt file; duplicates and blanks are dropped.
func loadFavorites(path string) []string {
	b, err := os.ReadFile(path)
	var raw, out []string
	if err != nil || json.Unmarshal(b, &raw) != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, f := range raw {
		if f != "" && !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

func saveFavorites(path string, l []string) error {
	if path == "" {
		return errors.New("no config directory")
	}
	if l == nil {
		l = []string{}
	}
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// toggleFav returns a new list: t removed when present, otherwise t added at the front.
func toggleFav(l []string, t string) []string {
	for i, f := range l {
		if f == t {
			return append(append([]string{}, l[:i]...), l[i+1:]...)
		}
	}
	return append([]string{t}, l...)
}

// favCount is how many favourites exist in the loaded data.
func (ix *Index) favCount(l []string) int {
	n := 0
	for _, f := range l {
		if len(ix.byText[f]) > 0 {
			n++
		}
	}
	return n
}

// FilterFav is Filter with favourites pinned first in favourites order; favOnly drops the rest.
func (ix *Index) FilterFav(q, cat string, l []string, favOnly bool) []int {
	base := ix.Filter(q, cat)
	rank := map[int]int{}
	for r, f := range l {
		for _, i := range ix.byText[f] {
			if _, dup := rank[i]; !dup {
				rank[i] = r
			}
		}
	}
	var pinned, rest []int
	for _, i := range base {
		if _, ok := rank[i]; ok {
			pinned = append(pinned, i)
		} else if !favOnly {
			rest = append(rest, i)
		}
	}
	sort.SliceStable(pinned, func(a, b int) bool { return rank[pinned[a]] < rank[pinned[b]] })
	return append(pinned, rest...)
}

type favReport struct {
	Count       int  `json:"count"`
	PinnedOK    bool `json:"pinned_ok"`
	RoundtripOK bool `json:"roundtrip_ok"`
}

// selftestFavorites exercises favourites against a temp file, never the real config dir.
func selftestFavorites(ix *Index) (favReport, error) {
	var r favReport
	dir, err := os.MkdirTemp("", "kao-fav")
	if err != nil {
		return r, err
	}
	defer os.RemoveAll(dir)
	// two faces from the end of the data whose text is unique, so ranks are unambiguous
	var pick []string
	for i := len(ix.Entries) - 1; i >= 0 && len(pick) < 2; i-- {
		if t := ix.Entries[i].T; len(ix.byText[t]) == 1 {
			pick = append(pick, t)
		}
	}
	if len(pick) < 2 {
		return r, errors.New("favorites: not enough unique faces")
	}
	var l []string
	for _, t := range pick {
		l = toggleFav(l, t)
	}
	r.Count = ix.favCount(l)
	m := ix.FilterFav("", "", l, false)
	only := ix.FilterFav("", "", l, true)
	r.PinnedOK = len(m) == len(ix.Entries) && len(only) == 2 &&
		ix.Entries[m[0]].T == l[0] && ix.Entries[m[1]].T == l[1] &&
		ix.Entries[only[0]].T == l[0] && ix.Entries[only[1]].T == l[1]
	p := filepath.Join(dir, "favorites.json")
	if err := saveFavorites(p, l); err != nil {
		return r, err
	}
	back := loadFavorites(p)
	r.RoundtripOK = len(back) == 2 && back[0] == l[0] && back[1] == l[1]
	switch {
	case r.Count != 2:
		return r, fmt.Errorf("favorites: chip count %d, want 2", r.Count)
	case !r.PinnedOK:
		return r, errors.New("favorites: favourites not pinned first")
	case !r.RoundtripOK:
		return r, errors.New("favorites: save/reload mismatch")
	}
	return r, nil
}

type queryResult struct {
	Matches int `json:"matches"`
	Shown   int `json:"shown"`
}

type selftestReport struct {
	Version    string                 `json:"version"`
	Total      int                    `json:"total"`
	Categories int                    `json:"categories"`
	Queries    map[string]queryResult `json:"queries"`
	Favorites  favReport              `json:"favorites"`
	Theme      map[string]themeEntry  `json:"theme"`
	ContrastOK bool                   `json:"contrast_ok"`
}

func runSelftest(out string) error {
	ix, err := loadIndex(rawData)
	if err != nil {
		return err
	}
	rep := selftestReport{Version: appVersion, Total: len(ix.Entries), Categories: len(ix.Cats), Queries: map[string]queryResult{}}
	for _, q := range []string{"", "e", "cat", "sad", "shrug", "gf", "creepy smile"} {
		n := len(ix.Filter(q, ""))
		rep.Queries[q] = queryResult{n, min(n, firstPage)}
	}
	if rep.Favorites, err = selftestFavorites(ix); err != nil {
		return err
	}
	if rep.Theme, rep.ContrastOK, err = selftestTheme(); err != nil {
		return err
	}
	b, err := json.Marshal(rep)
	if err != nil {
		return err
	}
	if out == "" {
		_, err = os.Stdout.Write(append(b, '\n'))
		return err
	}
	return os.WriteFile(out, append(b, '\n'), 0o644)
}
