// Package onlineart finds cover and banner art for sideloaded games on the
// web (issue #36). The only source is Steam's public store search, which
// needs no key and no login, plus the Steam CDN images for one app id. No
// other source, no API keys, no scraping beyond these JSON and image
// endpoints. The endpoint bases are injectable so tests use httptest; tests
// must never hit the network.
package onlineart

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

const (
	// DefaultSearchBase is Steam's storefront, serving the public search and
	// app-details JSON.
	DefaultSearchBase = "https://store.steampowered.com"
	// DefaultCDNBase serves per-app images:
	// /store_item_assets/steam/apps/<id>/{library_600x900.jpg,library_hero.jpg,header.jpg,logo.png}.
	DefaultCDNBase = "https://shared.steamstatic.com"
	// DefaultAPIBase serves the keyless store-browse API (IStoreBrowseService),
	// which names the real hashed asset paths newer apps keep.
	DefaultAPIBase = "https://api.steampowered.com"
	// Timeout bounds one lookup or download.
	Timeout = 15 * time.Second
	// MaxImageBytes caps one downloaded image.
	MaxImageBytes = 10 << 20
	// MaxCandidates caps one search answer.
	MaxCandidates = 8
)

// Candidate is one search hit: a Steam app the user can apply the art of.
type Candidate struct {
	AppID    int    `json:"app_id"`
	Name     string `json:"name"`
	ThumbURL string `json:"thumb_url"`
}

// Client looks art up online. The zero value is not usable; use New or set
// the fields directly in tests. SearchBase, CDNBase and APIBase name the
// endpoints (defaults DefaultSearchBase, DefaultCDNBase, DefaultAPIBase);
// HTTP does the requests (default an http.Client with Timeout).
type Client struct {
	HTTP       *http.Client
	SearchBase string
	CDNBase    string
	APIBase    string
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: Timeout}
}

func (c *Client) searchBase() string {
	if c.SearchBase != "" {
		return c.SearchBase
	}
	return DefaultSearchBase
}

func (c *Client) cdnBase() string {
	if c.CDNBase != "" {
		return c.CDNBase
	}
	return DefaultCDNBase
}

func (c *Client) apiBase() string {
	if c.APIBase != "" {
		return c.APIBase
	}
	return DefaultAPIBase
}

// New returns a Client for the live Steam endpoints.
func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: Timeout}}
}

type searchItem struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	TinyImage string `json:"tiny_image"`
}

// Search asks the public store search for title and returns up to
// MaxCandidates hits, in the store's order.
func (c *Client) Search(title string) ([]Candidate, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("title is empty")
	}
	u := c.searchBase() + "/api/storesearch/?" + url.Values{
		"term": {title}, "l": {"english"}, "cc": {"US"},
	}.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam search returned status %d", resp.StatusCode)
	}
	var body struct {
		Items []searchItem `json:"items"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		return nil, fmt.Errorf("steam search: %w", err)
	}
	out := []Candidate{}
	for _, it := range body.Items {
		if it.ID <= 0 || strings.TrimSpace(it.Name) == "" {
			continue
		}
		out = append(out, Candidate{AppID: it.ID, Name: strings.TrimSpace(it.Name), ThumbURL: it.TinyImage})
		if len(out) >= MaxCandidates {
			break
		}
	}
	return out, nil
}

// NormalizeTitle lowers a title for comparison: lowercase, no punctuation,
// no ™®©, single spaces. "Hollow Knight™" and "hollow-knight" both become
// "hollow knight".
func NormalizeTitle(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	spaces := true // collapse leading runs too
	for _, r := range s {
		switch {
		case r == '™' || r == '®' || r == '©':
			// dropped
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			spaces = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '/' || r == ':' || r == '.' || r == ',' || r == '\'' || r == '"' || r == '!' || r == '?' || r == '(' || r == ')' || r == '[' || r == ']' || r == '+' || r == '&' || r == '*':
			if !spaces {
				b.WriteByte(' ')
				spaces = true
			}
		default:
			if !spaces {
				b.WriteByte(' ')
				spaces = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// ExactMatch returns the one candidate whose normalized name equals the
// normalized title, or nil when zero or several match. The auto path applies
// art only on an exact single match; anything ambiguous does nothing.
func ExactMatch(title string, all []Candidate) *Candidate {
	want := NormalizeTitle(title)
	var hit *Candidate
	n := 0
	for i := range all {
		if NormalizeTitle(all[i].Name) == want {
			n++
			hit = &all[i]
		}
	}
	if n == 1 {
		return hit
	}
	return nil
}

// CoverURL, HeroURL, HeaderURL and LogoURL name one app's images on the CDN.
func (c *Client) CoverURL(appID int) string {
	return fmt.Sprintf("%s/store_item_assets/steam/apps/%d/library_600x900.jpg", c.cdnBase(), appID)
}
func (c *Client) HeroURL(appID int) string {
	return fmt.Sprintf("%s/store_item_assets/steam/apps/%d/library_hero.jpg", c.cdnBase(), appID)
}
func (c *Client) HeaderURL(appID int) string {
	return fmt.Sprintf("%s/store_item_assets/steam/apps/%d/header.jpg", c.cdnBase(), appID)
}
func (c *Client) LogoURL(appID int) string {
	return fmt.Sprintf("%s/store_item_assets/steam/apps/%d/logo.png", c.cdnBase(), appID)
}

// StoreAssets is one app's real art URLs from the store-browse API, plus its
// store name. Empty URLs mean the app has no such art. Newer apps keep their
// assets under hashed paths, so the fixed CDN names above all 404 for them;
// GetItems names the real paths.
type StoreAssets struct {
	Name      string
	CoverURL  string
	BannerURL string
	LogoURL   string
}

// GetItemAssets asks the keyless store-browse API for the app's real asset
// paths and resolves the cover, banner and logo URLs, preferring the _2x
// file when the app has one:
//
//	cover: library_capsule_2x, else library_capsule;
//	banner: library_hero_2x, else library_hero, else header_2x, else header;
//	logo: library_logo_2x, else library_logo.
//
// The full URL is the CDN base + "/store_item_assets/" + the asset_url_format
// with ${FILENAME} replaced by the hashed file path. An error means the API
// failed; empty URLs with a nil error mean it answered but lists no usable
// art (the caller falls back to the fixed CDN names).
func (c *Client) GetItemAssets(appID int) (StoreAssets, error) {
	if appID <= 0 {
		return StoreAssets{}, fmt.Errorf("a Steam app id is a number")
	}
	input, err := json.Marshal(map[string]any{
		"ids":     []any{map[string]any{"appid": appID}},
		"context": map[string]any{"language": "english", "country_code": "US"},
		"data_request": map[string]any{
			"include_assets": true,
		},
	})
	if err != nil {
		return StoreAssets{}, err
	}
	u := c.apiBase() + "/IStoreBrowseService/GetItems/v1/?" + url.Values{
		"input_json": {string(input)},
	}.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return StoreAssets{}, err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return StoreAssets{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return StoreAssets{}, fmt.Errorf("steam GetItems returned status %d", resp.StatusCode)
	}
	var body struct {
		Response struct {
			StoreItems []struct {
				Name   string            `json:"name"`
				Assets map[string]string `json:"assets"`
			} `json:"store_items"`
		} `json:"response"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		return StoreAssets{}, fmt.Errorf("steam GetItems: %w", err)
	}
	if len(body.Response.StoreItems) == 0 {
		return StoreAssets{}, nil
	}
	item := body.Response.StoreItems[0]
	assets := item.Assets
	if len(assets) == 0 {
		return StoreAssets{Name: strings.TrimSpace(item.Name)}, nil
	}
	format, ok := assets["asset_url_format"]
	if !ok || strings.TrimSpace(format) == "" {
		return StoreAssets{Name: strings.TrimSpace(item.Name)}, nil
	}
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(assets[k]); v != "" {
				return v
			}
		}
		return ""
	}
	resolve := func(filename string) string {
		if filename == "" {
			return ""
		}
		path := strings.ReplaceAll(format, "${FILENAME}", filename)
		base := strings.TrimSuffix(c.cdnBase(), "/")
		path = strings.TrimPrefix(path, "/")
		return base + "/store_item_assets/" + path
	}
	cover := resolve(pick("library_capsule_2x", "library_capsule"))
	banner := resolve(pick("library_hero_2x", "library_hero", "header_2x", "header"))
	logo := resolve(pick("library_logo_2x", "library_logo"))
	return StoreAssets{
		Name:      strings.TrimSpace(item.Name),
		CoverURL:  cover,
		BannerURL: banner,
		LogoURL:   logo,
	}, nil
}

// imageKind sniffs the first bytes of a download: PNG, JPEG or WebP. The
// type comes from the content, never from the headers or URL.
func imageKind(head []byte) string {
	switch {
	case bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n")):
		return "png"
	case bytes.HasPrefix(head, []byte("\xff\xd8\xff")):
		return "jpeg"
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return "webp"
	}
	return ""
}

// Download fetches one image URL. A 404 returns (nil, nil): the kind is
// simply absent (e.g. no hero art). Anything else that fails, is too large
// or is not a PNG/JPEG/WebP image is an error.
func (c *Client) Download(rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("art returned status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxImageBytes {
		return nil, fmt.Errorf("the image is larger than %d MB", MaxImageBytes>>20)
	}
	if len(data) == 0 || imageKind(data) == "" {
		return nil, fmt.Errorf("that is not a PNG, JPEG or WebP image")
	}
	return data, nil
}

// Banner downloads the app's wide art: library_hero.jpg, falling back to
// header.jpg. (nil, nil) means the app has neither.
func (c *Client) Banner(appID int) ([]byte, error) {
	data, err := c.Download(c.HeroURL(appID))
	if err != nil || data != nil {
		return data, err
	}
	return c.Download(c.HeaderURL(appID))
}

// AppInfo asks the public app-details endpoint for the app's store name and
// release year. ("", 0, nil) means unknown: delisted or absent. It is only
// the cheap extra of an art apply, never a lookup of its own.
func (c *Client) AppInfo(appID int) (string, int, error) {
	u := fmt.Sprintf("%s/api/appdetails?appids=%d&l=english", c.searchBase(), appID)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", 0, err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("steam appdetails returned status %d", resp.StatusCode)
	}
	var body map[string]struct {
		Success bool `json:"success"`
		Data    struct {
			Name        string `json:"name"`
			ReleaseDate struct {
				Date string `json:"date"`
			} `json:"release_date"`
		} `json:"data"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		return "", 0, fmt.Errorf("steam appdetails: %w", err)
	}
	for _, e := range body {
		if !e.Success {
			return "", 0, nil
		}
		return strings.TrimSpace(e.Data.Name), parseYear(e.Data.ReleaseDate.Date), nil
	}
	return "", 0, nil
}

// parseYear takes the first plausible release year (1970..next year) out of
// a store date like "12 Dec, 2023", "2023" or "Q1 2024". 0 means none found.
func parseYear(date string) int {
	now := time.Now().Year()
	year := 0
	num := 0
	digits := 0
	flush := func() {
		if digits == 4 && num >= 1970 && num <= now+1 {
			if year == 0 {
				year = num
			}
		}
		num, digits = 0, 0
	}
	for _, r := range date {
		if r >= '0' && r <= '9' {
			num = num*10 + int(r-'0')
			digits++
			if digits > 4 {
				num, digits = 0, 0
			}
			continue
		}
		flush()
	}
	flush()
	return year
}
