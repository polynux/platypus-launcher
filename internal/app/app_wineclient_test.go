package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/internal/jobs"
	"platypus/internal/ledger"
	"platypus/internal/plugins"
	"platypus/internal/session"
	"platypus/internal/settings"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// wineClientManifest is fixturesManifest speaking protocol 1.19: the
// wine-client launch delegate and the client.setup job (issue #191).
const wineClientManifest = `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,
	"protocol":{"versions":["1.0","1.1","1.2","1.3","1.4","1.5","1.6","1.7","1.8","1.9","1.10","1.11","1.12","1.13","1.14","1.15","1.16","1.17","1.18","1.19"]},"store":"echo","exec":"bin/platypus-echo-app",
	"capabilities":["library","install","launch","auth","discard"]}`

// wineClientEnv is fixturesApp with a 1.19 echo plugin: installs and launches
// of the fake wine game run through it.
func wineClientEnv(t *testing.T) *fixturesEnv {
	t.Helper()
	root := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetGamesFolder(filepath.Join(root, "Games")); err != nil {
		t.Fatal(err)
	}
	events := make(chan types.SessionEvent, 64)
	a := &App{settings: st, ctx: context.Background(), optHome: root}
	if a.ledger, err = ledger.Open(filepath.Join(root, "install-history.json")); err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	a.plugins = plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: st,
		Home: root, Env: map[string]string{"ECHO_STEP_MS": "2"},
		OnNotify: a.pluginNotify,
	})
	t.Cleanup(a.plugins.Shutdown)
	installEchoApp(t, pluginsDir)
	if err := os.WriteFile(filepath.Join(pluginsDir, "echo", "plugin.json"), []byte(wineClientManifest), 0o644); err != nil {
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
	a.jobs.SetOnInstalled(a.recordInstall)
	a.jobs.SetOnFinished(a.jobFinished)
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); a.jobs.Run(runCtx) }()
	t.Cleanup(func() { cancel(); <-done })
	a.sessions = session.NewTracker(func(e types.SessionEvent) {
		events <- e
		a.playSession(e)
	})
	a.sessions.SetLogDir(filepath.Join(root, "logs"))
	a.stores = stores.NewRegistry(a.pluginStores)
	if err := a.plugins.Call(context.Background(), "echo", "auth.complete",
		map[string]any{"fields": map[string]string{"code": "letmein"}}, nil); err != nil {
		t.Fatal(err)
	}
	return &fixturesEnv{a: a, events: events, root: root}
}

func wineGame() types.Game {
	return types.Game{ID: stores.GameID("echo", "wine"), Source: "echo", Store: "echo",
		StoreAppID: "wine", Title: "Echo Wine Game", IsInstalled: true, ClientLaunch: true}
}

func waitSessionState(t *testing.T, e *fixturesEnv, gameID, state string) types.SessionEvent {
	t.Helper()
	deadline := time.After(30 * time.Second)
	for {
		select {
		case ev := <-e.events:
			if ev.GameID == gameID && ev.State == state {
				return ev
			}
		case <-deadline:
			t.Fatalf("timed out waiting for session %s of %s", state, gameID)
		}
	}
}

// Launching before the client is set up fails with the setup prompt, not a
// broken spec: setup is what puts the client executable in its prefix.
func TestWineClientLaunchBeforeSetup(t *testing.T) {
	e := wineClientEnv(t)
	if _, err := e.a.LaunchGame(wineGame()); err == nil || !strings.Contains(err.Error(), "isn't set up yet") {
		t.Fatalf("LaunchGame before setup: %v, want the setup prompt", err)
	}
}

// client.setup installs the fake client into the core-owned prefix, and the
// game then launches through it and stops like any client launch.
func TestWineClientSetupAndLaunch(t *testing.T) {
	e := wineClientEnv(t)
	j, err := e.a.SetupClient("echo")
	if err != nil {
		t.Fatalf("SetupClient: %v", err)
	}
	if j.Kind != jobs.KindClientSetup {
		t.Fatalf("job kind = %q, want client-setup", j.Kind)
	}
	j = e.waitJob(t, j.ID, jobs.StateDone)
	exe := filepath.Join(stores.ClientPrefixDir("echo", e.a.effectivePrefixesFolder()),
		"drive_c", "echo-client", "echo-client.sh")
	if st, err := os.Stat(exe); err != nil || st.Mode().Perm()&0o111 == 0 {
		t.Fatalf("client executable %s: %v (mode %v)", exe, err, st)
	}
	if _, ok := e.a.ClientSetupJob("echo"); ok {
		t.Fatal("ClientSetupJob still held after the setup finished")
	}

	msg, err := e.a.LaunchGame(wineGame())
	if err != nil {
		t.Fatalf("LaunchGame: %v", err)
	}
	if !strings.Contains(msg, "Echo client") {
		t.Fatalf("launch message = %q, want the client named", msg)
	}
	ev := waitSessionState(t, e, wineGame().ID, "running")
	if ev.PID == 0 {
		t.Fatal("running session names no game pid")
	}
	if err := e.a.StopGame(wineGame().ID); err != nil {
		t.Fatalf("StopGame: %v", err)
	}
	waitSessionState(t, e, wineGame().ID, "stopped")
}

// While a setup job holds the client, its games refuse to launch; queuing a
// second setup refuses too, like any active job.
func TestWineClientSetupHoldsLaunches(t *testing.T) {
	e := wineClientEnv(t)
	j, err := e.a.SetupClient("echo")
	if err != nil {
		t.Fatalf("SetupClient: %v", err)
	}
	if _, ok := e.a.ClientSetupJob("echo"); !ok {
		t.Fatal("ClientSetupJob not held while the setup runs")
	}
	if err := e.a.heldByJob(wineGame()); err == nil || !strings.Contains(err.Error(), "being set up") {
		t.Fatalf("heldByJob during setup: %v, want the setup refusal", err)
	}
	if _, err := e.a.SetupClient("echo"); err == nil {
		t.Fatal("a second SetupClient queued while one runs")
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	if err := e.a.heldByJob(wineGame()); err != nil {
		t.Fatalf("heldByJob after setup: %v", err)
	}
}

// Setup needs a plugin that offers launches over protocol 1.19: the fixtures
// manifest (1.16) and unknown sources fail clearly.
func TestWineClientSetupGates(t *testing.T) {
	e := fixturesApp(t, nil)
	if _, err := e.a.SetupClient("echo"); err == nil || !strings.Contains(err.Error(), "older protocol") {
		t.Fatalf("SetupClient on 1.16: %v, want the protocol refusal", err)
	}
	if _, err := e.a.SetupClient("no-such-store"); err == nil {
		t.Fatal("SetupClient on an unknown source succeeded")
	}
}
