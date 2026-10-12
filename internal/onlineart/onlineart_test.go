package onlineart

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
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
	if got[0] != (Candidate{AppID: 123, Name: "Hollow Knight", ThumbURL: "https://cdn/x/123.jpg"}) {
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
		"Hollow Knight™":      "hollow knight",
		"hollow-knight":       "hollow knight",
		"Half-Life 2®":        "half life 2",
		"  DOOM © (1993)  ":   "doom 1993",
		"Civilization VI":     "civilization vi",
		"Tom Clancy's XDefiant": "tom clancy s xdefiant",
	} {
		if got := NormalizeTitle(in); got != want {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExactMatchNeedsExactlyOne(t *testing.T) {
	all := []Candidate{
		{AppID: 1, Name: "Hollow Knight"},
		{AppID: 2, Name: "Hollow Knight™"},
		{AppID: 3, Name: "Hollow Knight: Silksong"},
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
	"library_capsule":"hash_a/library_capsule.jpg",
	"library_capsule_2x":"hash_b/library_capsule_2x.jpg",
	"library_hero":"hash_c/library_hero.jpg",
	"library_hero_2x":"hash_d/library_hero_2x.jpg",
	"header":"hash_e/header.jpg",
	"header_2x":"hash_f/header_2x.jpg",
	"library_logo":"hash_g/logo.png",
	"library_logo_2x":"hash_h/logo_2x.png"
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

func TestGetItemAssetsErrorWhenAPIFails(t *testing.T) {
	srv := browseServer(t, "", http.StatusInternalServerError)
	defer srv.Close()
	c := &Client{SearchBase: srv.URL, CDNBase: srv.URL, APIBase: srv.URL}
	if _, err := c.GetItemAssets(123); err == nil {
		t.Fatal("500 gave nil, want an error so the caller falls back to fixed names")
	}
}
