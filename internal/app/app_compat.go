package app

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"platypus/internal/compat"
	"platypus/internal/settings"
	"platypus/internal/types"
)

// Compatibility and protection: the ProtonDB rating, the native flag, DRM and
// anti-cheat (internal/compat). Plugins supply the Steam AppID hint and the
// DRM facts through game.details (protocol 1.10); the user can set a Steam
// AppID per game. Nothing is matched by title.

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

type compatHint struct {
	at      time.Time
	details *types.GameDetails
	failed  bool // the ask failed; kept only for compatHintRetry
}

// newCompatService opens the compat cache in dataDir.
func newCompatService(dataDir string, st *settings.Store) (*compat.Service, error) {
	cache, err := compat.OpenCache(filepath.Join(dataDir, "compat-cache.json"))
	fetch := compat.NewFetcher("Platypus/1.0 (Linux; compatibility metadata)")
	svc := &compat.Service{
		Cache:   cache,
		Ratings: &compat.ProtonDB{Fetch: fetch},
		Deck:    &compat.SteamDeck{Fetch: fetch},
		AntiCheat: &compat.AntiCheatStore{
			Path:  filepath.Join(dataDir, "compat-awacy.json"),
			URL:   compat.AWACYURL,
			Fetch: compat.NewFetcher("Platypus/1.0 (Linux; compatibility metadata)"),
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
		d = nil // remembered for compatHintRetry, then asked again
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
// Steam's Deck verdict rides along either way, when Steam has one.
// Plugin games get their ProtonDB tier only through a Steam AppID hint or one
// the user set; nothing is matched by title.
func (a *App) GetGameFix(game types.Game) *types.GameFixInfo {
	return a.gameFix(game, a.pluginHints(game), func(appID string) (compat.Rating, bool) {
		return a.compat.ProtonDB(a.baseCtx(), appID)
	}, func(appID string) (compat.DeckResult, bool) {
		return a.compat.DeckRating(a.baseCtx(), appID)
	})
}

// gameFix is GetGameFix with the rating lookups chosen by the caller: the
// library asks without waiting.
func (a *App) gameFix(game types.Game, hints *types.GameDetails, protonDB func(appID string) (compat.Rating, bool), deckDB func(appID string) (compat.DeckResult, bool)) *types.GameFixInfo {
	storeAppID := ""
	if game.Source == types.SourceSteam {
		storeAppID = game.StoreAppID
	}
	if fix := a.data.Fix(game.Title, storeAppID); fix != nil {
		a.attachDeck(fix, game, hints, deckDB)
		return fix
	}
	if a.compat == nil {
		return nil
	}
	appID, _ := a.ratingAppID(game, hints)
	if appID == "" {
		return nil
	}
	var fix *types.GameFixInfo
	if r, ok := protonDB(appID); ok {
		if tier := compat.ShownTier(r.Summary); tier != "" {
			fix = &types.GameFixInfo{
				Source:      "protondb",
				Tier:        tier,
				Description: fmt.Sprintf("ProtonDB Tier: %s (Confidence: %s, %d reports, as of %s)", tier, r.Summary.Confidence, r.Summary.Total, r.FetchedAt.UTC().Format("2006-01-02")),
			}
		}
	}
	if fix == nil {
		// No tier worth showing, but a Deck verdict alone still earns a note:
		// the library cards read it.
		fix = &types.GameFixInfo{}
	}
	a.attachDeck(fix, game, hints, deckDB)
	if fix.Source == "" && fix.Tier == "" && fix.Deck == "" {
		return nil // too few reports for a tier, and no Deck verdict either
	}
	if fix.Source == "" {
		fix.Source = "steamdeck"
	}
	return fix
}

// attachDeck puts Steam's Deck verdict on a fix, when Steam has one for the
// game's Steam AppID. deckDB is the blocking or the cached lookup, like gameFix.
func (a *App) attachDeck(fix *types.GameFixInfo, game types.Game, hints *types.GameDetails, deckDB func(appID string) (compat.DeckResult, bool)) {
	if fix == nil || a.compat == nil || deckDB == nil {
		return
	}
	appID, _ := a.ratingAppID(game, hints)
	if appID == "" {
		return
	}
	if d, ok := deckDB(appID); ok && d.Found {
		fix.Deck = d.Report.Rating
		fix.DeckNotes = d.Report.Notes
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
		if d, ok := a.compat.DeckRating(a.baseCtx(), c.SteamAppID); ok && d.Found {
			c.Deck = deckRating(d, c.SteamAppID)
		}
	}
	if hints != nil && hints.DRM != nil {
		c.DRM = compat.StoreDRM(hints.DRM, a.pluginName(game), now)
		c.DRMNotes = hints.DRM.Notes
	}
	// AreWeAntiCheatYet goes first: it is the only source with the Linux status.
	if cat := a.compat.AntiCheatCatalogue(a.baseCtx()); cat != nil {
		if e, ok := cat.BySteam(c.SteamAppID); ok {
			c.AntiCheat = append(c.AntiCheat, compat.AntiCheatBadges(e, cat.FetchedAt)...)
		} else if e, ok := cat.ByEpic(game.StoreAppID); ok && game.Store == types.StoreEpic {
			c.AntiCheat = append(c.AntiCheat, compat.AntiCheatBadges(e, cat.FetchedAt)...)
		}
	}
	// PCGamingWiki is asked by Steam AppID only; it adds the anti-cheat names
	// AreWeAntiCheatYet doesn't list (see Compat.Wiki).
	if c.SteamAppID != "" {
		if w, ok := a.compat.WikiPage(a.baseCtx(), c.SteamAppID); ok && w.Found {
			drm := compat.WikiDRMBadges(w.Record, w.FetchedAt, hints != nil && hints.DRM != nil)
			ac := newAntiCheat(c.AntiCheat, compat.WikiAntiCheatBadges(w.Record, w.FetchedAt))
			c.DRM = append(c.DRM, drm...)
			c.AntiCheat = append(c.AntiCheat, ac...)
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
	if game.IsInstalled && game.InstallDir != "" {
		local := a.compat.Local(game.InstallDir)
		c.AntiCheat = append(c.AntiCheat, compat.LocalBadges(local.Hits, local.FetchedAt)...)
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
	if next != "" && next != prev && a.settings != nil && a.settings.ArtworkOnline() && a.isCustomGameID(game.ID) {
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

// deckRating is the UI's view of a Steam Deck answer.
func deckRating(d compat.DeckResult, appID string) *types.DeckRating {
	return &types.DeckRating{
		Rating:    d.Report.Rating,
		Notes:     d.Report.Notes,
		FetchedAt: d.FetchedAt.Unix(),
		Stale:     d.Stale,
		URL:       compat.DeckPage(appID),
	}
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

// newAntiCheat keeps the badges of add whose name isn't already in have
// (compared case-insensitively), so one system shows once.
func newAntiCheat(have, add []types.CompatBadge) []types.CompatBadge {
	seen := map[string]bool{}
	for _, b := range have {
		seen[strings.ToLower(b.Label)] = true
	}
	out := []types.CompatBadge{}
	for _, b := range add {
		if !seen[strings.ToLower(b.Label)] {
			out = append(out, b)
		}
	}
	return out
}
