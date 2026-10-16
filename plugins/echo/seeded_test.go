package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

// A seeded library lists the same facts every run: same seed, same library.
func TestSeededLibraryIsDeterministic(t *testing.T) {
	t.Setenv("ECHO_GAMES", "25")
	t.Setenv("ECHO_SEED", "42")

	first := bulkGames()
	second := bulkGames()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("bulk games differ between identical runs:\nfirst  %+v\nsecond %+v", first[0], second[0])
	}

	var played, unplayed, withStatus, native int
	for _, e := range first {
		if e["installed"] == true {
			// installed pattern is ECHO_GAMES's own (every fourth).
		}
		if lp, _ := e["lastPlayed"].(int64); lp > 0 {
			played++
		} else {
			unplayed++
		}
		if st, _ := e["status"].(string); st != "" {
			withStatus++
			switch st {
			case "backlog", "playing", "finished", "abandoned":
			default:
				t.Fatalf("bulk game has status %q, want a 1.16 value", st)
			}
		}
		if e["native"] == true {
			native++
		}
	}
	if played == 0 || unplayed == 0 {
		t.Fatalf("seeded lastPlayed: played %d, unplayed %d, want both", played, unplayed)
	}
	if withStatus == 0 {
		t.Fatal("no seeded game carries a status hint")
	}
	if native == 0 {
		t.Fatal("no seeded game is native")
	}

	// Store playtime is reported for some bulk games and stable.
	seen := map[string]int{}
	for _, id := range []string{"bulk-0000", "bulk-0001", "bulk-0002"} {
		i, ok := seededBulkIndex(id)
		if !ok {
			t.Fatalf("no index for %s", id)
		}
		if m1, r1 := seededBulkMinutes(42, i); r1 {
			if m2, r2 := seededBulkMinutes(42, i); !r2 || m1 != m2 {
				t.Fatalf("playtime for %s unstable: %d/%d", id, m1, m2)
			}
			seen[id] = m1
		}
	}
	if len(seen) == 0 {
		t.Fatal("no seeded bulk game reports playtime")
	}
	if got, err := playtimeList(); err != nil {
		t.Fatal(err)
	} else {
		raw, _ := json.Marshal(got)
		var reply struct {
			Games []struct {
				ID      string `json:"id"`
				Minutes int    `json:"minutes"`
			} `json:"games"`
		}
		if err := json.Unmarshal(raw, &reply); err != nil {
			t.Fatal(err)
		}
		byID := map[string]int{}
		for _, g := range reply.Games {
			byID[g.ID] = g.Minutes
		}
		for id, want := range seen {
			if byID[id] != want {
				t.Fatalf("playtime.list[%s] = %d, want seeded %d", id, byID[id], want)
			}
		}
	}

	// Details carry the seeded releaseDate/compat facts, stably.
	d1 := seededBulkDetails("bulk-0001")
	d2 := seededBulkDetails("bulk-0001")
	if !reflect.DeepEqual(d1, d2) {
		t.Fatalf("seeded details unstable: %+v vs %+v", d1, d2)
	}
	if d1["releaseDate"] == nil || d1["platforms"] == nil {
		t.Fatalf("seeded details miss facts: %+v", d1)
	}
}

// Without ECHO_SEED the bulk games are exactly what ECHO_GAMES always
// listed: no lastPlayed, status or seeded details.
func TestBulkGamesWithoutSeedAreUnchanged(t *testing.T) {
	t.Setenv("ECHO_GAMES", "8")
	t.Setenv("ECHO_SEED", "")
	for _, e := range bulkGames() {
		if _, ok := e["lastPlayed"]; ok {
			t.Fatalf("unseeded bulk game carries lastPlayed: %+v", e)
		}
		if _, ok := e["status"]; ok {
			t.Fatalf("unseeded bulk game carries status: %+v", e)
		}
	}
	if d := seededBulkDetails("bulk-0001"); d != nil {
		t.Fatalf("unseeded details = %+v, want nil", d)
	}
}

// Explicit ECHO_COMPAT facts win over the seeded ones for bulk games.
func TestExplicitCompatWinsOverSeeded(t *testing.T) {
	t.Setenv("ECHO_SEED", "7")
	compatMu.Lock()
	compatFacts["bulk-0002"] = map[string]any{"steamAppId": "99999"}
	compatMu.Unlock()
	defer func() {
		compatMu.Lock()
		delete(compatFacts, "bulk-0002")
		compatMu.Unlock()
	}()
	seeded := seededBulkDetails("bulk-0002")
	merged := map[string]any{}
	for k, v := range seeded {
		merged[k] = v
	}
	for k, v := range compatFor("bulk-0002") {
		merged[k] = v
	}
	if merged["steamAppId"] != "99999" {
		t.Fatalf("merged steamAppId = %v, want the explicit 99999", merged["steamAppId"])
	}
}
