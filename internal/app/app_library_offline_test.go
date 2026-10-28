package app

// Last good listings (issue #44): when a source fails, its games come from
// lastGood or the on-disk snapshot, with a stale SourceWarning, so the
// library survives an offline restart. A Steam failure is a normal warning.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/stores"
	"platypus/internal/types"
)

type offlineFakeSource struct {
	id    string
	games []types.Game
	err   error
}

func (f offlineFakeSource) ID() string { return f.id }
func (f offlineFakeSource) Library(context.Context) ([]types.Game, error) {
	return f.games, f.err
}
func (f offlineFakeSource) Details(context.Context, types.Game) (*types.GameDetails, error) {
	return &types.GameDetails{}, nil
}
func (f offlineFakeSource) Launch(context.Context, types.Game, bool) (*stores.Launch, error) {
	return nil, nil
}

func TestFailedSourceServesLastGoodWithStaleWarning(t *testing.T) {
	t.Parallel()
	src := offlineFakeSource{id: "echo", games: []types.Game{{ID: "p.echo.tiny", Source: "echo", Title: "Tiny"}}}
	a := libraryApp(t, src)
	all := []stores.Store{src}
	// First a success, then a failure: the failure serves the last good
	// listing with a stale warning.
	lib, err := a.assembleLibrary(all, [][]types.Game{src.games}, []error{nil}, []bool{true})
	if err != nil || len(lib.Games) != 1 {
		t.Fatalf("success: %+v, %v", lib, err)
	}
	a.library.set("echo", src.games, nil)
	a.library.set("echo", nil, errors.New("store down"))
	good, ok := a.library.lastGood("echo")
	if !ok || len(good) != 1 {
		t.Fatalf("lastGood = %v, %v", good, ok)
	}
	lib, err = a.assembleLibrary(all, [][]types.Game{good}, []error{errors.New("store down")}, []bool{true})
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Games) != 1 || lib.Games[0].ID != "p.echo.tiny" {
		t.Fatalf("fallen back to %+v", lib.Games)
	}
	if len(lib.Warnings) != 1 || !lib.Warnings[0].Stale || lib.Warnings[0].Source != "echo" {
		t.Fatalf("warnings = %+v, want one stale warning", lib.Warnings)
	}
}

func TestSnapshotSurvivesARestart(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	snapDir := librarySnapshotDir(dataDir)
	games := []types.Game{{ID: "p.echo.tiny", Source: "echo", Title: "Tiny"}}
	saveLibrarySnapshot(snapDir, "echo", games)
	if got := loadLibrarySnapshot(snapDir, "echo"); len(got) != 1 || got[0].ID != "p.echo.tiny" {
		t.Fatalf("snapshot = %+v", got)
	}
	// A fresh process with an empty cache reads the snapshot as lastGood.
	a := &App{ctx: context.Background()}
	a.library.snapDir = snapDir
	if good, ok := a.library.lastGood("echo"); !ok || len(good) != 1 {
		t.Fatalf("lastGood after restart = %v, %v", good, ok)
	}
	// Unsafe source names get no snapshot.
	saveLibrarySnapshot(snapDir, "../evil", games)
	if _, err := os.Stat(filepath.Join(snapDir, "evil.json")); !os.IsNotExist(err) {
		t.Fatal("snapshot escaped its folder")
	}
}

func TestSteamFailureIsANormalWarning(t *testing.T) {
	t.Parallel()
	src := offlineFakeSource{id: types.SourceSteam}
	a := libraryApp(t, src)
	all := []stores.Store{src}
	lib, err := a.assembleLibrary(all, [][]types.Game{nil}, []error{errors.New("no steam")}, []bool{true})
	if err != nil {
		t.Fatalf("a Steam failure must not fail the scan: %v", err)
	}
	if len(lib.Warnings) != 1 || lib.Warnings[0].Source != types.SourceSteam || lib.Warnings[0].Stale {
		t.Fatalf("warnings = %+v, want one non-stale Steam warning", lib.Warnings)
	}
}

// An uninstalled store's installed games stay in the library as "store
// missing" (issue #161): rows that were never installed can't launch without
// the store, so they drop out.
func TestMissingStoreGamesStayAsStoreMissing(t *testing.T) {
	t.Parallel()
	a := libraryApp(t)
	a.library.snapDir = t.TempDir()
	installed := types.Game{ID: "p.echo.tiny", Source: "echo", Title: "Tiny", IsInstalled: true, Executable: "/g/tiny/tiny", InstallDir: "/g/tiny"}
	owned := types.Game{ID: "p.echo.huge", Source: "echo", Title: "Huge"}
	saveLibrarySnapshot(a.library.snapDir, "echo", []types.Game{installed, owned})
	lib, err := a.GetLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Games) != 1 || lib.Games[0].ID != "p.echo.tiny" || !lib.Games[0].StoreMissing {
		t.Fatalf("games = %+v, want the installed row flagged store missing", lib.Games)
	}
	if len(lib.Warnings) != 1 || lib.Warnings[0].Source != "echo" || !lib.Warnings[0].Stale {
		t.Fatalf("warnings = %+v, want one stale warning for the missing store", lib.Warnings)
	}
}

// A store that is still installed lists live: its snapshot never doubles as
// "store missing" rows.
func TestPresentStoreSnapshotIsNotStoreMissing(t *testing.T) {
	t.Parallel()
	src := offlineFakeSource{id: "echo", games: []types.Game{{ID: "p.echo.tiny", Source: "echo", Title: "Tiny", IsInstalled: true}}}
	a := libraryApp(t, src)
	a.library.snapDir = t.TempDir()
	saveLibrarySnapshot(a.library.snapDir, "echo", src.games)
	all := []stores.Store{src}
	lib, err := a.assembleLibrary(all, [][]types.Game{src.games}, []error{nil}, []bool{true})
	if err != nil || len(lib.Games) != 1 {
		t.Fatalf("lib = %+v, %v", lib, err)
	}
	if lib.Games[0].StoreMissing {
		t.Fatalf("games = %+v, a present store must not flag store missing", lib.Games)
	}
}
