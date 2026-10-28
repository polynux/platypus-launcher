package jobs

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"platypus/internal/plugins"
	"platypus/internal/types"
)

// A verify or move job holds its game like any other, runs through the same
// queue, and never asks the plugin to clean up data it doesn't have.

func TestVerifyJobKeepsItsReport(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	j, err := q.AddVerify("echo", "g", "tiny", "Tiny", dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if j.Kind != KindVerify || j.Repair {
		t.Fatalf("queued %+v", j)
	}
	b.waitStarts(1, time.Second)
	waitState(t, q, j.ID, StateRunning)
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: j.ID, Verify: &types.VerifyReport{Checked: 12, Damaged: 3}})
	got := waitState(t, q, j.ID, StateDone)
	if got.Verify == nil || got.Verify.Damaged != 3 || got.Verify.Checked != 12 {
		t.Fatalf("report = %+v", got.Verify)
	}
}

func TestVerifyReportFromAnotherPluginIsIgnored(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	j, err := q.AddVerify("echo", "g", "tiny", "Tiny", dir, false)
	if err != nil {
		t.Fatal(err)
	}
	b.waitStarts(1, time.Second)
	waitState(t, q, j.ID, StateRunning)
	q.Notify("steam", "job.done", plugins.JobUpdate{JobID: j.ID, Verify: &types.VerifyReport{Damaged: 9}})
	if got := waitState(t, q, j.ID, StateRunning); got.Verify != nil {
		t.Fatalf("another plugin set the report: %+v", got.Verify)
	}
}

func TestOneJobPerGameCoversVerifyAndMove(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	if _, err := q.AddVerify("echo", "g", "tiny", "Tiny", dir, false); err != nil {
		t.Fatal(err)
	}
	if _, err := q.AddMove("echo", "g", "tiny", "Tiny", dir, filepath.Join(t.TempDir(), "Tiny")); err == nil {
		t.Fatal("a move was queued next to a verify of the same game")
	}
}

func TestMoveNeedsAbsoluteFolders(t *testing.T) {
	q, _ := newQueue(t, &fakeBackend{})
	if _, err := q.AddMove("echo", "g", "tiny", "Tiny", "relative", "/tmp/x"); err == nil {
		t.Fatal("relative source accepted")
	}
	if _, err := q.AddMove("echo", "g", "tiny", "Tiny", "/tmp/x", "relative"); err == nil {
		t.Fatal("relative destination accepted")
	}
	if _, err := q.AddVerify("echo", "g", "tiny", "Tiny", "relative", false); err == nil {
		t.Fatal("relative verify folder accepted")
	}
}

func TestCanceledMoveHasNoDataForThePluginToDiscard(t *testing.T) {
	clock := newClock()
	d := &discardBackend{fakeBackend: &fakeBackend{}}
	q := newCleanupQueue(t, d, clock)
	from := filepath.Join(t.TempDir(), "Games", "echo")
	j, err := q.AddMove("echo", "g", "tiny", "Tiny", from, filepath.Join(t.TempDir(), "Tiny"))
	if err != nil {
		t.Fatal(err)
	}
	waitState(t, q, j.ID, StateRunning)
	if err := q.Cancel(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	clock.advance(30 * 24 * time.Hour)
	q.Cleanup(context.Background())
	if n := len(d.discards()); n != 0 {
		t.Fatalf("a canceled move asked the plugin to discard %d times", n)
	}
	if got := q.List(); len(got) != 1 || got[0].CleanedAt != 0 {
		t.Fatalf("canceled move changed: %+v", got)
	}
}

func TestSetLeftoverKeepsTheDoneJob(t *testing.T) {
	q, _ := newQueue(t, &fakeBackend{})
	j, err := q.AddMove("echo", "g", "tiny", "Tiny", "/tmp/a", "/tmp/b")
	if err != nil {
		t.Fatal(err)
	}
	waitState(t, q, j.ID, StateRunning)
	q.SetLeftover(j.ID, "/tmp/a")
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: j.ID})
	got := waitState(t, q, j.ID, StateDone)
	if got.Leftover != "/tmp/a" {
		t.Fatalf("leftover = %q", got.Leftover)
	}
}

// Discard deletes downloads. A verify or move has none, so the queue refuses
// it and the plugin is never asked.
func TestDiscardRefusesVerifyAndMove(t *testing.T) {
	clock := newClock()
	d := &discardBackend{fakeBackend: &fakeBackend{}}
	q := newCleanupQueue(t, d, clock)
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	v, err := q.AddVerify("echo", "g1", "tiny", "Tiny", dir, false)
	if err != nil {
		t.Fatal(err)
	}
	m, err := q.AddMove("echo", "g2", "tiny", "Tiny", dir, filepath.Join(t.TempDir(), "Tiny"))
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range []types.Job{v, m} {
		waitState(t, q, j.ID, StateRunning)
		if err := q.Cancel(context.Background(), j.ID); err != nil {
			t.Fatal(err)
		}
		if err := q.Discard(context.Background(), j.ID); !errors.Is(err, ErrNoData) {
			t.Fatalf("%s: discard err = %v, want ErrNoData", j.Kind, err)
		}
	}
	if n := len(d.discards()); n != 0 {
		t.Fatalf("the plugin was asked to discard %d times", n)
	}
}

// coreBackend says whether a move runs in core (its goroutine), or in the
// plugin.
type coreBackend struct {
	*fakeBackend
	core bool
}

func (c coreBackend) RunsInCore(types.Job) bool { return c.core }

// A plugin that crashes must not take a core move with it: the move's
// goroutine is still working on the folders. A plugin's own move restarts.
func TestPluginExitLeavesACoreMoveAlone(t *testing.T) {
	for _, core := range []bool{true, false} {
		b := &fakeBackend{}
		q, _ := newQueue(t, coreBackend{b, core})
		j, err := q.AddMove("echo", "g", "tiny", "Tiny", "/tmp/a", "/tmp/b")
		if err != nil {
			t.Fatal(err)
		}
		waitState(t, q, j.ID, StateRunning)
		q.PluginExited("echo", errors.New("crashed"))
		if core {
			if got := q.List()[0]; got.State != StateRunning {
				t.Fatalf("core=%v: a core move was requeued: %+v", core, got)
			}
			continue
		}
		// The requeued move is started again straight away, so a queued
		// state can't be seen. The restart shows what the crash left.
		b.waitStarts(2, 3*time.Second)
		got := waitState(t, q, j.ID, StateRunning)
		if got.Attempts != 2 || got.Phase != "plugin restarted, resuming" {
			t.Fatalf("core=%v: a plugin move was not requeued: %+v", core, got)
		}
	}
}

func TestHoldingFindsAVerifyOrMoveOfTheGame(t *testing.T) {
	q, _ := newQueue(t, &fakeBackend{})
	if _, ok := q.Holding("g", KindVerify, KindMove); ok {
		t.Fatal("an empty queue holds a game")
	}
	j, err := q.AddVerify("echo", "g", "tiny", "Tiny", filepath.Join(t.TempDir(), "Games", "echo"), false)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := q.Holding("g", KindVerify, KindMove); !ok || got.ID != j.ID {
		t.Fatalf("Holding = %+v, %v", got, ok)
	}
	if _, ok := q.Holding("g", KindUninstall); ok {
		t.Fatal("a verify was reported as an uninstall")
	}
	if _, ok := q.Holding("other", KindVerify, KindMove); ok {
		t.Fatal("another game's verify is held here")
	}
}

// A move that was placed before a cancel stays done: the game is where the
// move put it.
func TestFinishMoveEndsAPlacedMoveDone(t *testing.T) {
	q, _ := newQueue(t, &fakeBackend{})
	j, err := q.AddMove("echo", "g", "tiny", "Tiny", "/tmp/a", "/tmp/b")
	if err != nil {
		t.Fatal(err)
	}
	waitState(t, q, j.ID, StateRunning)
	if err := q.Cancel(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	q.FinishMove(j.ID)
	got := q.List()[0]
	if got.State != StateDone || got.CanceledAt != 0 || got.Error != "" {
		t.Fatalf("placed move ended %+v", got)
	}
}

// The finished hook sees installs and uninstalls, not verifies.
func TestFinishedHookRunsForInstallsAndUninstalls(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	var mu sync.Mutex
	var seen []string
	q.SetOnFinished(func(j types.Job) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, j.Kind)
	})
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	v, err := q.AddVerify("echo", "g", "tiny", "Tiny", dir, false)
	if err != nil {
		t.Fatal(err)
	}
	b.waitStarts(1, time.Second)
	waitState(t, q, v.ID, StateRunning)
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: v.ID})
	in, err := q.Add(KindInstall, "echo", "g", "tiny", "Tiny", dir)
	if err != nil {
		t.Fatal(err)
	}
	b.waitStarts(2, time.Second)
	waitState(t, q, in.ID, StateRunning)
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: in.ID})
	mu.Lock()
	defer mu.Unlock()
	// Verifies report through the same hook (issue #145: the activity feed
	// records them); the library invalidation stays the App's choice.
	if len(seen) != 2 || seen[0] != KindVerify || seen[1] != KindInstall {
		t.Fatalf("finished hook saw %v, want [verify install]", seen)
	}
}

// Updates and moves change what the store lists, so the finished hook sees
// them like installs and uninstalls (the library cache drops their source).
// Verifies report through the same hook for the activity feed (issue #145).
func TestFinishedHookRunsForUpdatesAndMoves(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	var mu sync.Mutex
	var seen []string
	q.SetOnFinished(func(j types.Job) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, j.Kind)
	})
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	up, err := q.Add(KindUpdate, "echo", "g-up", "tiny", "Tiny", dir)
	if err != nil {
		t.Fatal(err)
	}
	b.waitStarts(1, time.Second)
	waitState(t, q, up.ID, StateRunning)
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: up.ID})
	mv, err := q.AddMove("echo", "g-mv", "tiny", "Tiny", dir, dir+"-to")
	if err != nil {
		t.Fatal(err)
	}
	b.waitStarts(2, time.Second)
	waitState(t, q, mv.ID, StateRunning)
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: mv.ID})
	deadline := time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		got := append([]string(nil), seen...)
		mu.Unlock()
		if len(got) == 2 {
			if got[0] != KindUpdate || got[1] != KindMove {
				t.Fatalf("finished hook saw %v, want [update move]", got)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("finished hook saw %v, want [update move]", got)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
