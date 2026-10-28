package session

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/types"
)

func TestWatchClientBusyThenIdleFailsAfterGrace(t *testing.T) {
	rec := newRecorder()
	tr := NewTracker(rec.emit)
	var polls atomic.Int32
	status := func(time.Time) (*types.ClientStatus, error) {
		if polls.Add(1) == 1 {
			return &types.ClientStatus{State: "downloading"}, nil
		}
		return &types.ClientStatus{State: "idle"}, nil
	}
	if !tr.WatchClient("idle-game", "Steam", func() int { return 0 }, time.Second, WithClientStatus(status)) {
		t.Fatal("WatchClient refused launch")
	}
	ev := rec.waitState(t, "failed", 35*time.Second)
	if !strings.Contains(ev.Message, "finished its work but didn't start the game") {
		t.Fatalf("failure message = %q", ev.Message)
	}
}

func TestWatchClientCompletedHandoffWaitsForDelayedProcess(t *testing.T) {
	rec := newRecorder()
	tr := NewTracker(rec.emit)
	var pid atomic.Int32
	time.AfterFunc(20*time.Second, func() { pid.Store(322170) })
	status := func(time.Time) (*types.ClientStatus, error) {
		return &types.ClientStatus{State: "launching"}, nil // Steam's Completed handoff.
	}
	if !tr.WatchClient("delayed-game", "Steam", func() int { return int(pid.Load()) }, 30*time.Second, WithClientStatus(status)) {
		t.Fatal("WatchClient refused launch")
	}
	ev := rec.waitState(t, "running", 25*time.Second)
	if ev.PID != 322170 {
		t.Fatalf("running PID = %d, want delayed game process", ev.PID)
	}
	pid.Store(0)
	rec.waitState(t, "stopped", 5*time.Second)
}

func TestWatchClientWithoutStatusUsesOriginalTimeoutFailure(t *testing.T) {
	rec := newRecorder()
	tr := NewTracker(rec.emit)
	if !tr.WatchClient("no-status-game", "Steam", func() int { return 0 }, 100*time.Millisecond) {
		t.Fatal("WatchClient refused launch")
	}
	ev := rec.waitState(t, "failed", 5*time.Second)
	if !strings.Contains(ev.Message, "never started the game. Check the Steam window") {
		t.Fatalf("failure message = %q", ev.Message)
	}
}

func TestClientStatusChangedRequiresStateOrPercentagePoint(t *testing.T) {
	previousProgress := .40
	previous := &types.ClientStatus{State: "downloading", Progress: &previousProgress}
	progress := .405
	if clientStatusChanged(previous, &types.ClientStatus{State: "downloading", Progress: &progress}) {
		t.Fatal("sub-percentage change should not emit")
	}
	progress = .41
	if !clientStatusChanged(previous, &types.ClientStatus{State: "downloading", Progress: &progress}) {
		t.Fatal("one percentage point change should emit")
	}
}

// Issue #149 review: a window opened while a store client works reads the
// launch-status snapshot, and that snapshot hides the client's status when the
// user switched the notices off — the polling keeps going, its events don't.
func TestSnapshotHidesClientStatusWhenNoticesOff(t *testing.T) {
	rec := newRecorder()
	tr := NewTracker(rec.emit)
	var polls atomic.Int32
	status := func(time.Time) (*types.ClientStatus, error) {
		polls.Add(1)
		return &types.ClientStatus{State: "idle"}, nil
	}
	if !tr.WatchClient("quiet-game", "Steam", func() int { return 0 }, 5*time.Second,
		WithClientStatus(status), WithClientNotices(false)) {
		t.Fatal("WatchClient refused launch")
	}
	// Polls are 2 s apart: the second one proves the first was stored.
	deadline := time.Now().Add(10 * time.Second)
	for polls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	tr.mu.Lock()
	held := tr.active["quiet-game"] != nil && tr.active["quiet-game"].clientStatus != nil
	tr.mu.Unlock()
	if !held {
		t.Fatal("the status was never polled: the snapshot would hide nothing")
	}
	for _, e := range tr.Snapshot() {
		if e.GameID == "quiet-game" && e.ClientStatus != nil {
			t.Fatalf("snapshot leaked the client status with notices off: %+v", e.ClientStatus)
		}
	}
}

// The same snapshot keeps the status when the notices are on, and the events
// carry it either way (see the poll above): turning the switch off only stops
// what the window shows, never the launch.
func TestSnapshotKeepsClientStatusWhenNoticesOn(t *testing.T) {
	rec := newRecorder()
	tr := NewTracker(rec.emit)
	status := func(time.Time) (*types.ClientStatus, error) {
		return &types.ClientStatus{State: "idle"}, nil
	}
	if !tr.WatchClient("chatty-game", "Steam", func() int { return 0 }, 5*time.Second, WithClientStatus(status)) {
		t.Fatal("WatchClient refused launch")
	}
	deadline := time.After(10 * time.Second)
	for {
		select {
		case e := <-rec.ch:
			if e.State == "starting" && e.ClientStatus != nil {
				found := false
				for _, s := range tr.Snapshot() {
					if s.GameID == "chatty-game" && s.ClientStatus != nil {
						found = true
					}
				}
				if !found {
					t.Fatal("the snapshot dropped the client status with notices on")
				}
				return
			}
		case <-deadline:
			t.Fatal("no event carried the client status")
		}
	}
}
