package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"hydra-launcher/internal/jsonrpc"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/stats"
	"hydra-launcher/internal/types"
)

// Playtime, achievements and redeem: the `playtime`, `achievements` and
// `redeem` capabilities (protocol 1.7, docs/plans/launcher-parity.md §2.4).
// Playtime is read for the whole library at once and merged into the library
// as max(local, store) for display only; it is never written back. Progress
// is fetched when a game page opens. Redeem sends one typed code at a time.

const (
	playtimeTimeout     = 30 * time.Second
	achievementsTimeout = 30 * time.Second
	redeemTimeout       = 45 * time.Second
	// A burst of sessions ending (or a quick restart) refreshes playtime once.
	playtimeDebounce = 3 * time.Second
	// Progress is cached this long per game; the page reopens without a call.
	achievementsTTL = 5 * time.Minute
)

// statsCache holds what the stats capabilities fetched. The zero value is
// ready to use.
type statsCache struct {
	mu sync.Mutex
	// playtime is the store's minutes per store game id, by plugin id.
	playtime map[string]map[string]int
	// fresh marks plugins whose playtime was read at least once this run.
	fresh        map[string]bool
	debounce     *time.Timer
	achievements map[string]achievementsEntry // by Hydra game id
	redeemMu     sync.Mutex                   // one submission at a time
	// changed tells the UI that a playtime report changed a library value.
	// info and warn are the Wails log calls; tests leave all three nil.
	changed func()
	info    func(format string, args ...any)
	warn    func(format string, args ...any)
}

func (c *statsCache) logInfo(format string, args ...any) {
	if c.info != nil {
		c.info(format, args...)
	}
}

func (c *statsCache) logWarn(format string, args ...any) {
	if c.warn != nil {
		c.warn(format, args...)
	}
}

type achievementsEntry struct {
	at       time.Time
	progress types.AchievementProgress
}

// setPlaytime stores a plugin's report and says whether it changed anything,
// so an unchanged report doesn't ask the UI to rescan.
func (c *statsCache) setPlaytime(plugin string, minutes map[string]int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fresh == nil {
		c.fresh = map[string]bool{}
	}
	c.fresh[plugin] = true
	if c.playtime == nil {
		c.playtime = map[string]map[string]int{}
	}
	if maps.Equal(c.playtime[plugin], minutes) {
		return false
	}
	c.playtime[plugin] = minutes
	return true
}

func (c *statsCache) isFresh(plugin string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.fresh[plugin]
}

// applyPlaytime merges the cached store playtime into the games of one
// plugin's library. Games the store didn't report keep their local value.
func (c *statsCache) applyPlaytime(games []types.Game) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range games {
		g := &games[i]
		store, ok := c.playtime[g.Source][g.StoreAppID]
		if !ok || g.StoreAppID == "" {
			continue
		}
		local := g.PlaytimeMin
		g.PlaytimeLocalMin, g.PlaytimeStoreMin = nil, &store
		if local > 0 {
			g.PlaytimeLocalMin = &local
		}
		g.PlaytimeMin = stats.MergePlaytime(local, store)
	}
}

func (c *statsCache) cachedAchievements(gameID string) (types.AchievementProgress, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.achievements[gameID]
	if !ok || time.Since(e.at) > achievementsTTL {
		return types.AchievementProgress{}, false
	}
	return e.progress, true
}

func (c *statsCache) putAchievements(gameID string, p types.AchievementProgress) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.achievements == nil {
		c.achievements = map[string]achievementsEntry{}
	}
	c.achievements[gameID] = achievementsEntry{at: time.Now(), progress: p}
}

// startPlaytimeOnce reads a plugin's playtime the first time its library is
// listed this run: listing starts the plugin, so this is "at plugin start".
func (a *App) startPlaytimeOnce(plugin string) {
	if !a.stats.isFresh(plugin) {
		go a.refreshPlaytime(plugin)
	}
}

// refreshPlaytime asks one plugin for its playtime. It does nothing unless the
// plugin is ready and offers playtime. A changed report tells the UI to rescan.
func (a *App) refreshPlaytime(plugin string) {
	p, ok := a.plugins.Plugin(plugin)
	if !ok || p.Status != plugins.StatusReady || !a.plugins.Has(plugin, "playtime") {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, playtimeTimeout)
	defer cancel()
	var reply stats.PlaytimeReply
	if err := a.plugins.Call(ctx, plugin, "playtime.list", map[string]any{}, &reply); err != nil {
		a.stats.logWarn("playtime %s: %v", plugin, err)
		return
	}
	if a.stats.setPlaytime(plugin, stats.PlaytimeByGame(reply)) && a.stats.changed != nil {
		a.stats.changed()
	}
}

// schedulePlaytimeRefresh runs after a session ends. Several ends close
// together cause one refresh, of the plugins that are running right now.
func (a *App) schedulePlaytimeRefresh() {
	a.stats.mu.Lock()
	defer a.stats.mu.Unlock()
	if a.stats.debounce != nil {
		a.stats.debounce.Stop()
	}
	a.stats.debounce = time.AfterFunc(playtimeDebounce, a.refreshRunningPlaytime)
}

func (a *App) refreshRunningPlaytime() {
	for _, p := range a.plugins.Plugins() {
		if id := p.Manifest.ID; a.plugins.Runtime(id).Initialized {
			a.refreshPlaytime(id)
		}
	}
}

// GetGameAchievements returns the user's own progress for a plugin game. It
// returns nil, not an error, when the plugin doesn't offer progress for this
// game: the game page then shows the store's catalogue list instead.
func (a *App) GetGameAchievements(game types.Game) (*types.AchievementProgress, error) {
	p, ok := a.plugins.Plugin(game.Source)
	if !ok || p.Status != plugins.StatusReady || !a.plugins.Has(game.Source, "achievements") {
		return nil, nil
	}
	if game.Features != nil && !slices.Contains(game.Features, "achievements") {
		return nil, nil
	}
	if cached, ok := a.stats.cachedAchievements(game.ID); ok {
		return &cached, nil
	}
	ctx, cancel := context.WithTimeout(a.ctx, achievementsTimeout)
	defer cancel()
	var reply stats.AchievementsReply
	if err := a.plugins.Call(ctx, game.Source, "achievements.list", map[string]any{"id": game.StoreAppID}, &reply); err != nil {
		return nil, err
	}
	progress := stats.NormalizeAchievements(reply)
	a.stats.putAchievements(game.ID, progress)
	return &progress, nil
}

// RedeemCode sends a typed code to a store plugin that offers redeem. The code
// goes to that plugin and nowhere else; the log shows only a masked form.
func (a *App) RedeemCode(source, code string) (types.RedeemResult, error) {
	p, ok := a.plugins.Plugin(source)
	if !ok {
		return types.RedeemResult{}, fmt.Errorf("%s isn't a store plugin", source)
	}
	if p.Status != plugins.StatusReady {
		return types.RedeemResult{}, fmt.Errorf("%s is %s", p.Manifest.Name, p.Status)
	}
	if !a.plugins.Has(source, "redeem") {
		return types.RedeemResult{}, fmt.Errorf("%s doesn't redeem codes", p.Manifest.Name)
	}
	clean, err := stats.CleanCode(code)
	if err != nil {
		return types.RedeemResult{}, err
	}
	if !a.stats.redeemMu.TryLock() {
		return types.RedeemResult{}, errors.New("a code is already being redeemed")
	}
	defer a.stats.redeemMu.Unlock()

	a.stats.logInfo("redeem %s: %s", source, stats.MaskCode(clean))
	ctx, cancel := context.WithTimeout(a.ctx, redeemTimeout)
	defer cancel()
	var reply struct {
		Message string   `json:"message"`
		Titles  []string `json:"titles"`
	}
	if err := a.plugins.Call(ctx, source, "redeem.submit", map[string]any{"code": clean}, &reply); err != nil {
		var re *jsonrpc.Error
		if errors.As(err, &re) && re.Message != "" {
			return types.RedeemResult{}, errors.New(re.Message)
		}
		return types.RedeemResult{}, err
	}
	return types.RedeemResult{Message: reply.Message, Titles: stats.TitlesOf(reply.Titles)}, nil
}
