package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"platypus/internal/compat"
	"platypus/internal/dataplug"
	"platypus/internal/types"
)

// Compat facts (internal/compat, protocol 1.10) around the echo test plugin.
// Echo's "tiny" game carries a Steam AppID hint, Linux among its platforms and
// a DRM block (plugins/echo/compat.go). ProtonDB is a fake: no test touches
// the network.

// fakeRatingSource answers ProtonDB from a table and counts the asks.
type fakeRatingSource struct {
	mu    sync.Mutex
	sums  map[string]compat.Summary
	calls int
}

func (f *fakeRatingSource) Summary(_ context.Context, appID string) (compat.Summary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	s, ok := f.sums[appID]
	if !ok {
		return compat.Summary{}, compat.ErrNotFound
	}
	return s, nil
}

func (f *fakeRatingSource) asks() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// compatEnv is the echo app at the given protocol list, with a fake ProtonDB
// and an AreWeAntiCheatYet server that has nothing (404).
func compatEnv(t *testing.T, protocols string) (*versionsEnv, *fakeRatingSource) {
	t.Helper()
	env := versionsApp(t, protocols, `["library","install","auth","discard"]`)
	src := &fakeRatingSource{sums: map[string]compat.Summary{
		"620": {Tier: "platinum", Confidence: "strong", Total: 343, Score: 0.88},
		"730": {Tier: "gold", Confidence: "strong", Total: 2},
	}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	cache, err := compat.OpenCache("")
	if err != nil {
		t.Fatal(err)
	}
	env.a.data = dataplug.NewRegistry()
	env.a.compat = &compat.Service{
		Cache:     cache,
		Ratings:   src,
		Online:    env.a.settings.RatingsOnline,
		Now:       time.Now,
		Scan:      func(string) ([]compat.LocalHit, bool) { return nil, false },
		AntiCheat: &compat.AntiCheatStore{URL: srv.URL, Fetch: compat.NewFetcher("platypus-test")},
	}
	return env, src
}

func kinds(bs []types.CompatBadge) map[string]bool {
	out := map[string]bool{}
	for _, b := range bs {
		out[b.Kind] = true
	}
	return out
}

func TestCompatBelow110ReadsNoPluginFacts(t *testing.T) {
	env, src := compatEnv(t, `["1.0","1.1","1.9"]`)
	if got := env.m.Runtime("echo").Protocol; got != "1.9" {
		t.Fatalf("negotiated %q, want 1.9", got)
	}
	c := env.a.GetGameCompat(env.game)
	if c.SteamAppID != "" || c.Native || len(c.DRM) != 0 || c.DRMNotes != "" || c.ProtonDB != nil {
		t.Fatalf("a 1.9 plugin's facts were read: %+v", c)
	}
	if src.asks() != 0 {
		t.Fatalf("asked ProtonDB with no AppID (%d asks)", src.asks())
	}
}

func TestCompatAt110ReadsHintPlatformsAndDRM(t *testing.T) {
	env, _ := compatEnv(t, `["1.0","1.1","1.10"]`)
	if got := env.m.Runtime("echo").Protocol; got != "1.10" {
		t.Fatalf("negotiated %q, want 1.10", got)
	}
	c := env.a.GetGameCompat(env.game)
	if c.SteamAppID != "620" || c.SteamAppIDSource != "plugin" {
		t.Fatalf("AppID %q from %q, want 620 from the plugin", c.SteamAppID, c.SteamAppIDSource)
	}
	if !c.Native {
		t.Fatal("platforms with linux should show a native game")
	}
	if c.ProtonDB == nil || c.ProtonDB.Tier != "platinum" || c.ProtonDB.URL != "https://www.protondb.com/app/620" {
		t.Fatalf("ProtonDB %+v", c.ProtonDB)
	}
	got := kinds(c.DRM)
	for _, k := range []string{"online", "launcher", "denuvo"} {
		if !got[k] {
			t.Errorf("no %s badge in %+v", k, c.DRM)
		}
	}
	if got["drm-free"] {
		t.Error("drmFree false became a DRM-free badge")
	}
	for _, b := range c.DRM {
		if b.Source != "Echo" || b.CheckedAt == 0 {
			t.Errorf("badge %+v lacks its source or date", b)
		}
	}
	if c.DRMNotes == "" {
		t.Error("the store's notes were dropped")
	}
}

func TestUserAppIDBeatsPluginHint(t *testing.T) {
	env, src := compatEnv(t, `["1.0","1.1","1.10"]`)
	c, err := env.a.SetGameSteamAppID(env.game, "730")
	if err != nil {
		t.Fatal(err)
	}
	if c.SteamAppID != "730" || c.SteamAppIDSource != "user" {
		t.Fatalf("AppID %q from %q, want the user's 730", c.SteamAppID, c.SteamAppIDSource)
	}
	if c.ProtonDB == nil || !c.ProtonDB.FewReports {
		t.Fatalf("730 has two reports: want few reports, got %+v", c.ProtonDB)
	}
	if src.asks() == 0 {
		t.Fatal("the user's AppID wasn't asked of ProtonDB")
	}
	// Clearing it falls back to the plugin's hint.
	c, err = env.a.SetGameSteamAppID(env.game, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.SteamAppID != "620" || c.SteamAppIDSource != "plugin" {
		t.Fatalf("after clearing: %q from %q", c.SteamAppID, c.SteamAppIDSource)
	}
}

func TestNoTitleMatchWithoutAnAppID(t *testing.T) {
	env, src := compatEnv(t, `["1.0","1.1","1.9"]`)
	// Same title, an Epic store, no hint, no user choice: nothing to look up.
	game := env.game
	game.Store = types.StoreEpic
	game.StoreAppID = "echo-tiny-epic"
	if c := env.a.GetGameCompat(game); c.SteamAppID != "" || c.ProtonDB != nil {
		t.Fatalf("matched by title: %+v", c)
	}
	if src.asks() != 0 {
		t.Fatal("asked ProtonDB without an AppID")
	}
	if fix := env.a.GetGameFix(game); fix != nil {
		t.Fatalf("a fix from no AppID: %+v", fix)
	}
}

func TestGameFixShowsOnlyEnoughReports(t *testing.T) {
	env, _ := compatEnv(t, `["1.0","1.1","1.10"]`)
	fix := env.a.GetGameFix(env.game)
	if fix == nil || fix.Source != "protondb" || fix.Tier != "platinum" {
		t.Fatalf("fix %+v, want the platinum tier of 620", fix)
	}
	if _, err := env.a.SetGameSteamAppID(env.game, "730"); err != nil {
		t.Fatal(err)
	}
	if fix := env.a.GetGameFix(env.game); fix != nil {
		t.Fatalf("two reports shown as a tier: %+v", fix)
	}
}

func TestRatingsSwitchOffMakesNoRequests(t *testing.T) {
	env, src := compatEnv(t, `["1.0","1.1","1.10"]`)
	ls, err := env.a.SetRatingsOnline(false)
	if err != nil {
		t.Fatal(err)
	}
	if ls.RatingsOnline {
		t.Fatal("settings still say online")
	}
	c := env.a.GetGameCompat(env.game)
	if c.RatingsOnline || c.ProtonDB != nil {
		t.Fatalf("offline with an empty cache got %+v", c.ProtonDB)
	}
	if src.asks() != 0 {
		t.Fatalf("asked the network with the switch off (%d asks)", src.asks())
	}
	if ls, err := env.a.SetRatingsOnline(true); err != nil || !ls.RatingsOnline {
		t.Fatalf("switching back on: %+v %v", ls, err)
	}
}

// fakeWikiSource answers PCGamingWiki from a table (a Steam AppID to a record).
type fakeWikiSource struct {
	recs map[string]compat.WikiRecord
}

func (f *fakeWikiSource) Lookup(_ context.Context, appID string) (compat.WikiRecord, error) {
	r, ok := f.recs[appID]
	if !ok {
		return compat.WikiRecord{}, compat.ErrNotFound
	}
	return r, nil
}

func TestWikiFactsMergeWithThePluginAndCreditTheWiki(t *testing.T) {
	env, _ := compatEnv(t, `["1.0","1.1","1.10"]`)
	env.a.compat.Wiki = &fakeWikiSource{recs: map[string]compat.WikiRecord{
		"620": {
			Title:     "Portal 2",
			DRM:       []string{"Steam"},
			Removed:   []compat.RemovedDRM{{Name: "Denuvo", Date: "2023-04-10"}},
			AntiCheat: []string{"Easy Anti-Cheat"},
		},
	}}
	c := env.a.GetGameCompat(env.game)
	// The plugin's own DRM facts win over the wiki's current DRM; the removed
	// DRM is history, so it is shown with its date.
	drm := kinds(c.DRM)
	if drm["drm"] {
		t.Errorf("the wiki's current DRM shown beside the plugin's own: %+v", c.DRM)
	}
	if !drm["drm-removed"] {
		t.Errorf("removed DRM not shown: %+v", c.DRM)
	}
	ac := c.AntiCheat
	if len(ac) != 1 || ac[0].Label != "Easy Anti-Cheat" || ac[0].Source != "PCGamingWiki" {
		t.Fatalf("anti-cheat %+v, want the wiki's name", ac)
	}
	if c.Wiki == nil || c.Wiki.Title != "Portal 2" || c.Wiki.URL != "https://www.pcgamingwiki.com/wiki/Portal_2" || c.Wiki.FetchedAt == 0 || c.Wiki.LicenceURL != compat.PCGWLicenceURL {
		t.Fatalf("credit %+v", c.Wiki)
	}
}

func TestNoWikiPageMeansNoWikiFacts(t *testing.T) {
	env, _ := compatEnv(t, `["1.0","1.1","1.10"]`)
	env.a.compat.Wiki = &fakeWikiSource{recs: map[string]compat.WikiRecord{}}
	c := env.a.GetGameCompat(env.game)
	if c.Wiki != nil {
		t.Fatalf("a credit with no page: %+v", c.Wiki)
	}
	for _, b := range append(c.DRM, c.AntiCheat...) {
		if b.Source == "PCGamingWiki" {
			t.Fatalf("a PCGamingWiki badge without a page: %+v", b)
		}
	}
}
