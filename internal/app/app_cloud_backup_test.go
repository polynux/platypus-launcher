package app

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/savebackup"
	"platypus/internal/session"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// fakeSteamStore is a built-in Steam stand-in with a canned Steam Cloud
// answer, without any network.
type fakeSteamStore struct {
	has, cached bool
}

func (f fakeSteamStore) ID() string { return types.SourceSteam }
func (f fakeSteamStore) Library(context.Context) ([]types.Game, error) {
	return nil, nil
}
func (f fakeSteamStore) Details(context.Context, types.Game) (*types.GameDetails, error) {
	return &types.GameDetails{}, nil
}
func (f fakeSteamStore) Launch(context.Context, types.Game, bool) (*stores.Launch, error) {
	return nil, nil
}
func (f fakeSteamStore) HasSteamCloudCached(string) (bool, bool) { return f.has, f.cached }

func withFakeSteam(a *App, has, cached bool) {
	a.stores = stores.NewRegistry(a.pluginStores, fakeSteamStore{has: has, cached: cached})
}

func TestGameHasCloudSyncPluginCases(t *testing.T) {
	t.Parallel()
	a, _ := cloudApp(t, nil, nil)
	g := echoGame(t, a)
	if !a.gameHasCloudSync(g) {
		t.Fatal("echo cloud game with default mode should have cloud sync")
	}
	if err := a.SetCloudPluginMode("echo", types.CloudOff); err != nil {
		t.Fatal(err)
	}
	if a.gameHasCloudSync(g) {
		t.Fatal("plugin mode off should have no cloud sync")
	}
	if err := a.SetCloudPluginMode("echo", types.CloudAuto); err != nil {
		t.Fatal(err)
	}
	if err := a.SetCloudGameMode(g.ID, types.CloudOff); err != nil {
		t.Fatal(err)
	}
	if a.gameHasCloudSync(g) {
		t.Fatal("game mode off should have no cloud sync")
	}
	if err := a.SetCloudGameMode(g.ID, ""); err != nil {
		t.Fatal(err)
	}
	g.Features = []string{"install"}
	if a.gameHasCloudSync(g) {
		t.Fatal("game features without cloud-saves should have no cloud sync")
	}
}

func TestGameHasCloudSyncNeedsCapability(t *testing.T) {
	t.Parallel()
	e := backupsEnv(t) // echo without cloud-saves
	game := types.Game{ID: "p.echo.tiny", Source: "echo", Store: "echo", StoreAppID: "tiny", Title: "Echo Tiny"}
	if e.a.gameHasCloudSync(game) {
		t.Fatal("plugin without cloud-saves should have no cloud sync")
	}
}

func TestGameHasCloudSyncSteam(t *testing.T) {
	t.Parallel()
	a, _ := cloudApp(t, nil, nil)
	steamGame := types.Game{ID: "steam_10", Source: types.SourceSteam, Store: types.StoreSteam, StoreAppID: "10", Title: "Steam Game"}
	// Unknown (nothing cached, no Steam store with an answer): false, no request.
	if a.gameHasCloudSync(steamGame) {
		t.Fatal("uncached Steam game should be unknown=false")
	}
	withFakeSteam(a, true, true)
	if !a.gameHasCloudSync(steamGame) {
		t.Fatal("cached Steam Cloud should be true")
	}
	withFakeSteam(a, false, true)
	if a.gameHasCloudSync(steamGame) {
		t.Fatal("cached no-cloud Steam game should be false")
	}
	withFakeSteam(a, true, false)
	if a.gameHasCloudSync(steamGame) {
		t.Fatal("uncached Steam game should be false")
	}
}

func TestAutoBackupStateRules(t *testing.T) {
	t.Parallel()
	a, _ := cloudApp(t, nil, nil)
	g := echoGame(t, a)
	// A cloud game is off by default with the cloud reason.
	st := a.autoBackupState(g)
	if st.On || st.Reason != "cloud" {
		t.Fatalf("cloud default = %+v, want off/cloud", st)
	}
	// The per-game "on" forces it.
	if err := a.settings.SetGameSaveBackup(g.ID, "on"); err != nil {
		t.Fatal(err)
	}
	if st := a.autoBackupState(g); !st.On {
		t.Fatalf("forced cloud = %+v, want on", st)
	}
	if err := a.settings.SetGameSaveBackup(g.ID, "off"); err != nil {
		t.Fatal(err)
	}
	if st := a.autoBackupState(g); st.On || st.Reason != "off" {
		t.Fatalf("per-game off = %+v", st)
	}
	if err := a.settings.SetGameSaveBackup(g.ID, ""); err != nil {
		t.Fatal(err)
	}

	// A game without cloud sync is on by default (unset after-session means on).
	plain := g
	plain.Features = []string{"install"}
	if st := a.autoBackupState(plain); !st.On {
		t.Fatalf("plain default = %+v, want on", st)
	}
	if err := a.settings.SetBackupAfterSession(false); err != nil {
		t.Fatal(err)
	}
	if st := a.autoBackupState(plain); st.On {
		t.Fatalf("explicit after-session off = %+v, want off", st)
	}
	if err := a.settings.SetGameSaveBackup(plain.ID, "on"); err != nil {
		t.Fatal(err)
	}
	if st := a.autoBackupState(plain); !st.On {
		t.Fatalf("per-game on overrides the switch = %+v", st)
	}
}

func TestAfterSessionDefaultOnSkipsCloud(t *testing.T) {
	t.Parallel()
	a, _ := cloudApp(t, nil, nil)
	a.savebk = savebackup.New(savebackup.Options{
		Settings: a.settings, DataDir: filepath.Join(t.TempDir(), "data"),
		CacheDir: filepath.Join(t.TempDir(), "cache"), Home: t.TempDir(),
	})
	g := echoGame(t, a)
	saves := filepath.Join(t.TempDir(), "saves")
	if err := os.MkdirAll(saves, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(saves, "a.sav"), []byte("progress"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetGameSavePaths(g.ID, []string{saves}); err != nil {
		t.Fatal(err)
	}
	// Cloud game, default settings: skipped, no backup.
	a.backupAfterSession(g)
	if got := a.savebk.List(g.ID); len(got) != 0 {
		t.Fatalf("cloud game backed up: %+v", got)
	}
	// Forced per-game on: backed up.
	if err := a.settings.SetGameSaveBackup(g.ID, "on"); err != nil {
		t.Fatal(err)
	}
	a.backupAfterSession(g)
	if got := a.savebk.List(g.ID); len(got) != 1 {
		t.Fatalf("forced cloud backups = %d, want 1", len(got))
	}
}

func TestGateCloudNoPathsPassesWithoutConfirmation(t *testing.T) {
	t.Parallel()
	a, _ := cloudApp(t, nil, nil)
	g := echoGame(t, a)
	// No save paths known (no user paths, no manifest entry for Echo Tiny).
	gate, err := a.backupBeforeDelete(context.Background(), g, "uninstall", false)
	if err != nil {
		t.Fatalf("cloud gate with no paths should pass, err = %v", err)
	}
	if gate.NoPaths || !gate.Cloud || gate.BackupID != "" {
		t.Fatalf("cloud gate = %+v, want Cloud pass", gate)
	}
	if pv := a.savesPreview(g); pv.NoPaths || !pv.Cloud {
		t.Fatalf("cloud preview = %+v, want Cloud", pv)
	}

	// Without cloud, the same game still asks.
	plain := g
	plain.Features = []string{"install"}
	gate, err = a.backupBeforeDelete(context.Background(), plain, "uninstall", false)
	if err == nil || gate.NoPaths != true {
		t.Fatalf("plain gate = %+v, err = %v, want noPaths error", gate, err)
	}
}

func TestClientSessionRunsAfterSessionHook(t *testing.T) {
	t.Parallel()
	tr := session.NewTracker(func(types.SessionEvent) {})
	done := make(chan struct{})
	var running atomic.Int32
	running.Store(1)
	find := func() int {
		if running.Load() > 0 {
			return 424242
		}
		return 0
	}
	ok := tr.WatchClient("g-client", "Steam", find, 5*time.Second,
		session.WithOnExit(func(session.ExitInfo) { close(done) }))
	if !ok {
		t.Fatal("WatchClient refused")
	}
	time.Sleep(100 * time.Millisecond)
	running.Store(0)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("client onExit hook never ran")
	}
}
