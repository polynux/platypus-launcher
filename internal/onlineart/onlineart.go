// Package onlineart finds cover and banner art for sideloaded games on the
// web (issues #36, art fallbacks). Steam's public store search is the first
// source, which needs no key and no login, plus the Steam CDN images for one
// app id. GOG's catalog and Lutris (mirroring IGDB covers) are keyless
// fallbacks, used only when Steam is thin and on demand. No other source, no
// API keys, no scraping beyond these JSON and image endpoints (igdb.com
// itself is never scraped: it sits behind a Cloudflare challenge). The
// endpoint bases are injectable so tests use httptest; tests must never hit
// the network.
package onlineart

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
	// DefaultGOGBase serves the keyless GOG catalog search.
	DefaultGOGBase = "https://catalog.gog.com"
	// DefaultGOGInfoBase serves the keyless GOG product details.
	DefaultGOGInfoBase = "https://api.gog.com"
	// DefaultLutrisBase serves the keyless Lutris games API (which mirrors
	// IGDB covers).
	DefaultLutrisBase = "https://lutris.net"
	// DefaultIGDBImageBase serves the larger IGDB cover copies behind the
	// Lutris coverart ids.
	DefaultIGDBImageBase = "https://images.igdb.com"
	// ArtContact is where the art sources can reach Platypus's maintainers;
	// every request to GOG, Lutris or IGDB names it in the User-Agent.
	ArtContact = "https://github.com/polynux/platypus-launcher"
	// Timeout bounds one lookup or download.
	Timeout = 15 * time.Second
	// MaxImageBytes caps one downloaded image.
	MaxImageBytes = 10 << 20
	// MaxCandidates caps one search answer.
	MaxCandidates = 8
)

// Candidate is one search hit: art the user can apply. Source names where
// the art comes from: "steam", "gog" or "lutris" (Lutris mirrors IGDB
// covers). ID is the source's id: the Steam app id as a string, the GOG
// product id, or the Lutris slug. AppID keeps the Steam app id for backward
// compatibility (0 for non-Steam hits). Year is the release year when known.
// CoverURL, BannerURL and LogoURL carry the source's image addresses (GOG:
// coverVertical, coverHorizontal, logo; Lutris: the upgraded IGDB cover with
// CoverFallbackURL the lutris.net coverart URL); Steam hits resolve theirs
// on apply. SteamAppID is a mapped Steam app id from Lutris provider_games,
// 0 when the hit maps to none. Developers, Publishers and Genres carry the
// GOG catalog's facts for the info lookup (issue #38); empty for other
// sources.
type Candidate struct {
	Source           string   `json:"source"`
	ID               string   `json:"id"`
	AppID            int      `json:"app_id"`
	Name             string   `json:"name"`
	Year             int      `json:"year,omitempty"`
	ThumbURL         string   `json:"thumb_url"`
	CoverURL         string   `json:"cover_url,omitempty"`
	BannerURL        string   `json:"banner_url,omitempty"`
	LogoURL          string   `json:"logo_url,omitempty"`
	CoverFallbackURL string   `json:"cover_fallback_url,omitempty"`
	SteamAppID       int      `json:"steam_app_id,omitempty"`
	Developers       []string `json:"developers,omitempty"`
	Publishers       []string `json:"publishers,omitempty"`
	Genres           []string `json:"genres,omitempty"`
}

// Client looks art up online. The zero value is not usable; use New or set
// the fields directly in tests. SearchBase, CDNBase and APIBase name the
// Steam endpoints (defaults DefaultSearchBase, DefaultCDNBase,
// DefaultAPIBase); GOGBase, LutrisBase and IGDBBase name the fallback
// endpoints (defaults DefaultGOGBase, DefaultLutrisBase,
// DefaultIGDBImageBase); Version is the Platypus version named in the
// User-Agent (default "1.0"); HTTP does the requests (default an http.Client
// with Timeout).
type Client struct {
	HTTP        *http.Client
	SearchBase  string
	CDNBase     string
	APIBase     string
	GOGBase     string
	GOGInfoBase string
	LutrisBase  string
	IGDBBase    string
	Version     string
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

func (c *Client) gogBase() string {
	if c.GOGBase != "" {
		return c.GOGBase
	}
	return DefaultGOGBase
}

func (c *Client) gogInfoBase() string {
	if c.GOGInfoBase != "" {
		return c.GOGInfoBase
	}
	return DefaultGOGInfoBase
}

func (c *Client) lutrisBase() string {
	if c.LutrisBase != "" {
		return c.LutrisBase
	}
	return DefaultLutrisBase
}

func (c *Client) igdbBase() string {
	if c.IGDBBase != "" {
		return c.IGDBBase
	}
	return DefaultIGDBImageBase
}

// userAgent names Platypus and its contact on every request to GOG, Lutris
// or IGDB: "Platypus/<version> (+<contact>)".
func (c *Client) userAgent() string {
	v := strings.TrimSpace(c.Version)
	if v == "" {
		v = "1.0"
	}
	return "Platypus/" + v + " (+" + ArtContact + ")"
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
	req.Header.Set("User-Agent", c.userAgent())
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
		out = append(out, Candidate{Source: "steam", ID: strconv.Itoa(it.ID), AppID: it.ID, Name: strings.TrimSpace(it.Name), ThumbURL: it.TinyImage})
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
	req.Header.Set("User-Agent", c.userAgent())
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
				Name string `json:"name"`
				// Assets mixes strings (file paths) with numbers and bools
				// the lookup ignores (e.g. last_modified); decode raw and
				// read only the string values, so new or numeric fields
				// can never break the decode.
				Assets map[string]json.RawMessage `json:"assets"`
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
	assets := map[string]string{}
	for k, raw := range item.Assets {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			continue // a number, bool or object: not a file path
		}
		assets[k] = s
	}
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
	req.Header.Set("User-Agent", c.userAgent())
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
	req.Header.Set("User-Agent", c.userAgent())
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

// gogID is a GOG product id: the catalog serves it as a string.
type gogID string

// UnmarshalJSON accepts the string form the catalog serves (and a number,
// defensively, so a numeric id can never break the decode).
func (g *gogID) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*g = gogID(s)
		return nil
	}
	var n int64
	if err := json.Unmarshal(b, &n); err == nil {
		*g = gogID(strconv.FormatInt(n, 10))
		return nil
	}
	var f float64
	if err := json.Unmarshal(b, &f); err == nil {
		*g = gogID(strconv.FormatInt(int64(f), 10))
		return nil
	}
	return fmt.Errorf("gog product id is not a string: %s", strings.TrimSpace(string(b)))
}

type gogProduct struct {
	ID              gogID    `json:"id"`
	Title           string   `json:"title"`
	ReleaseDate     string   `json:"releaseDate"`
	CoverVertical   string   `json:"coverVertical"`
	CoverHorizontal string   `json:"coverHorizontal"`
	Logo            string   `json:"logo"`
	Developers      []string `json:"developers"`
	Publishers      []string `json:"publishers"`
	Genres          []struct {
		Name string `json:"name"`
	} `json:"genres"`
}

// gogCatalogFacts carries the catalog's developer, publisher and genre facts.
func gogCatalogFacts(p gogProduct) (dev, pub, genres []string) {
	dev = append([]string{}, p.Developers...)
	pub = append([]string{}, p.Publishers...)
	for _, g := range p.Genres {
		if s := strings.TrimSpace(g.Name); s != "" {
			genres = append(genres, s)
		}
	}
	return dev, pub, genres
}

// SearchGOG asks the keyless GOG catalog for title and returns up to
// MaxCandidates hits, in the catalog's order. Cover art is coverVertical,
// banner art coverHorizontal, logo art logo.
func (c *Client) SearchGOG(title string) ([]Candidate, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("title is empty")
	}
	u := strings.TrimSuffix(c.gogBase(), "/") + "/v1/catalog?" + url.Values{
		"limit": {"8"}, "query": {"like:" + title}, "locale": {"en-US"},
	}.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent())
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gog search returned status %d", resp.StatusCode)
	}
	var body struct {
		Products []gogProduct `json:"products"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		return nil, fmt.Errorf("gog search: %w", err)
	}
	out := []Candidate{}
	for _, p := range body.Products {
		id := strings.TrimSpace(string(p.ID))
		name := strings.TrimSpace(p.Title)
		if id == "" || name == "" {
			continue
		}
		cover := strings.TrimSpace(p.CoverVertical)
		banner := strings.TrimSpace(p.CoverHorizontal)
		logo := strings.TrimSpace(p.Logo)
		if cover == "" && banner == "" {
			continue // nothing to apply a cover or banner from
		}
		thumb := banner
		if thumb == "" {
			thumb = cover
		}
		dev, pub, genres := gogCatalogFacts(p)
		out = append(out, Candidate{
			Source: "gog", ID: id,
			Name: name, Year: parseYear(p.ReleaseDate), ThumbURL: thumb,
			CoverURL: cover, BannerURL: banner, LogoURL: logo,
			Developers: dev, Publishers: pub, Genres: genres,
		})
		if len(out) >= MaxCandidates {
			break
		}
	}
	return out, nil
}

// lutrisProvider names one store behind a Lutris hit.
type lutrisProvider struct {
	Service string `json:"service"`
	Slug    string `json:"slug"`
}

type lutrisResult struct {
	Name          string           `json:"name"`
	Slug          string           `json:"slug"`
	Year          int              `json:"year"`
	Coverart      string           `json:"coverart"`
	ProviderGames []lutrisProvider `json:"provider_games"`
	// The single-game endpoint names the Steam app directly.
	SteamID int `json:"steamid"`
}

// steamAppIDOf returns the hit's Steam app id from provider_games (or the
// single-game endpoint's steamid), 0 when it maps to no Steam app.
func steamAppIDOf(r lutrisResult) int {
	if r.SteamID > 0 {
		return r.SteamID
	}
	for _, p := range r.ProviderGames {
		if !strings.EqualFold(strings.TrimSpace(p.Service), "steam") {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(p.Slug))
		if err == nil && n > 0 {
			return n
		}
	}
	return 0
}

// lutrisImageID takes the IGDB image id out of a Lutris coverart URL of the
// form …/media/igdb/cover_big/<imageId>.jpg. "" means the coverart carries
// no image id (e.g. the generic /media/igdb/library_capsule.jpg): those are
// unreliable and skipped.
func lutrisImageID(raw string) string {
	const marker = "/media/igdb/cover_big/"
	i := strings.Index(raw, marker)
	if i < 0 {
		return ""
	}
	rest := raw[i+len(marker):]
	if q := strings.IndexAny(rest, "?#"); q >= 0 {
		rest = rest[:q]
	}
	if !strings.HasSuffix(rest, ".jpg") {
		return ""
	}
	id := strings.TrimSuffix(rest, ".jpg")
	if id == "" || strings.Contains(id, "/") {
		return ""
	}
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			continue
		}
		return ""
	}
	return id
}

// igdbCoverURL names the larger copy of an IGDB image id on IGDB's open CDN.
func (c *Client) igdbCoverURL(imageID string) string {
	return strings.TrimSuffix(c.igdbBase(), "/") + "/igdb/image/upload/t_cover_big_2x/" + imageID + ".jpg"
}

// lutrisCandidate builds the candidate for one Lutris hit. A hit whose
// coverart carries no image id is unreliable: it is skipped, unless it maps
// to a Steam app, in which case it is offered as a Steam candidate (no
// thumb: the generic art is never used).
func (c *Client) lutrisCandidate(r lutrisResult) (Candidate, bool) {
	name := strings.TrimSpace(r.Name)
	slug := strings.TrimSpace(r.Slug)
	if name == "" || slug == "" {
		return Candidate{}, false
	}
	steam := steamAppIDOf(r)
	if id := lutrisImageID(strings.TrimSpace(r.Coverart)); id != "" {
		cover := c.igdbCoverURL(id)
		return Candidate{
			Source: "lutris", ID: slug,
			Name: name, Year: r.Year, ThumbURL: cover,
			CoverURL: cover, CoverFallbackURL: strings.TrimSpace(r.Coverart),
			SteamAppID: steam,
		}, true
	}
	if steam > 0 {
		return Candidate{
			Source: "steam", ID: strconv.Itoa(steam), AppID: steam,
			Name: name, Year: r.Year, SteamAppID: steam,
		}, true
	}
	return Candidate{}, false
}

// SearchLutris asks the keyless Lutris games API (which mirrors IGDB covers)
// for title and returns up to MaxCandidates hits, in Lutris's order. Lutris
// gives covers only: banner_url is a 184x69 thumbnail and is never used.
func (c *Client) SearchLutris(title string) ([]Candidate, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("title is empty")
	}
	u := strings.TrimSuffix(c.lutrisBase(), "/") + "/api/games?" + url.Values{
		"search": {title},
	}.Encode()
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent())
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lutris search returned status %d", resp.StatusCode)
	}
	var body struct {
		Results []lutrisResult `json:"results"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		return nil, fmt.Errorf("lutris search: %w", err)
	}
	out := []Candidate{}
	for _, r := range body.Results {
		if cand, ok := c.lutrisCandidate(r); ok {
			out = append(out, cand)
			if len(out) >= MaxCandidates {
				break
			}
		}
	}
	return out, nil
}

// LutrisGame fetches one Lutris game by slug for an apply: the cover (the
// upgraded IGDB copy, falling back to the coverart URL) plus the mapped
// Steam app id, if any.
func (c *Client) LutrisGame(slug string) (Candidate, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return Candidate{}, fmt.Errorf("lutris slug is empty")
	}
	u := strings.TrimSuffix(c.lutrisBase(), "/") + "/api/games/" + url.PathEscape(slug)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return Candidate{}, err
	}
	req.Header.Set("User-Agent", c.userAgent())
	resp, err := c.http().Do(req)
	if err != nil {
		return Candidate{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Candidate{}, fmt.Errorf("lutris has no game %q", slug)
	}
	if resp.StatusCode != http.StatusOK {
		return Candidate{}, fmt.Errorf("lutris game returned status %d", resp.StatusCode)
	}
	var r lutrisResult
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := dec.Decode(&r); err != nil {
		return Candidate{}, fmt.Errorf("lutris game: %w", err)
	}
	if strings.TrimSpace(r.Slug) == "" {
		r.Slug = slug
	}
	cand, ok := c.lutrisCandidate(r)
	if !ok {
		return Candidate{}, fmt.Errorf("lutris has no cover art for %q", slug)
	}
	if cand.Source == "steam" {
		// A slug whose only value is its Steam mapping still applies: the
		// caller takes the Steam path.
		return cand, nil
	}
	return cand, nil
}

// SteamNeedsFallback says whether the fallbacks should be queried: Steam is
// thin (fewer than 3 hits) or names nothing like the title.
func SteamNeedsFallback(title string, steam []Candidate) bool {
	if len(steam) < 3 {
		return true
	}
	want := NormalizeTitle(title)
	for _, s := range steam {
		if s.Source != "" && s.Source != "steam" {
			continue
		}
		if NormalizeTitle(s.Name) == want {
			return false
		}
	}
	return true
}

// MergeCandidates merges Steam hits first, then GOG, then Lutris, dropping
// duplicates by normalised name + year (preferring Steam, then GOG, then
// Lutris) and capping the total at MaxCandidates.
func MergeCandidates(steam, gog, lutris []Candidate) []Candidate {
	out := []Candidate{}
	seen := map[string]bool{}
	for _, list := range [][]Candidate{steam, gog, lutris} {
		for _, cand := range list {
			if strings.TrimSpace(cand.Name) == "" {
				continue
			}
			key := NormalizeTitle(cand.Name) + "\x00" + strconv.Itoa(cand.Year)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, cand)
			if len(out) >= MaxCandidates {
				return out
			}
		}
	}
	return out
}

// GOGDetails is the product info for a sideloaded game's details (issue
// #38): the description HTML plus the release date, languages and store URL.
// Developers, publishers and genres come from the catalog search hit, not
// from this endpoint.
type GOGDetails struct {
	Title       string
	Slug        string
	ReleaseDate string
	Languages   []string
	StoreURL    string
	Lead        string
	Full        string
}

// GOGGame fetches one GOG product's description for the info lookup. The
// User-Agent names Platypus like the art search.
func (c *Client) GOGGame(id string) (GOGDetails, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return GOGDetails{}, fmt.Errorf("gog product id is empty")
	}
	u := strings.TrimSuffix(c.gogInfoBase(), "/") + "/products/" + url.PathEscape(id) + "?expand=description"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return GOGDetails{}, err
	}
	req.Header.Set("User-Agent", c.userAgent())
	resp, err := c.http().Do(req)
	if err != nil {
		return GOGDetails{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return GOGDetails{}, fmt.Errorf("gog has no product %q", id)
	}
	if resp.StatusCode != http.StatusOK {
		return GOGDetails{}, fmt.Errorf("gog product returned status %d", resp.StatusCode)
	}
	var body struct {
		Title       string            `json:"title"`
		Slug        string            `json:"slug"`
		ReleaseDate string            `json:"release_date"`
		Languages   map[string]string `json:"languages"`
		Links       struct {
			ProductCard string `json:"product_card"`
		} `json:"links"`
		Description struct {
			Lead string `json:"lead"`
			Full string `json:"full"`
		} `json:"description"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	if err := dec.Decode(&body); err != nil {
		return GOGDetails{}, fmt.Errorf("gog product: %w", err)
	}
	d := GOGDetails{
		Title: strings.TrimSpace(body.Title), Slug: strings.TrimSpace(body.Slug),
		ReleaseDate: strings.TrimSpace(body.ReleaseDate),
		StoreURL:    strings.TrimSpace(body.Links.ProductCard),
		Lead:        body.Description.Lead, Full: body.Description.Full,
	}
	for _, lang := range body.Languages {
		if s := strings.TrimSpace(lang); s != "" {
			d.Languages = append(d.Languages, s)
		}
	}
	return d, nil
}

// LutrisDetails is one Lutris game's info for a sideloaded game's details
// (issue #38): the plain-text description plus genres and year.
type LutrisDetails struct {
	Name        string
	Slug        string
	Year        int
	Description string
	Genres      []string
}

// LutrisInfo fetches one Lutris game by slug for the info lookup: the
// description (plain text), genres and year. The User-Agent names Platypus
// like the art search.
func (c *Client) LutrisInfo(slug string) (LutrisDetails, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return LutrisDetails{}, fmt.Errorf("lutris slug is empty")
	}
	u := strings.TrimSuffix(c.lutrisBase(), "/") + "/api/games/" + url.PathEscape(slug)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return LutrisDetails{}, err
	}
	req.Header.Set("User-Agent", c.userAgent())
	resp, err := c.http().Do(req)
	if err != nil {
		return LutrisDetails{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return LutrisDetails{}, fmt.Errorf("lutris has no game %q", slug)
	}
	if resp.StatusCode != http.StatusOK {
		return LutrisDetails{}, fmt.Errorf("lutris game returned status %d", resp.StatusCode)
	}
	var body struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Year        int    `json:"year"`
		Description string `json:"description"`
		Genres      []struct {
			Name string `json:"name"`
		} `json:"genres"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		return LutrisDetails{}, fmt.Errorf("lutris game: %w", err)
	}
	d := LutrisDetails{
		Name: strings.TrimSpace(body.Name), Slug: strings.TrimSpace(body.Slug),
		Year: body.Year, Description: strings.TrimSpace(body.Description),
	}
	if d.Slug == "" {
		d.Slug = slug
	}
	for _, g := range body.Genres {
		if s := strings.TrimSpace(g.Name); s != "" {
			d.Genres = append(d.Genres, s)
		}
	}
	return d, nil
}
