package app

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
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

// fixturesApp is an App around the real echo plugin with fixture behaviour
// knobs, wired for installs and launches: settings, queue, ledger, sessions
// with an event channel, and the store registry.
type fixturesEnv struct {
	a      *App
	events chan types.SessionEvent
	root   string
}

const fixturesManifest = `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,
	"protocol":{"versions":["1.0","1.1","1.2","1.3","1.4","1.5","1.6","1.7","1.8","1.9","1.10","1.11","1.12","1.13","1.14","1.15","1.16"]},"store":"echo","exec":"bin/platypus-echo-app",
	"capabilities":["library","install","launch","auth","discard","playtime"]}`

func fixturesApp(t *testing.T, env map[string]string) *fixturesEnv {
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
	merged := map[string]string{"ECHO_STEP_MS": "2"}
	for k, v := range env {
		merged[k] = v
	}
	a.plugins = plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: st,
		Home: root, Env: merged,
		OnNotify: a.pluginNotify,
	})
	t.Cleanup(a.plugins.Shutdown)
	installEchoApp(t, pluginsDir)
	if err := os.WriteFile(filepath.Join(pluginsDir, "echo", "plugin.json"), []byte(fixturesManifest), 0o644); err != nil {
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

func (e *fixturesEnv) waitJob(t *testing.T, id, state string) types.Job {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range e.a.jobs.List() {
			if j.ID == id && (j.State == state || j.State == jobs.StateFailed) {
				if j.State != state {
					t.Fatalf("job %s ended %s: %s", id, j.State, j.Error)
				}
				return j
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s never reached %s", id, state)
	return types.Job{}
}

func (e *fixturesEnv) install(t *testing.T, storeID, title string) types.Game {
	t.Helper()
	game := types.Game{ID: stores.GameID("echo", storeID), Source: "echo", Store: "echo",
		StoreAppID: storeID, Title: title, Installable: true}
	j, err := e.a.InstallGame(game)
	if err != nil {
		t.Fatal(err)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p, _ := e.a.plugins.Plugin("echo")
	s := stores.NewPlugin(e.a.plugins, p, func() []string {
		return e.a.plugins.Folders("echo", e.a.settings.GamesFolder())
	})
	lib, err := s.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range lib {
		if g.StoreAppID == storeID {
			if !g.IsInstalled {
				t.Fatalf("%s not installed after its job", storeID)
			}
			return g
		}
	}
	t.Fatalf("%s not listed after install", storeID)
	return types.Game{}
}

// A fixture crash game run through the App leaves the abnormal exit in the
// session log, and the core's crash hints read it back (#46).
func TestFixtureCrashGameProducesHintsThroughTheApp(t *testing.T) {
	e := fixturesApp(t, map[string]string{
		"ECHO_FIXTURES": `{"crash-vcrun":{"mode":"crash","log":"vcruntime","afterMs":200}}`,
	})
	g := e.install(t, "crash-vcrun", "Echo crash-vcrun")
	e.launchUntilFailed(t, g)
	installTestRunnerPlugin(t, e.a)
	got := e.a.GameCrashHints(g)
	if got == nil || len(got.Hints) == 0 {
		t.Fatalf("GameCrashHints = %+v, want the crash hints", got)
	}
	if got.Hints[0].Rule != "vcredist" || got.Hints[0].Param != "vcruntime140_1.dll" {
		t.Fatalf("first hint = %+v, want the vcruntime vcredist hint", got.Hints[0])
	}
	if got.Log == "" {
		t.Fatal("hints name no log")
	}
}

func installTestRunnerPlugin(t *testing.T, a *App) {
	t.Helper()
	dir := filepath.Join(a.plugins.Dir(), "runner")
	bin := filepath.Join(dir, "bin", "platypus-runner")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "build", "-o", bin, "./plugins/runner")
	cmd.Dir = repoPath("")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build runner plugin: %v: %s", err, output)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), mustRead(t, repoPath("plugins/runner/plugin.json")), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.plugins.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := a.plugins.Approve("runner"); err != nil {
		t.Fatal(err)
	}
}

// A fixture that exits 0 at once is a short clean run: no failed session,
// no crash hints.
func TestFixtureCleanExitHasNoHints(t *testing.T) {
	e := fixturesApp(t, map[string]string{
		"ECHO_FIXTURES": `{"quick-ok":{"mode":"exit0"}}`,
	})
	g := e.install(t, "quick-ok", "Echo quick-ok")
	if _, err := e.a.LaunchGame(g); err != nil {
		t.Fatalf("LaunchGame: %v", err)
	}
	deadline := time.After(20 * time.Second)
	for {
		select {
		case ev := <-e.events:
			if ev.GameID == g.ID && (ev.State == "stopped" || ev.State == "failed") {
				if ev.State == "failed" {
					t.Fatalf("a clean exit reported failed: %q", ev.Message)
				}
				if got := e.a.GameCrashHints(g); got != nil {
					t.Fatalf("GameCrashHints = %+v, want nil for a clean run", got)
				}
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for the stopped launch")
		}
	}
}

// A seeded library returns the same facts each run: the same seed lists the
// same library, playtime and details through the real plugin process.
func TestSeededLibraryIsStableThroughTheApp(t *testing.T) {
	e := fixturesApp(t, map[string]string{"ECHO_GAMES": "30", "ECHO_SEED": "7"})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	call := func(method string, params, result any) {
		t.Helper()
		if err := e.a.plugins.Call(ctx, "echo", method, params, result); err != nil {
			t.Fatalf("%s: %v", method, err)
		}
	}
	var first, second struct {
		Games []json.RawMessage `json:"games"`
	}
	call("library.list", nil, &first)
	call("library.list", nil, &second)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("library.list differs between identical runs with the same seed")
	}
	bulk := 0
	var played, withStatus int
	for _, raw := range first.Games {
		var g struct {
			ID         string `json:"id"`
			Installed  bool   `json:"installed"`
			LastPlayed int64  `json:"lastPlayed"`
			Status     string `json:"status"`
		}
		if err := json.Unmarshal(raw, &g); err != nil {
			t.Fatal(err)
		}
		if len(g.ID) < 5 || g.ID[:5] != "bulk-" {
			continue
		}
		bulk++
		if g.LastPlayed > 0 {
			played++
		}
		if g.Status != "" {
			withStatus++
		}
	}
	if bulk != 30 {
		t.Fatalf("bulk games = %d, want 30", bulk)
	}
	if played == 0 || played == bulk {
		t.Fatalf("seeded lastPlayed: %d of %d played, want a mix", played, bulk)
	}
	if withStatus == 0 {
		t.Fatal("no seeded bulk game carries a status hint")
	}

	var pt1, pt2 struct {
		Games []struct {
			ID      string `json:"id"`
			Minutes int    `json:"minutes"`
		} `json:"games"`
	}
	call("playtime.list", nil, &pt1)
	call("playtime.list", nil, &pt2)
	if !reflect.DeepEqual(pt1, pt2) {
		t.Fatal("playtime.list differs between identical runs with the same seed")
	}
	bulkPT := 0
	for _, g := range pt1.Games {
		if len(g.ID) >= 5 && g.ID[:5] == "bulk-" {
			bulkPT++
		}
	}
	if bulkPT == 0 {
		t.Fatal("no seeded bulk game reports playtime")
	}

	var det1, det2 json.RawMessage
	call("game.details", map[string]any{"id": "bulk-0003"}, &det1)
	call("game.details", map[string]any{"id": "bulk-0003"}, &det2)
	if string(det1) != string(det2) {
		t.Fatal("game.details differs between identical runs with the same seed")
	}
	var det struct {
		ReleaseDate string `json:"releaseDate"`
		SteamAppID  string `json:"steamAppId"`
	}
	if err := json.Unmarshal(det1, &det); err != nil {
		t.Fatal(err)
	}
	if det.ReleaseDate == "" || det.SteamAppID == "" {
		t.Fatalf("seeded details miss facts: %s", det1)
	}
}

// launchUntilFailed launches g and waits for its session to end abnormally.
func (e *fixturesEnv) launchUntilFailed(t *testing.T, g types.Game) {
	t.Helper()
	if _, err := e.a.LaunchGame(g); err != nil {
		t.Fatalf("LaunchGame: %v", err)
	}
	deadline := time.After(20 * time.Second)
	for {
		select {
		case ev := <-e.events:
			if ev.GameID == g.ID && ev.State == "failed" {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for the failed launch")
		}
	}
}

// With no prefixes folder chosen, the runner looks for a game's prefix in the
// default folder, the one the launch uses, so winetricks is offered (#46).
func TestCrashHintsOfferWinetricksForTheDefaultPrefixesFolder(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "winetricks"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	e := fixturesApp(t, map[string]string{
		"ECHO_FIXTURES": `{"crash-vcrun":{"mode":"crash","log":"vcruntime","afterMs":200}}`,
	})
	g := e.install(t, "crash-vcrun", "Echo crash-vcrun")
	if e.a.prefixesFolder() != "" {
		t.Fatalf("prefixes folder = %q, want the default", e.a.prefixesFolder())
	}
	prefix := filepath.Join(e.a.effectivePrefixesFolder(), g.ID)
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		t.Fatal(err)
	}
	e.launchUntilFailed(t, g)
	installTestRunnerPlugin(t, e.a)
	got := e.a.GameCrashHints(g)
	if got == nil || !got.Winetricks {
		t.Fatalf("GameCrashHints = %+v, want winetricks offered for the prefix in the default folder", got)
	}
}
