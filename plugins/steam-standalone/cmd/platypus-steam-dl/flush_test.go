package main

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"platypus/internal/depot"
)

// STEAM-12: job state must flush on a timer as well as every N chunks, so
// SIGINT-cancel loses at most the timer window, not up to 50 chunks.
// All writes go through f.mark: the timer snapshots (deep-copies Done)
// under the lock and marshals outside it, so -race stays clean.
func TestJobFlusherTimerSaves(t *testing.T) {
	dir := t.TempDir()
	st := &depot.JobState{AppID: 7, Done: map[string]bool{}, Total: 10}
	f := newJobFlusher(dir, "job1", st, 20*time.Millisecond)
	defer f.stop()
	f.mark("ab")
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

// Concurrent markers while the timer fires must not race: every Done
// write holds the flusher lock and every save marshals a deep copy.
func TestJobFlusherConcurrentMarks(t *testing.T) {
	dir := t.TempDir()
	st := &depot.JobState{AppID: 7, Done: map[string]bool{}, Total: 1 << 20}
	f := newJobFlusher(dir, "job1", st, 5*time.Millisecond)
	defer f.stop()
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				f.mark(fmt.Sprintf("w%d-%d", w, i))
			}
		}(w)
	}
	wg.Wait()
	f.saveNow()
	loaded, err := depot.LoadJob(dir, "job1")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Done) != 800 {
		t.Fatalf("want 800 done entries, got %d", len(loaded.Done))
	}
}
