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

// Online artwork for sideloaded games (issue #36): Steam's public store
// search finds the app, its CDN images land in the game's custom art through
// the customize store, the same as a manual pick. The "Find artwork online"
// switch guards everything: when off, nothing goes out.

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

// SearchArtOnline searches Steam's public store for title and returns up to
// 8 candidates. It is the on-demand half of the lookup; failures are the
// caller's to show. When the switch is off it refuses: nothing goes out.
func (a *App) SearchArtOnline(title string) ([]types.ArtCandidate, error) {
	if !a.settings.ArtworkOnline() {
		return nil, fmt.Errorf("finding artwork online is switched off")
	}
	cands, err := a.artClient().Search(title)
	if err != nil {
		return nil, err
	}
	out := make([]types.ArtCandidate, 0, len(cands))
	for _, c := range cands {
		out = append(out, types.ArtCandidate{AppID: c.AppID, Name: c.Name, ThumbURL: c.ThumbURL})
	}
	return out, nil
}

// ApplyOnlineArt downloads the Steam app's cover and banner (and logo when
// it has one) into the game's custom art, the same as a manual pick: the
// user's explicit choice replaces what is there. A 404 on one kind skips
// that kind; it is not an error. When nothing could be applied the error
// says so. The lookup's source is remembered for the attribution line. When
// the game's compat AppID is unset it is set too, so ratings work, and a
// sideloaded game with no year yet takes the store's (both best-effort).
func (a *App) ApplyOnlineArt(gameID string, appID int) (types.Customization, error) {
	if !a.settings.ArtworkOnline() {
		return types.Customization{}, fmt.Errorf("finding artwork online is switched off")
	}
	if !a.isCustomGameID(gameID) {
		return types.Customization{}, fmt.Errorf("art lookup is for sideloaded games")
	}
	if appID <= 0 {
		return types.Customization{}, fmt.Errorf("a Steam app id is a number")
	}
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
	if !a.isCustomGameID(gameID) {
		return nil, nil, errGameGone
	}
	entry := s.Get(gameID)
	have := entry.Art
	want := []artDownload{}
	for _, d := range []artDownload{{customize.Cover, cover}, {customize.Banner, banner}, {customize.Logo, logo}} {
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
	applied = []customize.Kind{}
	for _, d := range want {
		if err := a.storeArtBytes(gameID, d.kind, d.data); err != nil {
			return applied, nil, err
		}
		applied = append(applied, d.kind)
	}
	if len(applied) > 0 {
		if err := s.SetArtSource(gameID, "steam:"+strconv.Itoa(appID), effectiveName); err != nil {
			return applied, nil, err
		}
	}
	return applied, logoErr, nil
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

// autoArtwork is the background half of the lookup, run after AddCustomGame
// when the switch is on: it searches and applies automatically ONLY when
// exactly one result's name matches the title after normalising. Otherwise
// it does nothing. The apply shares autoApplyArt with the AppID path, so both
// replace only missing or online-sourced art, never hand-picked art.
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
	m := onlineart.ExactMatch(title, cands)
	if m == nil {
		return
	}
	a.autoApplyArt(gameID, m.AppID, m.Name, title)
}

// autoApplyArt fetches the app's art with auto semantics (missing or
// online-sourced kinds only, hand-picked kept) and emits "library" when art
// lands, so the UI refreshes the card. The game-removed guard drops silently;
// other failures warn once with label; the logo stays best-effort.
func (a *App) autoApplyArt(gameID string, appID int, name, label string) {
	applied, logoErr, err := a.fetchAndStoreArt(gameID, appID, name, true)
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
	if _, year, err := a.artClient().AppInfo(appID); err == nil {
		a.noteOnlineExtras(gameID, appID, year)
	}
	a.host.Emit("library", gameID)
}

// maybeAutoArtwork starts the background lookup after an add, only when the
// switch is on. It never blocks the add.
func (a *App) maybeAutoArtwork(gameID, title string) {
	if a.settings == nil || !a.settings.ArtworkOnline() {
		return
	}
	go a.autoArtwork(gameID, title)
}
