package steam

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"platypus/internal/types"
)

// StoreClient fetches public app metadata from the Steam Store API and caches
// it for the session (the endpoint is rate limited to ~200 requests / 5 min).
type StoreClient struct {
	http  *http.Client
	mu    sync.Mutex
	cache map[string]*storeApp
}

func NewStoreClient() *StoreClient {
	return &StoreClient{http: &http.Client{Timeout: 8 * time.Second}, cache: map[string]*storeApp{}}
}

type storeApp struct {
	ShortDescription string `json:"short_description"`
	HeaderImage      string `json:"header_image"`
	Developers       []string
	Publishers       []string
	Genres           []struct {
		Description string `json:"description"`
	} `json:"genres"`
	ReleaseDate struct {
		Date string `json:"date"`
	} `json:"release_date"`
	Screenshots []struct {
		Thumb string `json:"path_thumbnail"`
		Full  string `json:"path_full"`
	} `json:"screenshots"`
	Achievements struct {
		Total       int `json:"total"`
		Highlighted []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"highlighted"`
	} `json:"achievements"`
}

func (c *StoreClient) app(appID string) (*storeApp, error) {
	c.mu.Lock()
	if a, ok := c.cache[appID]; ok {
		c.mu.Unlock()
		return a, nil
	}
	c.mu.Unlock()

	url := fmt.Sprintf("https://store.steampowered.com/api/appdetails?appids=%s&l=english", appID)
	resp, err := c.http.Get(url)
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
	c.cache[appID] = a
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
		d.Screenshots = append(d.Screenshots, s.Full)
	}
	for _, g := range a.Genres {
		d.Genres = append(d.Genres, g.Description)
	}
	d.Developers = a.Developers
	d.Publishers = a.Publishers
	d.ReleaseDate = a.ReleaseDate.Date
	if d.Achievements == nil && a.Achievements.Total > 0 {
		ach := &types.Achievements{Total: a.Achievements.Total, Unlocked: -1}
		for _, h := range a.Achievements.Highlighted {
			ach.List = append(ach.List, types.Achievement{Name: strings.TrimSpace(h.Name), Icon: h.Path})
		}
		d.Achievements = ach
	}
	return d, nil
}
