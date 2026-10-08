package session

import (
	"errors"
	"os/exec"
	"sync"
	"testing"
	"time"

	"hydra-launcher/internal/types"
)

// CORE-3: concurrent launches for one game must not double-start. Exactly
// one StartDirect wins; losers get ErrAlreadyRunning without starting a
// process, and their return must not clear the winner's tracking.
func TestConcurrentStartDirectOnlyOneWins(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tr := NewTracker(func(e types.SessionEvent) {})

	const n = 8
	cmds := make([]*exec.Cmd, n)
	pids := make([]int, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range cmds {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cmds[i] = exec.Command("sleep", "30")
			pid, err := tr.StartDirect("game-race", cmds[i], "sleep 30")
			pids[i], errs[i] = pid, err
		}(i)
	}
	wg.Wait()

	wins := 0
	winner := -1
	for i, err := range errs {
		if err == nil {
			wins++
			winner = i
			continue
		}
		if !errors.Is(err, ErrAlreadyRunning) {
			t.Fatalf("loser %d got unexpected error: %v", i, err)
		}
	}
	if wins != 1 {
		t.Fatalf("want exactly 1 winner, got %d (errs=%v)", wins, errs)
	}
	// Losers must never have started a process.
	for i, cmd := range cmds {
		if i != winner && cmd.Process != nil {
			t.Fatalf("loser %d started pid %d", i, cmd.Process.Pid)
		}
	}
	// The winner's tracking survives the losers' returns.
	if !tr.Running("game-race") {
		t.Fatal("tracker lost the winner after losers returned")
	}
	snap := tr.Snapshot()
	found := false
	for _, e := range snap {
		if e.GameID == "game-race" && e.State == "running" && e.PID == pids[winner] {
			found = true
		}
	}
	if !found {
		t.Fatalf("snapshot missing winner (pid=%d): %#v", pids[winner], snap)
	}

	// Cleanup: killing the winner clears tracking (compare-and-delete holds).
	if cmds[winner].Process != nil {
		_ = cmds[winner].Process.Kill()
	}
	deadline := time.Now().Add(5 * time.Second)
	for tr.Running("game-race") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if tr.Running("game-race") {
		t.Fatal("tracker still reports running after winner exited")
	}
}

// CORE-3: a second WatchClient for the same game must not steal the claim.
func TestWatchClientSecondClaimFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tr := NewTracker(func(e types.SessionEvent) {})
	never := func() int { return 0 }
	if !tr.WatchClient("game-wc", "Steam", never, 500*time.Millisecond) {
		t.Fatal("first WatchClient claim failed")
	}
	if tr.WatchClient("game-wc", "Steam", never, 500*time.Millisecond) {
		t.Fatal("second WatchClient claim succeeded for a tracked game")
	}
	snap := tr.Snapshot()
	if len(snap) != 1 || snap[0].GameID != "game-wc" {
		t.Fatalf("snapshot after double claim: %#v", snap)
	}
}
