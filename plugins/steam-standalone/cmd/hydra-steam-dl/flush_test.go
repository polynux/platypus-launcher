package main

import (
	"testing"
	"time"

	"hydra-launcher/internal/depot"
)

// STEAM-12: job state must flush on a timer as well as every N chunks, so
// SIGINT-cancel loses at most the timer window, not up to 50 chunks.
func TestJobFlusherTimerSaves(t *testing.T) {
	dir := t.TempDir()
	st := &depot.JobState{AppID: 7, Done: map[string]bool{}, Total: 10}
	f := newJobFlusher(dir, "job1", st, 20*time.Millisecond)
	defer f.stop()
	st.Done["ab"] = true
	deadline := time.Now().Add(2 * time.Second)
	for {
		loaded, _ := depot.LoadJob(dir, "job1")
		if loaded != nil && loaded.Done["ab"] {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timer flush never saved job state")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
