package main

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"hydra-launcher/internal/compat"
	"hydra-launcher/internal/settings"
	"hydra-launcher/internal/types"
)

// Compatibility and protection: the ProtonDB rating, the native flag, DRM and
// anti-cheat (internal/compat). Plugins supply the Steam AppID hint and the
// DRM facts through game.details (protocol 1.10); the user can set a Steam
// AppID per game. Nothing is matched by title.

// compatHintTTL is how long a plugin's game.details facts are kept, so a
// library card doesn't ask the plugin on every render.
const compatHintTTL = time.Hour

// compatHintCache keeps the plugin facts per game id.
type compatHintCache struct {
	mu sync.Mutex
	m  map[string]compatHint
}

type compatHint struct {
	at      time.Time
	details *types.GameDetails
}

// newCompatService opens the compat cache in dataDir.
func newCompatService(dataDir string, st *settings.Store) (*compat.Service, error) {
	cache, err := compat.OpenCache(filepath.Join(dataDir, "compat-cache.json"))
	fetch := compat.NewFetcher("Hydra-Launcher/1.0 (Linux; compatibility metadata)")
	svc := &compat.Service{
		Cache:   cache,
		Ratings: &compat.ProtonDB{Fetch: fetch},
		AntiCheat: &compat.AntiCheatStore{
			Path:  filepath.Join(dataDir, "compat-awacy.json"),
			URL:   compat.AWACYURL,
			Fetch: compat.NewFetcher("Hydra-Launcher/1.0 (Linux; compatibility metadata)"),
		},
		Wiki:   &compat.PCGW{Fetch: compat.NewPCGWFetcher()},
		Online: st.RatingsOnline,
	}
	svc.AntiCheat.Fetch.Interval = 0
	svc.AntiCheat.Fetch.Concurrency = 1
	return svc, err
}

// baseCtx is the context for compat lookups: the app's while it runs.
func (a *App) baseCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// pluginHints returns the game.details facts of a store plugin's game, cached
// for compatHintTTL. Steam and sideloaded games have none (nil).
func (a *App) pluginHints(game types.Game) *types.GameDetails {
	if game.Source == types.SourceSteam || game.Source == types.SourceCustom {
		return nil
	}
	a.compatHints.mu.Lock()
	h, ok := a.compatHints.m[game.ID]
	a.compatHints.mu.Unlock()
	if ok && time.Since(h.at) < compatHintTTL {
		return h.details
	}
	s, ok := a.stores.Get(sourceOf(game))
	if !ok {
		return nil
	}
	ctx, cancel := context.WithTimeout(a.baseCtx(), launchTimeout)
	defer cancel()
	d, err := s.Details(ctx, game)
	if err != nil {
		return nil // not cached: the next look asks again
	}
	a.compatHints.mu.Lock()
	if a.compatHints.m == nil {
		a.compatHints.m = map[string]compatHint{}
	}
	a.compatHints.m[game.ID] = compatHint{at: time.Now(), details: d}
	a.compatHints.mu.Unlock()
	return d
}

// ratingAppID is the Steam AppID ratings use for a game, and where it came
// from. The user's choice wins, then the plugin's hint, then a Steam game's
// own AppID. "" when there is none.
func (a *App) ratingAppID(game types.Game, hints *types.GameDetails) (id, source string) {
	if id := a.settings.CompatAppID(game.ID); id != "" {
		return id, "user"
	}
	if hints != nil && hints.SteamAppID != "" {
		return hints.SteamAppID, "plugin"
	}
	if (game.Source == types.SourceSteam || game.Store == types.StoreSteam) && game.StoreAppID != "" {
		return game.StoreAppID, "steam"
	}
	return "", ""
}

// GetGameFix returns the curated fix from data plugins, or the game's ProtonDB
// tier when it has one worth showing (a Steam AppID and enough reports).
// Plugin games get their ProtonDB tier only through a Steam AppID hint or one
// the user set; nothing is matched by title.
func (a *App) GetGameFix(game types.Game) *types.GameFixInfo {
	return a.gameFix(game, func(appID string) (compat.Rating, bool) {
		return a.compat.ProtonDB(a.baseCtx(), appID)
	})
}

// gameFix is GetGameFix with the ProtonDB lookup chosen by the caller: the
// library asks without waiting for ProtonDB.
func (a *App) gameFix(game types.Game, protonDB func(appID string) (compat.Rating, bool)) *types.GameFixInfo {
	storeAppID := ""
	if game.Source == types.SourceSteam {
		storeAppID = game.StoreAppID
	}
	if fix := a.data.Fix(game.Title, storeAppID); fix != nil {
		return fix
	}
	if a.compat == nil {
		return nil
	}
	appID, _ := a.ratingAppID(game, a.pluginHints(game))
	if appID == "" {
		return nil
	}
	r, ok := protonDB(appID)
	if !ok {
		return nil
	}
	tier := compat.ShownTier(r.Summary)
	if tier == "" {
		return nil // too few reports to show a tier
	}
	return &types.GameFixInfo{
		Source:      "protondb",
		Tier:        tier,
		Description: fmt.Sprintf("ProtonDB Tier: %s (Confidence: %s, %d reports, as of %s)", tier, r.Summary.Confidence, r.Summary.Total, r.FetchedAt.UTC().Format("2006-01-02")),
	}
}

// GetGameCompat returns the compatibility and protection facts of a game.
func (a *App) GetGameCompat(game types.Game) types.Compat {
	c := types.Compat{
		RatingsOnline: a.settings.RatingsOnline(),
		DRM:           []types.CompatBadge{},
		AntiCheat:     []types.CompatBadge{},
	}
	if a.compat == nil {
		return c
	}
	hints := a.pluginHints(game)
	now := time.Now()
	c.Native = game.Native || (hints != nil && slices.Contains(hints.Platforms, "linux"))
	c.SteamAppID, c.SteamAppIDSource = a.ratingAppID(game, hints)
	if c.SteamAppID != "" {
		if r, ok := a.compat.ProtonDB(a.baseCtx(), c.SteamAppID); ok {
			c.ProtonDB = protonRating(r, c.SteamAppID)
		}
	}
	if hints != nil && hints.DRM != nil {
		c.DRM = compat.StoreDRM(hints.DRM, a.pluginName(game), now)
		c.DRMNotes = hints.DRM.Notes
	}
	// PCGamingWiki is asked by Steam AppID only; its anti-cheat names win over
	// AreWeAntiCheatYet's when it names any (see Compat.Wiki).
	wikiAnticheat := false
	if c.SteamAppID != "" {
		if w, ok := a.compat.WikiPage(a.baseCtx(), c.SteamAppID); ok && w.Found {
			drm := compat.WikiDRMBadges(w.Record, w.FetchedAt, hints != nil && hints.DRM != nil)
			ac := compat.WikiAntiCheatBadges(w.Record, w.FetchedAt)
			c.DRM = append(c.DRM, drm...)
			c.AntiCheat = append(c.AntiCheat, ac...)
			wikiAnticheat = len(ac) > 0
			if len(drm)+len(ac) > 0 {
				c.Wiki = &types.WikiCredit{
					Title:      w.Record.Title,
					URL:        compat.PCGWPage(w.Record.Title),
					FetchedAt:  w.FetchedAt.Unix(),
					LicenceURL: compat.PCGWLicenceURL,
				}
			}
		}
	}
	if cat := a.compat.AntiCheatCatalogue(a.baseCtx()); cat != nil && !wikiAnticheat {
		if e, ok := cat.BySteam(c.SteamAppID); ok {
			c.AntiCheat = append(c.AntiCheat, compat.AntiCheatBadges(e, cat.FetchedAt)...)
		} else if e, ok := cat.ByEpic(game.StoreAppID); ok && game.Store == types.StoreEpic {
			c.AntiCheat = append(c.AntiCheat, compat.AntiCheatBadges(e, cat.FetchedAt)...)
		}
	}
	if game.IsInstalled && game.InstallDir != "" {
		local := a.compat.Local(game.InstallDir)
		c.AntiCheat = append(c.AntiCheat, compat.LocalBadges(local.Hits, local.FetchedAt)...)
	}
	return c
}

// SetGameSteamAppID saves the Steam AppID the user chose for a game's ratings
// ("" clears it) and returns the game's compat facts.
func (a *App) SetGameSteamAppID(game types.Game, appID string) (types.Compat, error) {
	if err := a.settings.SetCompatAppID(game.ID, appID); err != nil {
		return types.Compat{}, err
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

// pluginName is the name a plugin's game is credited to in the UI.
func (a *App) pluginName(game types.Game) string {
	if p, ok := a.plugins.Plugin(game.Source); ok && p.Manifest.Name != "" {
		return p.Manifest.Name
	}
	return game.Source
}

// protonRating is the UI's view of a ProtonDB answer.
func protonRating(r compat.Rating, appID string) *types.ProtonRating {
	tier := compat.ShownTier(r.Summary)
	return &types.ProtonRating{
		Tier:       tier,
		Confidence: r.Summary.Confidence,
		Total:      r.Summary.Total,
		FetchedAt:  r.FetchedAt.Unix(),
		Stale:      r.Stale,
		NoReports:  !r.Found,
		FewReports: r.Found && tier == "",
		URL:        compat.ProtonDBPage(appID),
	}
}
