package session

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"hydra-launcher/internal/types"
)

// CORE-1: a window that subscribes after launch must still see the running
// game via a snapshot API. There was no Snapshot to call at all.
func TestSnapshotReportsRunningDirectGame(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	events := make(chan types.SessionEvent, 16)
	tr := NewTracker(func(e types.SessionEvent) { events <- e })

	cmd := exec.Command("sleep", "30")
	if _, err := tr.StartDirect("game-direct", cmd, "sleep 30", nil); err != nil {
		t.Fatalf("StartDirect: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// Drain the initial running event.
	select {
	case e := <-events:
		if e.State != "running" {
			t.Fatalf("want running event, got %q", e.State)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for running event")
	}

	// Simulate a late window: a fresh snapshot must still report running.
	snap := tr.Snapshot()
	found := false
	for _, e := range snap {
		if e.GameID == "game-direct" && e.State == "running" && e.PID != 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("late snapshot missing running game-direct: %#v", snap)
	}

	if !tr.Running("game-direct") {
		t.Fatal("Running(game-direct) = false while process alive")
	}
}

func TestSnapshotReportsStartingClientGame(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	events := make(chan types.SessionEvent, 16)
	tr := NewTracker(func(e types.SessionEvent) { events <- e })

	tr.WatchClient("game-client", "Steam", func() int { return 0 }, 300*time.Millisecond)

	select {
	case e := <-events:
		if e.State != "starting" {
			t.Fatalf("want starting event, got %q", e.State)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for starting event")
	}

	snap := tr.Snapshot()
	found := false
	for _, e := range snap {
		if e.GameID == "game-client" && (e.State == "starting" || e.State == "running") {
			found = true
		}
	}
	if !found {
		t.Fatalf("late snapshot missing starting game-client: %#v", snap)
	}
	_ = os.Getenv("HOME")
}
