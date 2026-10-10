package compat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AWACYURL is AreWeAntiCheatYet's games.json (MIT). Hydra fetches it at most
// weekly, into the cache, and shows "AreWeAntiCheatYet" wherever it is used.
const AWACYURL = "https://raw.githubusercontent.com/AreWeAntiCheatYet/AreWeAntiCheatYet/master/games.json"

// AWACYAttribution is the credit shown with every anti-cheat fact.
const AWACYAttribution = "AreWeAntiCheatYet (MIT)"

// AntiCheatGame is one entry of games.json, the fields Hydra reads.
type AntiCheatGame struct {
	Name        string                     `json:"name"`
	Slug        string                     `json:"slug"`
	URL         string                     `json:"url"`
	Native      bool                       `json:"native"`
	Status      string                     `json:"status"`
	AntiCheats  []string                   `json:"anticheats"`
	DateChanged string                     `json:"dateChanged"`
	StoreIDs    map[string]json.RawMessage `json:"storeIds"`
}

// AntiCheatData is the parsed catalogue, with its join indexes.
type AntiCheatData struct {
	FetchedAt time.Time       `json:"fetched_at"`
	Games     []AntiCheatGame `json:"games"`

	bySteam map[string]int
	byEpic  map[string]int
}

// index builds the join tables. A Steam id is a string in the file; an Epic
// id is {namespace, slug}, and both the slug and namespace are keys.
func (d *AntiCheatData) index() {
	d.bySteam = map[string]int{}
	d.byEpic = map[string]int{}
	for i, g := range d.Games {
		if raw, ok := g.StoreIDs["steam"]; ok {
			var id string
			if json.Unmarshal(raw, &id) == nil && validAppID(id) {
				d.bySteam[id] = i
			}
		}
		if raw, ok := g.StoreIDs["epic"]; ok {
			var e struct {
				Namespace string `json:"namespace"`
				Slug      string `json:"slug"`
			}
			if json.Unmarshal(raw, &e) == nil {
				for _, k := range []string{e.Slug, e.Namespace} {
					if k != "" {
						d.byEpic[strings.ToLower(k)] = i
					}
				}
			}
		}
	}
}

// BySteam finds the game for a Steam AppID.
func (d *AntiCheatData) BySteam(appID string) (AntiCheatGame, bool) {
	if d == nil || d.bySteam == nil {
		return AntiCheatGame{}, false
	}
	i, ok := d.bySteam[appID]
	if !ok {
		return AntiCheatGame{}, false
	}
	return d.Games[i], true
}

// ByEpic finds the game for an Epic id (its slug or namespace, any case).
func (d *AntiCheatData) ByEpic(id string) (AntiCheatGame, bool) {
	if d == nil || d.byEpic == nil || id == "" {
		return AntiCheatGame{}, false
	}
	i, ok := d.byEpic[strings.ToLower(id)]
	if !ok {
		return AntiCheatGame{}, false
	}
	return d.Games[i], true
}

// awacyFile is the on-disk form, kept beside the cache.
type awacyFile struct {
	FetchedAt time.Time       `json:"fetched_at"`
	Games     []AntiCheatGame `json:"games"`
}

// AntiCheatStore holds the catalogue: loaded from its file, refreshed from
// GitHub when older than AWACYTTL. Refreshes are serialized.
type AntiCheatStore struct {
	Path  string
	URL   string
	Fetch *Fetcher
	Now   func() time.Time
	mu    sync.Mutex
	data  *AntiCheatData
}

// AWACYTTL is how old the catalogue may be before a refresh is tried.
const AWACYTTL = 7 * 24 * time.Hour

// Load returns the catalogue, reloading it from the file if needed. It does
// not fetch: see Refresh.
func (s *AntiCheatStore) Load() *AntiCheatData {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data != nil {
		return s.data
	}
	body, err := os.ReadFile(s.Path)
	if err != nil {
		return nil
	}
	var f awacyFile
	if json.Unmarshal(body, &f) != nil || len(f.Games) == 0 {
		return nil
	}
	s.data = &AntiCheatData{FetchedAt: f.FetchedAt, Games: f.Games}
	s.data.index()
	return s.data
}

// Stale says whether the catalogue should be refreshed now.
func (s *AntiCheatStore) Stale(d *AntiCheatData) bool {
	return d == nil || s.now().Sub(d.FetchedAt) >= AWACYTTL
}

// Refresh downloads games.json, saves it and returns the new catalogue. On an
// error the old catalogue stays in use.
func (s *AntiCheatStore) Refresh(ctx context.Context) (*AntiCheatData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	body, err := s.Fetch.Get(ctx, s.URL)
	if err != nil {
		return nil, err
	}
	var games []AntiCheatGame
	if err := json.Unmarshal(body, &games); err != nil {
		return nil, fmt.Errorf("games.json is unreadable: %w", err)
	}
	if len(games) == 0 {
		return nil, fmt.Errorf("games.json is empty")
	}
	d := &AntiCheatData{FetchedAt: s.now(), Games: games}
	d.index()
	if err := s.save(d); err != nil {
		return nil, err
	}
	s.data = d
	return d, nil
}

func (s *AntiCheatStore) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *AntiCheatStore) save(d *AntiCheatData) error {
	if s.Path == "" {
		return nil
	}
	body, err := json.Marshal(awacyFile{FetchedAt: d.FetchedAt, Games: d.Games})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}
