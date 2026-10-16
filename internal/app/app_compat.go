// Package app coordinates compatibility hints, cached provider facts, and
// compatibility settings exposed to the UI.
package app

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"platypus/internal/types"
)

// compatHintTTL is how long successful store detail hints remain reusable.
const compatHintTTL = time.Hour

// compatHintRetry limits retries after a failed store detail request.
const compatHintRetry = 5 * time.Minute

// compatHintCache stores short-lived store detail hints by game id.
type compatHintCache struct {
	mu sync.Mutex
	m  map[string]compatHint
}

// compatHint is a cached detail answer and its success state.
type compatHint struct {
	at      time.Time
	details *types.GameDetails
	failed  bool
}

// baseCtx returns the app context, or a background context before startup.
func (a *App) baseCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// compatCtx bounds compatibility work when network access is unavailable.
func (a *App) compatCtx() (context.Context, context.CancelFunc) {
	if a.netOnline() {
		return context.WithCancel(a.baseCtx())
	}
	return context.WithTimeout(a.baseCtx(), time.Second)
}

// pluginHints returns cached or freshly fetched store details for non-Steam games.
func (a *App) pluginHints(game types.Game) *types.GameDetails {
	if game.Source == types.SourceSteam || game.Source == types.SourceCustom {
		return nil
	}
	if d, ok := a.cachedPluginHints(game); ok {
		return d
	}
	s, ok := a.stores.Get(sourceOf(game))
	if !ok {
		return nil
	}
	ctx, cancel := context.WithTimeout(a.baseCtx(), launchTimeout)
	defer cancel()
	d, err := s.Details(ctx, game)
	if err != nil {
		d = nil
	}
	a.compatHints.mu.Lock()
	if a.compatHints.m == nil {
		a.compatHints.m = map[string]compatHint{}
	}
	a.compatHints.m[game.ID] = compatHint{at: time.Now(), details: d, failed: err != nil}
	a.compatHints.mu.Unlock()
	return d
}

// cachedPluginHints returns an unexpired store detail hint without fetching it.
func (a *App) cachedPluginHints(game types.Game) (*types.GameDetails, bool) {
	if game.Source == types.SourceSteam || game.Source == types.SourceCustom {
		return nil, true
	}
	a.compatHints.mu.Lock()
	defer a.compatHints.mu.Unlock()
	h, ok := a.compatHints.m[game.ID]
	if !ok {
		return nil, false
	}
	ttl := compatHintTTL
	if h.failed {
		ttl = compatHintRetry
	}
	if time.Since(h.at) >= ttl {
		return nil, false
	}
	return h.details, true
}

// ratingAppID resolves the Steam AppID and records how it was obtained.
func (a *App) ratingAppID(game types.Game, hints *types.GameDetails) (id, source string) {
	if a.settings != nil {
		if id := a.settings.CompatAppID(game.ID); id != "" {
			return id, "user"
		}
	}
	if hints != nil && hints.SteamAppID != "" {
		return hints.SteamAppID, "plugin"
	}
	if (game.Source == types.SourceSteam || game.Store == types.StoreSteam) && game.StoreAppID != "" {
		return game.StoreAppID, "steam"
	}
	return "", ""
}

// GetGameFix returns the curated fix for a game and queues compatibility facts.
func (a *App) GetGameFix(game types.Game) *types.GameFixInfo {
	hints := a.pluginHints(game)
	a.requestCompatFacts([]types.Game{game}, func(types.Game) *types.GameDetails { return hints })
	return a.gameFix(game)
}

// gameFix finds a curated fix using the title and, for Steam, its store AppID.
func (a *App) gameFix(game types.Game) *types.GameFixInfo {
	storeAppID := ""
	if game.Source == types.SourceSteam {
		storeAppID = game.StoreAppID
	}
	if fix := a.data.Fix(game.Title, storeAppID); fix != nil {
		return fix
	}
	return nil
}

// storeDRMBadges converts DRM facts from store details into generic badges.
func storeDRMBadges(d *types.DRM, source string, checked time.Time) []types.CompatBadge {
	if d == nil {
		return nil
	}
	var out []types.CompatBadge
	at := checked.Unix()
	add := func(kind, label, detail string) {
		out = append(out, types.CompatBadge{Kind: kind, Label: label, Detail: detail, Source: source, CheckedAt: at})
	}
	if d.DRMFree != nil && *d.DRMFree {
		add("drm-free", "DRM-free", "The store sells it without DRM.")
	}
	if d.OnlineRequired != nil && *d.OnlineRequired {
		add("online", "Online required", "The store says the game can't run offline.")
	}
	if launcher := storeLauncherName(d.Launcher); launcher != "" {
		add("launcher", "Needs "+launcher, "The store says the game runs through "+launcher+".")
	}
	if d.DenuvoLikely != nil && *d.DenuvoLikely {
		add("denuvo", "Denuvo likely", "Inferred from the store's ownership check, not confirmed.")
	}
	return out
}

// storeLauncherName expands known launcher identifiers for display.
func storeLauncherName(raw string) string {
	names := map[string]string{"ubisoft-connect": "Ubisoft Connect", "ea-app": "EA app", "origin": "EA app", "steam": "Steam", "gog-galaxy": "GOG Galaxy"}
	if name, ok := names[raw]; ok {
		return name
	}
	if len(raw) > 60 {
		return raw[:60]
	}
	return raw
}

// GetGameCompat returns compatibility facts currently cached for the game.
func (a *App) GetGameCompat(game types.Game) types.Compat {
	c := types.Compat{RatingsOnline: a.settings == nil || a.settings.RatingsOnline(), Badges: []types.CompatBadge{}}
	hints := a.pluginHints(game)
	a.requestCompatFacts([]types.Game{game}, func(types.Game) *types.GameDetails { return hints })
	c.Native = game.Native || (hints != nil && slices.Contains(hints.Platforms, "linux"))
	c.SteamAppID, c.SteamAppIDSource = a.ratingAppID(game, hints)
	if fact, ok := a.cachedProviderFact(game.ID, c.SteamAppID); ok {
		c.Badges = factBadges(fact)
		c.Notes = factNotes(fact)
		if fact.Report != nil {
			c.Report = &types.CompatReport{Count: fact.Report.Count, State: fact.Report.State, Tier: fact.Report.Tier}
		}
		for _, l := range fact.Links {
			c.Links = append(c.Links, types.CompatLink{Label: l.Label, URL: l.URL})
		}
		if fact.Wiki != nil {
			c.Wiki = &types.WikiCredit{Title: fact.Wiki.Title, URL: fact.Wiki.URL, FetchedAt: fact.Wiki.FetchedAt, LicenceURL: fact.Wiki.LicenceURL}
		}
	}
	if hints != nil && hints.DRM != nil {
		storeFacts := storeDRMBadges(hints.DRM, a.pluginName(game), time.Now())
		c.Badges = append(c.Badges, storeFacts...)
		if hints.DRM.Notes != "" {
			c.Notes = append(c.Notes, hints.DRM.Notes)
		}
	}
	return c
}

// SetGameSteamAppID saves a user AppID override and returns the refreshed facts.
func (a *App) SetGameSteamAppID(game types.Game, appID string) (types.Compat, error) {
	prev := ""
	if a.settings != nil {
		prev = a.settings.CompatAppID(game.ID)
	}
	if err := a.settings.SetCompatAppID(game.ID, appID); err != nil {
		return types.Compat{}, err
	}
	next := strings.TrimSpace(appID)
	if next != "" && next != prev && a.settings.ArtworkOnline() && a.isCustomGameID(game.ID) {
		if id, err := strconv.Atoi(next); err == nil && id > 0 {
			label := game.Title
			if strings.TrimSpace(label) == "" {
				label = next
			}
			go a.autoApplyArt(game.ID, id, "", label)
		}
	}
	return a.GetGameCompat(game), nil
}

// SetRatingsOnline enables or disables online compatibility lookups.
func (a *App) SetRatingsOnline(on bool) (types.LauncherSettings, error) {
	if err := a.settings.SetRatingsOnline(on); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}

// SetArtworkOnline enables or disables online artwork lookups.
func (a *App) SetArtworkOnline(on bool) (types.LauncherSettings, error) {
	if err := a.settings.SetArtworkOnline(on); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}

// SetGameInfoOnline enables or disables online game metadata lookups.
func (a *App) SetGameInfoOnline(on bool) (types.LauncherSettings, error) {
	if err := a.settings.SetGameInfoOnline(on); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}

// SetChatDesktopNotify enables or disables desktop notifications for chat.
func (a *App) SetChatDesktopNotify(on bool) (types.LauncherSettings, error) {
	if err := a.settings.SetChatDesktopNotify(on); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}

// pluginName returns a store plugin's display name when one is available.
func (a *App) pluginName(game types.Game) string {
	if p, ok := a.plugins.Plugin(game.Source); ok && p.Manifest.Name != "" {
		return p.Manifest.Name
	}
	return game.Source
}
