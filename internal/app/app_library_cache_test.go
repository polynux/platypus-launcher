package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/dataplug"
	"platypus/internal/jobs"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// countingSource is a store that counts its Library calls.
type countingSource struct {
	fakeSource
	calls atomic.Int64
}

func (f *countingSource) Library(ctx context.Context) ([]types.Game, error) {
	f.calls.Add(1)
	return f.fakeSource.Library(ctx)
}

func cacheApp(t *testing.T, srcs ...stores.Store) *App {
	t.Helper()
	return libraryApp(t, srcs...)
}

func gameOf(source, id, title string) types.Game {
	return types.Game{ID: source + "_" + id, Source: source, Title: title}
}

// A second GetLibrary right after the first lists nothing again: every
// source is served from the cache.
func TestLibraryCacheServesFreshSources(t *testing.T) {
	t.Parallel()
	a := cacheApp(t,
		&countingSource{fakeSource: fakeSource{id: "a", games: []types.Game{gameOf("a", "1", "One")}}},
		&countingSource{fakeSource: fakeSource{id: "b", games: []types.Game{gameOf("b", "1", "Uno")}}},
	)
	a.libraryEmit = func(types.Library) {}
	lib, err := a.GetLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Games) != 2 {
		t.Fatalf("first scan has %d games, want 2", len(lib.Games))
	}
	again, err := a.GetLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Games) != 2 {
		t.Fatalf("cached scan has %d games, want 2", len(again.Games))
	}
	for _, s := range a.stores.All() {
		if n := s.(*countingSource).calls.Load(); n != 1 {
			t.Fatalf("source %s listed %d times for two scans, want 1", s.ID(), n)
		}
	}
}

// Invalidating one source re-lists only that source: the other's new games
// stay unseen until its own entry goes stale.
func TestLibraryCacheRefreshesOnlyTheInvalidatedSource(t *testing.T) {
	t.Parallel()
	sa := &countingSource{fakeSource: fakeSource{id: "a", games: []types.Game{gameOf("a", "1", "One")}}}
	sb := &countingSource{fakeSource: fakeSource{id: "b", games: []types.Game{gameOf("b", "1", "Uno")}}}
	a := cacheApp(t, sa, sb)
	a.libraryEmit = func(types.Library) {}
	if _, err := a.GetLibrary(); err != nil {
		t.Fatal(err)
	}
	sa.games = []types.Game{gameOf("a", "1", "One"), gameOf("a", "2", "Two")}
	sb.games = []types.Game{gameOf("b", "1", "Uno"), gameOf("b", "2", "Dos")}
	a.invalidateLibrary("b")
	lib, err := a.GetLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Games) != 3 {
		t.Fatalf("scan has %d games, want 3 (a's new game still cached away)", len(lib.Games))
	}
	if n := sa.calls.Load(); n != 1 {
		t.Fatalf("source a listed %d times, want 1", n)
	}
	if n := sb.calls.Load(); n != 2 {
		t.Fatalf("source b listed %d times, want 2", n)
	}
}

// Entries older than the TTL are listed again, so a missed invalidation heals.
func TestLibraryCacheExpiresAfterTTL(t *testing.T) {
	t.Parallel()
	sa := &countingSource{fakeSource: fakeSource{id: "a", games: []types.Game{gameOf("a", "1", "One")}}}
	a := cacheApp(t, sa)
	a.libraryEmit = func(types.Library) {}
	a.library.ttl = 20 * time.Millisecond
	if _, err := a.GetLibrary(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	if _, err := a.GetLibrary(); err != nil {
		t.Fatal(err)
	}
	if n := sa.calls.Load(); n != 2 {
		t.Fatalf("source a listed %d times, want 2", n)
	}
}

// A manual rescan lists every source again, whatever the cache holds.
func TestRescanLibraryListsEverything(t *testing.T) {
	t.Parallel()
	sa := &countingSource{fakeSource: fakeSource{id: "a", games: []types.Game{gameOf("a", "1", "One")}}}
	sb := &countingSource{fakeSource: fakeSource{id: "b", games: []types.Game{gameOf("b", "1", "Uno")}}}
	a := cacheApp(t, sa, sb)
	a.libraryEmit = func(types.Library) {}
	if _, err := a.GetLibrary(); err != nil {
		t.Fatal(err)
	}
	sa.games = []types.Game{gameOf("a", "1", "One"), gameOf("a", "2", "Two")}
	sb.games = []types.Game{gameOf("b", "1", "Uno"), gameOf("b", "2", "Dos")}
	lib, err := a.RescanLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Games) != 4 {
		t.Fatalf("rescan has %d games, want 4", len(lib.Games))
	}
	if n := sa.calls.Load(); n != 2 {
		t.Fatalf("source a listed %d times, want 2", n)
	}
	if n := sb.calls.Load(); n != 2 {
		t.Fatalf("source b listed %d times, want 2", n)
	}
}

// A finished job drops its source from the cache: the next scan re-lists
// that source and serves the rest cached.
func TestJobFinishedInvalidatesItsSource(t *testing.T) {
	t.Parallel()
	sa := &countingSource{fakeSource: fakeSource{id: "a", games: []types.Game{gameOf("a", "1", "One")}}}
	sb := &countingSource{fakeSource: fakeSource{id: "b", games: []types.Game{gameOf("b", "1", "Uno")}}}
	a := cacheApp(t, sa, sb)
	a.libraryEmit = func(types.Library) {}
	if _, err := a.GetLibrary(); err != nil {
		t.Fatal(err)
	}
	sb.games = []types.Game{gameOf("b", "1", "Uno installed")}
	a.jobFinished(types.Job{Kind: jobs.KindInstall, Source: "b", GameID: "b_1"})
	lib, err := a.GetLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if n := sa.calls.Load(); n != 1 {
		t.Fatalf("source a listed %d times, want 1", n)
	}
	if n := sb.calls.Load(); n != 2 {
		t.Fatalf("source b listed %d times, want 2", n)
	}
	for _, g := range lib.Games {
		if g.Source == "b" && g.Title != "Uno installed" {
			t.Fatalf("stale game served after the job: %+v", g)
		}
	}
}

// Installing then uninstalling through the real queue never serves a stale
// list: the scan after each completion shows what the store reports now, and
// a scan with nothing changed lists nothing again.
func TestLibraryFreshAfterInstallAndUninstall(t *testing.T) {
	t.Parallel()
	e := versionsApp(t, allVersions, echoCaps)
	e.a.data = dataplug.NewRegistry()
	e.a.libraryEmit = func(types.Library) {}
	e.a.sessionRunning = func(string) bool { return false }
	tiny := func(lib types.Library) types.Game {
		for _, g := range lib.Games {
			if g.StoreAppID == "tiny" && g.Source == "echo" {
				return g
			}
		}
		t.Fatal("tiny not in the library")
		return types.Game{}
	}
	lists := func() int { return e.calls(t)["library.list"] }

	lib, err := e.a.GetLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if g := tiny(lib); g.IsInstalled {
		t.Fatalf("tiny installed before the test installed it: %+v", g)
	}
	before := lists()

	job := e.install(t, nil)
	_ = job
	lib, err = e.a.GetLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if g := tiny(lib); !g.IsInstalled {
		t.Fatalf("stale list after the install completed: %+v", g)
	}
	if n := lists() - before; n != 1 {
		t.Fatalf("the scan after the install listed echo %d times, want 1", n)
	}

	// Nothing changed: the next scan lists nothing again.
	before = lists()
	if _, err := e.a.GetLibrary(); err != nil {
		t.Fatal(err)
	}
	if n := lists() - before; n != 0 {
		t.Fatalf("a quiet scan listed echo %d times, want 0", n)
	}

	// Uninstalling reads the folder through the cache, then the completion
	// drops it: the scan after shows the game as owned, not installed.
	installed := tiny(lib)
	installed.IsInstalled = true
	uj, err := e.a.UninstallGame(installed)
	if err != nil {
		t.Fatal(err)
	}
	e.waitJob(t, uj.ID, jobs.StateDone)
	lib, err = e.a.GetLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if g := tiny(lib); g.IsInstalled {
		t.Fatalf("stale list after the uninstall completed: %+v", g)
	}
	if n := lists() - before; n < 1 {
		t.Fatalf("the scan after the uninstall listed echo %d times, want at least 1", n)
	}
}
