package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/dataplug"
	"platypus/internal/jobs"
	"platypus/internal/ledger"
	"platypus/internal/plugins"
	"platypus/internal/session"
	"platypus/internal/settings"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// wineClientManifest is fixturesManifest speaking protocol 1.20: the
// wine-client launch delegate and the client.setup job (issue #191).
const wineClientManifest = `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,
	"protocol":{"versions":["1.0","1.1","1.2","1.3","1.4","1.5","1.6","1.7","1.8","1.9","1.10","1.11","1.12","1.13","1.14","1.15","1.16","1.17","1.18","1.19","1.20"]},"store":"echo","exec":"bin/platypus-echo-app",
	"capabilities":["library","install","launch","auth","discard"]}`

const fakeRunnerManifest = `{"id":"runner","name":"Fake runner","version":"1.0.0","manifestVersion":1,
	"protocol":{"versions":["1.17"],"transport":"jsonrpc-stdio"},
	"exec":"bin/platypus-fake-runner","capabilities":["runner"]}`

var (
	fakeRunnerOnce sync.Once
	fakeRunnerBin  string
	fakeRunnerErr  error
)

// sharedFakeRunnerBinary builds the fake runner once per test run (like the
// shared echo binary).
func sharedFakeRunnerBinary(t *testing.T) string {
	t.Helper()
	fakeRunnerOnce.Do(func() {
		dir, err := os.MkdirTemp("", "platypus-fake-runner-")
		if err != nil {
			fakeRunnerErr = err
			return
		}
		fakeRunnerBin = filepath.Join(dir, "platypus-fake-runner")
		cmd := exec.Command("go", "build", "-o", fakeRunnerBin, "platypus/internal/app/testdata/fakerunner")
		env := os.Environ()
		for i, kv := range env {
			switch {
			case len(kv) > 5 && kv[:5] == "HOME=":
				env[i] = "HOME=" + realHome
			case len(kv) > 8 && kv[:8] == "GOCACHE=":
				env[i] = "GOCACHE=" + realGoCache
			case len(kv) > 11 && kv[:11] == "GOMODCACHE=":
				env[i] = "GOMODCACHE=" + realModCache
			}
		}
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			fakeRunnerErr = errors.New(string(out) + ": " + err.Error())
		}
	})
	if fakeRunnerErr != nil {
		t.Fatalf("build fake runner: %v", fakeRunnerErr)
	}
	return fakeRunnerBin
}

func installFakeRunner(t *testing.T, pluginsDir string) {
	t.Helper()
	dir := filepath.Join(pluginsDir, "runner")
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin, err := os.ReadFile(sharedFakeRunnerBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "platypus-fake-runner"), bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(fakeRunnerManifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

// wineClientEnv is fixturesApp with a 1.20 echo plugin and the fake runner:
// installs and launches of the fake wine game run through them. env adds
// extra plugin environment (ECHO_SETUP_CONFIRM, ECHO_INSTALLER_SLEEP).
func wineClientEnv(t *testing.T, env map[string]string) *fixturesEnv {
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
	a := &App{settings: st, ctx: context.Background(), optHome: root, data: dataplug.NewRegistry()}
	if a.ledger, err = ledger.Open(filepath.Join(root, "install-history.json")); err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	merged := map[string]string{"ECHO_STEP_MS": "2", "FAKE_RUNNER_RECORD": filepath.Join(root, "runner-record.jsonl")}
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
	if err := os.WriteFile(filepath.Join(pluginsDir, "echo", "plugin.json"), []byte(wineClientManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	installFakeRunner(t, pluginsDir)
	if _, err := a.plugins.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := a.plugins.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	if err := a.plugins.Approve("runner"); err != nil {
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

// wineGameDirect is the same game without the clientLaunch flag: the launch
// spec still delegates to the wine client, so core owns the flow either way.
func wineGameDirect() types.Game {
	g := wineGame()
	g.ClientLaunch = false
	return g
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

func waitJobState(t *testing.T, e *fixturesEnv, id, state string) types.Job {
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

// runnerRecords reads what the fake runner saw: one JSON object per line.
func runnerRecords(t *testing.T, e *fixturesEnv) []map[string]any {
	t.Helper()
	path := filepath.Join(e.root, "runner-record.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []map[string]any
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatal(err)
		}
		out = append(out, rec)
	}
	return out
}

// parentPid reads /proc/<pid>/stat's ppid field (the fake client runs the
// game as a child and waits for it).
func parentPid(t *testing.T, pid int) int {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("no /proc on Windows")
	}
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		t.Fatalf("stat %d: %v", pid, err)
	}
	rest := string(stat[strings.LastIndexByte(string(stat), ')')+2:])
	fields := strings.Fields(rest)
	if len(fields) < 2 {
		t.Fatalf("parse %d: %q", pid, rest)
	}
	var ppid int
	if _, err := fmt.Sscanf(fields[1], "%d", &ppid); err != nil {
		t.Fatalf("parse %d: %v", pid, err)
	}
	return ppid
}

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func killProcess(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}
func findRunActions(recs []map[string]any, id string) []map[string]any {
	var out []map[string]any
	for _, rec := range recs {
		if rec["method"] == "runner.runAction" && rec["id"] == id {
			out = append(out, rec)
		}
	}
	return out
}

// Launching before the client is set up fails with client.notSetUp, not a
// broken spec: setup is what puts the client executable in its prefix.
func TestWineClientLaunchBeforeSetup(t *testing.T) {
	e := wineClientEnv(t, nil)
	_, err := e.a.LaunchGame(wineGame())
	if apperr.CodeOf(err) != "client.notSetUp" || !strings.Contains(err.Error(), "isn't set up yet") {
		t.Fatalf("LaunchGame before setup: %v, want the client.notSetUp prompt", err)
	}
}

func clientPrefix(t *testing.T, e *fixturesEnv) string {
	t.Helper()
	return stores.ClientPrefixDir("echo", e.a.effectivePrefixesFolder())
}

func clientExe(t *testing.T, e *fixturesEnv) string {
	t.Helper()
	return filepath.Join(clientPrefix(t, e), "drive_c", "echo-client", "echo-client.sh")
}

// client.setup installs the fake client into the core-owned prefix, and the
// game then launches through it and stops like any client launch.
func TestWineClientSetupAndLaunch(t *testing.T) {
	e := wineClientEnv(t, nil)
	j, err := e.a.SetupClient("echo", false)
	if err != nil {
		t.Fatalf("SetupClient: %v", err)
	}
	if j.Kind != jobs.KindClientSetup {
		t.Fatalf("job kind = %q, want client-setup", j.Kind)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	if st, err := os.Stat(clientExe(t, e)); err != nil || st.Mode().Perm()&0o111 == 0 {
		t.Fatalf("client executable %s: %v", clientExe(t, e), err)
	}
	if _, ok := e.a.ClientSetupJob("echo"); ok {
		t.Fatal("ClientSetupJob still held after the setup finished")
	}

	// The installer ran under the runner in the pinned client prefix, with
	// the default runner and none of the game's settings.
	wrap := findWrap(t, e, stores.ClientGameID("echo"))
	if wrap["wine_prefix"] != clientPrefix(t, e) {
		t.Fatalf("wrap wine_prefix = %v, want %s", wrap["wine_prefix"], clientPrefix(t, e))
	}
	if wrap["runner_id"] != "" {
		t.Fatalf("wrap runner_id = %v, want the default runner", wrap["runner_id"])
	}
	if opts, _ := wrap["options"].(map[string]any); opts != nil {
		for _, k := range []string{"use_gamemode", "use_mangohud", "use_gamescope", "eac_runtime", "battleye_runtime"} {
			if v, ok := opts[k].(bool); ok && v {
				t.Fatalf("wrap options enable %s for a client launch", k)
			}
		}
		if vars, ok := opts["env_vars"].(map[string]any); ok && len(vars) > 0 {
			t.Fatalf("wrap options carry per-game env vars: %v", vars)
		}
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
	// The client command is logged to the game's session log, like a direct
	// launch (S6).
	logPath := filepath.Join(e.root, "logs", wineGame().ID+".log")
	if data, err := os.ReadFile(logPath); err != nil || !strings.Contains(string(data), "client up") {
		t.Fatalf("session log %s: %v", logPath, err)
	}
	// A second launch while the client runs is already running, not a
	// second client.
	if _, err := e.a.LaunchGame(wineGame()); apperr.CodeOf(err) != "game.running" {
		t.Fatalf("second launch: %v, want game.running", err)
	}
	// The client outlives the game: stopping kills the found game pid and
	// its descendants, never the client (B2).
	parent := parentPid(t, ev.PID)
	if err := e.a.StopGame(wineGame().ID); err != nil {
		t.Fatalf("StopGame: %v", err)
	}
	waitSessionState(t, e, wineGame().ID, "stopped")
	if !processAlive(parent) {
		t.Fatal("stopping the game killed the client")
	}
	_ = killProcess(parent)
	// Stopping never ran the runner's stop action on the prefix.
	for _, rec := range findRunActions(runnerRecords(t, e), "stop") {
		t.Fatalf("stop ran the runner stop action for a wine-client launch: %v", rec)
	}
}

func findWrap(t *testing.T, e *fixturesEnv, gameID string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, rec := range runnerRecords(t, e) {
			if rec["method"] == "runner.wrap" && rec["game_id"] == gameID {
				return rec
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("fake runner never wrapped %s", gameID)
	return nil
}

// While a setup job holds the client, its games refuse to launch: flagged
// games before the launch spec runs, unflagged ones after it proves they
// need the client (S4). A canceled setup leaves nothing to discard (S3).
func TestWineClientLaunchRefusedDuringSetup(t *testing.T) {
	e := wineClientEnv(t, nil)
	// Install once so the client executable exists for the repair below.
	j0, err := e.a.SetupClient("echo", false)
	if err != nil {
		t.Fatalf("SetupClient: %v", err)
	}
	e.waitJob(t, j0.ID, jobs.StateDone)
	t.Setenv("ECHO_INSTALLER_SLEEP", "30")
	j, err := e.a.SetupClient("echo", false)
	if err != nil {
		t.Fatalf("SetupClient: %v", err)
	}
	waitJobState(t, e, j.ID, jobs.StateRunning)
	if _, ok := e.a.ClientSetupJob("echo"); !ok {
		t.Fatal("ClientSetupJob not held while the setup runs")
	}
	if _, err := e.a.LaunchGame(wineGame()); err == nil ||
		!strings.Contains(err.Error(), "being set up") {
		t.Fatalf("flagged launch during setup: %v, want the setup refusal", err)
	}
	if _, err := e.a.LaunchGame(wineGameDirect()); err == nil ||
		!strings.Contains(err.Error(), "being set up") {
		t.Fatalf("unflagged launch during setup: %v, want the setup refusal", err)
	}
	if err := e.a.CancelJob(j.ID); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
	waitJobState(t, e, j.ID, jobs.StateCanceled)
	if err := e.a.DiscardJob(j.ID); !errors.Is(err, jobs.ErrNoData) {
		t.Fatalf("DiscardJob of a setup: %v, want ErrNoData", err)
	}
}

// Refusing a launch while its setup runs is checked on the spec, not the
// library flag (covered above); the hold also lifts once the setup is done.
func TestWineClientSetupHoldsLaunches(t *testing.T) {
	e := wineClientEnv(t, nil)
	j, err := e.a.SetupClient("echo", false)
	if err != nil {
		t.Fatalf("SetupClient: %v", err)
	}
	if _, ok := e.a.ClientSetupJob("echo"); !ok {
		t.Fatal("ClientSetupJob not held while the setup runs")
	}
	if _, err := e.a.SetupClient("echo", false); err == nil {
		t.Fatal("a second SetupClient queued while one runs")
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	if _, ok := e.a.ClientSetupJob("echo"); ok {
		t.Fatal("ClientSetupJob still held after the setup finished")
	}
}

// Setup needs a plugin that offers launches over protocol 1.20: the fixtures
// manifest (1.16) and unknown sources fail clearly.
func TestWineClientSetupGates(t *testing.T) {
	e := fixturesApp(t, nil)
	if _, err := e.a.SetupClient("echo", false); err == nil || !strings.Contains(err.Error(), "older protocol") {
		t.Fatalf("SetupClient on 1.16: %v, want the protocol refusal", err)
	}
	if _, err := e.a.SetupClient("no-such-store", false); err == nil {
		t.Fatal("SetupClient on an unknown source succeeded")
	}
}

// When the installer asks for download confirmation, SetupClient fails with
// client.confirmDownload carrying the plugin's prompt; confirming queues
// the job, which then installs the client.
func TestWineClientSetupConfirm(t *testing.T) {
	e := wineClientEnv(t, map[string]string{"ECHO_SETUP_CONFIRM": "This downloads 800 MB from example.com."})
	_, err := e.a.SetupClient("echo", false)
	if apperr.CodeOf(err) != "client.confirmDownload" || !strings.Contains(err.Error(), "800 MB") {
		t.Fatalf("unconfirmed setup: %v, want the client.confirmDownload prompt", err)
	}
	j, err := e.a.SetupClient("echo", true)
	if err != nil {
		t.Fatalf("confirmed SetupClient: %v", err)
	}
	if !j.Confirmed {
		t.Fatal("queued setup job doesn't carry the confirmation")
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	if _, err := os.Stat(clientExe(t, e)); err != nil {
		t.Fatalf("confirmed setup didn't install the client: %v", err)
	}
}

// Canceling a setup kills the installer and stops the prefix's wineserver,
// and leaves nothing to discard (B1, S3).
func TestWineClientCancelSetup(t *testing.T) {
	e := wineClientEnv(t, nil)
	// The installer runs with the core's environment, not the plugin's.
	t.Setenv("ECHO_INSTALLER_SLEEP", "30")
	j, err := e.a.SetupClient("echo", false)
	if err != nil {
		t.Fatalf("SetupClient: %v", err)
	}
	waitJobState(t, e, j.ID, jobs.StateRunning)
	// The installer is actually running now: its wrap is recorded.
	installerWrap := findWrap(t, e, stores.ClientGameID("echo"))
	if installerWrap["wine_prefix"] != clientPrefix(t, e) {
		t.Fatalf("setup wrap wine_prefix = %v, want %s", installerWrap["wine_prefix"], clientPrefix(t, e))
	}
	if err := e.a.CancelJob(j.ID); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
	waitJobState(t, e, j.ID, jobs.StateCanceled)
	stops := findRunActions(runnerRecords(t, e), "stop")
	if len(stops) == 0 {
		t.Fatal("canceling the setup never stopped the prefix's wineserver")
	}
	if env, _ := stops[0]["launch"].(map[string]any)["env"].(map[string]any); env["WINEPREFIX"] != clientPrefix(t, e) {
		t.Fatalf("wineserver stop ran elsewhere: %v", stops[0])
	}
	if err := e.a.DiscardJob(j.ID); !errors.Is(err, jobs.ErrNoData) {
		t.Fatalf("DiscardJob of a setup: %v, want ErrNoData", err)
	}
}

// Setting up while a session from the client runs is refused (S4).
func TestWineClientSetupRefusedWhileRunning(t *testing.T) {
	e := wineClientEnv(t, nil)
	j, err := e.a.SetupClient("echo", false)
	if err != nil {
		t.Fatalf("SetupClient: %v", err)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	if _, err := e.a.LaunchGame(wineGame()); err != nil {
		t.Fatalf("LaunchGame: %v", err)
	}
	waitSessionState(t, e, wineGame().ID, "running")
	if _, err := e.a.SetupClient("echo", false); apperr.CodeOf(err) != "client.inUse" {
		t.Fatalf("SetupClient while running: %v, want client.inUse", err)
	}
	if err := e.a.StopGame(wineGame().ID); err != nil {
		t.Fatalf("StopGame: %v", err)
	}
	waitSessionState(t, e, wineGame().ID, "stopped")
	if _, err := e.a.SetupClient("echo", false); err != nil {
		t.Fatalf("SetupClient after stop: %v", err)
	}
}
