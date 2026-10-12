package app

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"platypus/internal/customize"
	"platypus/internal/onlineart"
	"platypus/internal/settings"
)

type artHost struct {
	stderrHost
	warns []string
	emits []string
}

func (h *artHost) Warnf(format string, args ...any) { h.warns = append(h.warns, fmt.Sprintf(format, args...)) }
func (h *artHost) Emit(event string, _ ...any)      { h.emits = append(h.emits, event) }

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// artFixture is a fake Steam: search, CDN images and appdetails, with a hit
// counter proving tests never touch the real network.
type artFixture struct {
	t      *testing.T
	srv    *httptest.Server
	hits   atomic.Int64
	search string
	images map[string][]byte // path suffix -> body; absent means 404
	detail string
}

func newArtFixture(t *testing.T) *artFixture {
	t.Helper()
	f := &artFixture{t: t, images: map[string][]byte{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/storesearch/"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"total":9,"items":[%s]}`, f.search)
		case strings.HasPrefix(r.URL.Path, "/api/appdetails"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, f.detail)
		case strings.HasPrefix(r.URL.Path, "/store_item_assets/"):
			for suffix, body := range f.images {
				if strings.HasSuffix(r.URL.Path, suffix) {
					w.Write(body)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func newArtApp(t *testing.T, f *artFixture, title string) (*App, *artHost, string) {
	t.Helper()
	root := t.TempDir()
	st, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "game.sh")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	g, err := st.AddCustomGame(title, exe)
	if err != nil {
		t.Fatal(err)
	}
	cs, err := customize.Open(filepath.Join(root, "custom-art"))
	if err != nil {
		t.Fatal(err)
	}
	h := &artHost{}
	a := &App{host: h, settings: st, artwork: &onlineart.Client{SearchBase: f.srv.URL, CDNBase: f.srv.URL}}
	a.custom.p.Store(cs)
	return a, h, g.ID
}

func (a *App) artBytes(t *testing.T, gameID string, kind string) []byte {
	t.Helper()
	s := a.custom.get()
	raw, err := os.ReadFile(s.Path(gameID, customize.Kind(kind)))
	if err != nil {
		t.Fatalf("read %s art: %v", kind, err)
	}
	return raw
}

const knightSearch = `{"id":123,"name":"Hollow Knight","tiny_image":"http://x/123.jpg"},{"id":124,"name":"Hollow Knight: Silksong","tiny_image":"http://x/124.jpg"}`
const knightDetail = `{"123":{"success":true,"data":{"name":"Hollow Knight","release_date":{"date":"24 Feb, 2017"}}}}`

func TestAutoArtworkAppliesOnExactMatch(t *testing.T) {
	f := newArtFixture(t)
	png := testPNG(t)
	f.search = knightSearch
	f.detail = knightDetail
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	f.images["logo.png"] = png
	a, h, id := newArtApp(t, f, "Hollow Knight")

	a.autoArtwork(id, "Hollow Knight")

	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, png) {
		t.Fatal("cover wasn't stored")
	}
	if got := a.artBytes(t, id, "banner"); !bytes.Equal(got, png) {
		t.Fatal("banner wasn't stored")
	}
	e := a.custom.get().Get(id)
	if e.ArtSource != "steam:123" || e.ArtSourceName != "Hollow Knight" {
		t.Fatalf("source = %q/%q, want steam:123/Hollow Knight", e.ArtSource, e.ArtSourceName)
	}
	if len(h.emits) != 1 || h.emits[0] != "library" {
		t.Fatalf("emits = %v, want one library event", h.emits)
	}
	if len(h.warns) != 0 {
		t.Fatalf("warns = %v, want silence on success", h.warns)
	}
	// Nice-to-haves: compat AppID set (was unset) and the year (had none).
	if got := a.settings.CompatAppID(id); got != "123" {
		t.Fatalf("compat app id = %q, want 123", got)
	}
	for _, g := range a.settings.CustomGames() {
		if g.ID == id && g.ReleaseYear != 2017 {
			t.Fatalf("release year = %d, want 2017", g.ReleaseYear)
		}
	}
}

func TestAutoArtworkAmbiguousOrEmptyDoesNothing(t *testing.T) {
	for _, tc := range []struct {
		name   string
		title  string
		search string
	}{
		{"several match", "Hollow Knight", `{"id":1,"name":"Hollow Knight"},{"id":2,"name":"Hollow Knight™"}`},
		{"none match", "Unknown Game", knightSearch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newArtFixture(t)
			png := testPNG(t)
			f.search = tc.search
			f.detail = knightDetail
			f.images["library_600x900.jpg"] = png
			f.images["library_hero.jpg"] = png
			a, h, id := newArtApp(t, f, tc.title)

			a.autoArtwork(id, tc.title)

			if got := a.custom.get().Get(id); len(got.Art) != 0 {
				t.Fatalf("art = %v, want none", got.Art)
			}
			if len(h.emits) != 0 || len(h.warns) != 0 {
				t.Fatalf("emits = %v warns = %v, want silence", h.emits, h.warns)
			}
		})
	}
}

func TestAutoArtworkNeverOverwritesUserArt(t *testing.T) {
	f := newArtFixture(t)
	png := testPNG(t)
	other := append(append([]byte{}, png...), 0x00)
	f.search = knightSearch
	f.detail = knightDetail
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	a, _, id := newArtApp(t, f, "Hollow Knight")

	// The user picked a cover by hand first.
	src := filepath.Join(t.TempDir(), "mine.png")
	if err := os.WriteFile(src, other, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.custom.get().SetArt(id, customize.Cover, src); err != nil {
		t.Fatal(err)
	}

	a.autoArtwork(id, "Hollow Knight")

	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, other) {
		t.Fatal("the user's cover was overwritten")
	}
	if got := a.artBytes(t, id, "banner"); !bytes.Equal(got, png) {
		t.Fatal("the banner wasn't applied alongside the user's cover")
	}
}

func TestAutoArtworkHero404FallsBackToHeader(t *testing.T) {
	f := newArtFixture(t)
	png := testPNG(t)
	f.search = knightSearch
	f.detail = knightDetail
	f.images["library_600x900.jpg"] = png
	f.images["header.jpg"] = png // no library_hero.jpg: 404
	a, _, id := newArtApp(t, f, "Hollow Knight")

	a.autoArtwork(id, "Hollow Knight")

	if got := a.artBytes(t, id, "banner"); !bytes.Equal(got, png) {
		t.Fatal("banner didn't fall back from hero to header")
	}
}

func TestApplyOnlineArtManualReplaces(t *testing.T) {
	f := newArtFixture(t)
	png := testPNG(t)
	f.detail = knightDetail
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	a, _, id := newArtApp(t, f, "Hollow Knight")

	old := filepath.Join(t.TempDir(), "old.png")
	if err := os.WriteFile(old, append(append([]byte{}, png...), 0x01), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.custom.get().SetArt(id, customize.Cover, old); err != nil {
		t.Fatal(err)
	}

	c, err := a.ApplyOnlineArt(id, 123)
	if err != nil {
		t.Fatal(err)
	}
	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, png) {
		t.Fatal("manual apply didn't replace the user's cover")
	}
	if c.ArtSource != "steam:123" || c.ArtSourceName != "Hollow Knight" {
		t.Fatalf("attribution = %q/%q", c.ArtSource, c.ArtSourceName)
	}
}

func TestApplyOnlineArtWithNoArtErrors(t *testing.T) {
	f := newArtFixture(t)
	f.detail = knightDetail // art endpoints all 404
	a, _, id := newArtApp(t, f, "Hollow Knight")
	if _, err := a.ApplyOnlineArt(id, 123); err == nil {
		t.Fatal("all-404 apply = nil, want an error to show")
	}
}

func TestArtworkOffMakesNoRequests(t *testing.T) {
	f := newArtFixture(t)
	f.search = knightSearch
	f.detail = knightDetail
	png := testPNG(t)
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	a, _, _ := newArtApp(t, f, "Hollow Knight")
	if err := a.settings.SetArtworkOnline(false); err != nil {
		t.Fatal(err)
	}

	if _, err := a.SearchArtOnline("Hollow Knight"); err == nil {
		t.Fatal("search with the switch off = nil, want a refusal")
	}
	// AddCustomGame with the switch off starts no background lookup.
	root := t.TempDir()
	exe := filepath.Join(root, "other.sh")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddCustomGame("Other", exe); err != nil {
		t.Fatal(err)
	}
	if hits := f.hits.Load(); hits != 0 {
		t.Fatalf("server hits = %d, want none: nothing may go out", hits)
	}
}

func TestSearchArtOnlineParses(t *testing.T) {
	f := newArtFixture(t)
	f.search = knightSearch
	a, _, _ := newArtApp(t, f, "Hollow Knight")
	got, err := a.SearchArtOnline("Hollow Knight")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].AppID != 123 || got[0].Name != "Hollow Knight" {
		t.Fatalf("candidates = %+v", got)
	}
}
