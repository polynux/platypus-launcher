package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"platypus/internal/plugins"
	"platypus/internal/types"
)

// compatFactRetry avoids asking a failing provider on every library load.
const compatFactRetry = 5 * time.Minute

// compatFactsCache stores provider answers per plugin and game, with in-flight batches.
type compatFactsCache struct {
	mu       sync.Mutex
	path     string
	facts    map[string]map[string]compatFactEntry
	inFlight map[string]bool
	pending  map[string]map[string]plugins.CompatLookupGame
}

// compatFactEntry keeps an answer's app id and expiry so overrides don't reuse stale ratings.
type compatFactEntry struct {
	Fact       plugins.CompatFact `json:"fact"`
	SteamAppID string             `json:"steamAppId,omitempty"`
	Expires    time.Time          `json:"expires"`
	RetryAt    time.Time          `json:"retryAt,omitempty"`
}

// compatFactFresh reports whether an answer or retry delay still covers this app id.
func compatFactFresh(e compatFactEntry, appID string, now time.Time) bool {
	if e.SteamAppID != appID {
		return false
	}
	return (e.Fact.ID != "" && now.Before(e.Expires)) || now.Before(e.RetryAt)
}

// openCompatFactsCache loads the persisted facts cache, falling back to an empty cache.
func openCompatFactsCache(path string) *compatFactsCache {
	c := &compatFactsCache{path: path, facts: map[string]map[string]compatFactEntry{}, inFlight: map[string]bool{}, pending: map[string]map[string]plugins.CompatLookupGame{}}
	b, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(b, &c.facts)
	}
	return c
}

// cachedCompatFact returns one provider's saved answer without starting a lookup.
func (a *App) cachedCompatFact(pluginID, gameID string) (plugins.CompatFact, bool) {
	if a.compatFacts == nil {
		return plugins.CompatFact{}, false
	}
	a.compatFacts.mu.Lock()
	defer a.compatFacts.mu.Unlock()
	e, ok := a.compatFacts.facts[pluginID][gameID]
	return e.Fact, ok && len(e.Fact.ID) != 0
}

// cachedProviderFact combines current-app-id answers across compatibility providers.
func (a *App) cachedProviderFact(gameID, appID string) (plugins.CompatFact, bool) {
	combined := plugins.CompatFact{ID: gameID, Badges: []plugins.CompatBadge{}}
	found := false
	if a.compatFacts == nil {
		return combined, false
	}
	a.compatFacts.mu.Lock()
	defer a.compatFacts.mu.Unlock()
	for _, games := range a.compatFacts.facts {
		entry, ok := games[gameID]
		if !ok || entry.Fact.ID == "" {
			continue
		}
		if entry.SteamAppID != appID {
			continue
		}
		found = true
		combined.Badges = append(combined.Badges, entry.Fact.Badges...)
		combined.Notes = append(combined.Notes, entry.Fact.Notes...)
		combined.Links = append(combined.Links, entry.Fact.Links...)
	}
	return combined, found
}

// requestCompatFacts queues stale games in one background batch per provider.
func (a *App) requestCompatFacts(games []types.Game, hints func(types.Game) *types.GameDetails) {
	if len(games) == 0 || a.compatFacts == nil || a.plugins == nil || !a.netOnline() || (a.settings != nil && !a.settings.RatingsOnline()) {
		return
	}
	now := time.Now()
	for _, plugin := range a.plugins.Plugins() {
		if plugin.Manifest.IsData() || !slices.Contains(plugin.Manifest.Capabilities, "compat-facts") {
			continue
		}
		pluginID := plugin.Manifest.ID
		batch := make([]plugins.CompatLookupGame, 0, len(games))
		for _, g := range games {
			d := (*types.GameDetails)(nil)
			if hints != nil {
				d = hints(g)
			}
			appID, _ := a.ratingAppID(g, d)
			a.compatFacts.mu.Lock()
			e := a.compatFacts.facts[pluginID][g.ID]
			fresh := compatFactFresh(e, appID, now)
			a.compatFacts.mu.Unlock()
			if fresh {
				continue
			}
			store := string(g.Store)
			if store == "" {
				store = sourceOf(g)
			}
			item := plugins.CompatLookupGame{ID: g.ID, Store: store, StoreAppID: g.StoreAppID, SteamAppID: appID, InstallDir: g.InstallDir}
			if d != nil {
				item.Platforms = d.Platforms
			}
			batch = append(batch, item)
		}
		if len(batch) == 0 {
			continue
		}
		a.compatFacts.mu.Lock()
		if a.compatFacts.inFlight[pluginID] {
			if a.compatFacts.pending[pluginID] == nil {
				a.compatFacts.pending[pluginID] = map[string]plugins.CompatLookupGame{}
			}
			for _, g := range batch {
				a.compatFacts.pending[pluginID][g.ID] = g
			}
			a.compatFacts.mu.Unlock()
			continue
		}
		a.compatFacts.inFlight[pluginID] = true
		a.compatFacts.mu.Unlock()
		go a.refreshCompatFacts(pluginID, batch)
	}
}

// refreshCompatFacts fetches a batch, persists answers, and notifies the UI once.
func (a *App) refreshCompatFacts(pluginID string, games []plugins.CompatLookupGame) {
	defer func() {
		a.compatFacts.mu.Lock()
		pending := a.compatFacts.pending[pluginID]
		delete(a.compatFacts.pending, pluginID)
		if len(pending) == 0 {
			delete(a.compatFacts.inFlight, pluginID)
		}
		a.compatFacts.mu.Unlock()
		if len(pending) > 0 {
			batch := make([]plugins.CompatLookupGame, 0, len(pending))
			for _, game := range pending {
				a.compatFacts.mu.Lock()
				e := a.compatFacts.facts[pluginID][game.ID]
				fresh := compatFactFresh(e, game.SteamAppID, time.Now())
				a.compatFacts.mu.Unlock()
				if !fresh {
					batch = append(batch, game)
				}
			}
			if len(batch) > 0 {
				go a.refreshCompatFacts(pluginID, batch)
			} else {
				a.compatFacts.mu.Lock()
				delete(a.compatFacts.inFlight, pluginID)
				a.compatFacts.mu.Unlock()
			}
		}
	}()
	ctx, cancel := a.compatCtx()
	defer cancel()
	var result plugins.CompatLookupResult
	err := a.plugins.Call(ctx, pluginID, "compat.lookup", map[string]any{"games": games}, &result)
	now := time.Now()
	a.compatFacts.mu.Lock()
	defer func() {
		a.compatFacts.mu.Unlock()
		a.host.Emit("library", "")
	}()
	if a.compatFacts.facts[pluginID] == nil {
		a.compatFacts.facts[pluginID] = map[string]compatFactEntry{}
	}
	if err != nil {
		for _, g := range games {
			e := a.compatFacts.facts[pluginID][g.ID]
			if e.SteamAppID != g.SteamAppID {
				e.Fact = plugins.CompatFact{}
				e.Expires = time.Time{}
			}
			e.SteamAppID = g.SteamAppID
			e.RetryAt = now.Add(compatFactRetry)
			a.compatFacts.facts[pluginID][g.ID] = e
		}
		a.saveCompatFactsLocked()
		return
	}
	a.cacheCompatFactResultLocked(pluginID, games, result.Facts, now)
}

// cacheCompatFactResultLocked saves returned facts and backs off games omitted from a partial answer.
// The caller holds compatFacts.mu; stale facts survive an error only for their original AppID.
func (a *App) cacheCompatFactResultLocked(pluginID string, games []plugins.CompatLookupGame, facts []plugins.CompatFact, now time.Time) {
	appIDs := make(map[string]string, len(games))
	returned := make(map[string]bool, len(facts))
	for _, g := range games {
		appIDs[g.ID] = g.SteamAppID
	}
	for _, fact := range facts {
		appID, requested := appIDs[fact.ID]
		if fact.ID == "" || !requested {
			continue
		}
		ttl := time.Duration(fact.TTLSeconds) * time.Second
		if ttl <= 0 {
			ttl = compatFactRetry
		}
		if fact.Badges == nil {
			fact.Badges = []plugins.CompatBadge{}
		}
		a.compatFacts.facts[pluginID][fact.ID] = compatFactEntry{Fact: fact, SteamAppID: appID, Expires: now.Add(ttl)}
		returned[fact.ID] = true
	}
	for _, g := range games {
		if returned[g.ID] {
			continue
		}
		e := a.compatFacts.facts[pluginID][g.ID]
		if e.SteamAppID != g.SteamAppID {
			e.Fact = plugins.CompatFact{}
			e.Expires = time.Time{}
		}
		e.SteamAppID = g.SteamAppID
		e.RetryAt = now.Add(compatFactRetry)
		a.compatFacts.facts[pluginID][g.ID] = e
	}
	a.saveCompatFactsLocked()
}

// saveCompatFactsLocked atomically persists the cache while its mutex is held.
func (a *App) saveCompatFactsLocked() {
	b, err := json.MarshalIndent(a.compatFacts.facts, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(a.compatFacts.path), 0o755); err != nil {
		return
	}
	tmp := a.compatFacts.path + ".new"
	if err := os.WriteFile(tmp, b, 0o600); err == nil {
		_ = os.Rename(tmp, a.compatFacts.path)
	}
}

// factBadges converts wire badges to the public compatibility model.
func factBadges(f plugins.CompatFact) []types.CompatBadge {
	out := make([]types.CompatBadge, 0, len(f.Badges))
	for _, b := range f.Badges {
		out = append(out, types.CompatBadge{Kind: b.Kind, Label: b.Label, Tone: b.Tone, Link: b.Link, Source: b.Source, CheckedAt: b.CheckedAt})
	}
	return out
}

// factNotes removes empty provider notes before exposing them to the game page.
func factNotes(f plugins.CompatFact) []string {
	out := make([]string, 0, len(f.Notes))
	for _, note := range f.Notes {
		if strings.TrimSpace(note) != "" {
			out = append(out, note)
		}
	}
	return out
}
