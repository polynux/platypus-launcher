//go:build !windows

package app

import (
	"strings"
	"testing"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/stores"
)

// wineClientOpenManifest is wineClientManifest speaking protocol 1.25:
// client.openSpec for opening the client with no game.
const wineClientOpenManifest = `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,
	"protocol":{"versions":["1.0","1.1","1.2","1.3","1.4","1.5","1.6","1.7","1.8","1.9","1.10","1.11","1.12","1.13","1.14","1.15","1.16","1.17","1.18","1.19","1.20","1.21","1.22","1.23","1.24","1.25"]},"store":"echo","exec":"bin/platypus-echo-app",
	"capabilities":["library","install","launch","auth","discard"]}`

func clientOpenEnv(t *testing.T, env map[string]string) *fixturesEnv {
	t.Helper()
	return wineClientEnvWith(t, env, wineClientOpenManifest)
}

// The card's status before any setup: a wine-client store reads as supported
// but not set up, with nothing holding it.
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

// After the setup installs the client, the card reads as set up. A 1.20
// plugin (no client.openSpec) still reads as not set up: the core has no
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
	if !st.Supported || st.SetUp {
		t.Fatalf("ClientState = %+v, want supported but not set up on 1.20", st)
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

// Open runs the client's own exe under the runner in the pinned prefix,
// tracked like a client session: the card reads as running, and a second
// open refuses instead of starting another copy.
func TestClientOpenFlow(t *testing.T) {
	e := clientOpenEnv(t, nil)
	setupEchoClient(t, e)
	msg, err := e.a.OpenClient("echo")
	if err != nil {
		t.Fatalf("OpenClient: %v", err)
	}
	if !strings.Contains(msg, "Opened Echo") {
		t.Fatalf("open message = %q, want the client named", msg)
	}
	cleanup := func() { _ = e.a.StopGame(stores.ClientGameID("echo")) }
	t.Cleanup(cleanup)
	ev := waitSessionState(t, e, stores.ClientGameID("echo"), "running")
	if ev.PID == 0 {
		t.Fatal("open session names no pid")
	}
	// The client ran under the runner in the pinned client prefix, with no
	// per-game settings.
	wrap := findWrap(t, e, stores.ClientGameID("echo"))
	if wrap["wine_prefix"] != clientPrefix(t, e) {
		t.Fatalf("wrap wine_prefix = %v, want %s", wrap["wine_prefix"], clientPrefix(t, e))
	}
	st, err := e.a.ClientState("echo")
	if err != nil {
		t.Fatalf("ClientState: %v", err)
	}
	if !st.Running {
		t.Fatalf("ClientState = %+v, want running while the client runs", st)
	}
	if _, err := e.a.OpenClient("echo"); apperr.CodeOf(err) != "client.running" {
		t.Fatalf("second OpenClient: %v, want client.running", err)
	}
	if err := e.a.StopGame(stores.ClientGameID("echo")); err != nil {
		t.Fatalf("StopGame: %v", err)
	}
	waitSessionState(t, e, stores.ClientGameID("echo"), "stopped")
	deadline := time.Now().Add(10 * time.Second)
	for {
		st, err := e.a.ClientState("echo")
		if err != nil {
			t.Fatalf("ClientState: %v", err)
		}
		if !st.Running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("ClientState = %+v, want idle after the stop", st)
		}
		time.Sleep(20 * time.Millisecond)
	}
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
