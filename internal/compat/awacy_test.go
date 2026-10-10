package compat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// awacyFixture has the shapes games.json uses: a Steam id as a string, an
// Epic id as {namespace, slug}, and one game with neither.
const awacyFixture = `[
 {"name":"Halo: The Master Chief Collection","slug":"halo","native":false,"status":"Supported",
  "anticheats":["Easy Anti-Cheat"],"storeIds":{"steam":"976730"},"dateChanged":"2024-01-19T04:40:56.000Z"},
 {"name":"Fortnite","slug":"fortnite","native":false,"status":"Denied",
  "anticheats":["Easy Anti-Cheat","BattlEye"],"storeIds":{"epic":{"namespace":"fn","slug":"fortnite"}},"dateChanged":"2025-05-01T00:00:00.000Z"},
 {"name":"No Ids","slug":"none","status":"Broken","anticheats":["VAC"],"storeIds":{}}
]`

func TestAntiCheatJoinBySteamAndEpic(t *testing.T) {
	d := &AntiCheatData{Games: mustParseGames(t, awacyFixture)}
	d.index()
	if g, ok := d.BySteam("976730"); !ok || g.Name != "Halo: The Master Chief Collection" {
		t.Fatalf("steam join: %+v ok %v", g, ok)
	}
	if _, ok := d.BySteam("620"); ok {
		t.Fatal("joined an AppID the catalogue doesn't list")
	}
	if g, ok := d.ByEpic("Fortnite"); !ok || g.Status != "Denied" {
		t.Fatalf("epic join by slug: %+v ok %v", g, ok)
	}
	if _, ok := d.ByEpic(""); ok {
		t.Fatal("empty epic id joined")
	}
}

func TestAntiCheatBadgesCarryLinuxStatusAndSource(t *testing.T) {
	d := &AntiCheatData{Games: mustParseGames(t, awacyFixture)}
	checked := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	badges := AntiCheatBadges(d.Games[1], checked)
	if len(badges) != 2 {
		t.Fatalf("badges %+v, want one per anti-cheat", badges)
	}
	for _, b := range badges {
		if b.Kind != "anticheat" || b.Source != AWACYAttribution || b.CheckedAt != checked.Unix() {
			t.Fatalf("badge %+v lacks kind, source or date", b)
		}
		if b.Detail != "Linux: denied by the developer. Last changed in the catalogue 2025-05-01." {
			t.Fatalf("detail %q", b.Detail)
		}
	}
	if got := AntiCheatBadges(d.Games[2], checked)[0].Detail; got[:13] != "Linux: broken" {
		t.Fatalf("broken status: %q", got)
	}
}

func TestAntiCheatStoreRefreshesAndKeepsOldOnFailure(t *testing.T) {
	var fail bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(awacyFixture))
	}))
	defer srv.Close()
	f, _ := testFetcher(srv.Client())
	path := filepath.Join(t.TempDir(), "awacy.json")
	ck := &clock{t: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	store := &AntiCheatStore{Path: path, URL: srv.URL, Fetch: f, Now: ck.now}

	if store.Load() != nil || !store.Stale(nil) {
		t.Fatal("a store with no file has data")
	}
	d, err := store.Refresh(context.Background())
	if err != nil || len(d.Games) != 3 {
		t.Fatalf("refresh: %v, %d games", err, len(d.Games))
	}
	if _, ok := d.BySteam("976730"); !ok {
		t.Fatal("refreshed catalogue has no join index")
	}
	if store.Stale(d) {
		t.Fatal("fresh catalogue marked stale")
	}
	ck.add(AWACYTTL + time.Hour)
	if !store.Stale(d) {
		t.Fatal("a week-old catalogue not marked stale")
	}

	// A new store reads the file back, joins included.
	reopened := &AntiCheatStore{Path: path, URL: srv.URL, Fetch: f, Now: ck.now}
	if g := reopened.Load(); g == nil {
		t.Fatal("saved catalogue not reloaded")
	} else if _, ok := g.BySteam("976730"); !ok {
		t.Fatal("reloaded catalogue has no join index")
	}

	fail = true
	if _, err := store.Refresh(context.Background()); err == nil {
		t.Fatal("a 404 refreshed the catalogue")
	}
	if g := store.Load(); g == nil || len(g.Games) != 3 {
		t.Fatal("a failed refresh dropped the old catalogue")
	}
}

func TestAntiCheatStoreRejectsEmptyCatalogue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	f, _ := testFetcher(srv.Client())
	store := &AntiCheatStore{Path: filepath.Join(t.TempDir(), "a.json"), URL: srv.URL, Fetch: f}
	if _, err := store.Refresh(context.Background()); err == nil {
		t.Fatal("an empty games.json was accepted")
	}
	if _, err := os.Stat(store.Path); err == nil {
		t.Fatal("an empty catalogue was saved")
	}
}

func mustParseGames(t *testing.T, body string) []AntiCheatGame {
	t.Helper()
	var games []AntiCheatGame
	if err := json.Unmarshal([]byte(body), &games); err != nil {
		t.Fatal(err)
	}
	return games
}
