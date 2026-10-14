package app

import (
	"path/filepath"
	"testing"

	"platypus/internal/settings"
	"platypus/internal/stores"
	"platypus/internal/types"
)

func gameStatusApp(t *testing.T) *App {
	t.Helper()
	st, err := settings.Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	return &App{settings: st}
}

func TestGameStatusSetAndGet(t *testing.T) {
	t.Parallel()
	a := gameStatusApp(t)
	if got, err := a.GetGameStatus("g"); err != nil || got != (types.GameStatusInfo{}) {
		t.Fatalf("unset = %+v, %v", got, err)
	}
	got, err := a.SetGameStatus("g", types.GameStatusPlaying)
	if err != nil || got.Status != types.GameStatusPlaying || got.UserNotes != "" {
		t.Fatalf("after SetGameStatus = %+v, %v", got, err)
	}
	got, err = a.SetGameNotes("g", "remember the cave")
	if err != nil || got.Status != types.GameStatusPlaying || got.UserNotes != "remember the cave" {
		t.Fatalf("after SetGameNotes = %+v, %v", got, err)
	}
	// The pair comes back together, so the UI updates without a rescan.
	got, err = a.GetGameStatus("g")
	if err != nil || got.Status != types.GameStatusPlaying || got.UserNotes != "remember the cave" {
		t.Fatalf("get = %+v, %v", got, err)
	}
	if _, err := a.SetGameStatus("g", "wishlist"); err == nil {
		t.Fatal("an unknown status was accepted")
	}
	if got, _ := a.GetGameStatus("g"); got.Status != types.GameStatusPlaying {
		t.Fatalf("a rejected status overwrote the saved one: %+v", got)
	}
	if _, err := a.SetGameNotes("g", "a\x00b"); err == nil {
		t.Fatal("control characters were accepted")
	}
}

func TestGameStatusLibraryCarriesValuesAfterRescan(t *testing.T) {
	t.Parallel()
	src := &fakeSource{id: "echo", games: []types.Game{
		{ID: "p.echo.tiny", Source: "echo", Title: "Tiny"},
		{ID: "p.echo.huge", Source: "echo", Title: "Huge"},
	}}
	a := libraryApp(t, src)
	all := []stores.Store{src}
	if _, err := a.SetGameStatus("p.echo.tiny", types.GameStatusBacklog); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetGameNotes("p.echo.tiny", "play after the DLC"); err != nil {
		t.Fatal(err)
	}
	for range 2 { // the scan and the rescan agree
		lib, err := a.assembleLibrary(all, [][]types.Game{src.games}, []error{nil}, []bool{true})
		if err != nil {
			t.Fatal(err)
		}
		if lib.Games[0].Status != types.GameStatusBacklog || lib.Games[0].UserNotes != "play after the DLC" {
			t.Fatalf("tiny carries %+v", lib.Games[0])
		}
		if lib.Games[1].Status != "" || lib.Games[1].UserNotes != "" {
			t.Fatalf("another game picked up values: %+v", lib.Games[1])
		}
	}
}

func TestGameStatusMissingGameKeepsRows(t *testing.T) {
	t.Parallel()
	src := &fakeSource{id: "echo", games: []types.Game{{ID: "p.echo.tiny", Source: "echo", Title: "Tiny"}}}
	a := libraryApp(t, src)
	if _, err := a.SetGameStatus("p.echo.gone", types.GameStatusFinished); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetGameNotes("p.echo.gone", "uninstalled but loved"); err != nil {
		t.Fatal(err)
	}
	lib, err := a.assembleLibrary([]stores.Store{src}, [][]types.Game{src.games}, []error{nil}, []bool{true})
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Games) != 1 || lib.Games[0].ID != "p.echo.tiny" {
		t.Fatalf("a missing game fabricated a library entry: %+v", lib.Games)
	}
	if got, _ := a.GetGameStatus("p.echo.gone"); got.Status != types.GameStatusFinished || got.UserNotes != "uninstalled but loved" {
		t.Fatalf("rows for a missing game were dropped: %+v", got)
	}
}
