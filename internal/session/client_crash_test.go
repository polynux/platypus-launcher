package session

import (
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/types"
)

// waitIdle waits until the tracker no longer holds gameID.
func waitIdle(t *testing.T, tr *Tracker, gameID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for tr.Running(gameID) {
		if time.Now().After(deadline) {
			t.Fatal("the client launch never ended")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// A game a store client started that ends soon after it was seen running is a
// crash: the client owns the process, so the run's length is the only signal.
func TestClientLaunchThatEndsSoonIsACrash(t *testing.T) {
	tr := NewTracker(func(types.SessionEvent) {})
	tr.SetLogDir(t.TempDir())
	// Seen running for two polls, then gone.
	var polls atomic.Int32
	find := func() int {
		if polls.Add(1) <= 2 {
			return 1234
		}
		return 0
	}
	before := time.Now()
	if !tr.WatchClient("game-quick", "Steam", find, time.Second) {
		t.Fatal("WatchClient did not claim the game")
	}
	waitIdle(t, tr, "game-quick")
	asked, ok := tr.ClientCrash("game-quick")
	if !ok {
		t.Fatal("a run that ended within a minute was not a crash")
	}
	if asked.Before(before) {
		t.Fatalf("the launch time %v is before the launch (%v)", asked, before)
	}
}

// A run that lasted (longer than the window) is not a crash, and a new
// launch clears an earlier one.
func TestClientLaunchThatRanLongIsNotACrash(t *testing.T) {
	tr := NewTracker(func(types.SessionEvent) {})
	tr.SetLogDir(t.TempDir())
	tr.clientQuick = 0 // every run now counts as long
	var polls atomic.Int32
	find := func() int {
		if polls.Add(1) <= 2 {
			return 1234
		}
		return 0
	}
	if !tr.WatchClient("game-long", "Steam", find, time.Second) {
		t.Fatal("WatchClient did not claim the game")
	}
	waitIdle(t, tr, "game-long")
	if _, ok := tr.ClientCrash("game-long"); ok {
		t.Fatal("a long run was reported as a crash")
	}
}
