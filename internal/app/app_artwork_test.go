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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/customize"
	"platypus/internal/onlineart"
	"platypus/internal/settings"
	"platypus/internal/types"
)

type artHost struct {
	stderrHost
	mu    sync.Mutex
	warns []string
	emits []string
}

func (h *artHost) Warnf(format string, args ...any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.warns = append(h.warns, fmt.Sprintf(format, args...))
}
func (h *artHost) Emit(event string, _ ...any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.emits = append(h.emits, event)
}
func (h *artHost) emitted() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string{}, h.emits...)
}

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
	fail   map[string]int    // path suffix -> status code (e.g. a logo 500)
	detail string
	browse string // GetItems store_items JSON object, e.g. `{"name":"Rivage","assets":{...}}`; empty means no items
	browseFail int // GetItems status code when non-zero (e.g. 500)
	// searchGate, when non-nil, holds the search handler until closed: the
	// test can remove the game mid-flight.
	searchGate chan struct{}
}

func newArtFixture(t *testing.T) *artFixture {
	t.Helper()
	f := &artFixture{t: t, images: map[string][]byte{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/storesearch/"):
			if f.searchGate != nil {
				<-f.searchGate
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"total":9,"items":[%s]}`, f.search)
		case strings.HasPrefix(r.URL.Path, "/api/appdetails"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, f.detail)
		case strings.HasPrefix(r.URL.Path, "/IStoreBrowseService/GetItems/"):
			if f.browseFail != 0 {
				w.WriteHeader(f.browseFail)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if f.browse == "" {
				fmt.Fprint(w, `{"response":{"store_items":[]}}`)
				return
			}
			fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, f.browse)
		case strings.HasPrefix(r.URL.Path, "/store_item_assets/"):
			for suffix, code := range f.fail {
				if strings.HasSuffix(r.URL.Path, suffix) {
					w.WriteHeader(code)
					return
				}
			}
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
	a := &App{host: h, settings: st, artwork: &onlineart.Client{SearchBase: f.srv.URL, CDNBase: f.srv.URL, APIBase: f.srv.URL}}
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

// A failing logo must not sink the apply: cover and banner still land, with
// one warn about the logo.
func TestLogoErrorStillAppliesCoverAndBanner(t *testing.T) {
	f := newArtFixture(t)
	png := testPNG(t)
	f.detail = knightDetail
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	f.fail = map[string]int{"logo.png": http.StatusInternalServerError}
	a, h, id := newArtApp(t, f, "Hollow Knight")

	if _, err := a.ApplyOnlineArt(id, 123); err != nil {
		t.Fatalf("logo 500 aborted the apply: %v", err)
	}
	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, png) {
		t.Fatal("cover wasn't stored")
	}
	if got := a.artBytes(t, id, "banner"); !bytes.Equal(got, png) {
		t.Fatal("banner wasn't stored")
	}
	if p := a.custom.get().Path(id, customize.Logo); p != "" {
		t.Fatalf("logo path = %q, want none stored", p)
	}
	if len(h.warns) != 1 || !strings.Contains(strings.ToLower(h.warns[0]), "logo") {
		t.Fatalf("warns = %v, want one logo warn", h.warns)
	}
}

// Removing the game mid-lookup stores nothing for it: no art, no source, no
// compat AppID, and silence (no warn, no event).
func TestAutoArtworkDropsWhenGameRemoved(t *testing.T) {
	f := newArtFixture(t)
	png := testPNG(t)
	f.search = knightSearch
	f.detail = knightDetail
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	f.searchGate = make(chan struct{})
	a, h, id := newArtApp(t, f, "Hollow Knight")

	done := make(chan struct{})
	go func() {
		a.autoArtwork(id, "Hollow Knight")
		close(done)
	}()
	// Wait until the lookup is inside the blocked search, then remove.
	deadline := time.Now().Add(5 * time.Second)
	for f.hits.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the lookup never reached the server")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := a.settings.RemoveCustomGame(id); err != nil {
		t.Fatal(err)
	}
	close(f.searchGate)
	<-done

	if got := a.custom.get().Get(id); len(got.Art) != 0 || got.ArtSource != "" {
		t.Fatalf("art for the removed game = %+v, want none", got)
	}
	if got := a.settings.CompatAppID(id); got != "" {
		t.Fatalf("compat app id for the removed game = %q, want none", got)
	}
	if len(h.warns) != 0 || len(h.emits) != 0 {
		t.Fatalf("warns = %v emits = %v, want silence", h.warns, h.emits)
	}
}

// Rivage (app 4094660): the fixed CDN names 404, but GetItems names hashed
// assets that download fine, so the manual apply still succeeds.
func TestApplyOnlineArtRivageHashedAssets(t *testing.T) {
	f := newArtFixture(t)
	png := testPNG(t)
	f.detail = `{"4094660":{"success":true,"data":{"name":"Rivage","release_date":{"date":"2025"}}}}`
	f.browse = `{"name":"Rivage","assets":{
		"asset_url_format":"steam/apps/4094660/${FILENAME}?t=1728660000",
		"library_capsule":"hash_a/rivage_cover.jpg",
		"library_capsule_2x":"hash_b/rivage_cover_2x.jpg",
		"library_hero":"hash_c/rivage_hero.jpg",
		"library_hero_2x":"hash_d/rivage_hero_2x.jpg",
		"header":"hash_e/rivage_header.jpg",
		"library_logo":"hash_f/rivage_logo.png"
	}}`
	// Only the hashed files exist; the fixed names all 404.
	f.images["rivage_cover_2x.jpg"] = png
	f.images["rivage_hero_2x.jpg"] = png
	f.images["rivage_logo.png"] = png
	a, _, id := newArtApp(t, f, "Rivage")

	c, err := a.ApplyOnlineArt(id, 4094660)
	if err != nil {
		t.Fatalf("Rivage apply with hashed assets: %v", err)
	}
	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, png) {
		t.Fatal("cover wasn't stored from the hashed GetItems URL")
	}
	if got := a.artBytes(t, id, "banner"); !bytes.Equal(got, png) {
		t.Fatal("banner wasn't stored from the hashed GetItems URL")
	}
	if c.ArtSource != "steam:4094660" || c.ArtSourceName != "Rivage" {
		t.Fatalf("attribution = %q/%q, want steam:4094660/Rivage", c.ArtSource, c.ArtSourceName)
	}
}

// When GetItems fails, the fixed CDN names still apply (older apps).
func TestApplyOnlineArtFallsBackWhenGetItemsFails(t *testing.T) {
	f := newArtFixture(t)
	png := testPNG(t)
	f.detail = knightDetail
	f.browseFail = http.StatusInternalServerError
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	a, _, id := newArtApp(t, f, "Hollow Knight")

	if _, err := a.ApplyOnlineArt(id, 123); err != nil {
		t.Fatalf("fixed-name fallback after GetItems 500: %v", err)
	}
	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, png) {
		t.Fatal("cover wasn't stored from the fixed fallback")
	}
}

// waitForArt polls until the game has cover art or the deadline passes.
func waitForArt(t *testing.T, a *App, gameID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if got := a.custom.get().Get(gameID); len(got.Art) != 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the background art apply")
}

func waitForEmit(t *testing.T, h *artHost) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if len(h.emitted()) != 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the library event")
}

// A changed AppID auto-applies in the background and emits library.
func TestSetAppIDAutoApplies(t *testing.T) {
	f := newArtFixture(t)
	png := testPNG(t)
	f.detail = knightDetail
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	a, h, id := newArtApp(t, f, "My Game")
	game := types.Game{ID: id, Source: types.SourceCustom, Store: types.StoreCustom, Title: "My Game"}

	if _, err := a.SetGameSteamAppID(game, "123"); err != nil {
		t.Fatal(err)
	}
	waitForArt(t, a, id)
	waitForEmit(t, h)
	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, png) {
		t.Fatal("cover wasn't auto-applied after the AppID change")
	}
	e := a.custom.get().Get(id)
	if e.ArtSource != "steam:123" {
		t.Fatalf("source = %q, want steam:123", e.ArtSource)
	}
	if got := a.settings.CompatAppID(id); got != "123" {
		t.Fatalf("compat app id = %q, want 123", got)
	}
}

// Setting the same AppID again does nothing new.
func TestSetSameAppIDDoesNothing(t *testing.T) {
	f := newArtFixture(t)
	png := testPNG(t)
	f.detail = knightDetail
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	a, h, id := newArtApp(t, f, "My Game")
	game := types.Game{ID: id, Source: types.SourceCustom, Store: types.StoreCustom, Title: "My Game"}

	if _, err := a.SetGameSteamAppID(game, "123"); err != nil {
		t.Fatal(err)
	}
	waitForArt(t, a, id)
	waitForEmit(t, h)
	// Let any trailing request from the first apply land before the baseline.
	time.Sleep(500 * time.Millisecond)
	hits := f.hits.Load()
	nEmits := len(h.emitted())
	coverBefore := a.artBytes(t, id, "cover")

	if _, err := a.SetGameSteamAppID(game, "123"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	if got := f.hits.Load(); got != hits {
		t.Fatalf("same AppID made %d more requests, want none", got-hits)
	}
	if got := len(h.emitted()); got != nEmits {
		t.Fatalf("same AppID emitted %d more events, want none", got-nEmits)
	}
	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, coverBefore) {
		t.Fatal("same AppID changed the stored art, want it untouched")
	}
}

// Hand-picked art is kept while online art is replaced by the AppID auto path.
func TestSetAppIDKeepsHandPickedReplacesOnline(t *testing.T) {
	newPNG := func(t *testing.T, seed byte) []byte {
		t.Helper()
		// testPNG makes a blank image; append a seed byte so two downloads differ.
		return append(testPNG(t), seed)
	}
	t.Run("hand-picked kept", func(t *testing.T) {
		f := newArtFixture(t)
		online := newPNG(t, 0x01)
		f.detail = knightDetail
		f.images["library_600x900.jpg"] = online
		f.images["library_hero.jpg"] = online
		a, h, id := newArtApp(t, f, "My Game")
		mine := newPNG(t, 0x02)
		src := filepath.Join(t.TempDir(), "mine.png")
		if err := os.WriteFile(src, mine, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := a.custom.get().SetArt(id, customize.Cover, src); err != nil {
			t.Fatal(err)
		}
		game := types.Game{ID: id, Source: types.SourceCustom, Store: types.StoreCustom, Title: "My Game"}
		if _, err := a.SetGameSteamAppID(game, "123"); err != nil {
			t.Fatal(err)
		}
		waitForArt(t, a, id)
		waitForEmit(t, h)
		if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, mine) {
			t.Fatal("the user's cover was overwritten by the AppID auto apply")
		}
		if got := a.artBytes(t, id, "banner"); !bytes.Equal(got, online) {
			t.Fatal("the banner wasn't applied alongside the user's cover")
		}
	})
	t.Run("online replaced", func(t *testing.T) {
		f := newArtFixture(t)
		first := newPNG(t, 0x03)
		f.detail = knightDetail
		f.images["library_600x900.jpg"] = first
		f.images["library_hero.jpg"] = first
		a, h, id := newArtApp(t, f, "My Game")
		if _, err := a.ApplyOnlineArt(id, 123); err != nil {
			t.Fatal(err)
		}
		second := newPNG(t, 0x04)
		f.images["library_600x900.jpg"] = second
		f.images["library_hero.jpg"] = second
		game := types.Game{ID: id, Source: types.SourceCustom, Store: types.StoreCustom, Title: "My Game"}
		if _, err := a.SetGameSteamAppID(game, "124"); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			raw, err := os.ReadFile(a.custom.get().Path(id, customize.Cover))
			if err == nil && bytes.Equal(raw, second) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, second) {
			t.Fatal("online art wasn't replaced by the changed AppID")
		}
		if got := len(h.emitted()); got == 0 {
			t.Fatal("no library event after the online replacement")
		}
	})
}

// With the switch off, setting an AppID makes no requests.
func TestSetAppIDWithSwitchOffMakesNoRequests(t *testing.T) {
	f := newArtFixture(t)
	png := testPNG(t)
	f.detail = knightDetail
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	a, h, id := newArtApp(t, f, "My Game")
	if err := a.settings.SetArtworkOnline(false); err != nil {
		t.Fatal(err)
	}
	before := f.hits.Load()
	game := types.Game{ID: id, Source: types.SourceCustom, Store: types.StoreCustom, Title: "My Game"}
	if _, err := a.SetGameSteamAppID(game, "123"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if got := f.hits.Load(); got != before {
		t.Fatalf("switch off made %d requests, want none", got-before)
	}
	if got := a.custom.get().Get(id); len(got.Art) != 0 {
		t.Fatalf("switch off stored art %+v, want none", got.Art)
	}
	if len(h.emitted()) != 0 {
		t.Fatalf("switch off emitted %v, want none", h.emitted())
	}
}

// A store game is a no-op for the art lookup (the save itself still works).
func TestSetAppIDForStoreGameIsNoOp(t *testing.T) {
	f := newArtFixture(t)
	png := testPNG(t)
	f.detail = knightDetail
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	a, h, _ := newArtApp(t, f, "My Game")
	before := f.hits.Load()
	storeGame := types.Game{ID: "store-1", Source: types.SourceSteam, Store: types.StoreSteam, Title: "Store Game"}
	if _, err := a.SetGameSteamAppID(storeGame, "123"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if got := f.hits.Load(); got != before {
		t.Fatalf("store game made %d art requests, want none", got-before)
	}
	if got := a.custom.get().Get("store-1"); len(got.Art) != 0 {
		t.Fatalf("store game stored art %+v, want none", got.Art)
	}
	if len(h.emitted()) != 0 {
		t.Fatalf("store game emitted %v, want none", h.emitted())
	}
}
