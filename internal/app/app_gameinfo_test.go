package app

import (
	"strings"
	"testing"
	"time"

	"platypus/internal/types"
)

const infoSteamDetail = `{"123":{"success":true,"data":{
	"name":"Hollow Knight",
	"short_description":"A brave knight.",
	"detailed_description":"<p>Brave <script>evil()</script><a href=\"https://example.com\">link</a></p>",
	"developers":["Team Cherry"],"publishers":["Team Cherry"],
	"genres":[{"description":"Action"}],
	"release_date":{"date":"24 Feb, 2017"},
	"screenshots":[{"path_full":"https://cdn/x/shot.jpg"}],
	"website":"https://teamcherry.com.au"
}}}`

const infoGOGCatalog = `{"id":"1308320804","title":"Hollow Knight","releaseDate":"2017.02.24",` +
	`"coverVertical":"https://images.gog-statics.com/cover_v.jpg",` +
	`"coverHorizontal":"https://images.gog-statics.com/banner_h.png","logo":"",` +
	`"developers":["Team Cherry"],"publishers":["Team Cherry"],` +
	`"genres":[{"name":"Action"},{"name":"Adventure"}]}`

const infoGOGInfo = `{"title":"Hollow Knight","slug":"hollow_knight",` +
	`"release_date":"2017-02-24T18:55:00+0100",` +
	`"languages":{"en":"English","fr":"fran\u00e7ais"},` +
	`"links":{"product_card":"https://www.gog.com/game/hollow_knight"},` +
	`"description":{"lead":"<b>Lead</b> intro","full":"<p>Full <script>bad()</script></p>"}}`

const infoLutrisGame = `{"name":"Rivage","slug":"rivage","year":2026,` +
	`"description":"A brave game & friends","genres":[{"name":"Adventure"}],` +
	`"coverart":"https://lutris.net/media/igdb/cover_big/coay6k.jpg","provider_games":[]}`

func infoGame(t *testing.T, a *App, id string) types.Game {
	t.Helper()
	return types.Game{ID: id, Source: types.SourceCustom, Store: types.StoreCustom, Title: "Hollow Knight"}
}

// Steam details serve a sideloaded game with a compat AppID, sanitized.
func TestGameInfoSteamDetails(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	f.steamDetail = infoSteamDetail
	a, _, id := newArtApp(t, f, "Hollow Knight")
	if err := a.settings.SetCompatAppID(id, "123"); err != nil {
		t.Fatal(err)
	}
	d, err := a.GetGameDetails(infoGame(t, a, id))
	if err != nil {
		t.Fatal(err)
	}
	if d.Summary != "A brave knight." {
		t.Fatalf("summary = %q", d.Summary)
	}
	if !strings.Contains(d.DescriptionHTML, "Brave") || strings.Contains(d.DescriptionHTML, "evil") || strings.Contains(d.DescriptionHTML, "script") {
		t.Fatalf("description not sanitized: %q", d.DescriptionHTML)
	}
	if len(d.Developers) != 1 || d.Developers[0] != "Team Cherry" {
		t.Fatalf("developers = %v", d.Developers)
	}
	if d.ReleaseDate != "24 Feb, 2017" {
		t.Fatalf("release = %q", d.ReleaseDate)
	}
	if d.InfoSource != "steam" || d.InfoSourceName != "Steam" {
		t.Fatalf("attribution = %q/%q", d.InfoSource, d.InfoSourceName)
	}
	if d.StoreURL != "https://store.steampowered.com/app/123/" {
		t.Fatalf("store url = %q", d.StoreURL)
	}
}

// The compat AppID beats info_source.
func TestGameInfoSourceOrder(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	f.steamDetail = infoSteamDetail
	f.gog = infoGOGCatalog
	f.gogInfo = infoGOGInfo
	f.search = ""
	a, _, id := newArtApp(t, f, "Hollow Knight")
	if _, err := a.SearchArtOnline("Hollow Knight"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.LinkGameInfo(id, "gog", "1308320804"); err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetCompatAppID(id, "123"); err != nil {
		t.Fatal(err)
	}
	d, err := a.GetGameDetails(infoGame(t, a, id))
	if err != nil {
		t.Fatal(err)
	}
	if d.InfoSource != "steam" {
		t.Fatalf("source = %q, want steam to win over gog", d.InfoSource)
	}
	if d.Summary != "A brave knight." {
		t.Fatalf("summary = %q, want the Steam details", d.Summary)
	}
}

// GOG parsing: description sanitized, catalog facts carried, store URL kept.
func TestGameInfoGOGDetails(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	f.search = ""
	f.gog = infoGOGCatalog
	f.gogInfo = infoGOGInfo
	a, _, id := newArtApp(t, f, "Hollow Knight")
	if _, err := a.SearchArtOnline("Hollow Knight"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.LinkGameInfo(id, "gog", "1308320804"); err != nil {
		t.Fatal(err)
	}
	d, err := a.GetGameDetails(infoGame(t, a, id))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.DescriptionHTML, "Full") || strings.Contains(d.DescriptionHTML, "script") {
		t.Fatalf("gog description not sanitized: %q", d.DescriptionHTML)
	}
	if d.Summary != "Lead intro" {
		t.Fatalf("gog summary = %q, want the plain lead", d.Summary)
	}
	if len(d.Developers) != 1 || d.Developers[0] != "Team Cherry" {
		t.Fatalf("gog developers = %v", d.Developers)
	}
	if len(d.Genres) != 2 || d.Genres[0] != "Action" {
		t.Fatalf("gog genres = %v", d.Genres)
	}
	if d.ReleaseDate != "24 Feb, 2017" {
		t.Fatalf("gog release = %q", d.ReleaseDate)
	}
	if len(d.Languages) == 0 {
		t.Fatal("gog languages empty")
	}
	if d.StoreURL != "https://www.gog.com/game/hollow_knight" {
		t.Fatalf("gog store url = %q", d.StoreURL)
	}
	if d.InfoSource != "gog" || d.InfoSourceName != "GOG" {
		t.Fatalf("attribution = %q/%q", d.InfoSource, d.InfoSourceName)
	}
}

// Lutris parsing: plain text in Summary, genres and year, store URL.
func TestGameInfoLutrisDetails(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	f.lutrisGame = infoLutrisGame
	a, _, id := newArtApp(t, f, "Rivage")
	if _, err := a.LinkGameInfo(id, "lutris", "rivage"); err != nil {
		t.Fatal(err)
	}
	d, err := a.GetGameDetails(infoGame(t, a, id))
	if err != nil {
		t.Fatal(err)
	}
	if d.Summary != "A brave game & friends" {
		t.Fatalf("lutris summary = %q", d.Summary)
	}
	if d.DescriptionHTML != "" {
		t.Fatalf("lutris html = %q, want plain Summary only", d.DescriptionHTML)
	}
	if len(d.Genres) != 1 || d.Genres[0] != "Adventure" {
		t.Fatalf("lutris genres = %v", d.Genres)
	}
	if d.ReleaseDate != "2026" {
		t.Fatalf("lutris release = %q", d.ReleaseDate)
	}
	if d.StoreURL != "https://lutris.net/games/rivage" {
		t.Fatalf("lutris store url = %q", d.StoreURL)
	}
	if d.InfoSource != "lutris" || d.InfoSourceName != "Lutris / IGDB" {
		t.Fatalf("attribution = %q/%q", d.InfoSource, d.InfoSourceName)
	}
}

// Applying GOG/Lutris art auto-links the info source when the game has none.
func TestGameInfoAutoLinkFromArt(t *testing.T) {
	t.Parallel()
	t.Run("gog", func(t *testing.T) {
		f := newArtFixture(t)
		png := testPNG(t)
		f.gog = `{"id":"4242","title":"Rivage","releaseDate":"2026.01.01",` +
			`"coverVertical":"` + f.srv.URL + `/gog/cover_4242.jpg",` +
			`"coverHorizontal":"` + f.srv.URL + `/gog/banner_4242.png","logo":"",` +
			`"developers":[],"publishers":[],"genres":[]}`
		f.gogInfo = infoGOGInfo
		f.images["gog/cover_4242.jpg"] = png
		f.images["gog/banner_4242.png"] = png
		a, _, id := newArtApp(t, f, "Rivage")
		if _, err := a.ApplyOnlineArt(id, "gog", "4242"); err != nil {
			t.Fatal(err)
		}
		if got := a.settings.InfoSource(id); got != "gog:4242" {
			t.Fatalf("info source = %q, want gog:4242", got)
		}
	})
	t.Run("lutris", func(t *testing.T) {
		f := newArtFixture(t)
		png := testPNG(t)
		f.lutrisGame = `{"name":"Rivage","slug":"rivage","year":2026,` +
			`"description":"A game","genres":[],` +
			`"coverart":"` + f.srv.URL + `/media/igdb/cover_big/coay6k.jpg","provider_games":[]}`
		f.images["t_cover_big_2x/coay6k.jpg"] = png
		a, _, id := newArtApp(t, f, "Rivage")
		if _, err := a.ApplyOnlineArt(id, "lutris", "rivage"); err != nil {
			t.Fatal(err)
		}
		if got := a.settings.InfoSource(id); got != "lutris:rivage" {
			t.Fatalf("info source = %q, want lutris:rivage", got)
		}
	})
	t.Run("keeps existing", func(t *testing.T) {
		f := newArtFixture(t)
		png := testPNG(t)
		f.gog = `{"id":"4242","title":"Rivage","releaseDate":"2026.01.01",` +
			`"coverVertical":"` + f.srv.URL + `/gog/cover_4242.jpg",` +
			`"coverHorizontal":"` + f.srv.URL + `/gog/banner_4242.png","logo":""}`
		f.images["gog/cover_4242.jpg"] = png
		f.images["gog/banner_4242.png"] = png
		a, _, id := newArtApp(t, f, "Rivage")
		if err := a.settings.SetCompatAppID(id, "123"); err != nil {
			t.Fatal(err)
		}
		if _, err := a.ApplyOnlineArt(id, "gog", "4242"); err != nil {
			t.Fatal(err)
		}
		if got := a.settings.InfoSource(id); got != "" {
			t.Fatalf("info source = %q, want none: compat already gives info", got)
		}
	})
}

// LinkGameInfo: gog/lutris store the link, steam sets the compat AppID with
// the art auto-apply, "" unlinks both.
func TestLinkGameInfo(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	png := testPNG(t)
	f.detail = knightDetail
	f.steamDetail = infoSteamDetail
	f.images["library_600x900.jpg"] = png
	f.images["library_hero.jpg"] = png
	a, h, id := newArtApp(t, f, "My Game")
	game := types.Game{ID: id, Source: types.SourceCustom, Store: types.StoreCustom, Title: "My Game"}

	link, err := a.LinkGameInfo(id, "gog", "1308320804")
	if err != nil || link != "gog:1308320804" {
		t.Fatalf("gog link = %q, %v", link, err)
	}
	if got := a.settings.InfoSource(id); got != "gog:1308320804" {
		t.Fatalf("stored = %q", got)
	}
	if _, err := a.LinkGameInfo(id, "steam", "123"); err != nil {
		t.Fatal(err)
	}
	if got := a.settings.CompatAppID(id); got != "123" {
		t.Fatalf("compat = %q, want 123", got)
	}
	waitForArt(t, a, id)
	waitForEmit(t, h)
	if _, err := a.LinkGameInfo(id, "", ""); err != nil {
		t.Fatal(err)
	}
	if got := a.settings.InfoSource(id); got != "" {
		t.Fatalf("info after unlink = %q", got)
	}
	if got := a.settings.CompatAppID(id); got != "" {
		t.Fatalf("compat after unlink = %q", got)
	}
	_ = game
}

func TestLinkGameInfoBadSource(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	a, _, id := newArtApp(t, f, "My Game")
	if _, err := a.LinkGameInfo(id, "itch", "123"); err == nil {
		t.Fatal("unknown source = nil, want an error")
	}
	if _, err := a.LinkGameInfo("no-such-game", "gog", "1"); err == nil {
		t.Fatal("unknown game = nil, want an error")
	}
}

// With the switch off nothing goes out and linking is refused.
func TestGameInfoOffMakesNoRequests(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	f.steamDetail = infoSteamDetail
	a, _, id := newArtApp(t, f, "Hollow Knight")
	if err := a.settings.SetCompatAppID(id, "123"); err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetGameInfoOnline(false); err != nil {
		t.Fatal(err)
	}
	before := f.hits.Load()
	d, err := a.GetGameDetails(infoGame(t, a, id))
	if err != nil {
		t.Fatal(err)
	}
	if d.Summary != "" || d.InfoSource != "" {
		t.Fatalf("switch off gave %+v, want empty without a request", d)
	}
	if got := f.hits.Load(); got != before {
		t.Fatalf("switch off made %d requests, want none", got-before)
	}
	if _, err := a.LinkGameInfo(id, "gog", "1"); err == nil {
		t.Fatal("link with the switch off = nil, want a refusal")
	}
}

// The disk cache: a second call makes no request, the TTL refreshes, a
// changed link refetches, and offline serves the stale cache.
func TestGameInfoDiskCache(t *testing.T) {
	t.Parallel()
	f := newArtFixture(t)
	f.search = ""
	f.gog = infoGOGCatalog
	f.gogInfo = infoGOGInfo
	a, _, id := newArtApp(t, f, "Hollow Knight")
	if _, err := a.SearchArtOnline("Hollow Knight"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.LinkGameInfo(id, "gog", "1308320804"); err != nil {
		t.Fatal(err)
	}
	game := infoGame(t, a, id)
	first, err := a.GetGameDetails(game)
	if err != nil {
		t.Fatal(err)
	}
	hitsAfterFirst := f.hits.Load()
	if hitsAfterFirst == 0 {
		t.Fatal("first call made no requests")
	}
	second, err := a.GetGameDetails(game)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.hits.Load(); got != hitsAfterFirst {
		t.Fatalf("second call made %d more requests, want none (cache)", got-hitsAfterFirst)
	}
	if second.Summary != first.Summary {
		t.Fatal("cached details differ")
	}
	// The TTL refreshes: an old entry fetches again.
	a.infoCacheMu.Lock()
	e := a.infoCache[id]
	e.FetchedAt = time.Now().Add(-8 * 24 * time.Hour).Unix()
	a.infoCache[id] = e
	a.infoCacheMu.Unlock()
	if _, err := a.GetGameDetails(game); err != nil {
		t.Fatal(err)
	}
	if got := f.hits.Load(); got == hitsAfterFirst {
		t.Fatal("stale TTL made no request, want a refresh")
	}
	// A changed link refetches.
	hitsBeforeRelink := f.hits.Load()
	f.gog = `{"id":"9999","title":"Hollow Knight","releaseDate":"2017.02.24",` +
		`"coverVertical":"https://images.gog-statics.com/cover_v.jpg",` +
		`"coverHorizontal":"https://images.gog-statics.com/banner_h.png","logo":""}`
	if _, err := a.LinkGameInfo(id, "gog", "9999"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetGameDetails(game); err != nil {
		t.Fatal(err)
	}
	if got := f.hits.Load(); got == hitsBeforeRelink {
		t.Fatal("changed link made no request, want a refetch")
	}
	// Offline serves the stale cache without an error.
	f.gogInfoFail = 500
	d, err := a.GetGameDetails(game)
	if err != nil {
		t.Fatalf("offline gave %v, want stale cache, nil", err)
	}
	if d.Summary == "" {
		t.Fatal("offline gave empty details, want the stale cache")
	}
	// No link, no cache: empty details, nil error (no toast).
	a2, _, id2 := newArtApp(t, f, "Fresh")
	d, err = a2.GetGameDetails(infoGame(t, a2, id2))
	if err != nil {
		t.Fatalf("unlinked gave %v, want nil", err)
	}
	if d.Summary != "" || len(d.Screenshots) != 0 {
		t.Fatalf("unlinked gave %+v, want empty", d)
	}
}
