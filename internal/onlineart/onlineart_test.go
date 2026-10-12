package onlineart

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// searchServer serves a fixed storesearch answer and counts its hits.
func searchServer(t *testing.T, body string, hits *atomic.Int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// Not an art request: the sandbox's remote-desktop agent
			// probes each new localhost listener with GET / (User-Agent
			// "node"). Ignore it so hit counts stay deterministic.
			w.WriteHeader(http.StatusNotFound)
			return
		}
		hits.Add(1)
		if !strings.HasPrefix(r.URL.Path, "/api/storesearch/") {
			t.Errorf("search path = %s, want /api/storesearch/", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
}

func TestSearchParsesItems(t *testing.T) {
	var hits atomic.Int64
	srv := searchServer(t, `{"total":3,"items":[
		{"id":123,"name":"Hollow Knight","tiny_image":"https://cdn/x/123.jpg"},
		{"id":0,"name":"Bad","tiny_image":"https://cdn/x/0.jpg"},
		{"id":124,"name":"","tiny_image":"https://cdn/x/124.jpg"},
		{"id":125,"name":"Hollow Knight: Silksong","tiny_image":"https://cdn/x/125.jpg"}
	]}`, &hits)
	defer srv.Close()
	c := &Client{SearchBase: srv.URL, CDNBase: srv.URL}
	got, err := c.Search("hollow knight")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("candidates = %+v, want the 2 valid items", got)
	}
	if got[0] != (Candidate{Source: "steam", ID: "123", AppID: 123, Name: "Hollow Knight", ThumbURL: "https://cdn/x/123.jpg"}) {
		t.Fatalf("first candidate = %+v", got[0])
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1", hits.Load())
	}
}

func TestSearchCapsAtEight(t *testing.T) {
	var hits atomic.Int64
	items := ""
	for i := 1; i <= 12; i++ {
		items += fmt.Sprintf(`{"id":%d,"name":"Game %d","tiny_image":"x"},`, i, i)
	}
	srv := searchServer(t, `{"total":12,"items":[`+strings.TrimSuffix(items, ",")+`]}`, &hits)
	defer srv.Close()
	c := &Client{SearchBase: srv.URL, CDNBase: srv.URL}
	got, err := c.Search("game")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != MaxCandidates {
		t.Fatalf("candidates = %d, want %d", len(got), MaxCandidates)
	}
}

func TestNormalizeTitle(t *testing.T) {
	for in, want := range map[string]string{
		"Hollow Knight™":        "hollow knight",
		"hollow-knight":         "hollow knight",
		"Half-Life 2®":          "half life 2",
		"  DOOM © (1993)  ":     "doom 1993",
		"Civilization VI":       "civilization vi",
		"Tom Clancy's XDefiant": "tom clancy s xdefiant",
	} {
		if got := NormalizeTitle(in); got != want {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExactMatchNeedsExactlyOne(t *testing.T) {
	all := []Candidate{
		{Source: "steam", ID: "1", AppID: 1, Name: "Hollow Knight"},
		{Source: "steam", ID: "2", AppID: 2, Name: "Hollow Knight™"},
		{Source: "steam", ID: "3", AppID: 3, Name: "Hollow Knight: Silksong"},
	}
	if m := ExactMatch("Hollow Knight", all); m != nil {
		t.Fatalf("two matches gave %+v, want nil", m)
	}
	if m := ExactMatch("Hollow Knight: Silksong", all); m == nil || m.AppID != 3 {
		t.Fatalf("one match gave %+v, want app 3", m)
	}
	if m := ExactMatch("Unknown Game", all); m != nil {
		t.Fatalf("no match gave %+v, want nil", m)
	}
	if m := ExactMatch("hollow-knight", all); m != nil {
		t.Fatalf("two normalized matches gave %+v, want nil", m)
	}
}

func TestDownloadSniffsContent(t *testing.T) {
	png := pngBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.png":
			w.Write(png)
		case "/fake.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			fmt.Fprint(w, "this is not an image, just text with a lying header")
		case "/missing.png":
			w.WriteHeader(http.StatusNotFound)
		case "/big.png":
			w.Write(append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, MaxImageBytes+1)...))
		}
	}))
	defer srv.Close()
	c := &Client{SearchBase: srv.URL, CDNBase: srv.URL}
	if _, err := c.Download(srv.URL + "/ok.png"); err != nil {
		t.Fatalf("valid png: %v", err)
	}
	if _, err := c.Download(srv.URL + "/fake.jpg"); err == nil {
		t.Fatal("lying content-type with text body passed the sniff")
	}
	if data, err := c.Download(srv.URL + "/missing.png"); err != nil || data != nil {
		t.Fatalf("404 = %v, %v; want nil, nil", data, err)
	}
	if _, err := c.Download(srv.URL + "/big.png"); err == nil {
		t.Fatal("over-cap download passed")
	}
}

func TestBannerFallsBackToHeader(t *testing.T) {
	jpg := jpegBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/library_hero.jpg") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/header.jpg") {
			w.Write(jpg)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := &Client{SearchBase: srv.URL, CDNBase: srv.URL}
	got, err := c.Banner(123)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, jpg) {
		t.Fatal("banner didn't fall back to header.jpg")
	}
}

func TestAppInfoParsesNameAndYear(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"123":{"success":true,"data":{"name":"Hollow Knight","release_date":{"date":"24 Feb, 2017"}}}}`)
	}))
	defer srv.Close()
	c := &Client{SearchBase: srv.URL, CDNBase: srv.URL}
	name, year, err := c.AppInfo(123)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Hollow Knight" || year != 2017 {
		t.Fatalf("appinfo = %q, %d; want Hollow Knight, 2017", name, year)
	}
}

func TestParseYear(t *testing.T) {
	for in, want := range map[string]int{
		"24 Feb, 2017": 2017,
		"2023":         2023,
		"Q1 2024":      2024,
		"Coming soon":  0,
		"":             0,
		"1969":         0,
	} {
		if got := parseYear(in); got != want {
			t.Errorf("parseYear(%q) = %d, want %d", in, got, want)
		}
	}
}

// browseServer serves a fixed GetItems answer for any app id.
func browseServer(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/IStoreBrowseService/GetItems/v1/") {
			t.Errorf("browse path = %s, want /IStoreBrowseService/GetItems/v1/", r.URL.Path)
		}
		if q := r.URL.Query().Get("input_json"); !strings.Contains(q, "4094660") && !strings.Contains(q, "123") {
			t.Errorf("input_json = %q, want the app id inside", q)
		}
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
}

const rivageBrowse = `{"response":{"store_items":[{"name":"Rivage","assets":{
	"asset_url_format":"steam/apps/4094660/${FILENAME}?t=1728660000",
	"main_capsule":"hash_0/main_capsule.jpg",
	"small_capsule":"hash_0/small_capsule.jpg",
	"hero_capsule":"hash_0/hero_capsule.jpg",
	"page_background":"hash_0/page_bg.jpg",
	"library_capsule":"hash_a/library_capsule.jpg",
	"library_capsule_2x":"hash_b/library_capsule_2x.jpg",
	"library_hero":"hash_c/library_hero.jpg",
	"library_hero_2x":"hash_d/library_hero_2x.jpg",
	"header":"hash_e/header.jpg",
	"header_2x":"hash_f/header_2x.jpg",
	"library_logo":"hash_g/logo.png",
	"library_logo_2x":"hash_h/logo_2x.png",
	"last_modified":1728660000,
	"package_discount":0,
	"library_hero_is_animated":false
}}]}}`

func TestGetItemAssetsPrefers2x(t *testing.T) {
	srv := browseServer(t, rivageBrowse, 0)
	defer srv.Close()
	c := &Client{SearchBase: srv.URL, CDNBase: srv.URL, APIBase: srv.URL}
	got, err := c.GetItemAssets(4094660)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Rivage" {
		t.Fatalf("name = %q, want Rivage", got.Name)
	}
	for url, want := range map[string]string{
		got.CoverURL:  "hash_b/library_capsule_2x.jpg",
		got.BannerURL: "hash_d/library_hero_2x.jpg",
		got.LogoURL:   "hash_h/logo_2x.png",
	} {
		if !strings.HasPrefix(url, srv.URL+"/store_item_assets/steam/apps/4094660/") {
			t.Errorf("url = %q, want the CDN base + store_item_assets prefix", url)
		}
		if !strings.Contains(url, want) {
			t.Errorf("url = %q, want it to contain %q", url, want)
		}
		if strings.Contains(url, "${FILENAME}") {
			t.Errorf("url = %q, want the placeholder substituted", url)
		}
	}
}

func TestGetItemAssetsFallsBackToNon2xAndHeader(t *testing.T) {
	body := `{"response":{"store_items":[{"name":"Old Game","assets":{
		"asset_url_format":"steam/apps/123/${FILENAME}?t=1",
		"library_capsule":"c.jpg",
		"header":"h.jpg"
	}}]}}`
	srv := browseServer(t, body, 0)
	defer srv.Close()
	c := &Client{SearchBase: srv.URL, CDNBase: srv.URL, APIBase: srv.URL}
	got, err := c.GetItemAssets(123)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.CoverURL, "c.jpg") {
		t.Fatalf("cover = %q, want c.jpg", got.CoverURL)
	}
	if !strings.Contains(got.BannerURL, "h.jpg") {
		t.Fatalf("banner = %q, want the header fallback h.jpg", got.BannerURL)
	}
	if got.LogoURL != "" {
		t.Fatalf("logo = %q, want empty when the app has none", got.LogoURL)
	}
}

func TestGetItemAssetsEmptyWhenNoAssets(t *testing.T) {
	for _, body := range []string{
		`{"response":{"store_items":[]}}`,
		`{"response":{"store_items":[{"name":"No Art","assets":{}}]}}`,
		`{"response":{"store_items":[{"name":"No Format","assets":{"library_capsule":"c.jpg"}}]}}`,
	} {
		srv := browseServer(t, body, 0)
		c := &Client{SearchBase: srv.URL, CDNBase: srv.URL, APIBase: srv.URL}
		got, err := c.GetItemAssets(123)
		srv.Close()
		if err != nil {
			t.Fatalf("body %s: %v", body, err)
		}
		if got.CoverURL != "" || got.BannerURL != "" || got.LogoURL != "" {
			t.Fatalf("body %s gave URLs %+v, want none", body, got)
		}
	}
}

// Regression: the live assets object mixes file paths with numbers and
// bools (last_modified, …). Non-string values must be skipped, never fail
// the decode — Rivage fell back to the 404 fixed names over this.
func TestGetItemAssetsIgnoresNonStringFields(t *testing.T) {
	body := `{"response":{"store_items":[{"name":"Rivage","assets":{
		"asset_url_format":"steam/apps/4094660/${FILENAME}?t=1728660000",
		"main_capsule":"hash_0/main_capsule.jpg",
		"small_capsule":"hash_0/small_capsule.jpg",
		"hero_capsule":"hash_0/hero_capsule.jpg",
		"page_background":"hash_0/page_bg.jpg",
		"library_capsule_2x":"hash_b/cover_2x.jpg",
		"library_hero_2x":"hash_d/hero_2x.jpg",
		"last_modified":1728660000,
		"package_discount":0,
		"library_hero_is_animated":false,
		"discount_block":null
	}}]}}`
	srv := browseServer(t, body, 0)
	defer srv.Close()
	c := &Client{SearchBase: srv.URL, CDNBase: srv.URL, APIBase: srv.URL}
	got, err := c.GetItemAssets(4094660)
	if err != nil {
		t.Fatalf("decode with numeric/bool/null asset fields: %v", err)
	}
	if !strings.Contains(got.CoverURL, "hash_b/cover_2x.jpg") {
		t.Fatalf("cover = %q, want the 2x path", got.CoverURL)
	}
	if !strings.Contains(got.BannerURL, "hash_d/hero_2x.jpg") {
		t.Fatalf("banner = %q, want the 2x path", got.BannerURL)
	}
}

func TestGetItemAssetsErrorWhenAPIFails(t *testing.T) {
	srv := browseServer(t, "", http.StatusInternalServerError)
	defer srv.Close()
	c := &Client{SearchBase: srv.URL, CDNBase: srv.URL, APIBase: srv.URL}
	if _, err := c.GetItemAssets(123); err == nil {
		t.Fatal("500 gave nil, want an error so the caller falls back to fixed names")
	}
}

// gogServer serves a fixed catalog answer and counts its hits, checking the
// query shape and the User-Agent.
func gogServer(t *testing.T, body string, hits *atomic.Int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// Not an art request: the sandbox's remote-desktop agent
			// probes each new localhost listener with GET / (User-Agent
			// "node"). Ignore it so hit counts stay deterministic.
			w.WriteHeader(http.StatusNotFound)
			return
		}
		hits.Add(1)
		if !strings.HasPrefix(r.URL.Path, "/v1/catalog") {
			t.Errorf("gog path = %s, want /v1/catalog", r.URL.Path)
		}
		if got := r.Header.Get("User-Agent"); !strings.HasPrefix(got, "Platypus/") || !strings.Contains(got, "github.com/polynux/platypus-launcher") {
			t.Errorf("gog User-Agent = %q, want Platypus/<version> + contact", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
}

const hollowKnightGOG = `{"products":[
	{"id":"1308320804","title":"Hollow Knight","releaseDate":"2017.02.24",
	 "coverVertical":"https://images.gog-statics.com/cover_v.jpg",
	 "coverHorizontal":"https://images.gog-statics.com/banner_h.png",
	 "logo":"https://images.gog-statics.com/logo.png"},
	{"id":"999","title":"","releaseDate":"2020.01.01","coverVertical":"https://x/v.jpg","coverHorizontal":"https://x/h.jpg","logo":""},
	{"id":"1000","title":"No Art","releaseDate":"2020.01.01","coverVertical":"","coverHorizontal":"","logo":""}
]}`

func TestSearchGOGParsesProducts(t *testing.T) {
	var hits atomic.Int64
	srv := gogServer(t, hollowKnightGOG, &hits)
	defer srv.Close()
	c := &Client{GOGBase: srv.URL}
	got, err := c.SearchGOG("Hollow Knight")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("candidates = %+v, want the 1 valid product", got)
	}
	g := got[0]
	if g.Source != "gog" || g.ID != "1308320804" || g.Name != "Hollow Knight" || g.Year != 2017 {
		t.Fatalf("candidate = %+v, want gog/1308320804/Hollow Knight/2017", g)
	}
	if g.CoverURL != "https://images.gog-statics.com/cover_v.jpg" || g.BannerURL != "https://images.gog-statics.com/banner_h.png" || g.LogoURL != "https://images.gog-statics.com/logo.png" {
		t.Fatalf("art urls = %+v", g)
	}
	if g.ThumbURL != "https://images.gog-statics.com/banner_h.png" {
		t.Fatalf("thumb = %q, want the horizontal cover", g.ThumbURL)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1", hits.Load())
	}
}

func TestSearchGOGSendsQueryShape(t *testing.T) {
	var query url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"products":[]}`)
	}))
	defer srv.Close()
	c := &Client{GOGBase: srv.URL}
	if _, err := c.SearchGOG("Hollow Knight"); err != nil {
		t.Fatal(err)
	}
	if query.Get("limit") != "8" || query.Get("locale") != "en-US" || query.Get("query") != "like:Hollow Knight" {
		t.Fatalf("query = %v, want limit=8 locale=en-US query=like:<title>", query)
	}
}

// lutrisServer serves a fixed games search answer and counts its hits,
// checking the User-Agent.
func lutrisServer(t *testing.T, body string, hits *atomic.Int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			// Not an art request: the sandbox's remote-desktop agent
			// probes each new localhost listener with GET / (User-Agent
			// "node"). Ignore it so hit counts stay deterministic.
			w.WriteHeader(http.StatusNotFound)
			return
		}
		hits.Add(1)
		if got := r.Header.Get("User-Agent"); !strings.HasPrefix(got, "Platypus/") || !strings.Contains(got, "github.com/polynux/platypus-launcher") {
			t.Errorf("lutris User-Agent = %q, want Platypus/<version> + contact", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
}

const rivageLutris = `{"count":1,"results":[
	{"name":"Rivage","slug":"rivage","year":2026,"coverart":"https://lutris.net/media/igdb/cover_big/coay6k.jpg",
	 "provider_games":[{"name":"Rivage","slug":"rivage","service":"igdb"}]}
]}`

func TestSearchLutrisParsesResults(t *testing.T) {
	var hits atomic.Int64
	srv := lutrisServer(t, rivageLutris, &hits)
	defer srv.Close()
	c := &Client{LutrisBase: srv.URL, IGDBBase: srv.URL + "/igdb-test"}
	got, err := c.SearchLutris("Rivage")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("candidates = %+v, want 1", got)
	}
	l := got[0]
	if l.Source != "lutris" || l.ID != "rivage" || l.Name != "Rivage" || l.Year != 2026 {
		t.Fatalf("candidate = %+v, want lutris/rivage/Rivage/2026", l)
	}
	// The thumb and cover use the IGDB upgrade, not the lutris.net file.
	want := srv.URL + "/igdb-test/igdb/image/upload/t_cover_big_2x/coay6k.jpg"
	if l.ThumbURL != want || l.CoverURL != want {
		t.Fatalf("cover = %q thumb = %q, want %q", l.CoverURL, l.ThumbURL, want)
	}
	if l.CoverFallbackURL != "https://lutris.net/media/igdb/cover_big/coay6k.jpg" {
		t.Fatalf("fallback = %q", l.CoverFallbackURL)
	}
	if l.BannerURL != "" || l.LogoURL != "" {
		t.Fatalf("lutris gives covers only, got %+v", l)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want 1", hits.Load())
	}
}

// The generic library_capsule coverart carries no image id and is skipped;
// a hit that maps to a Steam app is still offered as a Steam candidate.
func TestSearchLutrisSkipsGenericCoverart(t *testing.T) {
	body := `{"count":2,"results":[
		{"name":"Hollow Knight","slug":"hollow-knight","year":2017,
		 "coverart":"https://lutris.net/media/igdb/library_capsule.jpg",
		 "provider_games":[{"name":"Hollow Knight","slug":"367520","service":"steam"}]},
		{"name":"No Art No Steam","slug":"no-art","year":2020,
		 "coverart":"https://lutris.net/media/igdb/library_capsule.jpg",
		 "provider_games":[]}
	]}`
	var hits atomic.Int64
	srv := lutrisServer(t, body, &hits)
	defer srv.Close()
	c := &Client{LutrisBase: srv.URL, IGDBBase: srv.URL}
	got, err := c.SearchLutris("Hollow Knight")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("candidates = %+v, want only the Steam-mapped hit", got)
	}
	s := got[0]
	if s.Source != "steam" || s.ID != "367520" || s.AppID != 367520 || s.Name != "Hollow Knight" || s.Year != 2017 {
		t.Fatalf("candidate = %+v, want steam/367520/Hollow Knight/2017", s)
	}
	if s.ThumbURL != "" {
		t.Fatalf("thumb = %q, want empty: the generic art is never used", s.ThumbURL)
	}
}

func TestLutrisImageID(t *testing.T) {
	for in, want := range map[string]string{
		"https://lutris.net/media/igdb/cover_big/coay6k.jpg": "coay6k",
		"http://x/media/igdb/cover_big/co3dk6.jpg?foo=1":     "co3dk6",
		"https://lutris.net/media/igdb/library_capsule.jpg":  "",
		"https://lutris.net/media/igdb/cover_big/.jpg":       "",
		"https://lutris.net/media/igdb/cover_big/a/b.jpg":    "",
		"https://lutris.net/media/igdb/cover_big/a b.jpg":    "",
		"https://lutris.net/games/banner/hollow-knight.jpg":  "",
		"": "",
	} {
		if got := lutrisImageID(in); got != want {
			t.Errorf("lutrisImageID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDownloadSendsUserAgent(t *testing.T) {
	var ua string
	png := pngBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		w.Write(png)
	}))
	defer srv.Close()
	c := &Client{SearchBase: srv.URL, CDNBase: srv.URL}
	if _, err := c.Download(srv.URL + "/cover.jpg"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(ua, "Platypus/") || !strings.Contains(ua, "github.com/polynux/platypus-launcher") {
		t.Fatalf("User-Agent = %q, want Platypus/<version> + contact", ua)
	}
}

func TestSteamNeedsFallback(t *testing.T) {
	mk := func(names ...string) []Candidate {
		out := []Candidate{}
		for i, n := range names {
			out = append(out, Candidate{Source: "steam", ID: string(rune('1' + i)), AppID: i + 1, Name: n})
		}
		return out
	}
	if !SteamNeedsFallback("Hollow Knight", nil) {
		t.Error("no hits: want fallback")
	}
	if !SteamNeedsFallback("Hollow Knight", mk("Hollow Knight")) {
		t.Error("1 hit: want fallback (fewer than 3)")
	}
	if !SteamNeedsFallback("Hollow Knight", mk("A", "B", "C")) {
		t.Error("3 hits but none matching: want fallback")
	}
	if SteamNeedsFallback("Hollow Knight", mk("Hollow Knight", "B", "C")) {
		t.Error("3 hits with a name match: want no fallback")
	}
	if SteamNeedsFallback("Hollow Knight", mk("Hollow Knight™", "B", "C", "D")) {
		t.Error("4 hits with a normalized match: want no fallback")
	}
}

func TestMergeCandidatesDedupesAndOrders(t *testing.T) {
	steam := []Candidate{{Source: "steam", ID: "1", AppID: 1, Name: "Hollow Knight", Year: 2017}}
	gog := []Candidate{
		{Source: "gog", ID: "g1", Name: "Hollow Knight", Year: 2017},
		{Source: "gog", ID: "g2", Name: "Hollow Knight", Year: 2025},
		{Source: "gog", ID: "g3", Name: "GOG Only", Year: 2020},
	}
	lutris := []Candidate{
		{Source: "lutris", ID: "l1", Name: "Hollow Knight", Year: 2017},
		{Source: "lutris", ID: "l2", Name: "Lutris Only", Year: 2021},
	}
	got := MergeCandidates(steam, gog, lutris)
	want := []string{"steam:1", "gog:g2", "gog:g3", "lutris:l2"}
	if len(got) != len(want) {
		t.Fatalf("merged = %+v, want %v", got, want)
	}
	for i, w := range want {
		if got[i].Source+":"+got[i].ID != w {
			t.Fatalf("merged[%d] = %s:%s, want %s (full %+v)", i, got[i].Source, got[i].ID, w, got)
		}
	}
}

func TestMergeCandidatesCapsAtEight(t *testing.T) {
	mk := func(src string, n int) []Candidate {
		out := []Candidate{}
		for i := 0; i < n; i++ {
			out = append(out, Candidate{Source: src, ID: fmt.Sprintf("%s-%d", src, i), Name: fmt.Sprintf("Game %s %d", src, i)})
		}
		return out
	}
	got := MergeCandidates(mk("steam", 3), mk("gog", 3), mk("lutris", 3))
	if len(got) != MaxCandidates {
		t.Fatalf("merged = %d, want %d", len(got), MaxCandidates)
	}
	if got[0].Source != "steam" || got[3].Source != "gog" || got[6].Source != "lutris" {
		t.Fatalf("order = %v, want steam, then gog, then lutris", got)
	}
}

func TestLutrisGameFetchesBySlug(t *testing.T) {
	var ua, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua, path = r.Header.Get("User-Agent"), r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"Rivage","slug":"rivage","year":2026,
			"coverart":"https://lutris.net/media/igdb/cover_big/coay6k.jpg",
			"provider_games":[{"name":"Rivage","slug":"rivage","service":"igdb"}]}`)
	}))
	defer srv.Close()
	c := &Client{LutrisBase: srv.URL, IGDBBase: srv.URL}
	got, err := c.LutrisGame("rivage")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/api/games/rivage" {
		t.Fatalf("path = %s, want /api/games/rivage", path)
	}
	if !strings.HasPrefix(ua, "Platypus/") {
		t.Fatalf("User-Agent = %q", ua)
	}
	if got.Source != "lutris" || got.ID != "rivage" || got.Name != "Rivage" {
		t.Fatalf("game = %+v", got)
	}
	if !strings.HasPrefix(got.CoverURL, srv.URL) || !strings.Contains(got.CoverURL, "coay6k.jpg") {
		t.Fatalf("cover = %q, want the injected IGDB base + image id", got.CoverURL)
	}
}

func TestLutrisGameNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := &Client{LutrisBase: srv.URL, IGDBBase: srv.URL}
	if _, err := c.LutrisGame("missing"); err == nil {
		t.Fatal("404 gave nil, want an error")
	}
}
