package session

import (
	"testing"
	"time"

	"platypus/internal/types"
)

type recorder struct{ ch chan types.SessionEvent }

func newRecorder() *recorder { return &recorder{ch: make(chan types.SessionEvent, 64)} }

func (r *recorder) emit(e types.SessionEvent) { r.ch <- e }

// waitState waits for the next event with the given state, skipping others.
func (r *recorder) waitState(t *testing.T, state string, within time.Duration) types.SessionEvent {
	t.Helper()
	deadline := time.After(within)
	for {
		select {
		case e := <-r.ch:
			if e.State == state {
				return e
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %q event", state)
		}
	}
}
