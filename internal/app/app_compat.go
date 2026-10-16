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

// Compatibility and protection: the native flag, store DRM facts and the
// facts of compat-facts plugins (ratings, verdicts, anti-cheat; see
// app_compat_facts.go). Plugins supply the Steam AppID hint and the DRM facts
// through game.details (protocol 1.10); the user can set a Steam AppID per
// game. Nothing is matched by title.

// compatHintTTL is how long a plugin's game.details facts are kept, so a
// library card doesn't ask the plugin on every render.
const compatHintTTL = time.Hour

// compatHintRetry is how long a failed game.details ask is remembered, so a
// slow or failing plugin isn't asked again on every look.
const compatHintRetry = 5 * time.Minute

// compatHintCache keeps the plugin facts per game id.
type compatHintCache struct {
	mu sync.Mutex
	m  map[string]compatHint
}

// compatHint is a cached detail answer and its success state.
type compatHint struct {
	at      time.Time
	details *types.GameDetails
	failed  bool // the ask failed; kept only for compatHintRetry
}

// baseCtx is the context for compat lookups: the app's while it runs.
func (a *App) baseCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// compatCtx is the context for compat lookups: the app's while online, with
// a 1 s deadline while offline (issue #44). Background work skips its
// fetches while offline; on-demand lookups still try, briefly.
func (a *App) compatCtx() (context.Context, context.CancelFunc) {
	if a.netOnline() {
		return context.WithCancel(a.baseCtx())
	}
	return context.WithTimeout(a.baseCtx(), time.Second)
}

// pluginHints returns the game.details facts of a store plugin's game, cached
// for compatHintTTL. Steam and sideloaded games have none (nil).
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

// cachedPluginHints is pluginHints without asking the plugin: the library scan
// uses it, so listing a big library never waits on per-game plugin calls. ok is
// false when nothing fresh is cached.
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

// ratingAppID is the Steam AppID ratings use for a game, and where it came
// from. The user's choice wins, then the plugin's hint, then a Steam game's
// own AppID. "" when there is none.
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

// GetGameFix returns a game's curated fix, if any. Provider lookups are
// queued in the background and never delay rendering.
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

// GetGameCompat returns the compatibility and protection facts of a game.
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

// SetGameSteamAppID saves the Steam AppID the user chose for a game's ratings
// ("" clears it) and returns the game's compat facts. When the game is
// sideloaded, the "Find artwork online" switch is on, and the id is non-empty
// and changed, it starts the art apply in the background for that app id
// (auto semantics: missing or online-sourced kinds only, hand-picked kept).
// The save never waits for the art; a successful apply emits "library".
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

// SetRatingsOnline saves the "Fetch ratings online" switch.
func (a *App) SetRatingsOnline(on bool) (types.LauncherSettings, error) {
	if err := a.settings.SetRatingsOnline(on); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}

// SetArtworkOnline saves the "Find artwork online" switch (issue #36).
func (a *App) SetArtworkOnline(on bool) (types.LauncherSettings, error) {
	if err := a.settings.SetArtworkOnline(on); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}

// SetGameInfoOnline saves the "Find game info online" switch (issue #38).
func (a *App) SetGameInfoOnline(on bool) (types.LauncherSettings, error) {
	if err := a.settings.SetGameInfoOnline(on); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}

// SetChatDesktopNotify saves the "Desktop notifications for chat" switch
// (issue #12): a system notification for a new chat message when the window
// isn't focused.
func (a *App) SetChatDesktopNotify(on bool) (types.LauncherSettings, error) {
	if err := a.settings.SetChatDesktopNotify(on); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}

// pluginName is the name a plugin's game is credited to in the UI.
func (a *App) pluginName(game types.Game) string {
	if p, ok := a.plugins.Plugin(game.Source); ok && p.Manifest.Name != "" {
		return p.Manifest.Name
	}
	return game.Source
}
