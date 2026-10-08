package jobs

import (
	"context"
	"path/filepath"
	"testing"

	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

// CORE-15a: a retried job must pick up the fresh size probe instead of
// keeping a stale total from the previous attempt.
func TestPrepareRefreshesStaleTotal(t *testing.T) {
	b := &fakeBackend{size: plugins.InstallSize{DownloadBytes: 1000, DiskBytes: 2000}}
	q := &Queue{
		path:      filepath.Join(t.TempDir(), "jobs.json"),
		b:         b,
		freeSpace: func(string) (uint64, error) { return 10 << 30, nil },
		wake:      make(chan struct{}, 1),
		pending:   make(chan []types.Job, 1),
	}
	dir := t.TempDir()
	added, err := q.Add(KindInstall, "echo", "game-1", "store-1", "Game", dir)
	if err != nil {
		t.Fatal(err)
	}
	// Stale total left by an earlier attempt.
	q.mu.Lock()
	q.find(added.ID).Total = 500
	q.mu.Unlock()

	b.size = plugins.InstallSize{DownloadBytes: 1000, DiskBytes: 2000}
	added.InstallDir = dir
	if err := q.prepare(context.Background(), added); err != nil {
		t.Fatal(err)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if got := q.find(added.ID).Total; got != 1000 {
		t.Fatalf("Total = %d after fresh probe, want 1000 (stale kept)", got)
	}
}

// CORE-15b: a plugin must be able to reset the byte counter to zero (e.g.
// after restarting a resumed download from scratch), clamped into
// [0, Total].
func TestProgressAcceptsZeroBytes(t *testing.T) {
	b := &fakeBackend{}
	q := &Queue{
		path:      filepath.Join(t.TempDir(), "jobs.json"),
		b:         b,
		freeSpace: func(string) (uint64, error) { return 10 << 30, nil },
		wake:      make(chan struct{}, 1),
		pending:   make(chan []types.Job, 1),
	}
	added, err := q.Add(KindInstall, "echo", "game-1", "store-1", "Game", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	q.mu.Lock()
	j := q.find(added.ID)
	j.State = StateRunning
	j.Total = 1000
	q.mu.Unlock()

	q.Notify("echo", "job.progress", plugins.JobUpdate{JobID: added.ID, Bytes: 700, Total: 1000})
	q.Notify("echo", "job.progress", plugins.JobUpdate{JobID: added.ID, Bytes: 0, Total: 1000})
	q.mu.Lock()
	defer q.mu.Unlock()
	if got := q.find(added.ID).Bytes; got != 0 {
		t.Fatalf("Bytes = %d after zero progress, want 0", got)
	}

	q.mu.Unlock()
	q.Notify("echo", "job.progress", plugins.JobUpdate{JobID: added.ID, Bytes: 5000, Total: 1000})
	q.mu.Lock()
	if got := q.find(added.ID).Bytes; got != 1000 {
		t.Fatalf("Bytes = %d after over-total progress, want clamp to 1000", got)
	}
}
