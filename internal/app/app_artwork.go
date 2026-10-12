package app

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"platypus/internal/customize"
	"platypus/internal/onlineart"
	"platypus/internal/types"
)

// Online artwork for sideloaded games (issues #36, art fallbacks): Steam's
// public store search finds the app, its CDN images land in the game's
// custom art through the customize store, the same as a manual pick. GOG's
// catalog and Lutris (mirroring IGDB covers) are keyless fallbacks, used
// only when Steam is thin and on demand. The "Find artwork online" switch
// guards everything: when off, nothing goes out.

// artClient is the lookup client: the injected one in tests, the live Steam
// endpoints otherwise.
func (a *App) artClient() *onlineart.Client {
	if a.artwork != nil {
		return a.artwork
	}
	return onlineart.New()
}

// isCustomGameID says whether id is a sideloaded game: the lookup is for
// those only.
func (a *App) isCustomGameID(id string) bool {
	for _, g := range a.settings.CustomGames() {
		if g.ID == id {
			return true
		}
	}
	return false
}

// SearchArtOnline searches for title and returns up to 8 candidates. Steam
// comes first, as today; GOG and Lutris are queried only when Steam is thin
// (fewer than ~3 hits, or nothing named like the title). The results are
// merged with duplicates by normalised name + year dropped, preferring
// Steam, then GOG, then Lutris. When Steam itself fails the fallbacks may
// still answer; when everything fails the Steam error is what the caller
// shows. When the switch is off it refuses: nothing goes out.
func (a *App) SearchArtOnline(title string) ([]types.ArtCandidate, error) {
	if !a.settings.ArtworkOnline() {
		return nil, fmt.Errorf("finding artwork online is switched off")
	}
	cl := a.artClient()
	steam, serr := cl.Search(title)
	if serr != nil {
		gog, _ := cl.SearchGOG(title)
		lutris, _ := cl.SearchLutris(title)
		merged := onlineart.MergeCandidates(nil, gog, lutris)
		if len(merged) == 0 {
			return nil, serr
		}
		a.rememberArtCandidates(merged)
		return convertArtCandidates(merged), nil
	}
	if !onlineart.SteamNeedsFallback(title, steam) {
		return convertArtCandidates(steam), nil
	}
	gog, _ := cl.SearchGOG(title)
	lutris, _ := cl.SearchLutris(title)
	merged := onlineart.MergeCandidates(steam, gog, lutris)
	a.rememberArtCandidates(merged)
	return convertArtCandidates(merged), nil
}

// convertArtCandidates carries lookup hits over to the RPC type.
func convertArtCandidates(all []onlineart.Candidate) []types.ArtCandidate {
	out := make([]types.ArtCandidate, 0, len(all))
	for _, c := range all {
		out = append(out, types.ArtCandidate{
			Source: c.Source, ID: c.ID, AppID: c.AppID,
			Name: c.Name, Year: c.Year, ThumbURL: c.ThumbURL,
		})
	}
	return out
}

// artCacheMax bounds the remembered GOG hits: one search returns at most 8,
// so this holds about eight searches back.
const artCacheMax = 64

// artCacheKey names a remembered lookup hit.
func artCacheKey(source, id string) string { return source + ":" + id }

// rememberArtCandidates keeps the GOG hits of a search answer, so a later
// apply downloads the exact images the user saw. GOG has no lookup by
// product id and the apply's game title may differ from the searched title,
// so re-searching cannot reliably reproduce the hit. Steam hits need no
// remembering (the Steam apply resolves everything from the app id) and
// neither do Lutris ones (LutrisGame fetches by slug).
func (a *App) rememberArtCandidates(all []onlineart.Candidate) {
	a.artCacheMu.Lock()
	defer a.artCacheMu.Unlock()
	for _, c := range all {
		if c.Source != "gog" || c.ID == "" {
			continue
		}
		if a.artCache == nil {
			a.artCache = map[string]onlineart.Candidate{}
		}
		if len(a.artCache) >= artCacheMax {
			for k := range a.artCache {
				delete(a.artCache, k)
				break
			}
		}
		a.artCache[artCacheKey(c.Source, c.ID)] = c
	}
}

// cachedArtCandidate returns the remembered GOG hit for id, if a recent
// search saw it.
func (a *App) cachedArtCandidate(source, id string) (onlineart.Candidate, bool) {
	a.artCacheMu.Lock()
	defer a.artCacheMu.Unlock()
	c, ok := a.artCache[artCacheKey(source, id)]
	return c, ok
}

// ApplyOnlineArt downloads a candidate's art into the game's custom art,
// the same as a manual pick: the user's explicit choice replaces what is
// there. Source names the candidate's source ("steam", "gog" or "lutris");
// id is the Steam app id, the GOG product id or the Lutris slug. Steam
// covers, banners and logos land as today; GOG covers come from
// coverVertical, banners from coverHorizontal and logos from logo; Lutris
// gives a cover only. A Lutris hit that maps to a Steam app takes the Steam
// path when Steam has the art. A 404 on one kind skips that kind; it is not
// an error. When nothing could be applied the error says so. The lookup's
// source is remembered for the attribution line ("steam:<id>", "gog:<id>"
// or "lutris:<slug>"). When the game's compat AppID is unset it is set too
// (Steam paths only), so ratings work, and a sideloaded game with no year
// yet takes the store's (both best-effort).
func (a *App) ApplyOnlineArt(gameID, source, id string) (types.Customization, error) {
	if !a.settings.ArtworkOnline() {
		return types.Customization{}, fmt.Errorf("finding artwork online is switched off")
	}
	if !a.isCustomGameID(gameID) {
		return types.Customization{}, fmt.Errorf("art lookup is for sideloaded games")
	}
	source = strings.ToLower(strings.TrimSpace(source))
	id = strings.TrimSpace(id)
	switch source {
	case "gog":
		if id == "" {
			return types.Customization{}, fmt.Errorf("a GOG product id is a string")
		}
		return a.applyGOGArt(gameID, id)
	case "lutris":
		if id == "" {
			return types.Customization{}, fmt.Errorf("a Lutris slug is a string")
		}
		return a.applyLutrisArt(gameID, id)
	case "steam", "":
		appID, err := strconv.Atoi(id)
		if err != nil || appID <= 0 {
			return types.Customization{}, fmt.Errorf("a Steam app id is a number")
		}
		return a.applySteamArt(gameID, appID)
	default:
		return types.Customization{}, fmt.Errorf("unknown art source %q", source)
	}
}

// applySteamArt downloads the Steam app's cover and banner (and logo when
// it has one) with manual semantics: the apply replaces what is there.
func (a *App) applySteamArt(gameID string, appID int) (types.Customization, error) {
	name, year, err := a.artClient().AppInfo(appID)
	if err != nil {
		return types.Customization{}, err
	}
	if strings.TrimSpace(name) == "" {
		if store, serr := a.artClient().GetItemAssets(appID); serr == nil && strings.TrimSpace(store.Name) != "" {
			name = strings.TrimSpace(store.Name)
		}
	}
	if name == "" {
		name = strconv.Itoa(appID)
	}
	applied, logoErr, err := a.fetchAndStoreArt(gameID, appID, name, false)
	if err != nil {
		c, _ := a.customization(gameID)
		return c, err
	}
	if len(applied) == 0 {
		c, _ := a.customization(gameID)
		return c, fmt.Errorf("Steam has no cover or banner art for %s", name)
	}
	if logoErr != nil {
		a.host.Warnf("artwork logo for %s: %v", name, logoErr)
	}
	a.noteOnlineExtras(gameID, appID, year)
	return a.customization(gameID)
}

// customGameTitle returns the sideloaded game's title, "" when it is gone.
func (a *App) customGameTitle(gameID string) string {
	for _, g := range a.settings.CustomGames() {
		if g.ID == gameID {
			return g.Title
		}
	}
	return ""
}

// applyGOGArt downloads the GOG product's art with manual semantics. It
// applies the remembered hit when a recent search saw the product: the
// game's own title may differ from the searched title, and GOG has no lookup
// by product id. Only on a cache miss does it fall back to the catalog
// search for the game's title; when that misses too the error says to search
// again, since the title alone cannot reproduce the user's hit.
func (a *App) applyGOGArt(gameID, productID string) (types.Customization, error) {
	if hit, ok := a.cachedArtCandidate("gog", productID); ok {
		return a.applyGOGCandidate(gameID, hit)
	}
	cands, err := a.artClient().SearchGOG(a.customGameTitle(gameID))
	if err != nil {
		c, _ := a.customization(gameID)
		return c, err
	}
	for i := range cands {
		if cands[i].ID == productID {
			return a.applyGOGCandidate(gameID, cands[i])
		}
	}
	c, _ := a.customization(gameID)
	return c, fmt.Errorf("that GOG result is stale: search for the art again, then apply it")
}

// applyGOGCandidate stores one GOG hit's art with manual semantics.
func (a *App) applyGOGCandidate(gameID string, hit onlineart.Candidate) (types.Customization, error) {
	applied, logoErr, err := a.fetchAndStoreGOGArt(gameID, hit, false)
	if err != nil {
		c, _ := a.customization(gameID)
		return c, err
	}
	if len(applied) == 0 {
		c, _ := a.customization(gameID)
		return c, fmt.Errorf("GOG has no cover or banner art for %s", hit.Name)
	}
	if logoErr != nil {
		a.host.Warnf("artwork logo for %s: %v", hit.Name, logoErr)
	}
	a.noteOnlineYear(gameID, hit.Year)
	a.maybeAutoLinkInfo(gameID, "gog", hit.ID)
	return a.customization(gameID)
}

// applyLutrisArt downloads the Lutris game's cover with manual semantics. A
// hit that maps to a Steam app takes the Steam path when Steam has the art;
// otherwise the Lutris/IGDB cover lands.
func (a *App) applyLutrisArt(gameID, slug string) (types.Customization, error) {
	cand, err := a.artClient().LutrisGame(slug)
	if err != nil {
		c, _ := a.customization(gameID)
		return c, err
	}
	name := cand.Name
	if strings.TrimSpace(name) == "" {
		name = slug
	}
	if cand.SteamAppID > 0 {
		if c, ok := a.trySteamForLutris(gameID, cand.SteamAppID, name); ok {
			return c, nil
		}
		// Steam had nothing usable: fall through to the Lutris cover.
		if cand.CoverURL == "" {
			c, _ := a.customization(gameID)
			return c, fmt.Errorf("Steam has no cover or banner art for %s", name)
		}
	}
	applied, err := a.fetchAndStoreLutrisArt(gameID, cand, false)
	if err != nil {
		c, _ := a.customization(gameID)
		return c, err
	}
	if len(applied) == 0 {
		c, _ := a.customization(gameID)
		return c, fmt.Errorf("Lutris has no cover art for %s", name)
	}
	a.noteOnlineYear(gameID, cand.Year)
	a.maybeAutoLinkInfo(gameID, "lutris", cand.ID)
	return a.customization(gameID)
}

// trySteamForLutris applies the mapped Steam app's art with manual semantics
// for a Lutris hit. It reports whether Steam had art to give.
func (a *App) trySteamForLutris(gameID string, appID int, name string) (types.Customization, bool) {
	storeName, year, err := a.artClient().AppInfo(appID)
	if err != nil {
		return types.Customization{}, false
	}
	if strings.TrimSpace(storeName) == "" {
		if store, serr := a.artClient().GetItemAssets(appID); serr == nil && strings.TrimSpace(store.Name) != "" {
			storeName = strings.TrimSpace(store.Name)
		}
	}
	if storeName == "" {
		storeName = name
	}
	applied, logoErr, err := a.fetchAndStoreArt(gameID, appID, storeName, false)
	if err != nil || len(applied) == 0 {
		return types.Customization{}, false
	}
	if logoErr != nil {
		a.host.Warnf("artwork logo for %s: %v", storeName, logoErr)
	}
	a.noteOnlineExtras(gameID, appID, year)
	c, _ := a.customization(gameID)
	return c, true
}

// artDownload is one fetched image waiting to be stored.
type artDownload struct {
	kind customize.Kind
	data []byte
}

// errGameGone aborts an online art apply whose game was removed mid-flight:
// the art, compat AppID and year must land nowhere.
var errGameGone = errors.New("the game was removed")

// fetchAndStoreArt downloads the app's cover, banner and logo and stores
// each into the game's custom art. It first asks the store-browse API for the
// app's real hashed asset paths (newer apps 404 the fixed CDN names); when
// that fails or lists no assets it falls back to the fixed CDN names, which
// older apps still serve. A resolved URL that 404s falls back to its fixed
// name too. When auto is true (the background paths) a kind is replaced only
// when it has no art yet or its art came from an earlier online lookup (the
// game's ArtSource is set); art the user picked by hand (art present and
// ArtSource empty) is never overwritten. When auto is false (the manual
// path: the user's explicit choice) the apply replaces what is there.
// It returns the kinds stored and, separately, a logo download error: the
// logo is best-effort, so its failure only skips it (the caller warns once)
// while cover and banner still apply. A cover or banner download error aborts
// the apply; a 404 on one kind only skips it. When art was stored the
// lookup's source is remembered for the attribution line, using the GetItems
// name when the caller had none. Right before storing, a game that no longer
// exists aborts with errGameGone: nothing is stored for it.
func (a *App) fetchAndStoreArt(gameID string, appID int, name string, auto bool) (applied []customize.Kind, logoErr, err error) {
	s := a.custom.get()
	if s == nil {
		return nil, nil, fmt.Errorf("customization is not available")
	}
	cl := a.artClient()
	storeAssets, storeErr := cl.GetItemAssets(appID)
	if storeErr != nil {
		storeAssets = onlineart.StoreAssets{}
	}
	effectiveName := name
	if (effectiveName == "" || effectiveName == strconv.Itoa(appID)) && storeAssets.Name != "" {
		effectiveName = storeAssets.Name
	}
	// downloadOne tries the resolved URL first, then the fixed URL when the
	// resolved one is absent (404) and differs.
	downloadOne := func(resolved, fixed string) ([]byte, error) {
		if resolved != "" {
			data, err := cl.Download(resolved)
			if err != nil {
				return nil, err
			}
			if data != nil {
				return data, nil
			}
			if fixed == "" || fixed == resolved {
				return nil, nil
			}
		} else if fixed == "" {
			return nil, nil
		}
		return cl.Download(fixed)
	}
	coverURL := ""
	coverFixed := cl.CoverURL(appID)
	if storeErr == nil && storeAssets.CoverURL != "" {
		coverURL = storeAssets.CoverURL
	}
	cover, err := downloadOne(coverURL, coverFixed)
	if err != nil {
		return nil, nil, err
	}
	var banner []byte
	if storeErr == nil && storeAssets.BannerURL != "" {
		banner, err = downloadOne(storeAssets.BannerURL, "")
		if err != nil {
			return nil, nil, err
		}
		if banner == nil {
			banner, err = cl.Banner(appID)
			if err != nil {
				return nil, nil, err
			}
		}
	} else {
		banner, err = cl.Banner(appID)
		if err != nil {
			return nil, nil, err
		}
	}
	logoURL := ""
	logoFixed := cl.LogoURL(appID)
	if storeErr == nil && storeAssets.LogoURL != "" {
		logoURL = storeAssets.LogoURL
	}
	logo, logoErr := downloadOne(logoURL, logoFixed)
	if logoErr != nil {
		logo = nil
	}
	applied, serr := a.storeArtSet(gameID,
		[]artDownload{{customize.Cover, cover}, {customize.Banner, banner}, {customize.Logo, logo}},
		"steam:"+strconv.Itoa(appID), effectiveName, auto)
	if serr != nil {
		return applied, nil, serr
	}
	return applied, logoErr, nil
}

// fetchAndStoreGOGArt downloads the GOG product's cover (coverVertical),
// banner (coverHorizontal) and logo into the game's custom art, with the
// same auto/manual semantics as the Steam path. A 404 on one kind only
// skips it; the logo stays best-effort.
func (a *App) fetchAndStoreGOGArt(gameID string, cand onlineart.Candidate, auto bool) (applied []customize.Kind, logoErr, err error) {
	cl := a.artClient()
	download := func(raw string) ([]byte, error) {
		if strings.TrimSpace(raw) == "" {
			return nil, nil
		}
		return cl.Download(raw)
	}
	cover, err := download(cand.CoverURL)
	if err != nil {
		return nil, nil, err
	}
	banner, err := download(cand.BannerURL)
	if err != nil {
		return nil, nil, err
	}
	logo, logoErr := download(cand.LogoURL)
	if logoErr != nil {
		logo = nil
	}
	name := cand.Name
	if strings.TrimSpace(name) == "" {
		name = cand.ID
	}
	applied, serr := a.storeArtSet(gameID,
		[]artDownload{{customize.Cover, cover}, {customize.Banner, banner}, {customize.Logo, logo}},
		"gog:"+cand.ID, name, auto)
	if serr != nil {
		return applied, nil, serr
	}
	return applied, logoErr, nil
}

// fetchAndStoreLutrisArt downloads the Lutris/IGDB cover into the game's
// custom art, with the same auto/manual semantics as the Steam path. The
// larger IGDB copy is tried first, falling back to the coverart URL itself.
// Lutris gives covers only.
func (a *App) fetchAndStoreLutrisArt(gameID string, cand onlineart.Candidate, auto bool) ([]customize.Kind, error) {
	cl := a.artClient()
	download := func(raw string) ([]byte, error) {
		if strings.TrimSpace(raw) == "" {
			return nil, nil
		}
		return cl.Download(raw)
	}
	cover, err := download(cand.CoverURL)
	if err != nil || cover == nil {
		if cand.CoverFallbackURL != "" && cand.CoverFallbackURL != cand.CoverURL {
			cover, err = download(cand.CoverFallbackURL)
		}
	}
	if err != nil {
		return nil, err
	}
	if cover == nil {
		return nil, nil
	}
	name := cand.Name
	if strings.TrimSpace(name) == "" {
		name = cand.ID
	}
	return a.storeArtSet(gameID,
		[]artDownload{{customize.Cover, cover}},
		"lutris:"+cand.ID, name, auto)
}

// storeArtSet stores downloaded art into the game's custom art and remembers
// the lookup's source for the attribution line. When auto is true (the
// background paths) a kind is replaced only when it has no art yet or its
// art came from an earlier online lookup (the game's ArtSource is set); art
// the user picked by hand (art present and ArtSource empty) is never
// overwritten. When auto is false (a manual apply: the user's explicit
// choice) the apply replaces what is there. Right before storing, a game
// that no longer exists aborts with errGameGone: nothing is stored for it.
func (a *App) storeArtSet(gameID string, downloads []artDownload, source, name string, auto bool) ([]customize.Kind, error) {
	s := a.custom.get()
	if s == nil {
		return nil, fmt.Errorf("customization is not available")
	}
	if !a.isCustomGameID(gameID) {
		return nil, errGameGone
	}
	entry := s.Get(gameID)
	have := entry.Art
	want := []artDownload{}
	for _, d := range downloads {
		if d.data == nil {
			continue
		}
		if auto {
			if _, ok := have[d.kind]; ok && entry.ArtSource == "" {
				continue // hand-picked: never overwritten by an auto apply
			}
		}
		want = append(want, d)
	}
	applied := []customize.Kind{}
	for _, d := range want {
		if err := a.storeArtBytes(gameID, d.kind, d.data); err != nil {
			return applied, err
		}
		applied = append(applied, d.kind)
	}
	if len(applied) > 0 {
		if err := s.SetArtSource(gameID, source, name); err != nil {
			return applied, err
		}
	}
	return applied, nil
}

// storeArtBytes stores downloaded bytes as kind of gameID through the
// customize store, which re-checks the content (type, size, dimensions).
func (a *App) storeArtBytes(gameID string, kind customize.Kind, data []byte) error {
	s := a.custom.get()
	if s == nil {
		return fmt.Errorf("customization is not available")
	}
	f, err := os.CreateTemp("", "platypus-art-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return s.SetArt(gameID, kind, name)
}

// noteOnlineExtras sets the compat AppID (when unset, so ratings work) and
// the release year (when the sideloaded game has none yet) from the store's
// data. Best-effort: errors are ignored, the art is already in.
func (a *App) noteOnlineExtras(gameID string, appID int, year int) {
	if a.settings.CompatAppID(gameID) == "" {
		_ = a.settings.SetCompatAppID(gameID, strconv.Itoa(appID))
	}
	if year <= 0 {
		return
	}
	for _, g := range a.settings.CustomGames() {
		if g.ID == gameID && g.ReleaseYear == 0 {
			_ = a.settings.SetCustomGameYear(gameID, year)
		}
	}
}

// noteOnlineYear records the release year from a GOG/Lutris apply when the
// sideloaded game has none yet. Best-effort: errors are ignored, the art is
// already in. The compat AppID stays Steam-only: only the Steam paths set
// it (see noteOnlineExtras).
func (a *App) noteOnlineYear(gameID string, year int) {
	if year <= 0 {
		return
	}
	for _, g := range a.settings.CustomGames() {
		if g.ID == gameID && g.ReleaseYear == 0 {
			_ = a.settings.SetCustomGameYear(gameID, year)
		}
	}
}

// autoArtwork is the background half of the lookup, run after AddCustomGame
// when the switch is on: Steam is searched first, as today, and art applies
// automatically ONLY when exactly one result's name matches the title after
// normalising. Only when Steam has no exact single match, GOG is tried, then
// Lutris, with the same normalised exact-single-match rule. At most one
// search request goes out per source per add. Otherwise it does nothing.
// The applies share the auto semantics, so all three replace only missing or
// online-sourced art, never hand-picked art.
func (a *App) autoArtwork(gameID, title string) {
	if !a.settings.ArtworkOnline() {
		return
	}
	cl := a.artClient()
	cands, err := cl.Search(title)
	if err != nil {
		a.host.Warnf("artwork lookup for %q: %v", title, err)
		return
	}
	if m := onlineart.ExactMatch(title, cands); m != nil {
		a.autoApplyArt(gameID, m.AppID, m.Name, title)
		return
	}
	gog, err := cl.SearchGOG(title)
	if err == nil {
		if m := onlineart.ExactMatch(title, gog); m != nil {
			a.autoApplyCandidate(gameID, *m, title)
			return
		}
	}
	lutris, err := cl.SearchLutris(title)
	if err != nil {
		return
	}
	if m := onlineart.ExactMatch(title, lutris); m != nil {
		a.autoApplyCandidate(gameID, *m, title)
	}
}

// autoApplyCandidate applies a GOG/Lutris auto hit with auto semantics and
// emits "library" when art lands, so the UI refreshes the card. A hit that
// maps to a Steam app takes the Steam path when Steam has art. The
// game-removed guard drops silently; other failures warn once with label.
func (a *App) autoApplyCandidate(gameID string, cand onlineart.Candidate, label string) {
	if cand.Source == "steam" {
		a.autoApplyArt(gameID, cand.AppID, cand.Name, label)
		return
	}
	if cand.SteamAppID > 0 && a.autoSteam(gameID, cand.SteamAppID, cand.Name, label) {
		return
	}
	var applied []customize.Kind
	var logoErr error
	var err error
	switch cand.Source {
	case "gog":
		applied, logoErr, err = a.fetchAndStoreGOGArt(gameID, cand, true)
	case "lutris":
		applied, err = a.fetchAndStoreLutrisArt(gameID, cand, true)
	default:
		return
	}
	if err != nil {
		if errors.Is(err, errGameGone) {
			return // removed mid-flight: drop silently
		}
		a.host.Warnf("artwork lookup for %q: %v", label, err)
		return
	}
	if logoErr != nil {
		a.host.Warnf("artwork logo for %q: %v", label, logoErr)
	}
	if len(applied) == 0 {
		return
	}
	a.noteOnlineYear(gameID, cand.Year)
	a.maybeAutoLinkInfo(gameID, cand.Source, cand.ID)
	a.host.Emit("library", gameID)
}

// autoApplyArt fetches the app's art with auto semantics (missing or
// online-sourced kinds only, hand-picked kept) and emits "library" when art
// lands, so the UI refreshes the card. The game-removed guard drops silently;
// other failures warn once with label; the logo stays best-effort.
func (a *App) autoApplyArt(gameID string, appID int, name, label string) {
	a.autoSteam(gameID, appID, name, label)
}

// autoSteam is the Steam half of an auto apply: it fetches with auto
// semantics and emits "library" when art lands. It reports whether there is
// nothing more to do (art landed, or the game is gone): a GOG/Lutris caller
// falls back to its own art only when Steam had nothing usable.
func (a *App) autoSteam(gameID string, appID int, name, label string) bool {
	applied, logoErr, err := a.fetchAndStoreArt(gameID, appID, name, true)
	if err != nil {
		if errors.Is(err, errGameGone) {
			return true // removed mid-flight: drop silently
		}
		a.host.Warnf("artwork lookup for %q: %v", label, err)
		return false
	}
	if logoErr != nil {
		a.host.Warnf("artwork logo for %q: %v", label, logoErr)
	}
	if len(applied) == 0 {
		return false
	}
	if _, year, err := a.artClient().AppInfo(appID); err == nil {
		a.noteOnlineExtras(gameID, appID, year)
	}
	a.host.Emit("library", gameID)
	return true
}

// maybeAutoArtwork starts the background lookup after an add, only when the
// switch is on. It never blocks the add.
func (a *App) maybeAutoArtwork(gameID, title string) {
	if a.settings == nil || !a.settings.ArtworkOnline() {
		return
	}
	go a.autoArtwork(gameID, title)
}
