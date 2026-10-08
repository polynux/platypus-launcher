package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"hydra-launcher/internal/depot"
)

// STEAM-12: after a finished install the plugin's jobs folder must not
// keep the job's state — runInstall removes the job file
// (depot.RemoveJob) and clears staging, but the deferred flusher.stop()
// saved again on return, resurrecting jobs/<job>.json after the removal.
// The leak breaks the "jobs folder empty after a job" invariant and
// leaves stale per-job state behind. The fix: stop() is idempotent and
// saves exactly once (on the first call); the success path must call
// flusher.stop() BEFORE RemoveJob, and the deferred stop after it must
// not save. Same leak on the update path (runUpdate).
func TestFlusherNoSaveAfterStop(t *testing.T) {
	dir := t.TempDir()
	st := &depot.JobState{AppID: 7, Done: map[string]bool{"ab": true}, Total: 10}
	f := newJobFlusher(dir, "jobX", st, time.Hour)
	f.mark("ab")
	// The success path's order: stop (final checkpoint), then RemoveJob,
	// then the deferred stop — which must never resurrect the file.
	f.stop()
	if _, err := os.Stat(filepath.Join(dir, "jobs", "jobX.json")); err != nil {
		t.Fatalf("stop should have flushed once: %v", err)
	}
	if err := depot.RemoveJob(dir, "jobX"); err != nil {
		t.Fatal(err)
	}
	f.stop() // the deferred stop: idempotent, must not save again
	if _, err := os.Stat(filepath.Join(dir, "jobs", "jobX.json")); !os.IsNotExist(err) {
		t.Fatalf("deferred stop resurrected jobs/jobX.json: %v", err)
	}
	// A marker arriving after stop must not re-save either.
	f.mark("cd")
	if _, err := os.Stat(filepath.Join(dir, "jobs", "jobX.json")); !os.IsNotExist(err) {
		t.Fatalf("mark after stop re-saved the job file: %v", err)
	}
}

func TestFlusherCrashSaveStillWorks(t *testing.T) {
	// The STEAM-12 guarantee stays: an in-flight job (never stopped)
	// still checkpoints on the timer and on mark.
	dir := t.TempDir()
	st := &depot.JobState{AppID: 7, Done: map[string]bool{}, Total: 10}
	f := newJobFlusher(dir, "jobY", st, 20*time.Millisecond)
	defer f.stop()
	f.mark("ef")
	deadline := time.Now().Add(2 * time.Second)
	for {
		loaded, _ := depot.LoadJob(dir, "jobY")
		if loaded != nil && loaded.Done["ef"] {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("live flusher never saved")
		}
		time.Sleep(10 * time.Millisecond)
	}
}