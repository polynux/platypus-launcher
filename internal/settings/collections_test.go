package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/types"
)

func collectionIDs(list []types.Collection) []string {
	ids := make([]string, len(list))
	for i, c := range list {
		ids[i] = c.ID
	}
	return ids
}

func TestCollectionsStartWithThePinnedBuiltIns(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "settings.json"))
	got := s.Collections()
	want := []string{types.CollectionFavourites, types.CollectionInstalled, types.CollectionRecent}
	if ids := collectionIDs(got); strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("built-ins = %v, want %v", ids, want)
	}
	for _, c := range got {
		if !c.Builtin || !c.Pinned {
			t.Fatalf("built-in %q should be builtin and pinned: %+v", c.ID, c)
		}
	}
	if got[0].Smart != nil || got[1].Smart == nil || got[2].Smart == nil {
		t.Fatalf("only the installed and recent built-ins are smart: %+v", got)
	}
}

func TestCollectionsPersistAndKeepMissingGames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, _ := Open(path)
	list, err := s.CreateCollection("  Couch co-op ", types.SmartFilter{})
	if err != nil {
		t.Fatal(err)
	}
	id := list[len(list)-1].ID
	if list[len(list)-1].Name != "Couch co-op" || !list[len(list)-1].Pinned {
		t.Fatalf("new collection = %+v", list[len(list)-1])
	}
	for _, g := range []string{"steam-1", "gone-game", "steam-1"} {
		if _, err := s.AddCollectionGame(id, g); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.RemoveCollectionGame(id, "never-added"); err != nil {
		t.Fatalf("removing a game that isn't there is not an error: %v", err)
	}

	again, _ := Open(path)
	var got types.Collection
	for _, c := range again.Collections() {
		if c.ID == id {
			got = c
		}
	}
	if strings.Join(got.Games, ",") != "steam-1,gone-game" {
		t.Fatalf("games after reload = %v; a game that left the library must stay listed", got.Games)
	}
}

func TestMissingCollectionGameKeepsItsLastKnownTitle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, _ := Open(path)
	list, _ := s.CreateCollection("Couch", types.SmartFilter{})
	id := list[len(list)-1].ID
	_, _ = s.AddCollectionGame(id, "steam-440")
	_, _ = s.AddCollectionGame(id, "steam-730")
	// The library knows both games: their titles are remembered, a game with
	// no title is skipped.
	if _, err := s.RememberCollectionTitles(map[string]string{"steam-440": "Team Fortress 2", "steam-730": "Counter-Strike 2", "other": "Other"}); err != nil {
		t.Fatal(err)
	}
	// steam-730 leaves the library (its plugin is disabled): it stays listed, with its title.
	again, _ := Open(path)
	var got types.Collection
	for _, c := range again.Collections() {
		if c.ID == id {
			got = c
		}
	}
	if strings.Join(got.Games, ",") != "steam-440,steam-730" {
		t.Fatalf("games after reload = %v", got.Games)
	}
	if got.Titles["steam-730"] != "Counter-Strike 2" || got.Titles["steam-440"] != "Team Fortress 2" {
		t.Fatalf("titles after reload = %v", got.Titles)
	}
	if _, ok := got.Titles["other"]; ok {
		t.Fatalf("a title for a game the collection doesn't hold was kept: %v", got.Titles)
	}
	// Removing a game forgets its title.
	if _, err := again.RemoveCollectionGame(id, "steam-730"); err != nil {
		t.Fatal(err)
	}
	for _, c := range again.Collections() {
		if c.ID == id {
			if _, ok := c.Titles["steam-730"]; ok {
				t.Fatalf("a removed game kept its title: %v", c.Titles)
			}
		}
	}
}

func TestCollectionsSurviveRescansByLeavingSettingsAlone(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "settings.json"))
	list, _ := s.CreateCollection("Backlog night", types.SmartFilter{})
	id := list[len(list)-1].ID
	_, _ = s.AddCollectionGame(id, "steam-7")
	// A rescan writes nothing to collections: the list is exactly what was stored.
	got := s.Collections()
	if len(got[len(got)-1].Games) != 1 || got[len(got)-1].Games[0] != "steam-7" {
		t.Fatalf("collection after a no-op read = %+v", got[len(got)-1])
	}
}

func TestSmartCollectionsValidateTheirFilter(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "settings.json"))
	manual, _ := s.CreateCollection("Manual", types.SmartFilter{})
	manualID := manual[len(manual)-1].ID
	yes := true
	cases := map[string]types.SmartFilter{
		"unknown badge":      {BadgeKind: "rating", BadgeValue: "diamond"},
		"unknown deck":       {BadgeKind: "verdict", BadgeValue: "great"},
		"unknown status":     {Status: "dropped"},
		"negative playtime":  {MinPlaytimeMinutes: -1},
		"years backwards":    {YearFrom: 2010, YearTo: 2000},
		"unknown collection": {Collection: "nope"},
	}
	for name, f := range cases {
		if _, err := s.CreateCollection("Bad", f); err == nil {
			t.Errorf("%s: a smart collection was created", name)
		}
	}
	if _, err := s.CreateCollection("Good", types.SmartFilter{Installed: &yes, BadgeKind: "rating", BadgeValue: "native", YearFrom: 1990, YearTo: 2000, Collection: manualID}); err != nil {
		t.Fatalf("a valid smart filter was refused: %v", err)
	}
	smartList, _ := s.CreateCollection("Smart", types.SmartFilter{Installed: &yes})
	smartID := smartList[len(smartList)-1].ID
	if _, err := s.CreateCollection("Nested", types.SmartFilter{Collection: smartID}); err == nil {
		t.Fatal("a smart filter tested a smart collection")
	}
}

func TestBuiltInsCannotBeRenamedDeletedOrEmptied(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "settings.json"))
	if _, err := s.UpdateCollection(types.CollectionFavourites, "Mine", types.SmartFilter{}); err == nil {
		t.Fatal("a built-in was renamed")
	}
	if _, err := s.DeleteCollection(types.CollectionInstalled); err == nil {
		t.Fatal("a built-in was deleted")
	}
	if _, err := s.PinCollection(types.CollectionInstalled, false); err != nil {
		t.Fatalf("a built-in can be unpinned: %v", err)
	}
}

func TestDeletingACollectionBlockedByASmartFilter(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "settings.json"))
	list, _ := s.CreateCollection("Base", types.SmartFilter{})
	baseID := list[len(list)-1].ID
	list, _ = s.CreateCollection("Derived", types.SmartFilter{Collection: baseID})
	if _, err := s.DeleteCollection(baseID); err == nil {
		t.Fatal("a collection a smart filter uses was deleted")
	}
	derivedID := list[len(list)-1].ID
	if _, err := s.DeleteCollection(derivedID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteCollection(baseID); err != nil {
		t.Fatalf("the collection is free once nothing uses it: %v", err)
	}
}

func TestReorderNeedsEveryCollectionOnce(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "settings.json"))
	list, _ := s.CreateCollection("One", types.SmartFilter{})
	oneID := list[len(list)-1].ID
	ids := collectionIDs(s.Collections())
	if _, err := s.ReorderCollections(ids[:len(ids)-1]); err == nil {
		t.Fatal("an order missing a collection was accepted")
	}
	dup := append(append([]string{}, ids[:len(ids)-1]...), ids[0])
	if _, err := s.ReorderCollections(dup); err == nil {
		t.Fatal("an order with a duplicate was accepted")
	}
	rev := []string{oneID}
	for i := len(ids) - 1; i >= 0; i-- {
		if ids[i] != oneID {
			rev = append(rev, ids[i])
		}
	}
	got, err := s.ReorderCollections(rev)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != oneID {
		t.Fatalf("order after reorder = %v", collectionIDs(got))
	}
}

func TestManualAndSmartDoNotSwitchKinds(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "settings.json"))
	list, _ := s.CreateCollection("Plain", types.SmartFilter{})
	id := list[len(list)-1].ID
	if _, err := s.UpdateCollection(id, "Plain", types.SmartFilter{BadgeKind: "rating", BadgeValue: "gold"}); err == nil {
		t.Fatal("a manual collection became smart")
	}
	if _, err := s.AddCollectionGame(types.CollectionInstalled, "steam-1"); err == nil {
		t.Fatal("a game was added to a smart collection")
	}
}

func TestCollectionsReturnedAreCopies(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "settings.json"))
	list, _ := s.CreateCollection("Copy", types.SmartFilter{})
	id := list[len(list)-1].ID
	_, _ = s.AddCollectionGame(id, "steam-1")
	got := s.Collections()
	got[len(got)-1].Games[0] = "changed"
	if s.Collections()[len(got)-1].Games[0] != "steam-1" {
		t.Fatal("a caller changed the store through the list it was given")
	}
}

func TestSavedTierSmartFilterMigratesToRatingBadge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	body := `{"version":2,"collections":[{"id":"legacy","name":"Legacy","smart":{"tier":"platinum"}}]}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := OpenWithHome(path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var migrated *types.SmartFilter
	for _, c := range s.Collections() {
		if c.ID == "legacy" {
			migrated = c.Smart
		}
	}
	if migrated == nil || migrated.BadgeKind != "rating" || migrated.BadgeValue != "platinum" {
		t.Fatalf("migrated filter = %+v", migrated)
	}
	if err := s.SetRatingsOnline(false); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), `"tier": "platinum"`) || !strings.Contains(string(saved), `"badge_kind": "rating"`) {
		t.Fatalf("saved migration = %s", saved)
	}
}
