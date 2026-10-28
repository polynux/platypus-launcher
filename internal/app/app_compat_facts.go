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
	// Online and Protection record which network sources the answer may hold
	// (issue #150: ratings and protection have their own switches). Protection
	// is a pointer: an entry saved before the split followed Online.
	Online     bool   `json:"online"`
	Protection *bool  `json:"protectionOnline,omitempty"`
	InstallDir string `json:"installDir,omitempty"`
}

// protection reports whether the answer was fetched with the protection
// sources allowed. An entry saved before the split (no field) followed the
// single old switch, which Online still holds.
func (e compatFactEntry) protection() bool {
	if e.Protection == nil {
		return e.Online
	}
	return *e.Protection
}

// fetchedWith records which sources the answer was fetched with, so switching
// one on later makes it stale instead of waiting for its expiry (issue #150).
func (e compatFactEntry) fetchedWith(g plugins.CompatLookupGame) compatFactEntry {
	e.Online = g.Online
	e.Protection = &g.ProtectionOnline
	return e
}

// compatFactFresh reports whether an answer or retry delay still covers this
// app id. A source the user switched on since the answer was fetched makes it
// stale: it may never have been asked.
func compatFactFresh(e compatFactEntry, appID, installDir string, ratings, protection bool, now time.Time) bool {
	if e.SteamAppID != appID || e.InstallDir != installDir {
		return false
	}
	if (ratings && !e.Online) || (protection && !e.protection()) {
		return false
	}
	return (e.Fact.ID != "" && now.Before(e.Expires)) || now.Before(e.RetryAt)
}

// lookupOnline reports whether a lookup request let any network source out.
// Ratings and protection have their own switches (issue #150), and an answer
// fetched with either allowed is an online snapshot.
func lookupOnline(g plugins.CompatLookupGame) bool { return g.Online || g.ProtectionOnline }

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
		if entry.Fact.Report != nil {
			combined.Report = entry.Fact.Report
		}
		if entry.Fact.Wiki != nil {
			combined.Wiki = entry.Fact.Wiki
		}
	}
	return combined, found
}

// requestCompatFacts queues stale games in one background batch per provider.
func (a *App) requestCompatFacts(games []types.Game, hints func(types.Game) *types.GameDetails) {
	if len(games) == 0 || a.compatFacts == nil || a.plugins == nil {
		return
	}
	online := a.netOnline() && (a.settings == nil || a.settings.RatingsOnline())
	// Protection facts have their own switch since #150: ProtonDB ratings and
	// PCGamingWiki/AreWeAntiCheatYet are fetched independently.
	protection := a.netOnline() && (a.settings == nil || a.settings.ProtectionOnline())
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
			fresh := compatFactFresh(e, appID, g.InstallDir, online, protection, now)
			a.compatFacts.mu.Unlock()
			if fresh {
				continue
			}
			store := string(g.Store)
			if store == "" {
				store = sourceOf(g)
			}
			item := plugins.CompatLookupGame{ID: g.ID, Store: store, StoreAppID: g.StoreAppID, SteamAppID: appID, InstallDir: g.InstallDir, Online: online, ProtectionOnline: protection}
			if d != nil {
				item.Platforms = d.Platforms
				item.StoreDRM = d.DRM != nil
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
				fresh := compatFactFresh(e, game.SteamAppID, game.InstallDir, game.Online, game.ProtectionOnline, time.Now())
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
			if !lookupOnline(g) {
				continue
			}
			e := a.compatFacts.facts[pluginID][g.ID]
			if e.SteamAppID != g.SteamAppID {
				e.Fact = plugins.CompatFact{}
				e.Expires = time.Time{}
			}
			e = e.fetchedWith(g)
			e.SteamAppID = g.SteamAppID
			e.InstallDir = g.InstallDir
			e.RetryAt = now.Add(compatFactRetry)
			a.compatFacts.facts[pluginID][g.ID] = e
		}
		a.saveCompatFactsLocked()
		return
	}
	a.cacheCompatFactResultLocked(pluginID, games, result.Facts, now)
}

// cacheCompatFactResultLocked stores each provider answer as a complete snapshot.
// The caller holds compatFacts.mu; source caches and merging belong to the provider.
func (a *App) cacheCompatFactResultLocked(pluginID string, games []plugins.CompatLookupGame, facts []plugins.CompatFact, now time.Time) {
	requested := make(map[string]plugins.CompatLookupGame, len(games))
	returned := make(map[string]bool, len(facts))
	for _, g := range games {
		requested[g.ID] = g
	}
	for _, fact := range facts {
		request, known := requested[fact.ID]
		if fact.ID == "" || !known {
			continue
		}
		ttl := time.Duration(fact.TTLSeconds) * time.Second
		if ttl <= 0 {
			ttl = compatFactRetry
		}
		if fact.Badges == nil {
			fact.Badges = []plugins.CompatBadge{}
		}
		entry := compatFactEntry{Fact: fact, SteamAppID: request.SteamAppID, Expires: now.Add(ttl), InstallDir: request.InstallDir}.fetchedWith(request)
		a.compatFacts.facts[pluginID][fact.ID] = entry
		returned[fact.ID] = true
	}
	for _, g := range games {
		if returned[g.ID] {
			continue
		}
		e := a.compatFacts.facts[pluginID][g.ID]
		if lookupOnline(g) {
			e = e.fetchedWith(g)
			e.SteamAppID = g.SteamAppID
			e.RetryAt = now.Add(compatFactRetry)
			a.compatFacts.facts[pluginID][g.ID] = e
		}
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
		out = append(out, types.CompatBadge{Kind: b.Kind, Label: b.Label, Detail: b.Detail, Tone: b.Tone, Link: b.Link, Source: b.Source, CheckedAt: b.CheckedAt})
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
