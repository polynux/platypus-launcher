package app

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/session"
	"platypus/internal/types"
)

// A game a store client started that ends at once gets one hint to turn the
// Proton log on, and names the log it will write (#46). Once that log exists for
// the next run, the usual hints come from it.
func TestClientLaunchCrashHintsUseTheProtonLog(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	a.sessions = session.NewTracker(func(types.SessionEvent) {})
	a.sessions.SetLogDir(t.TempDir())
	g := types.Game{ID: "clientstore-620", Source: "clientstore", StoreAppID: "620", Title: "Portal"}

	// The client reports the game running for two polls, then gone.
	var polls atomic.Int32
	find := func() int {
		if polls.Add(1) <= 2 {
			return 1234
		}
		return 0
	}
	if !a.sessions.WatchClient(g.ID, "Steam", find, time.Second) {
		t.Fatal("WatchClient did not claim the game")
	}
	for a.sessions.Running(g.ID) {
		time.Sleep(5 * time.Millisecond)
	}

	logPath := filepath.Join(home, "steam-620.log")
	got := a.GameCrashHints(g)
	if got == nil || len(got.Hints) != 1 || got.Hints[0].Rule != "protonLog" || got.Hints[0].Param != logPath {
		t.Fatalf("GameCrashHints = %+v, want the one protonLog hint naming %s", got, logPath)
	}
	if got.Winetricks {
		t.Error("winetricks offered for a Steam client launch")
	}

	// Proton writes its log during the next run: its hints apply.
	if err := os.WriteFile(logPath, []byte("err:module:import_dll Library vcruntime140_1.dll not found"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = a.GameCrashHints(g)
	if got == nil || len(got.Hints) == 0 || got.Hints[0].Rule != "vcredist" || got.Log != logPath {
		t.Fatalf("GameCrashHints with the Proton log = %+v, want the vcredist hint from it", got)
	}
}
