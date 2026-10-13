package app

import (
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/session"
	"platypus/internal/types"
)

// The game-running gate (issue #49): background work waits while a game
// runs, then runs when the last session ends.
func TestGameRunningGateDefersAndDrains(t *testing.T) {
	a := &App{sessions: session.NewTracker(func(types.SessionEvent) {})}
	if a.gameRunning() {
		t.Fatal("gameRunning with no sessions")
	}
	var ran atomic.Int32
	if deferred := a.deferWhileGameRuns(func() { ran.Add(1) }); deferred {
		t.Fatal("deferred with no game running")
	}

	pid, err := a.sessions.StartDirect("g1", exec.Command("sleep", "30"), "sleep 30", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = pid
	deadline := time.Now().Add(5 * time.Second)
	for !a.gameRunning() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !a.gameRunning() {
		t.Fatal("gameRunning = false while sleep runs")
	}
	if deferred := a.deferWhileGameRuns(func() { ran.Add(1) }); !deferred {
		t.Fatal("background work was not deferred while a game runs")
	}
	if got := ran.Load(); got != 0 {
		t.Fatalf("deferred work ran while the game runs (%d)", got)
	}
	if err := a.sessions.Stop("g1", true); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for a.gameRunning() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if a.gameRunning() {
		t.Fatal("gameRunning still true after stopping the game")
	}
	a.drainDeferred()
	deadline = time.Now().Add(5 * time.Second)
	for ran.Load() != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := ran.Load(); got != 1 {
		t.Fatalf("deferred work ran %d times, want 1", got)
	}
}
