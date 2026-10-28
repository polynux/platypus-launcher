package app

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/jsonrpc"
	"platypus/internal/plugins"
	"platypus/internal/stats"
	"platypus/internal/types"
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
	// A failed read waits this long before a library listing tries it again.
	playtimeBackoff = 5 * time.Minute
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
	fresh map[string]bool
	// inflight marks plugins with a playtime read on its way; retryAt is when
	// a failed read may be tried again.
	inflight map[string]bool
	retryAt  map[string]time.Time
	// sources maps a launched game's Platypus id to its store, so a session end
	// refreshes only that store. debounces holds the pending refresh per store.
	sources   map[string]string
	debounces map[string]*time.Timer
	// delay is the session-end debounce; zero means playtimeDebounce.
	delay        time.Duration
	achievements map[string]achievementsEntry // by Platypus game id
	redeemMu     sync.Mutex                   // one submission at a time
	// changed tells the UI that a playtime report changed a library value.
	// info and warn are the host log calls; tests leave all three nil.
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
// listed this run: listing starts the plugin, so this is "at plugin start". A
// read already on its way, or one that failed a moment ago, isn't repeated.
func (a *App) startPlaytimeOnce(plugin string) {
	if a.stats.isFresh(plugin) || a.stats.backingOff(plugin) {
		return
	}
	a.readPlaytime(plugin)
}

// readPlaytime reads a plugin's playtime in the background, unless a read of
// that plugin is already in flight. While offline the refresh is skipped
// (issue #44): the cached numbers stay until the next trigger.
func (a *App) readPlaytime(plugin string) {
	if !a.netOnline() {
		return
	}
	if !a.stats.claim(plugin) {
		return
	}
	go func() {
		err := a.refreshPlaytime(plugin)
		if err != nil {
			a.stats.logWarn("playtime %s: %v", plugin, err)
		}
		a.stats.release(plugin, err != nil)
	}()
}

// refreshPlaytime asks one plugin for its playtime. It does nothing unless the
// plugin is ready and offers playtime. A changed report tells the UI to rescan.
func (a *App) refreshPlaytime(plugin string) error {
	p, ok := a.plugins.Plugin(plugin)
	if !ok || p.Status != plugins.StatusReady || !a.plugins.Has(plugin, "playtime") {
		return nil
	}
	ctx, cancel := context.WithTimeout(a.ctx, playtimeTimeout)
	defer cancel()
	var reply stats.PlaytimeReply
	if err := a.plugins.Call(ctx, plugin, "playtime.list", map[string]any{}, &reply); err != nil {
		return err
	}
	if a.stats.setPlaytime(plugin, stats.PlaytimeByGame(reply)) && a.stats.changed != nil {
		a.stats.changed()
	}
	return nil
}

// noteSource remembers the store of a game Platypus launched.
func (c *statsCache) noteSource(gameID, source string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sources == nil {
		c.sources = map[string]string{}
	}
	c.sources[gameID] = source
}

// schedulePlaytimeRefresh runs after a session of a game ends. Several ends of
// the same store close together make one read, of that store only.
func (a *App) schedulePlaytimeRefresh(gameID string) {
	a.stats.mu.Lock()
	defer a.stats.mu.Unlock()
	plugin, ok := a.stats.sources[gameID]
	if !ok {
		return
	}
	if t := a.stats.debounces[plugin]; t != nil {
		t.Stop()
	}
	if a.stats.debounces == nil {
		a.stats.debounces = map[string]*time.Timer{}
	}
	delay := a.stats.delay
	if delay == 0 {
		delay = playtimeDebounce
	}
	a.stats.debounces[plugin] = time.AfterFunc(delay, func() { a.readPlaytime(plugin) })
}

// claim marks a playtime read of the plugin as on its way. It fails when one is.
func (c *statsCache) claim(plugin string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.inflight[plugin] {
		return false
	}
	if c.inflight == nil {
		c.inflight = map[string]bool{}
	}
	c.inflight[plugin] = true
	return true
}

// release ends the read claimed for the plugin. A failed read waits
// playtimeBackoff before the next library listing tries it again.
func (c *statsCache) release(plugin string, failed bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.inflight, plugin)
	if !failed {
		delete(c.retryAt, plugin)
		return
	}
	if c.retryAt == nil {
		c.retryAt = map[string]time.Time{}
	}
	c.retryAt[plugin] = time.Now().Add(playtimeBackoff)
}

// backingOff says whether a failed read of the plugin is still waiting to retry.
func (c *statsCache) backingOff(plugin string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().Before(c.retryAt[plugin])
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
	a.noteAchievementUnlocks(game, progress)
	return &progress, nil
}

// noteAchievementUnlocks logs newly seen unlocks to the activity feed (issue
// #145): the plugin's unlockedAt is the unlock time, so the row is stamped
// then. Rows already stored are skipped, so polls never duplicate them.
func (a *App) noteAchievementUnlocks(game types.Game, progress types.AchievementProgress) {
	for _, ach := range progress.List {
		if !ach.Unlocked || ach.UnlockedAt <= 0 {
			continue
		}
		if a.hasAchievementActivity(game.ID, ach.ID) {
			continue
		}
		a.recordActivityAt(ach.UnlockedAt*1000, "achievement", game.ID,
			"activity:log.achievementUnlocked",
			map[string]string{"achievement": ach.ID, "name": ach.Name, "title": game.Title})
	}
}

// RedeemCode sends a typed code to a store plugin that offers redeem. The code
// goes to that plugin and nowhere else; the log shows only a masked form.
func (a *App) RedeemCode(source, code string) (types.RedeemResult, error) {
	p, ok := a.plugins.Plugin(source)
	if !ok {
		return types.RedeemResult{}, apperr.New("redeem.notStore", fmt.Sprintf("%s isn't a store plugin", source), map[string]string{"source": source})
	}
	if p.Status != plugins.StatusReady {
		return types.RedeemResult{}, apperr.New("plugin.notReady", fmt.Sprintf("%s is %s", p.Manifest.Name, p.Status), map[string]string{"name": p.Manifest.Name, "status": string(p.Status)})
	}
	if !a.plugins.Has(source, "redeem") {
		return types.RedeemResult{}, apperr.New("redeem.unsupported", fmt.Sprintf("%s doesn't redeem codes", p.Manifest.Name), map[string]string{"name": p.Manifest.Name})
	}
	clean, err := stats.CleanCode(code)
	if err != nil {
		return types.RedeemResult{}, err
	}
	if !a.stats.redeemMu.TryLock() {
		return types.RedeemResult{}, apperr.New("redeem.busy", "a code is already being redeemed", nil)
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
