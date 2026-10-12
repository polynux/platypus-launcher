package session

import (
	"os/exec"
	"testing"
	"time"

	"platypus/internal/types"
)

func waitExit(t *testing.T, ch <-chan ExitInfo) ExitInfo {
	t.Helper()
	select {
	case info := <-ch:
		return info
	case <-time.After(10 * time.Second):
		t.Fatal("the exit callback never ran")
	}
	return ExitInfo{}
}

// The callback runs once the game is no longer tracked, so a sync started
// from it never races the tracker's own bookkeeping.
func TestOnExitRunsAfterTheGameIsUntracked(t *testing.T) {
	tr := NewTracker(func(types.SessionEvent) {})
	tr.SetLogDir(t.TempDir())
	got := make(chan ExitInfo, 1)
	tracked := make(chan bool, 1)
	_, err := tr.StartDirect("g-exit", exec.Command("true"), "true", nil, WithOnExit(func(i ExitInfo) {
		tracked <- tr.Running("g-exit")
		got <- i
	}))
	if err != nil {
		t.Fatal(err)
	}
	info := waitExit(t, got)
	if info.Stopped || info.Failed {
		t.Fatalf("a clean exit reported %+v", info)
	}
	if <-tracked {
		t.Fatal("the game was still tracked inside the exit callback")
	}
}

func TestOnExitReportsAStopAndAFailedStart(t *testing.T) {
	tr := NewTracker(func(types.SessionEvent) {})
	tr.SetLogDir(t.TempDir())

	stopped := make(chan ExitInfo, 1)
	if _, err := tr.StartDirect("g-stop", exec.Command("sleep", "30"), "sleep", nil, WithOnExit(func(i ExitInfo) { stopped <- i })); err != nil {
		t.Fatal(err)
	}
	if err := tr.Stop("g-stop", true); err != nil {
		t.Fatal(err)
	}
	if info := waitExit(t, stopped); !info.Stopped || info.Failed {
		t.Fatalf("a user stop reported %+v", info)
	}

	failed := make(chan ExitInfo, 1)
	if _, err := tr.StartDirect("g-fail", exec.Command("false"), "false", nil, WithOnExit(func(i ExitInfo) { failed <- i })); err != nil {
		t.Fatal(err)
	}
	if info := waitExit(t, failed); info.Stopped || !info.Failed {
		t.Fatalf("a quick failure reported %+v", info)
	}
}

func TestOnExitNeverRunsWhenTheProcessNeverStarts(t *testing.T) {
	tr := NewTracker(func(types.SessionEvent) {})
	tr.SetLogDir(t.TempDir())
	ran := make(chan struct{}, 1)
	_, err := tr.StartDirect("g-nostart", exec.Command("/no/such/binary"), "x", nil, WithOnExit(func(ExitInfo) { ran <- struct{}{} }))
	if err == nil {
		t.Fatal("started a missing binary")
	}
	select {
	case <-ran:
		t.Fatal("the callback ran for a process that never started")
	case <-time.After(200 * time.Millisecond):
	}
}
