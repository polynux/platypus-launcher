//go:build !windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// wineClientRunManifest is wineClientManifest speaking protocol 1.23: the
// core-run client command of game.install/game.update/game.uninstall.
const wineClientRunManifest = `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,
	"protocol":{"versions":["1.0","1.1","1.2","1.3","1.4","1.5","1.6","1.7","1.8","1.9","1.10","1.11","1.12","1.13","1.14","1.15","1.16","1.17","1.18","1.19","1.20","1.21","1.22","1.23"]},"store":"echo","exec":"bin/platypus-echo-app",
	"capabilities":["library","install","launch","auth","discard"]}`

const clientRunProtocols = "1.0,1.1,1.2,1.3,1.4,1.5,1.6,1.7,1.8,1.9,1.10,1.11,1.12,1.13,1.14,1.15,1.16,1.17,1.18,1.19,1.20,1.21,1.22,1.23"

func clientRunEnv(t *testing.T, env map[string]string) *fixturesEnv {
	t.Helper()
	merged := map[string]string{"ECHO_PROTOCOLS": clientRunProtocols, "ECHO_CLIENT_RUN": "1"}
	for k, v := range env {
		merged[k] = v
	}
	return wineClientEnvWith(t, merged, wineClientRunManifest)
}

func setupEchoClient(t *testing.T, e *fixturesEnv) {
	t.Helper()
	j, err := e.a.SetupClient("echo", false)
	if err != nil {
		t.Fatalf("SetupClient: %v", err)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
}

func tinyInstallGame() types.Game {
	return types.Game{ID: stores.GameID("echo", "tiny"), Source: "echo", Store: "echo",
		StoreAppID: "tiny", Title: "Echo Tiny", Installable: true}
}

func pollJob(t *testing.T, e *fixturesEnv, id string, within time.Duration) types.Job {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		for _, j := range e.a.jobs.List() {
			if j.ID == id {
				return j
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s never appeared", id)
	return types.Job{}
}

func waitRunning(t *testing.T, e *fixturesEnv, id string) types.Job {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range e.a.jobs.List() {
			if j.ID == id && j.State == jobs.StateRunning {
				return j
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s never started running", id)
	return types.Job{}
}

// A fake 1.23 plugin answering clientRun: the command runs in the client
// prefix via the runner wrap, and the job stays running until the plugin's
// job.done.
func TestClientRunInstallCompletes(t *testing.T) {
	e := clientRunEnv(t, map[string]string{"ECHO_CLIENT_RUN_DONE_MS": "200"})
	setupEchoClient(t, e)
	j, err := e.a.InstallGame(tinyInstallGame())
	if err != nil {
		t.Fatalf("InstallGame: %v", err)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	// The command ran under the runner in the pinned client prefix, with
	// the default runner and none of the game's settings.
	wrap := findWrap(t, e, stores.ClientGameID("echo"))
	if wrap["wine_prefix"] != clientPrefix(t, e) {
		t.Fatalf("wrap wine_prefix = %v, want %s", wrap["wine_prefix"], clientPrefix(t, e))
	}
	// The game landed in the job's installDir, like a normal install.
	if _, err := os.Stat(filepath.Join(e.a.settings.GamesFolder(), "echo", "tiny", "game.sh")); err != nil {
		t.Fatalf("installed game.sh: %v", err)
	}
}

// Canceling a clientRun job kills the command and stops the prefix's
// wineserver, then forwards the cancel so the plugin ends the job too.
func TestClientRunCancelKillsCommand(t *testing.T) {
	e := clientRunEnv(t, map[string]string{
		"ECHO_CLIENT_RUN_HOLD":  "1",
		"ECHO_CLIENT_RUN_SLEEP": "30",
	})
	setupEchoClient(t, e)
	j, err := e.a.InstallGame(tinyInstallGame())
	if err != nil {
		t.Fatalf("InstallGame: %v", err)
	}
	waitRunning(t, e, j.ID)
	// The command is up before the cancel lands.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if findWrap(t, e, stores.ClientGameID("echo")) != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("client command never started running")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := e.a.CancelJob(j.ID); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
	e.waitJob(t, j.ID, jobs.StateCanceled)
	// The cancel stopped the prefix's wineserver, and the held job never
	// reports done afterwards.
	time.Sleep(300 * time.Millisecond)
	if got := pollJob(t, e, j.ID, time.Second); got.State != jobs.StateCanceled {
		t.Fatalf("job state = %q, want canceled (a held job must not finish it)", got.State)
	}
	stops := findRunActions(runnerRecords(t, e), "stop")
	if len(stops) == 0 {
		t.Fatal("no runner stop action ran for the canceled client command")
	}
}

// clientOwnsLocation skips the job's installDir: the game lands in the
// client prefix instead, and the install folder stays untouched.
func TestClientRunClientOwnsLocation(t *testing.T) {
	e := clientRunEnv(t, map[string]string{
		"ECHO_CLIENT_RUN_OWNS":    "1",
		"ECHO_CLIENT_RUN_DONE_MS": "200",
	})
	setupEchoClient(t, e)
	folder := filepath.Join(e.root, "CustomGames", "tiny")
	game := tinyInstallGame()
	j, err := e.a.InstallGameTo(game, folder)
	if err != nil {
		t.Fatalf("InstallGameTo: %v", err)
	}
	e.waitJob(t, j.ID, jobs.StateDone)
	if _, err := os.Stat(filepath.Join(folder, "tiny")); !os.IsNotExist(err) {
		t.Fatalf("job's installDir holds game files: %v", err)
	}
	if _, err := os.Stat(filepath.Join(clientPrefix(t, e), "drive_c", "echo-client", "games", "tiny", "game.sh")); err != nil {
		t.Fatalf("client-owned game.sh: %v", err)
	}
}

// While a clientRun job runs, the client's games refuse to launch.
func TestClientRunLaunchRefused(t *testing.T) {
	e := clientRunEnv(t, map[string]string{"ECHO_CLIENT_RUN_HOLD": "1"})
	setupEchoClient(t, e)
	j, err := e.a.InstallGame(tinyInstallGame())
	if err != nil {
		t.Fatalf("InstallGame: %v", err)
	}
	waitRunning(t, e, j.ID)
	if _, err := e.a.LaunchGame(wineGame()); apperr.CodeOf(err) != "game.launchBusy" {
		t.Fatalf("launch during a clientRun job: %v, want game.launchBusy", err)
	}
	if _, err := e.a.LaunchGame(wineGameDirect()); apperr.CodeOf(err) != "game.launchBusy" {
		t.Fatalf("unflagged launch during a clientRun job: %v, want game.launchBusy", err)
	}
	if err := e.a.CancelJob(j.ID); err != nil {
		t.Fatalf("CancelJob: %v", err)
	}
	e.waitJob(t, j.ID, jobs.StateCanceled)
}

// A 1.22 plugin can't answer clientRun: the core rejects it with a
// readable error instead of running anything.
func TestClientRunOldPluginRejected(t *testing.T) {
	e := wineClientEnv(t, map[string]string{"ECHO_CLIENT_RUN": "1"})
	setupEchoClient(t, e)
	j, err := e.a.InstallGame(tinyInstallGame())
	if err != nil {
		t.Fatalf("InstallGame: %v", err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		got := pollJob(t, e, j.ID, 30*time.Second)
		if got.State == jobs.StateFailed {
			if !strings.Contains(got.Error, "1.23") {
				t.Fatalf("job error = %q, want the 1.23 rejection", got.Error)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("job state = %q, want failed", got.State)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
