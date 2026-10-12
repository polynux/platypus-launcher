package app

import (
	"fmt"
	"os"
	"strconv"

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
	if name == "" {
		name = strconv.Itoa(appID)
	}
	applied, err := a.fetchAndStoreArt(gameID, appID, name, false)
	if err != nil {
		c, _ := a.customization(gameID)
		return c, err
	}
	if len(applied) == 0 {
		c, _ := a.customization(gameID)
		return c, fmt.Errorf("Steam has no cover or banner art for %s", name)
	}
	a.noteOnlineExtras(gameID, appID, year)
	return a.customization(gameID)
}

// artDownload is one fetched image waiting to be stored.
type artDownload struct {
	kind customize.Kind
	data []byte
}

// fetchAndStoreArt downloads the app's cover, banner and logo and stores
// each into the game's custom art. skipHas leaves kinds the user already set
// alone (the auto path); otherwise the apply replaces (the manual path: the
// user's explicit choice). It returns the kinds stored. A download error
// aborts the apply; a 404 on one kind only skips it. When art was stored the
// lookup's source is remembered for the attribution line.
func (a *App) fetchAndStoreArt(gameID string, appID int, name string, skipHas bool) ([]customize.Kind, error) {
	s := a.custom.get()
	if s == nil {
		return nil, fmt.Errorf("customization is not available")
	}
	cl := a.artClient()
	cover, err := cl.Download(cl.CoverURL(appID))
	if err != nil {
		return nil, err
	}
	banner, err := cl.Banner(appID)
	if err != nil {
		return nil, err
	}
	logo, err := cl.Download(cl.LogoURL(appID))
	if err != nil {
		return nil, err
	}
	have := s.Get(gameID).Art
	want := []artDownload{}
	for _, d := range []artDownload{{customize.Cover, cover}, {customize.Banner, banner}, {customize.Logo, logo}} {
		if d.data == nil {
			continue
		}
		if skipHas {
			if _, ok := have[d.kind]; ok {
				continue
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
		if err := s.SetArtSource(gameID, "steam:"+strconv.Itoa(appID), name); err != nil {
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

// autoArtwork is the background half of the lookup, run after AddCustomGame
// when the switch is on: it searches and applies automatically ONLY when
// exactly one result's name matches the title after normalising. Otherwise
// it does nothing. Art the user set by hand is never overwritten: kinds
// with custom art are skipped. Failures are silent (at most one warn line)
// and the placeholder stays. A successful apply emits "library" so the UI
// refreshes the card's art.
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
	applied, err := a.fetchAndStoreArt(gameID, m.AppID, m.Name, true)
	if err != nil {
		a.host.Warnf("artwork lookup for %q: %v", title, err)
		return
	}
	if len(applied) == 0 {
		return
	}
	if _, year, err := cl.AppInfo(m.AppID); err == nil {
		a.noteOnlineExtras(gameID, m.AppID, year)
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
