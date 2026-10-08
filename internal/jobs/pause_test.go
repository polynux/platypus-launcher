package jobs

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"hydra-launcher/internal/jsonrpc"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

// discardBackend is a fakeBackend that can also delete a canceled job's data.
type discardBackend struct {
	*fakeBackend
	dmu       sync.Mutex
	discarded []types.Job
	discErr   error
}

func (d *discardBackend) Discard(_ context.Context, j types.Job) error {
	d.dmu.Lock()
	defer d.dmu.Unlock()
	if d.discErr != nil {
		return d.discErr
	}
	d.discarded = append(d.discarded, j)
	return nil
}

func (d *discardBackend) discards() []types.Job {
	d.dmu.Lock()
	defer d.dmu.Unlock()
	return append([]types.Job(nil), d.discarded...)
}

// fakeClock is the queue's injected time source.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func newClock() *fakeClock { return &fakeClock{t: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)} }

func TestPauseRunningJobKeepsItPausedAndAsksThePluginToStop(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", filepath.Join(t.TempDir(), "x"))
	waitState(t, q, j.ID, StateRunning)
	if err := q.Pause(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	got := waitState(t, q, j.ID, StatePaused)
	if got.Speed != 0 || got.CanceledAt != 0 {
		t.Fatalf("paused job should not look canceled: %+v", got)
	}
	if len(b.canceled) != 1 {
		t.Fatalf("plugin should be told to stop once, got %v", b.canceled)
	}
	// Late notifications for a paused job are ignored.
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: j.ID})
	q.Notify("echo", "job.failed", plugins.JobUpdate{JobID: j.ID, Error: "boom"})
	q.Notify("echo", "job.progress", plugins.JobUpdate{JobID: j.ID, Bytes: 5, Total: 10})
	if got := waitState(t, q, j.ID, StatePaused); got.Bytes != 0 {
		t.Fatalf("progress leaked into a paused job: %+v", got)
	}
	// A plugin crash doesn't restart a paused job either.
	q.PluginExited("echo", errors.New("signal: killed"))
	waitState(t, q, j.ID, StatePaused)
}

func TestResumeUsesTheSameJobID(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", filepath.Join(t.TempDir(), "x"))
	waitState(t, q, j.ID, StateRunning)
	b.waitStarts(1, 3*time.Second)
	q.Notify("echo", "job.progress", plugins.JobUpdate{JobID: j.ID, Bytes: 40, Total: 100})
	if err := q.Pause(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	waitState(t, q, j.ID, StatePaused)
	if err := q.Resume(j.ID); err != nil {
		t.Fatal(err)
	}
	got := waitState(t, q, j.ID, StateRunning)
	b.waitStarts(2, 3*time.Second)
	b.mu.Lock()
	ids := append([]string(nil), b.started...)
	b.mu.Unlock()
	if ids[0] != j.ID || ids[1] != j.ID {
		t.Fatalf("resume must call the plugin with the same jobId: %v", ids)
	}
	if got.Bytes != 40 {
		t.Fatalf("resume keeps the progress shown so far: %+v", got)
	}
	if got.Attempts != 1 {
		t.Fatalf("a resume isn't a crash retry, attempts = %d", got.Attempts)
	}
}

func TestPauseQueuedJobNeverReachesThePlugin(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	first, _ := q.Add(KindInstall, "echo", "g1", "g1", "One", filepath.Join(t.TempDir(), "x"))
	second, _ := q.Add(KindInstall, "echo", "g2", "g2", "Two", filepath.Join(t.TempDir(), "y"))
	waitState(t, q, first.ID, StateRunning)
	if err := q.Pause(context.Background(), second.ID); err != nil {
		t.Fatal(err)
	}
	waitState(t, q, second.ID, StatePaused)
	if len(b.canceled) != 0 {
		t.Fatalf("a queued job has nothing to stop: %v", b.canceled)
	}
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: first.ID})
	waitState(t, q, first.ID, StateDone)
	time.Sleep(50 * time.Millisecond)
	if got := waitState(t, q, second.ID, StatePaused); got.Attempts != 0 {
		t.Fatalf("paused job must not start: %+v", got)
	}
}

func TestPausedJobDoesNotBlockTheQueue(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	first, _ := q.Add(KindInstall, "echo", "g1", "g1", "One", filepath.Join(t.TempDir(), "x"))
	second, _ := q.Add(KindInstall, "echo", "g2", "g2", "Two", filepath.Join(t.TempDir(), "y"))
	waitState(t, q, first.ID, StateRunning)
	if err := q.Pause(context.Background(), first.ID); err != nil {
		t.Fatal(err)
	}
	waitState(t, q, first.ID, StatePaused)
	waitState(t, q, second.ID, StateRunning)
}

func TestPausedJobStillOwnsItsGame(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	dir := filepath.Join(t.TempDir(), "x")
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", dir)
	waitState(t, q, j.ID, StateRunning)
	_ = q.Pause(context.Background(), j.ID)
	waitState(t, q, j.ID, StatePaused)
	if _, err := q.Add(KindInstall, "echo", "g", "g", "G", dir); err == nil {
		t.Fatal("a second job for a game with a paused download was accepted")
	}
}

func TestPausedJobsSurviveClearAndTrim(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", filepath.Join(t.TempDir(), "x"))
	waitState(t, q, j.ID, StateRunning)
	_ = q.Pause(context.Background(), j.ID)
	waitState(t, q, j.ID, StatePaused)
	q.ClearFinished()
	q.mu.Lock()
	for i := 0; i < keepFinished+5; i++ {
		q.jobs = append(q.jobs, &types.Job{ID: newID(), State: StateDone})
	}
	q.trim()
	q.mu.Unlock()
	waitState(t, q, j.ID, StatePaused)
}

func TestPausedJobPersistsAcrossReopen(t *testing.T) {
	b := &fakeBackend{}
	q, path := newQueue(t, b)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", filepath.Join(t.TempDir(), "x"))
	waitState(t, q, j.ID, StateRunning)
	q.Notify("echo", "job.progress", plugins.JobUpdate{JobID: j.ID, Bytes: 7, Total: 10})
	_ = q.Pause(context.Background(), j.ID)
	waitState(t, q, j.ID, StatePaused)

	b2 := &fakeBackend{}
	q2, err := Open(path, b2, nil)
	if err != nil {
		t.Fatal(err)
	}
	q2.freeSpace = q.freeSpace
	got := q2.List()[0]
	if got.State != StatePaused || got.Bytes != 7 || got.ID != j.ID {
		t.Fatalf("reloaded: %+v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	t.Cleanup(func() { cancel(); <-done })
	go func() { defer close(done); q2.Run(ctx) }()
	time.Sleep(50 * time.Millisecond)
	if b2.starts() != 0 {
		t.Fatal("a paused job must not restart with Hydra")
	}
	if err := q2.Resume(j.ID); err != nil {
		t.Fatal(err)
	}
	waitState(t, q2, j.ID, StateRunning)
	b2.waitStarts(1, 3*time.Second)
	if b2.started[0] != j.ID {
		t.Fatalf("resumed with a different job id: %v", b2.started)
	}
}

func TestResumeRefusesWhenTheGameHasAnotherActiveJob(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", filepath.Join(t.TempDir(), "x"))
	waitState(t, q, j.ID, StateRunning)
	_ = q.Pause(context.Background(), j.ID)
	waitState(t, q, j.ID, StatePaused)
	// A job for the same game can only exist if the paused one is canceled
	// first, so force the situation the check guards against.
	q.mu.Lock()
	q.jobs = append(q.jobs, &types.Job{ID: "other", GameID: "g", Title: "G", Kind: KindInstall, State: StatePaused})
	q.mu.Unlock()
	if err := q.Resume(j.ID); err == nil {
		t.Fatal("resume must not double-book a game")
	}
}

func TestCancelPausedJobMarksItCanceled(t *testing.T) {
	b := &fakeBackend{}
	clock := newClock()
	q, _ := newQueue(t, b)
	q.SetClock(clock.now)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", filepath.Join(t.TempDir(), "x"))
	waitState(t, q, j.ID, StateRunning)
	_ = q.Pause(context.Background(), j.ID)
	waitState(t, q, j.ID, StatePaused)
	if err := q.Cancel(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	got := waitState(t, q, j.ID, StateCanceled)
	if got.CanceledAt != clock.now().UnixMilli() {
		t.Fatalf("canceled_at = %d, want the clock's now", got.CanceledAt)
	}
	if len(b.canceled) != 1 {
		t.Fatalf("the plugin was already stopped by the pause: %v", b.canceled)
	}
}

func TestCancelStampsTheTimeAndResumeClearsIt(t *testing.T) {
	b := &fakeBackend{}
	clock := newClock()
	q, _ := newQueue(t, b)
	q.SetClock(clock.now)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", filepath.Join(t.TempDir(), "x"))
	waitState(t, q, j.ID, StateRunning)
	_ = q.Cancel(context.Background(), j.ID)
	got := waitState(t, q, j.ID, StateCanceled)
	if got.CanceledAt != clock.now().UnixMilli() {
		t.Fatalf("canceled_at = %d", got.CanceledAt)
	}
	if err := q.Resume(j.ID); err != nil {
		t.Fatalf("a canceled job can be resumed until it is cleaned up: %v", err)
	}
	got = waitState(t, q, j.ID, StateRunning)
	if got.CanceledAt != 0 {
		t.Fatalf("a resumed job is no longer canceled: %+v", got)
	}
}

func newCleanupQueue(t *testing.T, b Backend, clock *fakeClock) *Queue {
	t.Helper()
	q, _ := newQueue(t, b)
	q.SetClock(clock.now)
	return q
}

func cancelOne(t *testing.T, q *Queue, game string) types.Job {
	t.Helper()
	j, err := q.Add(KindInstall, "echo", game, game, game, filepath.Join(t.TempDir(), game))
	if err != nil {
		t.Fatal(err)
	}
	waitState(t, q, j.ID, StateRunning)
	if err := q.Cancel(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	return waitState(t, q, j.ID, StateCanceled)
}

func TestCleanupWaitsForTheThreshold(t *testing.T) {
	b := &discardBackend{fakeBackend: &fakeBackend{}}
	clock := newClock()
	q := newCleanupQueue(t, b, clock)
	q.SetCleanupAfter(7 * 24 * time.Hour)
	j := cancelOne(t, q, "g")

	clock.advance(7*24*time.Hour - time.Minute)
	q.Cleanup(context.Background())
	if len(b.discards()) != 0 {
		t.Fatal("cleaned before the threshold")
	}
	clock.advance(2 * time.Minute)
	q.Cleanup(context.Background())
	waitCleaned(t, q, j.ID) // the background loop may be the one doing it
	d := b.discards()
	if len(d) != 1 || d[0].ID != j.ID || d[0].Kind != KindInstall || d[0].InstallDir != j.InstallDir {
		t.Fatalf("discard calls: %+v", d)
	}
	got := q.List()[0]
	if got.CleanedAt == 0 || got.State != StateCanceled || got.Leftover != "" {
		t.Fatalf("after cleanup: %+v", got)
	}
	// Once is enough.
	clock.advance(time.Hour)
	q.Cleanup(context.Background())
	if len(b.discards()) != 1 {
		t.Fatal("cleaned twice")
	}
}

func TestCleanupSkipsPausedAndActiveJobs(t *testing.T) {
	b := &discardBackend{fakeBackend: &fakeBackend{}}
	clock := newClock()
	q := newCleanupQueue(t, b, clock)
	q.SetCleanupAfter(time.Hour)
	paused, _ := q.Add(KindInstall, "echo", "p", "p", "P", filepath.Join(t.TempDir(), "p"))
	waitState(t, q, paused.ID, StateRunning)
	_ = q.Pause(context.Background(), paused.ID)
	waitState(t, q, paused.ID, StatePaused)
	running, _ := q.Add(KindInstall, "echo", "r", "r", "R", filepath.Join(t.TempDir(), "r"))
	waitState(t, q, running.ID, StateRunning)

	clock.advance(365 * 24 * time.Hour)
	q.Cleanup(context.Background())
	if len(b.discards()) != 0 {
		t.Fatalf("paused and running jobs are never auto-deleted: %+v", b.discards())
	}
}

func TestCleanupThresholdZeroDeletesOnCancel(t *testing.T) {
	b := &discardBackend{fakeBackend: &fakeBackend{}}
	clock := newClock()
	q := newCleanupQueue(t, b, clock)
	q.SetCleanupAfter(0)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", filepath.Join(t.TempDir(), "g"))
	waitState(t, q, j.ID, StateRunning)
	_ = q.Cancel(context.Background(), j.ID)
	waitCleaned(t, q, j.ID)
	if d := b.discards(); len(d) != 1 || d[0].ID != j.ID {
		t.Fatalf("discards: %+v", d)
	}
}

func TestChangingTheThresholdAppliesToAlreadyCanceledJobs(t *testing.T) {
	b := &discardBackend{fakeBackend: &fakeBackend{}}
	clock := newClock()
	q := newCleanupQueue(t, b, clock)
	q.SetCleanupAfter(7 * 24 * time.Hour)
	j := cancelOne(t, q, "g")
	clock.advance(24 * time.Hour)
	q.Cleanup(context.Background())
	if len(b.discards()) != 0 {
		t.Fatal("one day is below a seven day threshold")
	}
	// Tightening the threshold below the job's age cleans it without waiting
	// for the hourly pass.
	q.SetCleanupAfter(12 * time.Hour)
	waitCleaned(t, q, j.ID)
	// Loosening it never resurrects anything or cleans more.
	q.SetCleanupAfter(30 * 24 * time.Hour)
	if len(b.discards()) != 1 {
		t.Fatalf("discards: %+v", b.discards())
	}
}

func TestExplicitDiscardCleansImmediatelyAndOnlyCanceledJobs(t *testing.T) {
	b := &discardBackend{fakeBackend: &fakeBackend{}}
	clock := newClock()
	q := newCleanupQueue(t, b, clock)
	q.SetCleanupAfter(30 * 24 * time.Hour)
	running, _ := q.Add(KindInstall, "echo", "r", "r", "R", filepath.Join(t.TempDir(), "r"))
	waitState(t, q, running.ID, StateRunning)
	if err := q.Discard(context.Background(), running.ID); err == nil {
		t.Fatal("discarding a running download must be refused")
	}
	_ = q.Pause(context.Background(), running.ID)
	waitState(t, q, running.ID, StatePaused)
	if err := q.Discard(context.Background(), running.ID); err == nil {
		t.Fatal("discarding a paused download must be refused (cancel it first)")
	}
	if len(b.discards()) != 0 {
		t.Fatalf("nothing should have reached the plugin: %+v", b.discards())
	}
	_ = q.Cancel(context.Background(), running.ID)
	waitState(t, q, running.ID, StateCanceled)
	if err := q.Discard(context.Background(), running.ID); err != nil {
		t.Fatal(err)
	}
	if got := q.List()[0]; got.CleanedAt == 0 {
		t.Fatalf("not marked cleaned: %+v", got)
	}
	if err := q.Discard(context.Background(), running.ID); err != nil {
		t.Fatalf("discarding twice is a no-op: %v", err)
	}
	if len(b.discards()) != 1 {
		t.Fatalf("discards: %+v", b.discards())
	}
}

func TestDiscardWithoutPluginSupportForgetsTheJobAndSaysWhatRemains(t *testing.T) {
	for name, err := range map[string]error{
		"capability missing": ErrDiscardUnsupported,
		"method not found":   &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "no"},
	} {
		t.Run(name, func(t *testing.T) {
			b := &discardBackend{fakeBackend: &fakeBackend{}, discErr: err}
			q := newCleanupQueue(t, b, newClock())
			j := cancelOne(t, q, "g")
			if err := q.Discard(context.Background(), j.ID); err != nil {
				t.Fatalf("an unsupported discard isn't a failure: %v", err)
			}
			got := q.List()[0]
			if got.CleanedAt == 0 || got.Leftover == "" {
				t.Fatalf("job should be forgotten with a note about what may remain: %+v", got)
			}
		})
	}
}

func TestDiscardFailureKeepsTheJobForARetry(t *testing.T) {
	b := &discardBackend{fakeBackend: &fakeBackend{}, discErr: errors.New("disk on fire")}
	clock := newClock()
	q := newCleanupQueue(t, b, clock)
	j := cancelOne(t, q, "g")
	if err := q.Discard(context.Background(), j.ID); err == nil {
		t.Fatal("expected the plugin's error")
	}
	got := q.List()[0]
	if got.CleanedAt != 0 || got.Leftover == "" {
		t.Fatalf("a failed discard stays pending and says why: %+v", got)
	}
	b.dmu.Lock()
	b.discErr = nil
	b.dmu.Unlock()
	q.SetCleanupAfter(0)
	q.Cleanup(context.Background())
	if got := waitCleaned(t, q, j.ID); got.Leftover != "" {
		t.Fatalf("the next pass should clean it: %+v", got)
	}
}

func TestBackendWithoutDiscardForgetsAfterTheThreshold(t *testing.T) {
	b := &fakeBackend{} // no Discard method at all
	clock := newClock()
	q := newCleanupQueue(t, b, clock)
	q.SetCleanupAfter(time.Hour)
	j := cancelOne(t, q, "g")
	clock.advance(2 * time.Hour)
	q.Cleanup(context.Background())
	got := waitCleaned(t, q, j.ID)
	if got.Leftover == "" {
		t.Fatalf("%+v", got)
	}
}

func TestResumeAfterCleanupRestartsFromScratchButIsAllowed(t *testing.T) {
	b := &discardBackend{fakeBackend: &fakeBackend{}}
	q := newCleanupQueue(t, b, newClock())
	q.SetCleanupAfter(0)
	j := cancelOne(t, q, "g")
	waitCleaned(t, q, j.ID)
	if err := q.Retry(j.ID); err != nil {
		t.Fatal(err)
	}
	got := waitState(t, q, j.ID, StateRunning)
	if got.CleanedAt != 0 || got.CanceledAt != 0 || got.Leftover != "" {
		t.Fatalf("a restarted job carries no cleanup state: %+v", got)
	}
}

func TestUninstallCancelNeedsNoPluginCleanup(t *testing.T) {
	b := &discardBackend{fakeBackend: &fakeBackend{}}
	clock := newClock()
	q := newCleanupQueue(t, b, clock)
	q.SetCleanupAfter(0)
	j, _ := q.Add(KindUninstall, "echo", "g", "g", "G", "")
	waitState(t, q, j.ID, StateRunning)
	_ = q.Cancel(context.Background(), j.ID)
	waitCleaned(t, q, j.ID)
	if len(b.discards()) != 0 {
		t.Fatalf("an uninstall has no download to discard: %+v", b.discards())
	}
}

func TestCanceledJobsFromAnOlderFileGetATimestamp(t *testing.T) {
	b := &fakeBackend{}
	q, path := newQueue(t, b)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", filepath.Join(t.TempDir(), "x"))
	waitState(t, q, j.ID, StateRunning)
	_ = q.Cancel(context.Background(), j.ID)
	waitState(t, q, j.ID, StateCanceled)
	q.mu.Lock()
	q.find(j.ID).CanceledAt = 0 // a file written before canceled_at existed
	q.find(j.ID).UpdatedAt = 1234
	_ = q.save()
	q.mu.Unlock()
	q2, err := Open(path, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := q2.List()[0]; got.CanceledAt != 1234 {
		t.Fatalf("canceled_at should fall back to the last update: %+v", got)
	}
}

func waitCleaned(t *testing.T, q *Queue, id string) types.Job {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		for _, j := range q.List() {
			if j.ID == id && j.CleanedAt != 0 {
				return j
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s never cleaned: %+v", id, q.List())
		}
		time.Sleep(5 * time.Millisecond)
	}
}
