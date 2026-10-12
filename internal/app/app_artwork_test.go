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
	"platypus/internal/steam"
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
	t          *testing.T
	srv        *httptest.Server
	hits       atomic.Int64
	search     string
	images     map[string][]byte // path suffix -> body; absent means 404
	fail       map[string]int    // path suffix -> status code (e.g. a logo 500)
	detail     string
	browse     string // GetItems store_items JSON object, e.g. `{"name":"Rivage","assets":{...}}`; empty means no items
	browseFail int    // GetItems status code when non-zero (e.g. 500)
	gog        string // catalog products JSON objects, e.g. `{"id":"1","title":"G",...}`; empty means no products
	lutris     string // lutris search results JSON objects; empty means no results
	// lutrisGame is the single-game object served for a Lutris slug lookup;
	// lutrisGameFail, when non-zero, is its status code instead.
	lutrisGame     string
	lutrisGameFail int
	// gogInfo is the product object served for a GOG product lookup
	// (/products/<id>); gogInfoFail, when non-zero, is its status code.
	gogInfo     string
	gogInfoFail int
	// steamDetail is the store appdetails object served for sideloaded Steam
	// info lookups. Empty means the art detail doubles for it.
	steamDetail string
	// searchGate, when non-nil, holds the search handler until closed: the
	// test can remove the game mid-flight.
	searchGate chan struct{}
}

func newArtFixture(t *testing.T) *artFixture {
	t.Helper()
	f := &artFixture{t: t, images: map[string][]byte{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// Not an art request: the sandbox's remote-desktop agent
			// probes each new localhost listener with GET / (User-Agent
			// "node"). Ignore it so hit counts stay deterministic.
			w.WriteHeader(http.StatusNotFound)
			return
		}
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
			if f.steamDetail != "" {
				fmt.Fprint(w, f.steamDetail)
			} else {
				fmt.Fprint(w, f.detail)
			}
		case strings.HasPrefix(r.URL.Path, "/products/"):
			if f.gogInfoFail != 0 {
				w.WriteHeader(f.gogInfoFail)
				return
			}
			if f.gogInfo == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, f.gogInfo)
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
		case strings.HasPrefix(r.URL.Path, "/v1/catalog"):
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"products":[%s]}`, f.gog)
		case strings.HasPrefix(r.URL.Path, "/api/games/"):
			if f.lutrisGameFail != 0 {
				w.WriteHeader(f.lutrisGameFail)
				return
			}
			if f.lutrisGame == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, f.lutrisGame)
		case r.URL.Path == "/api/games":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"results":[%s]}`, f.lutris)
		default:
			// Images by suffix: Steam hashed/fixed names, GOG, IGDB and
			// lutris.net art alike. A test image URL points at this server.
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
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func newArtApp(t *testing.T, f *artFixture, title string) (*App, *artHost, string) {
	t.Helper()
	root := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), root)
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
	a := &App{host: h, settings: st, dataDir: root, optHome: root, optCacheDir: root, artwork: &onlineart.Client{SearchBase: f.srv.URL, CDNBase: f.srv.URL, APIBase: f.srv.URL, GOGBase: f.srv.URL, GOGInfoBase: f.srv.URL, LutrisBase: f.srv.URL, IGDBBase: f.srv.URL}, sideloadSteam: &steam.StoreClient{Base: f.srv.URL, HTTPClient: f.srv.Client()}}
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

	c, err := a.ApplyOnlineArt(id, "steam", "123")
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
	t.Parallel()
	f := newArtFixture(t)
	f.detail = knightDetail // art endpoints all 404
	a, _, id := newArtApp(t, f, "Hollow Knight")
	if _, err := a.ApplyOnlineArt(id, "steam", "123"); err == nil {
		t.Fatal("all-404 apply = nil, want an error to show")
	}
}

func TestArtworkOffMakesNoRequests(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	f.search = knightSearch
	f.detail = knightDetail
	png := testPNG(t)
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	a, _, id := newArtApp(t, f, "Hollow Knight")
	if err := a.settings.SetArtworkOnline(false); err != nil {
		t.Fatal(err)
	}

	if _, err := a.SearchArtOnline("Hollow Knight"); err == nil {
		t.Fatal("search with the switch off = nil, want a refusal")
	}
	if _, err := a.ApplyOnlineArt(id, "gog", "1308320804"); err == nil {
		t.Fatal("gog apply with the switch off = nil, want a refusal")
	}
	if _, err := a.ApplyOnlineArt(id, "lutris", "rivage"); err == nil {
		t.Fatal("lutris apply with the switch off = nil, want a refusal")
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
	t.Parallel()
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
	t.Parallel()
	f := newArtFixture(t)
	png := testPNG(t)
	f.detail = knightDetail
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	f.fail = map[string]int{"logo.png": http.StatusInternalServerError}
	a, h, id := newArtApp(t, f, "Hollow Knight")

	if _, err := a.ApplyOnlineArt(id, "steam", "123"); err != nil {
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
	t.Parallel()
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
	t.Parallel()
	f := newArtFixture(t)
	png := testPNG(t)
	f.detail = `{"4094660":{"success":true,"data":{"name":"Rivage","release_date":{"date":"2025"}}}}`
	f.browse = `{"name":"Rivage","assets":{
		"asset_url_format":"steam/apps/4094660/${FILENAME}?t=1728660000",
		"main_capsule":"hash_0/main_capsule.jpg",
		"small_capsule":"hash_0/small_capsule.jpg",
		"hero_capsule":"hash_0/hero_capsule.jpg",
		"page_background":"hash_0/page_bg.jpg",
		"library_capsule":"hash_a/rivage_cover.jpg",
		"library_capsule_2x":"hash_b/rivage_cover_2x.jpg",
		"library_hero":"hash_c/rivage_hero.jpg",
		"library_hero_2x":"hash_d/rivage_hero_2x.jpg",
		"header":"hash_e/rivage_header.jpg",
		"library_logo":"hash_f/rivage_logo.png",
		"last_modified":1728660000
	}}`
	// Only the hashed files exist; the fixed names all 404.
	f.images["rivage_cover_2x.jpg"] = png
	f.images["rivage_hero_2x.jpg"] = png
	f.images["rivage_logo.png"] = png
	a, _, id := newArtApp(t, f, "Rivage")

	c, err := a.ApplyOnlineArt(id, "steam", "4094660")
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
	t.Parallel()
	f := newArtFixture(t)
	png := testPNG(t)
	f.detail = knightDetail
	f.browseFail = http.StatusInternalServerError
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	a, _, id := newArtApp(t, f, "Hollow Knight")

	if _, err := a.ApplyOnlineArt(id, "steam", "123"); err != nil {
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
		if _, err := a.ApplyOnlineArt(id, "steam", "123"); err != nil {
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
	t.Parallel()
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
	t.Parallel()
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

// gogProductJSON builds one catalog product pointing its art at the fixture.
func gogProductJSON(base, id, title, date string) string {
	return fmt.Sprintf(`{"id":%q,"title":%q,"releaseDate":%q,`+
		`"coverVertical":%q,"coverHorizontal":%q,"logo":%q}`,
		id, title, date,
		base+"/gog/cover_"+id+".jpg", base+"/gog/banner_"+id+".png", base+"/gog/logo_"+id+".png")
}

// The fallbacks stay quiet while Steam answers well: three hits with a name
// match means one request, the Steam search.
func TestSearchArtOnlineSkipsFallbacksWhenSteamEnough(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	f.search = knightSearch + `,{"id":125,"name":"Hollow Knight: Voidheart","tiny_image":"http://x/125.jpg"}`
	f.gog = gogProductJSON(f.srv.URL, "1308320804", "Hollow Knight", "2017.02.24")
	f.lutris = `{"name":"Rivage","slug":"rivage","year":2026,"coverart":"https://lutris.net/media/igdb/cover_big/coay6k.jpg","provider_games":[]}`
	a, _, _ := newArtApp(t, f, "Hollow Knight")

	got, err := a.SearchArtOnline("Hollow Knight")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("candidates = %+v, want the 3 Steam hits only", got)
	}
	for _, c := range got {
		if c.Source != "steam" {
			t.Fatalf("candidate = %+v, want Steam only", c)
		}
	}
	if hits := f.hits.Load(); hits != 1 {
		t.Fatalf("server hits = %d, want 1: no fallback may go out", hits)
	}
}

// Steam names nothing like the title: the fallbacks are queried even with
// three hits, and merge in order with duplicates dropped.
func TestSearchArtOnlineMergesFallbacksWhenThin(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	f.search = `{"id":1,"name":"Unrelated A","tiny_image":"http://x/1.jpg"},{"id":2,"name":"Unrelated B","tiny_image":"http://x/2.jpg"},{"id":3,"name":"Unrelated C","tiny_image":"http://x/3.jpg"}`
	f.gog = gogProductJSON(f.srv.URL, "1308320804", "Hollow Knight", "2017.02.24")
	// A Lutris hit with the same name and year as the GOG one: the Lutris
	// duplicate drops, preferring GOG.
	f.lutris = `{"name":"Hollow Knight","slug":"hollow-knight","year":2017,"coverart":"` + f.srv.URL + `/media/igdb/cover_big/co9999.jpg","provider_games":[]},` +
		`{"name":"Rivage","slug":"rivage","year":2026,"coverart":"https://lutris.net/media/igdb/cover_big/coay6k.jpg","provider_games":[]}`
	a, _, _ := newArtApp(t, f, "Rivage")

	got, err := a.SearchArtOnline("Rivage")
	if err != nil {
		t.Fatal(err)
	}
	if hits := f.hits.Load(); hits != 3 {
		t.Fatalf("server hits = %d, want 3: steam, gog and lutris", hits)
	}
	if len(got) != 5 {
		t.Fatalf("candidates = %+v, want 3 steam + gog HK + lutris Rivage (5)", got)
	}
	if got[0].Source != "steam" || got[3].Source != "gog" || got[3].ID != "1308320804" {
		t.Fatalf("order = %+v, want steam first, then gog", got)
	}
	var hkGOG, hkLutris, rivage int
	for _, c := range got {
		switch {
		case c.Source == "gog" && c.Name == "Hollow Knight":
			hkGOG++
			if c.Year != 2017 {
				t.Fatalf("gog year = %d, want 2017", c.Year)
			}
		case c.Source == "lutris" && c.Name == "Hollow Knight":
			hkLutris++
		case c.Source == "lutris" && c.ID == "rivage":
			rivage++
		}
	}
	if hkGOG != 1 || hkLutris != 0 || rivage != 1 {
		t.Fatalf("gogHK = %d lutrisHK = %d rivage = %d, want 1/0/1", hkGOG, hkLutris, rivage)
	}
}

// A GOG manual apply stores cover, banner and logo with a gog: source, the
// year, and no compat AppID (Steam-only).
func TestApplyOnlineArtGOG(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	png := testPNG(t)
	f.gog = gogProductJSON(f.srv.URL, "1308320804", "Hollow Knight", "2017.02.24")
	f.images["gog/cover_1308320804.jpg"] = png
	f.images["gog/banner_1308320804.png"] = png
	f.images["gog/logo_1308320804.png"] = png
	a, _, id := newArtApp(t, f, "Hollow Knight")

	c, err := a.ApplyOnlineArt(id, "gog", "1308320804")
	if err != nil {
		t.Fatal(err)
	}
	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, png) {
		t.Fatal("cover wasn't stored from coverVertical")
	}
	if got := a.artBytes(t, id, "banner"); !bytes.Equal(got, png) {
		t.Fatal("banner wasn't stored from coverHorizontal")
	}
	if p := a.custom.get().Path(id, customize.Logo); p == "" {
		t.Fatal("logo wasn't stored")
	}
	if c.ArtSource != "gog:1308320804" || c.ArtSourceName != "Hollow Knight" {
		t.Fatalf("attribution = %q/%q, want gog:1308320804/Hollow Knight", c.ArtSource, c.ArtSourceName)
	}
	if got := a.settings.CompatAppID(id); got != "" {
		t.Fatalf("compat app id = %q, want none: it stays Steam-only", got)
	}
	for _, g := range a.settings.CustomGames() {
		if g.ID == id && g.ReleaseYear != 2017 {
			t.Fatalf("release year = %d, want 2017", g.ReleaseYear)
		}
	}
}

func TestApplyOnlineArtGOGUnknownID(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	f.gog = gogProductJSON(f.srv.URL, "1308320804", "Hollow Knight", "2017.02.24")
	a, _, id := newArtApp(t, f, "Hollow Knight")
	if _, err := a.ApplyOnlineArt(id, "gog", "no-such-product"); err == nil {
		t.Fatal("unknown GOG id = nil, want an error to show")
	}
}

// Searching "rivage" for a game titled "test" caches the GOG hit, so
// applying it works even though the saved title would never find it (the
// reviewer's case: GOG has no lookup by product id).
func TestApplyOnlineArtGOGUsesSearchCache(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	png := testPNG(t)
	f.search = "" // Steam knows nothing like it, so the fallbacks fire.
	f.gog = gogProductJSON(f.srv.URL, "4242", "Rivage", "2026.01.01")
	f.images["gog/cover_4242.jpg"] = png
	f.images["gog/banner_4242.png"] = png
	a, _, id := newArtApp(t, f, "test")

	got, err := a.SearchArtOnline("rivage")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("search found nothing to cache")
	}

	c, err := a.ApplyOnlineArt(id, "gog", "4242")
	if err != nil {
		t.Fatal(err)
	}
	if art := a.artBytes(t, id, "cover"); !bytes.Equal(art, png) {
		t.Fatal("cover wasn't stored from the cached GOG hit")
	}
	if c.ArtSource != "gog:4242" || c.ArtSourceName != "Rivage" {
		t.Fatalf("attribution = %q/%q, want gog:4242/Rivage", c.ArtSource, c.ArtSourceName)
	}
}

// Without a prior search, a GOG apply falls back to the game's own title;
// when that cannot reproduce the hit the error says to search again.
func TestApplyOnlineArtGOGCacheMissSaysSearchAgain(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	f.gog = gogProductJSON(f.srv.URL, "999", "Some Game", "2020.01.01")
	a, _, id := newArtApp(t, f, "test")

	_, err := a.ApplyOnlineArt(id, "gog", "1308320804")
	if err == nil {
		t.Fatal("stale GOG id = nil, want an error to show")
	}
	if !strings.Contains(err.Error(), "search for the art again") {
		t.Fatalf("error = %q, want it to say to search again", err)
	}
}

// A Lutris manual apply stores the upgraded IGDB cover with a lutris:
// source. Here only the IGDB copy exists: the lutris.net file 404s.
func TestApplyOnlineArtLutrisUsesIGDBUpgrade(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	png := testPNG(t)
	f.lutrisGame = `{"name":"Rivage","slug":"rivage","year":2026,` +
		`"coverart":"` + f.srv.URL + `/media/igdb/cover_big/coay6k.jpg","provider_games":[]}`
	f.images["t_cover_big_2x/coay6k.jpg"] = png
	a, _, id := newArtApp(t, f, "Rivage")

	c, err := a.ApplyOnlineArt(id, "lutris", "rivage")
	if err != nil {
		t.Fatal(err)
	}
	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, png) {
		t.Fatal("cover wasn't stored from the IGDB upgrade")
	}
	if c.ArtSource != "lutris:rivage" || c.ArtSourceName != "Rivage" {
		t.Fatalf("attribution = %q/%q, want lutris:rivage/Rivage", c.ArtSource, c.ArtSourceName)
	}
}

// When the IGDB copy is absent the lutris.net coverart itself applies.
func TestApplyOnlineArtLutrisFallsBackToCoverart(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	png := testPNG(t)
	f.lutrisGame = `{"name":"Rivage","slug":"rivage","year":2026,` +
		`"coverart":"` + f.srv.URL + `/media/igdb/cover_big/coay6k.jpg","provider_games":[]}`
	f.images["media/igdb/cover_big/coay6k.jpg"] = png // no t_cover_big_2x copy: 404
	a, _, id := newArtApp(t, f, "Rivage")

	if _, err := a.ApplyOnlineArt(id, "lutris", "rivage"); err != nil {
		t.Fatal(err)
	}
	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, png) {
		t.Fatal("cover wasn't stored from the coverart fallback")
	}
}

// A Lutris hit that maps to a Steam app takes the Steam path when Steam has
// the art: the generic coverart is skipped and attribution is Steam's.
func TestApplyOnlineArtLutrisUsesSteamWhenMapped(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	png := testPNG(t)
	f.detail = knightDetail
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	f.lutrisGame = `{"name":"Hollow Knight","slug":"hollow-knight","year":2017,` +
		`"coverart":"https://lutris.net/media/igdb/library_capsule.jpg","steamid":123}`
	a, _, id := newArtApp(t, f, "Hollow Knight")

	c, err := a.ApplyOnlineArt(id, "lutris", "hollow-knight")
	if err != nil {
		t.Fatal(err)
	}
	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, png) {
		t.Fatal("cover wasn't stored from the mapped Steam app")
	}
	if c.ArtSource != "steam:123" {
		t.Fatalf("attribution = %q, want steam:123", c.ArtSource)
	}
	if got := a.settings.CompatAppID(id); got != "123" {
		t.Fatalf("compat app id = %q, want 123 from the Steam path", got)
	}
}

// When Steam has nothing for the mapped app, the Lutris cover still lands.
func TestApplyOnlineArtLutrisCoverWhenSteamBare(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	png := testPNG(t)
	f.detail = knightDetail // Steam art endpoints all 404
	f.lutrisGame = `{"name":"Hollow Knight","slug":"hollow-knight","year":2017,` +
		`"coverart":"` + f.srv.URL + `/media/igdb/cover_big/co3dk6.jpg","steamid":123}`
	f.images["t_cover_big_2x/co3dk6.jpg"] = png
	a, _, id := newArtApp(t, f, "Hollow Knight")

	c, err := a.ApplyOnlineArt(id, "lutris", "hollow-knight")
	if err != nil {
		t.Fatal(err)
	}
	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, png) {
		t.Fatal("cover wasn't stored from Lutris when Steam was bare")
	}
	if c.ArtSource != "lutris:hollow-knight" {
		t.Fatalf("attribution = %q/%q, want lutris:hollow-knight", c.ArtSource, c.ArtSourceName)
	}
}

func TestApplyOnlineArtUnknownSource(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	a, _, id := newArtApp(t, f, "Hollow Knight")
	if _, err := a.ApplyOnlineArt(id, "itch", "123"); err == nil {
		t.Fatal("unknown source = nil, want an error to show")
	}
}

// The auto path tries GOG when Steam has no exact single match.
func TestAutoArtworkFallsBackToGOG(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	png := testPNG(t)
	f.search = "" // Steam knows nothing like it
	f.gog = gogProductJSON(f.srv.URL, "4242", "Rivage", "2026.01.01")
	f.images["gog/cover_4242.jpg"] = png
	f.images["gog/banner_4242.png"] = png
	a, h, id := newArtApp(t, f, "Rivage")

	a.autoArtwork(id, "Rivage")

	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, png) {
		t.Fatal("cover wasn't auto-applied from GOG")
	}
	e := a.custom.get().Get(id)
	if e.ArtSource != "gog:4242" || e.ArtSourceName != "Rivage" {
		t.Fatalf("source = %q/%q, want gog:4242/Rivage", e.ArtSource, e.ArtSourceName)
	}
	if len(h.emits) != 1 || h.emits[0] != "library" {
		t.Fatalf("emits = %v, want one library event", h.emits)
	}
	for _, g := range a.settings.CustomGames() {
		if g.ID == id && g.ReleaseYear != 2026 {
			t.Fatalf("release year = %d, want 2026", g.ReleaseYear)
		}
	}
}

// The auto path tries Lutris when neither Steam nor GOG has an exact single
// match. Steam's two near-misses and GOG's two exact hits are ambiguous;
// Lutris names it once.
func TestAutoArtworkFallsBackToLutris(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	png := testPNG(t)
	f.search = `{"id":1,"name":"Rivage"},{"id":2,"name":"Rivage™"}`
	f.gog = gogProductJSON(f.srv.URL, "1", "Rivage", "2026.01.01") + "," +
		gogProductJSON(f.srv.URL, "2", "Rivage", "2026.01.01")
	f.lutris = `{"name":"Rivage","slug":"rivage","year":2026,"coverart":"` + f.srv.URL + `/media/igdb/cover_big/coay6k.jpg","provider_games":[]}`
	f.images["t_cover_big_2x/coay6k.jpg"] = png
	a, h, id := newArtApp(t, f, "Rivage")

	a.autoArtwork(id, "Rivage")

	if got := a.artBytes(t, id, "cover"); !bytes.Equal(got, png) {
		t.Fatal("cover wasn't auto-applied from Lutris")
	}
	e := a.custom.get().Get(id)
	if e.ArtSource != "lutris:rivage" {
		t.Fatalf("source = %q, want lutris:rivage", e.ArtSource)
	}
	if len(h.emits) != 1 || h.emits[0] != "library" {
		t.Fatalf("emits = %v, want one library event", h.emits)
	}
}

// Ambiguous everywhere means nothing and silence.
func TestAutoArtworkAmbiguousFallbacksDoNothing(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	png := testPNG(t)
	f.search = `{"id":1,"name":"Rivage"},{"id":2,"name":"Rivage™"}`
	f.gog = gogProductJSON(f.srv.URL, "1", "Rivage", "2026.01.01") + "," +
		gogProductJSON(f.srv.URL, "2", "Rivage", "2026.01.01")
	f.lutris = `{"name":"Rivage X","slug":"rivage-x","year":2026,"coverart":"` + f.srv.URL + `/media/igdb/cover_big/coay6k.jpg","provider_games":[]}`
	f.images["gog/cover_1.jpg"] = png
	f.images["t_cover_big_2x/coay6k.jpg"] = png
	a, h, id := newArtApp(t, f, "Rivage")

	a.autoArtwork(id, "Rivage")

	if got := a.custom.get().Get(id); len(got.Art) != 0 {
		t.Fatalf("art = %v, want none", got.Art)
	}
	if len(h.emits) != 0 || len(h.warns) != 0 {
		t.Fatalf("emits = %v warns = %v, want silence", h.emits, h.warns)
	}
}
