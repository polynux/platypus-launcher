//go:build !windows

package app

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/playlog"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// wineClientOpenManifest is wineClientManifest speaking protocol 1.25:
// client.openSpec for opening the client with no game.
const wineClientOpenManifest = `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,
	"protocol":{"versions":["1.0","1.1","1.2","1.3","1.4","1.5","1.6","1.7","1.8","1.9","1.10","1.11","1.12","1.13","1.14","1.15","1.16","1.17","1.18","1.19","1.20","1.21","1.22","1.23","1.24","1.25"]},"store":"echo","exec":"bin/platypus-echo-app",
	"capabilities":["library","install","launch","auth","discard"]}`
const wineClient124Manifest = `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,
	"protocol":{"versions":["1.0","1.1","1.2","1.3","1.4","1.5","1.6","1.7","1.8","1.9","1.10","1.11","1.12","1.13","1.14","1.15","1.16","1.17","1.18","1.19","1.20","1.21","1.22","1.23","1.24"]},"store":"echo","exec":"bin/platypus-echo-app",
	"capabilities":["library","install","launch","auth","discard"]}`

func clientOpenEnv(t *testing.T, env map[string]string) *fixturesEnv {
	t.Helper()
	return wineClientEnvWith(t, env, wineClientOpenManifest)
}

// pollClientRunning waits until the card reads running (or not), so the
// tests only proceed once the open took effect.
func pollClientRunning(t *testing.T, e *fixturesEnv, want bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		st, err := e.a.ClientState("echo")
		if err != nil {
			t.Fatalf("ClientState: %v", err)
		}
		if st.Running == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("ClientState never read running=%v", want)
}

// The card's status before any setup: a wine-client store reads as
// supported but not set up, with nothing holding it.
func TestClientStateNotSetUp(t *testing.T) {
	e := clientOpenEnv(t, nil)
	st, err := e.a.ClientState("echo")
	if err != nil {
		t.Fatalf("ClientState: %v", err)
	}
	if !st.Supported || st.SetUp || st.Running || st.SetupJob != nil || st.ClientRunBusy {
		t.Fatalf("ClientState = %+v, want supported but not set up and idle", st)
	}
	if _, err := e.a.ClientState("no-such-store"); err == nil {
		t.Fatal("ClientState on an unknown plugin succeeded")
	}
	// A store without the wine-client delegate is not supported.
	f := fixturesApp(t, nil)
	st, err = f.a.ClientState("echo")
	if err != nil {
		t.Fatalf("ClientState: %v", err)
	}
	if st.Supported {
		t.Fatalf("ClientState = %+v, want unsupported on protocol 1.16", st)
	}
}

// A 1.24 wine-client store (no client.openSpec) gets no card, even set up:
// only 1.25 declares the standalone spec.
func TestClientStateUnsupportedOn124(t *testing.T) {
	e := wineClientEnvWith(t, nil, wineClient124Manifest)
	j, err := e.a.SetupClient("echo", false)
	if err != nil {
		t.Fatalf("SetupClient: %v", err)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	st, err := e.a.ClientState("echo")
	if err != nil {
		t.Fatalf("ClientState: %v", err)
	}
	if st.Supported || st.SetUp {
		t.Fatalf("ClientState = %+v, want unsupported on protocol 1.24", st)
	}
}

// After the setup installs the client, the card reads as set up. A 1.20
// plugin (no client.openSpec) still reads as not supported: the core has no
// standalone exe to check.
func TestClientStateSetUp(t *testing.T) {
	e := clientOpenEnv(t, nil)
	j, err := e.a.SetupClient("echo", false)
	if err != nil {
		t.Fatalf("SetupClient: %v", err)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	st, err := e.a.ClientState("echo")
	if err != nil {
		t.Fatalf("ClientState: %v", err)
	}
	if !st.Supported || !st.SetUp {
		t.Fatalf("ClientState = %+v, want supported and set up", st)
	}

	old := wineClientEnv(t, nil)
	j, err = old.a.SetupClient("echo", false)
	if err != nil {
		t.Fatalf("SetupClient: %v", err)
	}
	old.waitJob(t, j.ID, jobs.StateDone)
	st, err = old.a.ClientState("echo")
	if err != nil {
		t.Fatalf("ClientState: %v", err)
	}
	if st.Supported || st.SetUp {
		t.Fatalf("ClientState = %+v, want unsupported on protocol 1.20", st)
	}
}

// Opening a client that was never set up fails with client.notSetUp, like a
// wine-client launch.
func TestClientOpenBeforeSetup(t *testing.T) {
	e := clientOpenEnv(t, nil)
	if _, err := e.a.OpenClient("echo"); apperr.CodeOf(err) != "client.notSetUp" {
		t.Fatalf("OpenClient before setup: %v, want client.notSetUp", err)
	}
}

// Open runs the client's own exe under the runner in the pinned client
// prefix, tracked outside the session tracker: the card reads as running,
// and a second open refuses instead of starting another copy.
func TestClientOpenFlow(t *testing.T) {
	e := clientOpenEnv(t, nil)
	setupEchoClient(t, e)
	t.Cleanup(func() { e.a.stopOpenClient("echo") })
	msg, err := e.a.OpenClient("echo")
	if err != nil {
		t.Fatalf("OpenClient: %v", err)
	}
	if !strings.Contains(msg, "Opened Echo") {
		t.Fatalf("open message = %q, want the client named", msg)
	}
	pollClientRunning(t, e, true)
	// The client ran under the runner in the pinned client prefix, with no
	// per-game settings.
	wrap := findWrap(t, e, stores.ClientGameID("echo"))
	if wrap["wine_prefix"] != clientPrefix(t, e) {
		t.Fatalf("wrap wine_prefix = %v, want %s", wrap["wine_prefix"], clientPrefix(t, e))
	}
	if _, err := e.a.OpenClient("echo"); apperr.CodeOf(err) != "client.running" {
		t.Fatalf("second OpenClient: %v, want client.running", err)
	}
	e.a.stopOpenClient("echo")
	pollClientRunning(t, e, false)
}

// Two Opens at once start a single client: one wins, the other refuses.
func TestClientOpenDouble(t *testing.T) {
	e := clientOpenEnv(t, nil)
	setupEchoClient(t, e)
	t.Cleanup(func() { e.a.stopOpenClient("echo") })
	var wg sync.WaitGroup
	type result struct {
		msg string
		err error
	}
	res := make([]result, 2)
	for i := range res {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res[i].msg, res[i].err = e.a.OpenClient("echo")
		}(i)
	}
	wg.Wait()
	var opened, refused int
	for _, r := range res {
		switch {
		case r.err == nil && strings.Contains(r.msg, "Opened Echo"):
			opened++
		case apperr.CodeOf(r.err) == "client.running":
			refused++
		default:
			t.Fatalf("OpenClient: msg %q err %v, want one open and one refusal", r.msg, r.err)
		}
	}
	if opened != 1 || refused != 1 {
		t.Fatalf("double OpenClient: opened %d refused %d, want 1 and 1", opened, refused)
	}
	pollClientRunning(t, e, true)
}

// The opened client emits no session event: it is not a game, so the UI
// shows no activity row or toast, the tray and MeCard stay quiet, and the
// playlog records nothing.
func TestClientOpenEmitsNoSessionEvent(t *testing.T) {
	e := clientOpenEnv(t, nil)
	setupEchoClient(t, e)
	t.Cleanup(func() { e.a.stopOpenClient("echo") })
	if _, err := e.a.OpenClient("echo"); err != nil {
		t.Fatalf("OpenClient: %v", err)
	}
	pollClientRunning(t, e, true)
	timeout := time.After(2 * time.Second)
	for {
		select {
		case ev := <-e.events:
			if ev.GameID == stores.ClientGameID("echo") {
				t.Fatalf("open client emitted a session event: %+v", ev)
			}
		case <-timeout:
			return
		}
	}
}

// The opened client leaves no playlog row: Played and Recents never name
// the client pseudo id.
func TestClientOpenAddsNoPlaylogEntry(t *testing.T) {
	e := clientOpenEnv(t, nil)
	pl, err := playlog.Open(filepath.Join(e.root, "play-sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	e.a.playlog = pl
	setupEchoClient(t, e)
	t.Cleanup(func() { e.a.stopOpenClient("echo") })
	if _, err := e.a.OpenClient("echo"); err != nil {
		t.Fatalf("OpenClient: %v", err)
	}
	pollClientRunning(t, e, true)
	time.Sleep(time.Second)
	if got := pl.LastPlayed(stores.ClientGameID("echo")); got != 0 {
		t.Fatalf("LastPlayed of the open client = %d, want none", got)
	}
	for _, r := range pl.AllRecent() {
		if r.GameID == stores.ClientGameID("echo") {
			t.Fatalf("playlog holds a row for the open client: %+v", r)
		}
	}
}

// An open client holds no automatic update: the loop still queues other
// games' updates while it runs.
func TestClientOpenDoesNotHoldAutoUpdates(t *testing.T) {
	e := clientOpenEnv(t, nil)
	setupEchoClient(t, e)
	t.Cleanup(func() { e.a.stopOpenClient("echo") })
	if _, err := e.a.OpenClient("echo"); err != nil {
		t.Fatalf("OpenClient: %v", err)
	}
	pollClientRunning(t, e, true)
	g := tinyInstallGame()
	g.IsInstalled = true
	g.UpdateAvailable = true
	g.InstallDir = filepath.Join(e.root, "Games", "echo", "tiny")
	e.a.autoUpdateCheck(types.Library{Games: []types.Game{g}})
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range e.a.jobs.List() {
			if j.GameID == g.ID && j.Kind == jobs.KindUpdate {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("no automatic update queued while the client runs")
}

// While a setup job holds the client, opening refuses with client.busy.
func TestClientOpenRefusedDuringSetup(t *testing.T) {
	e := clientOpenEnv(t, nil)
	t.Setenv("ECHO_INSTALLER_SLEEP", "30")
	j, err := e.a.SetupClient("echo", false)
	if err != nil {
		t.Fatalf("SetupClient: %v", err)
	}
	waitJobState(t, e, j.ID, jobs.StateRunning)
	if _, err := e.a.OpenClient("echo"); apperr.CodeOf(err) != "client.busy" {
		t.Fatalf("OpenClient during setup: %v, want client.busy", err)
	}
	if err := e.a.CancelJob(j.ID); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
	waitJobState(t, e, j.ID, jobs.StateCanceled)
}

// While a clientRun job holds the client, opening refuses with client.busy.
func TestClientOpenRefusedDuringClientRun(t *testing.T) {
	e := clientOpenEnv(t, map[string]string{"ECHO_CLIENT_RUN": "1", "ECHO_CLIENT_RUN_HOLD": "1"})
	setupEchoClient(t, e)
	j, err := e.a.InstallGame(tinyInstallGame())
	if err != nil {
		t.Fatalf("InstallGame: %v", err)
	}
	waitRunning(t, e, j.ID)
	if _, err := e.a.OpenClient("echo"); apperr.CodeOf(err) != "client.busy" {
		t.Fatalf("OpenClient during a clientRun job: %v, want client.busy", err)
	}
	if err := e.a.CancelJob(j.ID); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
	e.waitJob(t, j.ID, jobs.StateCanceled)
}

// A 1.20 plugin has no standalone client to open.
func TestClientOpenOldPluginRejected(t *testing.T) {
	e := wineClientEnv(t, nil)
	setupEchoClient(t, e)
	if _, err := e.a.OpenClient("echo"); err == nil || !strings.Contains(err.Error(), "doesn't open its client") {
		t.Fatalf("OpenClient on 1.20: %v, want the no-standalone-client refusal", err)
	}
}

// A plugin answering -32601 to client.openSpec has no standalone client.
func TestClientOpenMissingMethod(t *testing.T) {
	e := clientOpenEnv(t, map[string]string{"ECHO_MISSING": "client.openSpec"})
	if _, err := e.a.OpenClient("echo"); err == nil || !strings.Contains(err.Error(), "doesn't open its client") {
		t.Fatalf("OpenClient with -32601: %v, want the no-standalone-client refusal", err)
	}
}
