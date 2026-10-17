package app

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"platypus/internal/apperr"
)

// The runner is a bundled plugin the launch depends on, so the supervisor's
// restart and error paths matter for it (ADR 0001, §3).

// A runner plugin whose binary changed is not ready: a launch stops with
// runner.unavailable before any process starts, rather than running unwrapped.
func TestRunnerPluginNotReadyStopsLaunch(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := nativeTestApp(t, home)
	exe := writeExe(t, filepath.Join(home, "game", "game"), elfBytes, true)
	if _, err := a.settings.AddCustomGame("Nat", exe); err != nil {
		t.Fatal(err)
	}
	g := a.customGames()[0]
	bin := filepath.Join(a.plugins.Dir(), "runner", "bin", "platypus-runner")
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	// The binary is running: replace it by rename, as an update would.
	if err := os.WriteFile(bin+".new", append(data, 0), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(bin+".new", bin); err != nil {
		t.Fatal(err)
	}
	if _, err := a.plugins.Discover(); err != nil {
		t.Fatal(err)
	}
	p, err := a.prepareLaunch(g, true)
	var ae *apperr.Error
	if ok := errors.As(err, &ae); err == nil || !ok || ae.Code != "runner.unavailable" {
		t.Fatalf("prepareLaunch = %+v, %v; want runner.unavailable", p, err)
	}
}

// A crashed runner plugin starts again on the next launch.
func TestRunnerPluginRestartsAfterACrash(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := nativeTestApp(t, home)
	exe := writeExe(t, filepath.Join(home, "game", "game"), elfBytes, true)
	if _, err := a.settings.AddCustomGame("Nat", exe); err != nil {
		t.Fatal(err)
	}
	g := a.customGames()[0]
	if _, err := a.prepareLaunch(g, true); err != nil {
		t.Fatal(err)
	}
	first := a.plugins.Running("runner")
	if first == 0 {
		t.Fatal("the runner plugin did not start")
	}
	if err := syscall.Kill(first, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for a.plugins.Running("runner") != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the killed runner plugin was never reaped")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := a.prepareLaunch(g, true); err != nil {
		t.Fatalf("launch after a crash: %v", err)
	}
	if second := a.plugins.Running("runner"); second == 0 || second == first {
		t.Fatalf("not restarted: %d → %d", first, second)
	}
}

// A runner the game asks for that isn't installed fails the launch with the
// plugin's message; nothing runs.
func TestRunnerMissingFailsLaunchWithItsMessage(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := nativeTestApp(t, home)
	exe := writeExe(t, filepath.Join(home, "game.exe"), []byte("MZ\x90\x00"), true)
	cg, err := a.settings.AddCustomGame("Win", exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetGameSettings(cg.ID, map[string]any{"runner_id": "no-such-runner"}); err != nil {
		t.Fatal(err)
	}
	g := a.customGames()[0]
	_, err = a.prepareLaunch(g, true)
	var ae *apperr.Error
	if ok := errors.As(err, &ae); err == nil || !ok || ae.Code != "game.runnerMissing" || ae.Params["id"] != "no-such-runner" {
		t.Fatalf("err = %v, want game.runnerMissing naming the missing runner", err)
	}
}

// With no runner plugin installed the store's direct spec runs as it is.
func TestWithoutARunnerPluginTheDirectSpecRuns(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := nativeTestApp(t, home)
	exe := writeExe(t, filepath.Join(home, "game.exe"), []byte("MZ\x90\x00"), true)
	if _, err := a.settings.AddCustomGame("Win", exe); err != nil {
		t.Fatal(err)
	}
	g := a.customGames()[0]
	a.plugins = nil
	p, err := a.prepareLaunch(g, true)
	if err != nil {
		t.Fatalf("prepareLaunch without a runner: %v", err)
	}
	if len(p.args) != 1 || p.args[0] != exe {
		t.Fatalf("args = %q, want the direct spec %q", p.args, exe)
	}
	if p.runner.Path != "" {
		t.Fatalf("direct spec reports a runner or prefix: %+v", p)
	}
}
