package main

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"platypus/internal/dataplug"
	"platypus/internal/plugins"
	"platypus/internal/settings"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// fakeSource is a store whose library is answered when the test says so.
type fakeSource struct {
	id    string
	games []types.Game
	err   error
	gate  chan struct{} // nil: answers at once
}

func (f *fakeSource) ID() string { return f.id }
func (f *fakeSource) Library(ctx context.Context) ([]types.Game, error) {
	if f.gate != nil {
		<-f.gate
	}
	return f.games, f.err
}
func (f *fakeSource) Details(context.Context, types.Game) (*types.GameDetails, error) {
	return &types.GameDetails{}, nil
}
func (f *fakeSource) Launch(context.Context, types.Game, bool) (*stores.Launch, error) {
	return nil, nil
}

func libraryApp(t *testing.T, srcs ...stores.Store) *App {
	t.Helper()
	st, err := settings.Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	mgr := plugins.NewManager(plugins.Options{Dir: filepath.Join(root, "plugins"), LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: st})
	t.Cleanup(mgr.Shutdown)
	return &App{settings: st, plugins: mgr, ctx: context.Background(), data: dataplug.NewRegistry(), stores: stores.NewRegistry(nil, srcs...)}
}

// Steam's local scan is not held up by a slow store: the library of the
// finished sources goes out as soon as one answers, and the scan still returns
// everything once the slow one does.
func TestLibraryScanSendsWhatIsReadyBeforeTheSlowStore(t *testing.T) {
	gate := make(chan struct{})
	steam := &fakeSource{id: types.SourceSteam, games: []types.Game{{ID: "steam_1", Source: types.SourceSteam, Title: "One"}}}
	slow := &fakeSource{id: "slow", games: []types.Game{{ID: "slow_a", Source: "slow", Title: "A"}}, gate: gate}
	a := libraryApp(t, steam, slow)
	var mu sync.Mutex
	var partials []types.Library
	sent := make(chan struct{}, 4)
	a.libraryEmit = func(l types.Library) {
		mu.Lock()
		partials = append(partials, l)
		mu.Unlock()
		sent <- struct{}{}
	}
	type result struct {
		lib types.Library
		err error
	}
	out := make(chan result, 1)
	go func() {
		lib, err := a.GetLibrary()
		out <- result{lib, err}
	}()
	<-sent // while the slow store is still out
	select {
	case <-out:
		t.Fatal("the scan returned before the slow store answered")
	default:
	}
	close(gate)
	r := <-out
	if r.err != nil {
		t.Fatal(r.err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(partials) != 1 || len(partials[0].Games) != 1 || partials[0].Games[0].ID != "steam_1" {
		t.Fatalf("partial = %+v, want Steam's one game", partials)
	}
	if len(r.lib.Games) != 2 {
		t.Fatalf("final library has %d games, want 2", len(r.lib.Games))
	}
}

// A failing plugin is a warning in the final library and the scan still ends.
func TestLibraryScanStillWarnsAboutAFailingStore(t *testing.T) {
	steam := &fakeSource{id: types.SourceSteam, games: []types.Game{{ID: "steam_1", Source: types.SourceSteam, Title: "One"}}}
	bad := &fakeSource{id: "bad", err: context.DeadlineExceeded}
	a := libraryApp(t, steam, bad)
	a.libraryEmit = func(types.Library) {}
	lib, err := a.GetLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Games) != 1 || len(lib.Warnings) != 1 || lib.Warnings[0].Source != "bad" {
		t.Fatalf("library = %+v", lib)
	}
}
