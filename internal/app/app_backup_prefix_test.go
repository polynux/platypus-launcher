package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/savebackup"
	"platypus/internal/session"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// offlineStore's launch always fails, as it does offline, logged out, or while
// the game is being uninstalled. It counts the launch requests.
type offlineStore struct{ launches *int }

func (s offlineStore) ID() string { return "offlinestore" }
func (s offlineStore) Library(context.Context) ([]types.Game, error) {
	return nil, nil
}
func (s offlineStore) Details(context.Context, types.Game) (*types.GameDetails, error) {
	return nil, nil
}
func (s offlineStore) Launch(context.Context, types.Game, bool) (*stores.Launch, error) {
	*s.launches++
	return nil, errors.New("the store is offline")
}

// A backup finds a game's saves in its prefix without asking the store for a
// launch (#74): the prefix comes from the runner plugin's layout on disk.
func TestBackupGetsThePrefixWithoutTheStore(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	a.ctx = context.Background()
	a.sessions = session.NewTracker(func(types.SessionEvent) {})
	launches := 0
	a.stores = stores.NewRegistry(nil, offlineStore{launches: &launches})
	installTestRunnerPlugin(t, a)

	root := filepath.Join(home, "prefix")
	if err := os.MkdirAll(filepath.Join(root, "pfx", "drive_c"), 0o755); err != nil {
		t.Fatal(err)
	}
	g := types.Game{ID: "offlinestore-1", Source: "offlinestore", StoreAppID: "1", Title: "Offline",
		Settings: types.GameSettings{WinePrefix: root, RunnerID: "no-such-runner"}}

	want := savebackup.Prefix{Dir: filepath.Join(root, "pfx"), User: "steamuser"}
	prefix, err := a.gamePrefix(g)
	if err != nil {
		t.Fatalf("gamePrefix: %v", err)
	}
	if prefix != want {
		t.Fatalf("gamePrefix = %+v, want %+v", prefix, want)
	}
	if ctx := a.backupCtx(g, "", "", prefix); ctx.Roots["prefix"] != want.Dir || !ctx.Windows {
		t.Fatalf("backup context = %+v, want the prefix %q", ctx, want.Dir)
	}

	// The gate itself runs with the store failing: it resolves the prefix and
	// finds no save paths, which is not an error.
	gate, err := a.backupGate(context.Background(), g, "manual", true)
	if err != nil {
		t.Fatalf("backupGate: %v", err)
	}
	if !gate.NoPaths {
		t.Fatalf("gate = %+v, want no known save paths", gate)
	}
	if launches != 0 {
		t.Fatalf("the store was asked for %d launches; backups must not ask it", launches)
	}
}
