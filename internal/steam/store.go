package steam

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"platypus/internal/locale"
	"platypus/internal/sanitize"
	"platypus/internal/types"
)

// StoreClient fetches public app metadata from the Steam Store API and caches
// it for the session (the endpoint is rate limited to ~200 requests / 5 min).
// Base overrides the store endpoint (default DefaultStoreBase) and HTTPClient
// overrides the transport; tests point both at httptest. Locale is the BCP 47
// tag the store answers in (issue #39, phase 4); empty means English. The
// cache keys on app id + Steam language so French and English never mix.
type StoreClient struct {
	http       *http.Client
	HTTPClient *http.Client
	Base       string
	// Locale is the BCP 47 tag for store lookups ("fr-FR", "fr", "en").
	Locale string
	mu         sync.Mutex
	cache      map[string]*storeApp
}

// DefaultStoreBase serves the public appdetails JSON.
const DefaultStoreBase = "https://store.steampowered.com"

func NewStoreClient() *StoreClient {
	return &StoreClient{http: &http.Client{Timeout: 8 * time.Second}, cache: map[string]*storeApp{}}
}

func (c *StoreClient) storeBase() string {
	if c != nil && strings.TrimSpace(c.Base) != "" {
		return strings.TrimSuffix(strings.TrimSpace(c.Base), "/")
	}
	return DefaultStoreBase
}

// steamLang is the store's `l=` value for the client's locale.
func (c *StoreClient) steamLang() string {
	if c == nil || strings.TrimSpace(c.Locale) == "" {
		return "english"
	}
	return locale.SteamLanguage(c.Locale)
}

func (c *StoreClient) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	if c != nil && c.http != nil {
		return c.http
	}
	return &http.Client{Timeout: 8 * time.Second}
}

type storeApp struct {
	ShortDescription    string `json:"short_description"`
	DetailedDescription string `json:"detailed_description"`
	AboutTheGame        string `json:"about_the_game"`
	SupportedLanguages  string `json:"supported_languages"`
	ControllerSupport   string `json:"controller_support"`
	RequiredAge         any    `json:"required_age"`
	Website             string `json:"website"`
	HeaderImage         string `json:"header_image"`
	Developers          []string
	Publishers          []string
	Genres              []struct {
		Description string `json:"description"`
	} `json:"genres"`
	Categories []struct {
		ID          int    `json:"id"`
		Description string `json:"description"`
	} `json:"categories"`
	ReleaseDate struct {
		Date string `json:"date"`
	} `json:"release_date"`
	Screenshots []struct {
		Thumb string `json:"path_thumbnail"`
		Full  string `json:"path_full"`
	} `json:"screenshots"`
	Movies []struct {
		Name      string            `json:"name"`
		Thumbnail string            `json:"thumbnail"`
		MP4       map[string]string `json:"mp4"`
		WebM      map[string]string `json:"webm"`
	} `json:"movies"`
	PcRequirements    any `json:"pc_requirements"`
	LinuxRequirements any `json:"linux_requirements"`
	Achievements      struct {
		Total       int `json:"total"`
		Highlighted []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"highlighted"`
	} `json:"achievements"`
}

func (c *StoreClient) app(appID string) (*storeApp, error) {
	lang := c.steamLang()
	key := appID + "\x00" + lang
	c.mu.Lock()
	if c.cache == nil {
		c.cache = map[string]*storeApp{}
	}
	if a, ok := c.cache[key]; ok {
		c.mu.Unlock()
		return a, nil
	}
	c.mu.Unlock()

	url := fmt.Sprintf("%s/api/appdetails?appids=%s&l=%s", c.storeBase(), appID, lang)
	resp, err := c.httpClient().Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("steam store returned status %d", resp.StatusCode)
	}
	var body map[string]struct {
		Success bool     `json:"success"`
		Data    storeApp `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	entry, ok := body[appID]
	var a *storeApp
	if ok && entry.Success {
		a = &entry.Data
	} else {
		a = &storeApp{} // delisted or private: cache the miss too
	}
	c.mu.Lock()
	if c.cache == nil {
		c.cache = map[string]*storeApp{}
	}
	c.cache[key] = a
	c.mu.Unlock()
	return a, nil
}

var tagRe = regexp.MustCompile(`<[^>]+>`)

// Details merges Store API metadata with the user's local library cache.
// Local achievement progress wins; the Store list is a fallback that has no
// unlock state.
func (c *StoreClient) Details(root, appID string) (*types.GameDetails, error) {
	d := &types.GameDetails{}
	userDir := ActiveUserDir(root)
	if userDir != "" {
		d.Achievements, d.Summary = LocalLibraryData(userDir, appID)
	}

	a, err := c.app(appID)
	if err != nil {
		// Offline: local data alone is still useful.
		if d.Achievements != nil || d.Summary != "" {
			return d, nil
		}
		return nil, err
	}
	if d.Summary == "" {
		d.Summary = strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(a.ShortDescription, "")))
	}
	for _, s := range a.Screenshots {
		if len(d.Screenshots) >= 30 {
			break
		}
		if u := sanitize.MediaURL(s.Full); u != "" {
			d.Screenshots = append(d.Screenshots, u)
		} else if u := sanitize.MediaURL(s.Thumb); u != "" {
			d.Screenshots = append(d.Screenshots, u)
		}
	}
	for _, g := range a.Genres {
		d.Genres = append(d.Genres, g.Description)
	}
	d.Developers = a.Developers
	d.Publishers = a.Publishers
	d.ReleaseDate = a.ReleaseDate.Date
	// Richer facts (protocol 1.12): sanitized HTML, validated URLs, bounded lists.
	desc := a.DetailedDescription
	if desc == "" || (a.AboutTheGame != "" && len([]rune(a.AboutTheGame)) > len([]rune(desc))) {
		if a.AboutTheGame != "" {
			desc = a.AboutTheGame
		}
	}
	if desc != "" {
		if h := sanitize.HTML(desc); h != "" {
			r := []rune(h)
			if len(r) > 200_000 {
				h = string(r[:200_000])
			}
			d.DescriptionHTML = h
		}
	}
	d.Website = sanitize.HTTPSURL(a.Website)
	d.StoreURL = "https://store.steampowered.com/app/" + appID + "/"
	d.Languages = parseLanguages(a.SupportedLanguages)
	if c := strings.ToLower(strings.TrimSpace(a.ControllerSupport)); c == "full" || c == "partial" {
		d.Controller = c
	}
	if req := requirementsOf(a.PcRequirements, a.LinuxRequirements); req != nil {
		d.Requirements = req
	}
	if age := requiredAge(a.RequiredAge); age != "" {
		d.AgeRating = age
	}
	d.Tags = sanitize.Strings(categoryNames(a.Categories), 100, 200)
	for _, v := range videosOf(a.Movies) {
		if len(d.Videos) >= 20 {
			break
		}
		d.Videos = append(d.Videos, v)
	}
	if d.Achievements == nil && a.Achievements.Total > 0 {
		ach := &types.Achievements{Total: a.Achievements.Total, Unlocked: -1}
		for _, h := range a.Achievements.Highlighted {
			ach.List = append(ach.List, types.Achievement{Name: strings.TrimSpace(h.Name), Icon: h.Path})
		}
		d.Achievements = ach
	}
	// The store page for the game (issue #17): the game page offers it as
	// "Open in store".
	if appID != "" {
		d.StoreURL = "https://store.steampowered.com/app/" + appID + "/"
	}
	return d, nil
}

// SteamCloudCategoryID is the appdetails category id for Steam Cloud.
const SteamCloudCategoryID = 23

// CachedHasSteamCloud reports whether the cached appdetails for appID include
// Steam Cloud (category 23), using only what the client already fetched: no
// store round trip. ok is false when nothing is cached for the app (unknown
// means false for callers).
func (c *StoreClient) CachedHasSteamCloud(appID string) (has, ok bool) {
	if c == nil || appID == "" {
		return false, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, a := range c.cache {
		if key != appID && len(key) <= len(appID) {
			continue
		}
		if key != appID && key[:len(appID)+1] != appID+"\x00" {
			continue
		}
		if a == nil {
			continue
		}
		ok = true
		for _, cat := range a.Categories {
			if cat.ID == SteamCloudCategoryID {
				return true, true
			}
		}
	}
	return false, ok
}

// parseLanguages turns the Store's supported_languages HTML ("English<strong>*</strong>,
// French, …") into plain language names.
func parseLanguages(raw string) []string {
	if raw == "" {
		return nil
	}
	text := html.UnescapeString(tagRe.ReplaceAllString(strings.ReplaceAll(raw, "<br", ",<br"), " "))
	text = strings.ReplaceAll(text, "*", " ")
	parts := strings.Split(text, ",")
	out := []string{}
	seen := map[string]bool{}
	for _, p := range parts {
		p = strings.Join(strings.Fields(strings.TrimSpace(p)), " ")
		if p == "" || seen[p] || len(out) >= 100 {
			if p != "" {
				seen[p] = true
			}
			continue
		}
		if len([]rune(p)) > 60 {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// requirementsOf merges pc and linux requirements ({minimum, recommended} as
// HTML strings, or false when absent) into sanitized requirements.
func requirementsOf(pc, linux any) *types.GameRequirements {
	min, rec := requirementSide(pc)
	if lmin, lrec := requirementSide(linux); lmin != "" {
		if min == "" {
			min = lmin
		}
		if rec == "" {
			rec = lrec
		}
	}
	if min == "" && rec == "" {
		return nil
	}
	return &types.GameRequirements{Minimum: min, Recommended: rec}
}

func requirementSide(v any) (min, rec string) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", ""
	}
	if s, ok := m["minimum"].(string); ok && s != "" {
		if h := sanitize.HTML(s); h != "" {
			r := []rune(h)
			if len(r) > 50_000 {
				h = string(r[:50_000])
			}
			min = h
		}
	}
	if s, ok := m["recommended"].(string); ok && s != "" {
		if h := sanitize.HTML(s); h != "" {
			r := []rune(h)
			if len(r) > 50_000 {
				h = string(r[:50_000])
			}
			rec = h
		}
	}
	return min, rec
}

// requiredAge renders the Store's required_age (number or numeric string;
// 0 means no restriction) as "17+" style text, "" when none.
func requiredAge(v any) string {
	var n int
	switch t := v.(type) {
	case float64:
		n = int(t)
	case int:
		n = t
	case string:
		n, _ = strconv.Atoi(strings.TrimSpace(t))
	}
	if n <= 0 {
		return ""
	}
	if n > 21 {
		n = 21
	}
	return strconv.Itoa(n) + "+"
}

func categoryNames(in []struct {
	ID          int    `json:"id"`
	Description string `json:"description"`
}) []string {
	out := make([]string, 0, len(in))
	for _, c := range in {
		out = append(out, c.Description)
	}
	return out
}

// videosOf picks playable files from the Store's movies: mp4 480/max or
// webm first; HLS-only entries are skipped.
func videosOf(movies []struct {
	Name      string            `json:"name"`
	Thumbnail string            `json:"thumbnail"`
	MP4       map[string]string `json:"mp4"`
	WebM      map[string]string `json:"webm"`
}) []types.GameVideo {
	out := []types.GameVideo{}
	for _, m := range movies {
		u, mime := pickMovie(m.MP4, m.WebM)
		if u == "" {
			continue
		}
		if u = sanitize.MediaURL(u); u == "" {
			continue
		}
		out = append(out, types.GameVideo{
			URL: u, Thumbnail: sanitize.MediaURL(m.Thumbnail),
			Title: sanitize.Clip(m.Name, 200), MIME: mime,
		})
	}
	return out
}

func pickMovie(mp4, webm map[string]string) (string, string) {
	if u := mp4["480"]; u != "" {
		return u, "video/mp4"
	}
	if u := mp4["max"]; u != "" {
		return u, "video/mp4"
	}
	for _, v := range mp4 {
		if v != "" {
			return v, "video/mp4"
		}
	}
	if u := webm["480"]; u != "" {
		return u, "video/webm"
	}
	if u := webm["max"]; u != "" {
		return u, "video/webm"
	}
	for _, v := range webm {
		if v != "" {
			return v, "video/webm"
		}
	}
	return "", ""
}
