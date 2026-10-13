package app

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"platypus/internal/sanitize"
	"platypus/internal/steam"
	"platypus/internal/types"
	"platypus/internal/apperr"
)

// Sideloaded game info from online sources (issue #38): a sideloaded game's
// info source is its Steam compat AppID when set, else its per-game
// info_source ("gog:<id>" or "lutris:<slug>"), else nothing. Details are
// cached on disk under the data dir, keyed by game id + source + language,
// refreshed after 7 days or when the link or language changes. Opening the
// game page never hits the network twice; offline or on a fetch error the
// stale cache serves, or empty details with no error (no toast).

// gameInfoTTL is how long a sideloaded game's cached details last before a
// fresh fetch.
const gameInfoTTL = 7 * 24 * time.Hour

// gameInfoEntry is one cached GameDetails with the link and language it was
// fetched for.
type gameInfoEntry struct {
	Link      string            `json:"link"`
	Language  string            `json:"language,omitempty"`
	FetchedAt int64             `json:"fetched_at"` // unix seconds
	Details   types.GameDetails `json:"details"`
}

// parseInfoLink splits a stored info link ("gog:<id>", "lutris:<slug>").
func parseInfoLink(link string) (source, id string, ok bool) {
	link = strings.TrimSpace(link)
	parts := strings.SplitN(link, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	source = strings.ToLower(strings.TrimSpace(parts[0]))
	id = strings.TrimSpace(parts[1])
	if (source != "gog" && source != "lutris") || id == "" {
		return "", "", false
	}
	return source, id, true
}

// infoLinkFor resolves a sideloaded game's current info link: its Steam
// compat AppID first ("steam:<app id>"), else its info_source, else "".
func (a *App) infoLinkFor(gameID string) (source, id, link string) {
	if a.settings == nil {
		return "", "", ""
	}
	if appID := strings.TrimSpace(a.settings.CompatAppID(gameID)); appID != "" {
		return "steam", appID, "steam:" + appID
	}
	if raw := strings.TrimSpace(a.settings.InfoSource(gameID)); raw != "" {
		if s, i, ok := parseInfoLink(raw); ok {
			return s, i, s + ":" + i
		}
	}
	return "", "", ""
}

// infoNames attributes a fetched GameDetails to its source.
func infoNames(source string) (infoSource, infoSourceName string) {
	switch source {
	case "steam":
		return "steam", "Steam"
	case "gog":
		return "gog", "GOG"
	case "lutris":
		return "lutris", "Lutris / IGDB"
	}
	return "", ""
}

// sideloadSteamClient is the Steam details client for sideloaded games: the
// injected one in tests, the live store endpoint otherwise. The client's
// locale follows the effective locale so descriptions come back translated.
func (a *App) sideloadSteamClient() *steam.StoreClient {
	if a.sideloadSteam != nil {
		a.sideloadSteam.Locale = a.Locale()
		return a.sideloadSteam
	}
	c := steam.NewStoreClient()
	c.Locale = a.Locale()
	return c
}

// infoCachePath names the disk cache file (issue #41: re-fetchable data
// lives in the cache dir; a cache file from before the move is still read).
func (a *App) infoCachePath() string {
	if dir := a.resolvedCacheDir(); dir != "" {
		return filepath.Join(dir, "game-info-cache.json")
	}
	if a.dataDir == "" {
		return ""
	}
	return filepath.Join(a.dataDir, "game-info-cache.json")
}

// infoCacheLegacyPath is where the cache lived before issue #41.
func (a *App) infoCacheLegacyPath() string {
	if a.dataDir == "" {
		return ""
	}
	return filepath.Join(a.dataDir, "game-info-cache.json")
}

// loadInfoCache reads the disk cache once.
func (a *App) loadInfoCache() {
	a.infoCacheMu.Lock()
	defer a.infoCacheMu.Unlock()
	if a.infoCacheLoaded {
		return
	}
	a.infoCacheLoaded = true
	if a.infoCache == nil {
		a.infoCache = map[string]gameInfoEntry{}
	}
	path := a.infoCachePath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// Before issue #41 the cache lived in the data dir: read it once.
		if legacy := a.infoCacheLegacyPath(); legacy != "" && legacy != path {
			data, err = os.ReadFile(legacy)
		}
		if err != nil {
			return
		}
	}
	var file map[string]gameInfoEntry
	if err := json.Unmarshal(data, &file); err != nil {
		return
	}
	for k, v := range file {
		a.infoCache[k] = v
	}
}

// cachedInfoEntry returns the cached entry for gameID, if any.
func (a *App) cachedInfoEntry(gameID string) (gameInfoEntry, bool) {
	a.loadInfoCache()
	a.infoCacheMu.Lock()
	defer a.infoCacheMu.Unlock()
	e, ok := a.infoCache[gameID]
	return e, ok
}

// storeInfoEntry saves one fetched details under gameID, tagged with the
// locale it was fetched in.
func (a *App) storeInfoEntry(gameID, link string, d *types.GameDetails) {
	a.loadInfoCache()
	a.infoCacheMu.Lock()
	if a.infoCache == nil {
		a.infoCache = map[string]gameInfoEntry{}
	}
	a.infoCache[gameID] = gameInfoEntry{Link: link, Language: a.Locale(), FetchedAt: time.Now().Unix(), Details: *d}
	snapshot := make(map[string]gameInfoEntry, len(a.infoCache))
	for k, v := range a.infoCache {
		snapshot[k] = v
	}
	a.infoCacheMu.Unlock()
	if path := a.infoCachePath(); path != "" {
		data, err := json.MarshalIndent(snapshot, "", "  ")
		if err == nil {
			dir := filepath.Dir(path)
			_ = os.MkdirAll(dir, 0o755)
			tmp, err := os.CreateTemp(dir, ".game-info-*.json")
			if err == nil {
				if _, err := tmp.Write(append(data, '\n')); err == nil {
					_ = tmp.Sync()
					_ = tmp.Close()
					_ = os.Rename(tmp.Name(), path)
				} else {
					tmp.Close()
				}
				_ = os.Remove(tmp.Name())
			}
		}
	}
}

// dropInfoEntry forgets the cached details of a game (link changed/unlinked).
func (a *App) dropInfoEntry(gameID string) {
	a.loadInfoCache()
	a.infoCacheMu.Lock()
	delete(a.infoCache, gameID)
	snapshot := make(map[string]gameInfoEntry, len(a.infoCache))
	for k, v := range a.infoCache {
		snapshot[k] = v
	}
	a.infoCacheMu.Unlock()
	if path := a.infoCachePath(); path != "" {
		if len(snapshot) == 0 {
			_ = os.Remove(path)
			return
		}
		if data, err := json.MarshalIndent(snapshot, "", "  "); err == nil {
			dir := filepath.Dir(path)
			_ = os.MkdirAll(dir, 0o755)
			tmp, err := os.CreateTemp(dir, ".game-info-*.json")
			if err == nil {
				if _, err := tmp.Write(append(data, '\n')); err == nil {
					_ = tmp.Sync()
					_ = tmp.Close()
					_ = os.Rename(tmp.Name(), path)
				} else {
					tmp.Close()
				}
				_ = os.Remove(tmp.Name())
			}
		}
	}
}

// getSideloadedDetails returns the online details of a sideloaded game: the
// fresh cache when new, a fetch when stale or missing, the stale cache when
// the fetch fails or the switch is off, else empty details with no error.
func (a *App) getSideloadedDetails(gameID string) (*types.GameDetails, error) {
	_, _, link := a.infoLinkFor(gameID)
	if link == "" {
		return &types.GameDetails{}, nil
	}
	cached, hasCached := a.cachedInfoEntry(gameID)
	if a.settings != nil && !a.settings.GameInfoOnline() {
		if hasCached {
			d := cached.Details
			return &d, nil
		}
		return &types.GameDetails{}, nil
	}
	// The cache keys on game id + link + language: a language switch refetches
	// even when the link is unchanged. Entries written before the language
	// tag existed count as English.
	cachedLang := cached.Language
	if cachedLang == "" {
		cachedLang = "en"
	}
	if hasCached && cached.Link == link && cachedLang == a.Locale() && time.Since(time.Unix(cached.FetchedAt, 0)) < gameInfoTTL {
		d := cached.Details
		return &d, nil
	}
	d, err := a.fetchSideloadedDetails(link)
	if err != nil {
		if hasCached {
			stale := cached.Details
			return &stale, nil
		}
		return &types.GameDetails{}, nil
	}
	a.storeInfoEntry(gameID, link, d)
	return d, nil
}

// fetchSideloadedDetails fetches one info link ("steam:<id>", "gog:<id>",
// "lutris:<slug>") with attribution. The caller's cache decides freshness;
// errors mean "serve stale or empty", never a toast.
func (a *App) fetchSideloadedDetails(link string) (*types.GameDetails, error) {
	parts := strings.SplitN(strings.TrimSpace(link), ":", 2)
	if len(parts) != 2 {
		return nil, apperr.New("info.source.unknown", fmt.Sprintf("unknown info source %q", link), map[string]string{"source": link})
	}
	source, id := strings.ToLower(strings.TrimSpace(parts[0])), strings.TrimSpace(parts[1])
	switch source {
	case "steam":
		if id == "" {
			return nil, apperr.New("steam.appid.invalid", "a Steam app id is a number", nil)
		}
		d, err := a.sideloadSteamClient().Details("", id)
		if err != nil {
			return nil, err
		}
		if d == nil {
			d = &types.GameDetails{}
		}
		d.InfoSource, d.InfoSourceName = infoNames("steam")
		return d, nil
	case "gog":
		return a.fetchGOGDetails(id)
	case "lutris":
		return a.fetchLutrisDetails(id)
	default:
		return nil, apperr.New("info.source.unknown", fmt.Sprintf("unknown info source %q", source), map[string]string{"source": source})
	}
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

// plainText strips HTML tags for a Summary.
func plainText(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

// formatGOGDate renders a GOG ISO release date ("2017-02-24T…") as
// "24 Feb, 2017", falling back to the raw date part.
func formatGOGDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05-0700", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.Format("02 Jan, 2006")
		}
	}
	if len(raw) >= 10 {
		return raw[:10]
	}
	return raw
}

// fetchGOGDetails builds a GameDetails from the GOG product endpoint plus
// the catalog search hit's facts (developers, publishers, genres) when a
// recent search saw the product.
func (a *App) fetchGOGDetails(productID string) (*types.GameDetails, error) {
	if strings.TrimSpace(productID) == "" {
		return nil, apperr.New("gog.productId.invalid", "a GOG product id is a string", nil)
	}
	g, err := a.artClient().GOGGame(productID)
	if err != nil {
		return nil, err
	}
	d := &types.GameDetails{}
	lead, full := g.Lead, g.Full
	if full == "" {
		full = lead
	}
	if r := []rune(full); len(r) > 200_000 {
		full = string(r[:200_000])
	}
	if h := sanitize.HTML(full); h != "" {
		d.DescriptionHTML = h
	}
	if s := plainText(lead); s != "" {
		if r := []rune(s); len(r) > 200_000 {
			s = string(r[:200_000])
		}
		d.Summary = s
	}
	if hit, ok := a.cachedArtCandidate("gog", productID); ok {
		d.Developers = sanitize.Strings(hit.Developers, 100, 200)
		d.Publishers = sanitize.Strings(hit.Publishers, 100, 200)
		d.Genres = sanitize.Strings(hit.Genres, 100, 200)
	}
	d.ReleaseDate = formatGOGDate(g.ReleaseDate)
	d.Languages = sanitize.Strings(g.Languages, 100, 60)
	if u := sanitize.HTTPSURL(g.StoreURL); u != "" {
		d.StoreURL = u
	} else if strings.TrimSpace(g.Slug) != "" {
		d.StoreURL = "https://www.gog.com/game/" + g.Slug
	}
	d.InfoSource, d.InfoSourceName = infoNames("gog")
	return d, nil
}

// fetchLutrisDetails builds a GameDetails from the Lutris game endpoint:
// the plain-text description goes in Summary (the UI renders it as text),
// genres and year alongside, with a link to the Lutris page.
func (a *App) fetchLutrisDetails(slug string) (*types.GameDetails, error) {
	if strings.TrimSpace(slug) == "" {
		return nil, apperr.New("lutris.slug.invalid", "a Lutris slug is a string", nil)
	}
	l, err := a.artClient().LutrisInfo(slug)
	if err != nil {
		return nil, err
	}
	d := &types.GameDetails{}
	if s := sanitize.Clip(l.Description, 200_000); s != "" {
		d.Summary = s
	}
	d.Genres = sanitize.Strings(l.Genres, 100, 200)
	if l.Year > 0 {
		d.ReleaseDate = strconv.Itoa(l.Year)
	}
	if s := strings.TrimSpace(l.Slug); s != "" {
		slug = s
	}
	if u := sanitize.HTTPSURL("https://lutris.net/games/" + slug); u != "" {
		d.StoreURL = u
	}
	d.InfoSource, d.InfoSourceName = infoNames("lutris")
	return d, nil
}

// maybeAutoLinkInfo remembers the GOG/Lutris info source after an art apply
// lands, when the game has no info yet (no compat AppID, no info_source).
// Best-effort like the year note: errors are ignored, the art is already in.
func (a *App) maybeAutoLinkInfo(gameID, source, id string) {
	if a.settings == nil || !a.settings.GameInfoOnline() {
		return
	}
	if strings.TrimSpace(a.settings.CompatAppID(gameID)) != "" {
		return
	}
	if strings.TrimSpace(a.settings.InfoSource(gameID)) != "" {
		return
	}
	_ = a.settings.SetInfoSource(gameID, source, id)
}

// LinkGameInfo links a sideloaded game to an online info source
// ("steam"/app id, "gog"/product id, "lutris"/slug) and returns the
// effective link ("steam:<id>", "gog:<id>", "lutris:<slug>"). Source "" with
// an empty id unlinks (both the info_source and the compat AppID). Linking
// steam sets the compat AppID exactly like SetGameSteamAppID, including its
// art auto-apply. When the switch is off linking is refused.
func (a *App) LinkGameInfo(gameID, source, id string) (string, error) {
	if !a.isCustomGameID(gameID) {
		return "", apperr.New("info.sideloadOnly", "info linking is for sideloaded games", nil)
	}
	if a.settings == nil || !a.settings.GameInfoOnline() {
		return "", apperr.New("info.off", "finding game info online is switched off", nil)
	}
	source = strings.ToLower(strings.TrimSpace(source))
	id = strings.TrimSpace(id)
	if source == "" && id == "" {
		if err := a.settings.SetInfoSource(gameID, "", ""); err != nil {
			return "", err
		}
		if err := a.settings.SetCompatAppID(gameID, ""); err != nil {
			return "", err
		}
		a.dropInfoEntry(gameID)
		return "", nil
	}
	switch source {
	case "steam":
		if _, err := strconv.Atoi(id); err != nil || strings.TrimSpace(id) == "" {
			return "", apperr.New("steam.appid.invalid", "a Steam app id is a number", nil)
		}
		var title string
		for _, g := range a.settings.CustomGames() {
			if g.ID == gameID {
				title = g.Title
			}
		}
		game := types.Game{ID: gameID, Source: types.SourceCustom, Store: types.StoreCustom, Title: title}
		if _, err := a.SetGameSteamAppID(game, id); err != nil {
			return "", err
		}
		_ = a.settings.SetInfoSource(gameID, "", "")
		a.dropInfoEntry(gameID)
		return "steam:" + strings.TrimSpace(id), nil
	case "gog", "lutris":
		if id == "" {
			return "", apperr.New("info.id.empty", "an info id is empty", nil)
		}
		_, _, prev := a.infoLinkFor(gameID)
		next := source + ":" + id
		if err := a.settings.SetInfoSource(gameID, source, id); err != nil {
			return "", err
		}
		if prev != next {
			a.dropInfoEntry(gameID)
		}
		return next, nil
	default:
		return "", apperr.New("info.source.unknown", fmt.Sprintf("unknown info source %q", source), map[string]string{"source": source})
	}
}

// sideloadedDetails routes GetGameDetails for sideloaded games.
func (a *App) sideloadedDetails(game types.Game) (*types.GameDetails, error) {
	return a.getSideloadedDetails(game.ID)
}
