package app

// The `workshop` capability (protocol 1.22): listing a game's subscriptions
// and queueing the sync job that writes them, driven through the echo plugin.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/dataplug"
	"platypus/internal/jobs"
	"platypus/internal/plugins"
	"platypus/internal/settings"
	"platypus/internal/stores"
	"platypus/internal/types"
)

const workshopManifest = `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,
	"protocol":{"versions":["1.0","1.1","1.22"]},"store":"echo","exec":"bin/platypus-echo-app",
	"capabilities":["library","install","launch","auth","workshop"]}`

type workshopEnv struct {
	a *App
	t *testing.T
}

func workshopApp(t *testing.T, manifest string) *workshopEnv {
	t.Helper()
	root := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetGamesFolder(filepath.Join(root, "Games")); err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, ctx: context.Background(), optHome: root, data: dataplug.NewRegistry()}
	pluginsDir := filepath.Join(root, "plugins")
	a.plugins = plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: st,
		Home: root, Env: map[string]string{"ECHO_STEP_MS": "2"}, OnNotify: a.pluginNotify,
	})
	t.Cleanup(a.plugins.Shutdown)
	installEchoApp(t, pluginsDir)
	if err := os.WriteFile(filepath.Join(pluginsDir, "echo", "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.plugins.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := a.plugins.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	a.jobs, err = jobs.Open(filepath.Join(root, "jobs.json"), jobBackend{a}, nil)
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); a.jobs.Run(runCtx) }()
	t.Cleanup(func() { cancel(); <-done })
	a.stores = stores.NewRegistry(a.pluginStores)
	if _, err := a.plugins.Initialize(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	if err := a.plugins.Call(context.Background(), "echo", "auth.complete",
		map[string]any{"fields": map[string]string{"code": "letmein"}}, nil); err != nil {
		t.Fatal(err)
	}
	return &workshopEnv{a: a, t: t}
}

// installedGame installs echo's tiny through the queue and returns the
// library entry the plugin reports (its folder is what SyncWorkshop names).
func (e *workshopEnv) installedGame() types.Game {
	e.t.Helper()
	game := types.Game{ID: stores.GameID("echo", "tiny"), Source: "echo", Store: "echo",
		StoreAppID: "tiny", Title: "Echo Tiny", Installable: true}
	j, err := e.a.InstallGame(game)
	if err != nil {
		e.t.Fatal(err)
	}
	e.waitJob(j.ID)
	lib, err := e.a.GetLibrary()
	if err != nil {
		e.t.Fatal(err)
	}
	for _, g := range lib.Games {
		if g.ID == game.ID {
			return g
		}
	}
	e.t.Fatalf("tiny is not in the library after its install")
	return types.Game{}
}

func (e *workshopEnv) waitJob(id string) types.Job {
	e.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range e.a.jobs.List() {
			if j.ID != id {
				continue
			}
			if j.State == jobs.StateDone {
				return j
			}
			if j.State == jobs.StateFailed || j.State == jobs.StateCanceled {
				e.t.Fatalf("job %s ended %s: %s", id, j.State, j.Error)
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	e.t.Fatalf("job %s never finished: %+v", id, e.a.jobs.List())
	return types.Job{}
}

func TestGameWorkshopListsAndSyncJobWritesIt(t *testing.T) {
	t.Parallel()
	e := workshopApp(t, workshopManifest)
	game := e.installedGame()

	var raw json.RawMessage
	if err := e.a.plugins.Call(context.Background(), "echo", "workshop.list", map[string]any{"id": "tiny"}, &raw); err != nil {
		t.Fatalf("direct workshop.list: %v", err)
	}
	t.Logf("direct reply: %s", raw)
	items, err := e.a.GetGameWorkshop(game)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "900000001" || items[0].Title != "Echo Extra Level" || !items[0].Installed {
		t.Fatalf("items = %+v", items)
	}
	if items[1].NeedsUpdate != true || items[1].SizeBytes != 1<<20 {
		t.Fatalf("item with an update = %+v", items[1])
	}

	j, err := e.a.SyncWorkshop(game)
	if err != nil {
		t.Fatal(err)
	}
	if j.Kind != jobs.KindWorkshop || j.InstallDir != game.InstallDir || j.StoreGameID != "tiny" {
		t.Fatalf("sync job = %+v", j)
	}
	e.waitJob(j.ID)
	if n := echoCalls(t, e.a)["workshop.sync"]; n != 1 {
		t.Fatalf("workshop.sync reached the plugin %d times, want 1", n)
	}

	items, err = e.a.GetGameWorkshop(game)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if !it.Installed || it.NeedsUpdate {
			t.Fatalf("after a sync: %+v", it)
		}
	}
}

func TestGameWorkshopIsOffOverOldProtocolsAndForUninstalledGames(t *testing.T) {
	t.Parallel()
	e := workshopApp(t, strings.Replace(workshopManifest, `,"1.22"`, "", 1))
	game := types.Game{ID: stores.GameID("echo", "tiny"), Source: "echo", Store: "echo",
		StoreAppID: "tiny", Title: "Echo Tiny"}
	if items, err := e.a.GetGameWorkshop(game); items != nil || err != nil {
		t.Fatalf("over 1.19: %v, %v; want nil and no error", items, err)
	}
	if n := echoCalls(t, e.a)["workshop.list"]; n != 0 {
		t.Fatalf("workshop.list was sent over 1.19: %d", n)
	}
	_, err := e.a.SyncWorkshop(game)
	if err == nil {
		t.Fatal("an uninstalled game was queued for a Workshop sync")
	}
	if apperr.CodeOf(err) != "workshop.unsupported" && apperr.CodeOf(err) != "game.notInstalled" {
		t.Fatalf("error code = %q (%v)", apperr.CodeOf(err), err)
	}
}

func TestWorkshopSyncRefusesAPluginWithoutTheCapability(t *testing.T) {
	t.Parallel()
	e := workshopApp(t, strings.Replace(workshopManifest, `,"workshop"`, "", 1))
	game := types.Game{ID: stores.GameID("echo", "tiny"), Source: "echo", Store: "echo",
		StoreAppID: "tiny", Title: "Echo Tiny", IsInstalled: true}
	if _, err := e.a.SyncWorkshop(game); err == nil || apperr.CodeOf(err) != "workshop.unsupported" {
		t.Fatalf("err = %v", err)
	}
}

// Placement (protocol 1.22, issue #214) rides through normalizeWorkshop:
// known kinds and relative display paths survive, anything else is dropped
// so a hostile or stale plugin can't smuggle absolute paths into the UI.
func TestNormalizeWorkshopPlacement(t *testing.T) {
	t.Parallel()
	got := normalizeWorkshop([]plugins.WorkshopItem{
		{ID: "1", Title: "Sub", PlacementKind: "install-subfolder", PlacementPath: "portal2/maps/workshop"},
		{ID: "2", Title: "Client", PlacementKind: "client-only", PlacementPath: "steamapps/workshop/content/730"},
		{ID: "3", Title: "Steam", PlacementKind: "steam-library", PlacementPath: "steamapps/workshop/content/4321"},
		{ID: "4", Title: "Bogus", PlacementKind: "elsewhere", PlacementPath: "workshop"},
		{ID: "5", Title: "Absolute", PlacementKind: "install-subfolder", PlacementPath: "/home/u/workshop"},
		{ID: "6", Title: "Escape", PlacementKind: "steam-library", PlacementPath: "../content/4321"},
		{ID: "7", Title: "Plain"},
	})
	if len(got) != 7 {
		t.Fatalf("items = %+v", got)
	}
	if got[0].PlacementKind != "install-subfolder" || got[0].PlacementPath != "portal2/maps/workshop" {
		t.Fatalf("subfolder = %+v", got[0])
	}
	if got[1].PlacementKind != "client-only" || got[1].PlacementPath != "steamapps/workshop/content/730" {
		t.Fatalf("client-only = %+v", got[1])
	}
	if got[2].PlacementKind != "steam-library" || got[2].PlacementPath != "steamapps/workshop/content/4321" {
		t.Fatalf("steam-library = %+v", got[2])
	}
	for _, tc := range []struct {
		item types.WorkshopItem
		kind string
		path string
	}{
		{got[3], "", "workshop"},
		{got[4], "install-subfolder", ""},
		{got[5], "steam-library", ""},
		{got[6], "", ""},
	} {
		if tc.item.PlacementKind != tc.kind || tc.item.PlacementPath != tc.path {
			t.Fatalf("dropped placement mishandled: %+v", tc.item)
		}
	}
}
