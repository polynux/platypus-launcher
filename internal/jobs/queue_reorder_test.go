package jobs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"platypus/internal/plugins"
	"platypus/internal/types"
)

func reorderIDs(list []types.Job) []string {
	ids := make([]string, len(list))
	for i, j := range list {
		ids[i] = j.ID
	}
	return ids
}

func equalReorderIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Moving the last queued job to the front makes it the next job to start:
// after the running job ends, the moved job runs before the one queued
// ahead of it.
func TestMoveToFrontMakesJobNextToStart(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	first, _ := q.Add(KindInstall, "echo", "g1", "g1", "One", filepath.Join(t.TempDir(), "x"))
	second, _ := q.Add(KindInstall, "echo", "g2", "g2", "Two", filepath.Join(t.TempDir(), "y"))
	third, _ := q.Add(KindInstall, "echo", "g3", "g3", "Three", filepath.Join(t.TempDir(), "z"))
	waitState(t, q, first.ID, StateRunning)

	if err := q.MoveToFront(third.ID); err != nil {
		t.Fatal(err)
	}
	ids := reorderIDs(q.List())
	want := []string{first.ID, third.ID, second.ID}
	if !equalReorderIDs(ids, want) {
		t.Fatalf("order after move = %v, want %v", ids, want)
	}

	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: first.ID})
	waitState(t, q, third.ID, StateRunning)
	if got := waitState(t, q, first.ID, StateDone); got.State != StateDone {
		t.Fatalf("first should be done: %+v", got)
	}
}

// A paused job moved to the front stays paused, keeps its spot, and starts
// next once resumed and the running job ends.
func TestMoveToFrontPausedJobStartsNextAfterResume(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	first, _ := q.Add(KindInstall, "echo", "g1", "g1", "One", filepath.Join(t.TempDir(), "x"))
	second, _ := q.Add(KindInstall, "echo", "g2", "g2", "Two", filepath.Join(t.TempDir(), "y"))
	third, _ := q.Add(KindInstall, "echo", "g3", "g3", "Three", filepath.Join(t.TempDir(), "z"))
	waitState(t, q, first.ID, StateRunning)
	if err := q.Pause(context.Background(), third.ID); err != nil {
		t.Fatal(err)
	}
	waitState(t, q, third.ID, StatePaused)

	if err := q.MoveToFront(third.ID); err != nil {
		t.Fatal(err)
	}
	if got := waitState(t, q, third.ID, StatePaused); got.State != StatePaused {
		t.Fatalf("move must not resume a paused job: %+v", got)
	}
	ids := reorderIDs(q.List())
	want := []string{first.ID, third.ID, second.ID}
	if !equalReorderIDs(ids, want) {
		t.Fatalf("order after move = %v, want %v", ids, want)
	}

	if err := q.Resume(third.ID); err != nil {
		t.Fatal(err)
	}
	waitState(t, q, third.ID, StateQueued)
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: first.ID})
	waitState(t, q, third.ID, StateRunning)
}

// The new order survives a core restart: reloading the file keeps the moved
// job ahead of the one queued before it.
func TestMoveToFrontPersistsAcrossReopen(t *testing.T) {
	b := &fakeBackend{}
	q, path := newQueue(t, b)
	first, _ := q.Add(KindInstall, "echo", "g1", "g1", "One", filepath.Join(t.TempDir(), "x"))
	second, _ := q.Add(KindInstall, "echo", "g2", "g2", "Two", filepath.Join(t.TempDir(), "y"))
	third, _ := q.Add(KindInstall, "echo", "g3", "g3", "Three", filepath.Join(t.TempDir(), "z"))
	waitState(t, q, first.ID, StateRunning)
	if err := q.MoveToFront(third.ID); err != nil {
		t.Fatal(err)
	}
	// changed(true) saves synchronously; the file already holds the order.
	q2, err := Open(path, &fakeBackend{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	q2.freeSpace = q.freeSpace
	ids := reorderIDs(q2.List())
	// The file keeps its order; the running job reloads as queued in the
	// same slot, still ahead. What matters is the moved job stayed ahead
	// of the one queued before it.
	want := []string{first.ID, third.ID, second.ID}
	if !equalReorderIDs(ids, want) {
		t.Fatalf("reloaded order = %v, want %v", ids, want)
	}
}

// The move is broadcast: the next jobs event carries the new order.
func TestMoveToFrontEmitsNewOrder(t *testing.T) {
	ch := make(chan []types.Job, 16)
	emit := func(list []types.Job) { ch <- list }
	path := filepath.Join(t.TempDir(), "jobs.json")
	q, err := Open(path, &fakeBackend{}, emit)
	if err != nil {
		t.Fatal(err)
	}
	q.freeSpace = func(string) (uint64, error) { return 10 << 30, nil }
	_, _ = q.Add(KindInstall, "echo", "g1", "g1", "One", filepath.Join(t.TempDir(), "x"))
	_, _ = q.Add(KindInstall, "echo", "g2", "g2", "Two", filepath.Join(t.TempDir(), "y"))
	third, _ := q.Add(KindInstall, "echo", "g3", "g3", "Three", filepath.Join(t.TempDir(), "z"))
	if err := q.MoveToFront(third.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		select {
		case got := <-ch:
			if len(got) == 3 && got[0].ID == third.ID {
				return // the event carries the moved job first
			}
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("no jobs event with %s first: %+v", third.ID, reorderIDs(q.List()))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Running and finished jobs don't move; unknown ids are an error.
func TestMoveToFrontLeavesOtherStates(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	first, _ := q.Add(KindInstall, "echo", "g1", "g1", "One", filepath.Join(t.TempDir(), "x"))
	second, _ := q.Add(KindInstall, "echo", "g2", "g2", "Two", filepath.Join(t.TempDir(), "y"))
	waitState(t, q, first.ID, StateRunning)
	before := reorderIDs(q.List())

	if err := q.MoveToFront(first.ID); err != nil {
		t.Fatal(err)
	}
	if got := reorderIDs(q.List()); !equalReorderIDs(got, before) {
		t.Fatalf("running job moved: %v", got)
	}
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: first.ID})
	waitState(t, q, first.ID, StateDone)
	before = reorderIDs(q.List())
	if err := q.MoveToFront(first.ID); err != nil {
		t.Fatal(err)
	}
	if got := reorderIDs(q.List()); !equalReorderIDs(got, before) {
		t.Fatalf("finished job moved: %v", got)
	}
	if err := q.MoveToFront("nope"); err == nil {
		t.Fatal("unknown job id must be an error")
	}
	// Moving the first waiter is a no-op that keeps the order.
	if err := q.MoveToFront(second.ID); err != nil {
		t.Fatal(err)
	}
	if got := reorderIDs(q.List()); !equalReorderIDs(got, before) {
		t.Fatalf("order changed by a no-op move: %v", got)
	}
}
