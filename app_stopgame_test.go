package main

import (
	"os/exec"
	"testing"
	"time"

	"platypus/internal/session"
	"platypus/internal/types"
)

func TestStopGameNotRunningIsAClearError(t *testing.T) {
	a := &App{sessions: session.NewTracker(func(types.SessionEvent) {})}
	err := a.StopGame("ghost")
	if err == nil || err.Error() != "that game is not running" {
		t.Fatalf("got %v", err)
	}
}

func TestStopGameEndsTheSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	events := make(chan types.SessionEvent, 16)
	a := &App{sessions: session.NewTracker(func(e types.SessionEvent) { events <- e })}
	// A native game: the stop hook has no wineserver to ask.
	p := &prepared{}
	if _, err := a.sessions.StartDirect("g", exec.Command("sleep", "30"), "sleep 30", nil, session.WithStopHook(p.stopHook())); err != nil {
		t.Fatal(err)
	}
	if err := a.StopGame("g"); err != nil {
		t.Fatal(err)
	}
	if err := a.StopGame("g"); err != nil {
		t.Fatalf("second stop should be a no-op: %v", err)
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e := <-events:
			if e.State == "failed" {
				t.Fatalf("a stop must not be reported as a failure: %+v", e)
			}
			if e.State == "stopped" {
				return
			}
		case <-deadline:
			t.Fatal("no stopped event")
		}
	}
}
