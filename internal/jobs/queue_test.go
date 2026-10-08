package jobs

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"hydra-launcher/internal/jsonrpc"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

type fakeBackend struct {
	mu       sync.Mutex
	size     plugins.InstallSize
	sizeErr  error
	startErr error
	started  []string
	canceled []string
}

func (f *fakeBackend) Size(context.Context, types.Job) (plugins.InstallSize, error) {
	return f.size, f.sizeErr
}

func (f *fakeBackend) Start(_ context.Context, j types.Job) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, j.ID)
	return f.startErr
}

func (f *fakeBackend) Cancel(_ context.Context, j types.Job) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.canceled = append(f.canceled, j.ID)
	return nil
}

func (f *fakeBackend) starts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.started)
}

// waitStarts blocks until the backend has been asked to start n jobs. The
// queue delivers game.install (Start) only after the size probe (prepare)
// has set Total, so a test that notifies after waitStarts mirrors what a
// real plugin can do.
func (f *fakeBackend) waitStarts(n int, d time.Duration) {
	deadline := time.Now().Add(d)
	for {
		f.mu.Lock()
		got := len(f.started)
		f.mu.Unlock()
		if got >= n {
			return
		}
		if time.Now().After(deadline) {
			panic(fmt.Sprintf("backend never started %d jobs (got %d)", n, got))
		}
		time.Sleep(time.Millisecond)
	}
}

func newQueue(t *testing.T, b Backend) (*Queue, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jobs.json")
	q, err := Open(path, b, nil)
	if err != nil {
		t.Fatal(err)
	}
	q.freeSpace = func(string) (uint64, error) { return 10 << 30, nil }
	ctx, cancel := context.WithCancel(context.Background())
	// Join Run on cleanup: cancel alone leaves a next/prepare (MkdirAll of
	// the install dir) in flight, whose syscall can land after the test
	// framework's TempDir RemoveAll has scanned the folder — "directory
	// not empty", a cleanup flake. Waiting for Run to return orders all
	// queue disk writes before the removal. Cleanups run LIFO, later
	// registrations first, so this runs before the TempDir teardown too.
	done := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		<-done
	})
	go func() {
		defer close(done)
		q.Run(ctx)
	}()
	return q, path
}

func waitState(t *testing.T, q *Queue, id, state string) types.Job {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		for _, j := range q.List() {
			if j.ID == id && j.State == state {
				return j
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("job %s never reached %s: %+v", id, state, q.List())
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func pct(v float64) *float64 { return &v }

func TestInstallFlow(t *testing.T) {
	b := &fakeBackend{size: plugins.InstallSize{DownloadBytes: 100, DiskBytes: 200}}
	q, _ := newQueue(t, b)
	dir := filepath.Join(t.TempDir(), "Games", "echo")
	j, err := q.Add(KindInstall, "echo", "p.echo.tiny", "tiny", "Tiny", dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.Add(KindInstall, "echo", "p.echo.tiny", "tiny", "Tiny", dir); err == nil {
		t.Fatal("second active job for one game accepted")
	}
	waitState(t, q, j.ID, StateRunning)
	// Real plugins only learn the job from game.install, which Start
	// delivers after the size probe has set Total. Wait for that, or the
	// progress below can race the probe and land with Total unset.
	b.waitStarts(1, 3*time.Second)

	q.Notify("echo", "job.progress", plugins.JobUpdate{JobID: j.ID, Bytes: 50, Speed: 10, Phase: "downloading"})
	got := waitState(t, q, j.ID, StateRunning)
	if got.Total != 100 || got.Percent != 50 || got.Phase != "downloading" {
		t.Fatalf("progress: %+v", got)
	}
	// Another plugin can't touch this job.
	q.Notify("evil", "job.done", plugins.JobUpdate{JobID: j.ID})
	q.Notify("echo", "job.progress", plugins.JobUpdate{JobID: j.ID, Percent: pct(80)})
	if got := waitState(t, q, j.ID, StateRunning); got.Percent != 80 {
		t.Fatalf("explicit percent: %+v", got)
	}
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: j.ID})
	if got := waitState(t, q, j.ID, StateDone); got.Percent != 100 {
		t.Fatalf("done: %+v", got)
	}
}

func TestNotEnoughSpace(t *testing.T) {
	b := &fakeBackend{size: plugins.InstallSize{DownloadBytes: 1 << 40, DiskBytes: 2 << 40}}
	q, _ := newQueue(t, b)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "Huge", filepath.Join(t.TempDir(), "x"))
	got := waitState(t, q, j.ID, StateFailed)
	if !strings.Contains(got.Error, "not enough space") || b.starts() != 0 {
		t.Fatalf("want space failure before start: %+v, starts %d", got, b.starts())
	}
}

func TestSizeUnsupportedSkipsCheck(t *testing.T) {
	b := &fakeBackend{sizeErr: &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "no"}}
	q, _ := newQueue(t, b)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", filepath.Join(t.TempDir(), "x"))
	waitState(t, q, j.ID, StateRunning)
}

func TestCrashRestartsJob(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", filepath.Join(t.TempDir(), "x"))
	waitState(t, q, j.ID, StateRunning)
	for i := 1; i < maxAttempts; i++ {
		q.PluginExited("echo", errors.New("signal: killed"))
		got := waitState(t, q, j.ID, StateRunning)
		if got.Attempts != i+1 {
			t.Fatalf("attempt %d: %+v", i+1, got)
		}
	}
	q.PluginExited("echo", errors.New("signal: killed"))
	if got := waitState(t, q, j.ID, StateFailed); !strings.Contains(got.Error, "crashed") {
		t.Fatalf("give up: %+v", got)
	}
	if b.starts() != maxAttempts {
		t.Fatalf("starts = %d", b.starts())
	}
}

func TestCancelAndRetry(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", filepath.Join(t.TempDir(), "x"))
	waitState(t, q, j.ID, StateRunning)
	if err := q.Cancel(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	waitState(t, q, j.ID, StateCanceled)
	if len(b.canceled) != 1 {
		t.Fatal("plugin wasn't asked to cancel")
	}
	// Late notifications for a canceled job are ignored.
	q.Notify("echo", "job.done", plugins.JobUpdate{JobID: j.ID})
	waitState(t, q, j.ID, StateCanceled)

	if err := q.Retry(j.ID); err != nil {
		t.Fatal(err)
	}
	waitState(t, q, j.ID, StateRunning)
}

// A plugin that only reports a percentage still shows bytes against the
// queued total.
func TestPercentOnlyProgressEstimatesBytes(t *testing.T) {
	b := &fakeBackend{}
	q, _ := newQueue(t, b)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", filepath.Join(t.TempDir(), "x"))
	waitState(t, q, j.ID, StateRunning)
	q.Notify("echo", "job.progress", plugins.JobUpdate{JobID: j.ID, Total: 1000})
	q.Notify("echo", "job.progress", plugins.JobUpdate{JobID: j.ID, Percent: pct(90)})
	if got := q.List()[0]; got.Bytes != 900 || got.Total != 1000 {
		t.Fatalf("bytes %d of %d", got.Bytes, got.Total)
	}
}

func TestResumeAfterRestart(t *testing.T) {
	b := &fakeBackend{}
	q, path := newQueue(t, b)
	j, _ := q.Add(KindInstall, "echo", "g", "g", "G", filepath.Join(t.TempDir(), "x"))
	waitState(t, q, j.ID, StateRunning)
	q.Notify("echo", "job.progress", plugins.JobUpdate{JobID: j.ID, Bytes: 7, Total: 10})
	q.mu.Lock()
	q.save()
	q.mu.Unlock()

	// A new Hydra process loads the file: the job is queued again and starts.
	b2 := &fakeBackend{}
	q2, err := Open(path, b2, nil)
	if err != nil {
		t.Fatal(err)
	}
	q2.freeSpace = q.freeSpace
	if got := q2.List()[0]; got.State != StateQueued || got.Bytes != 7 {
		t.Fatalf("reloaded: %+v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	// Same join discipline as newQueue: the framework's TempDir cleanup
	// must not race q2's next/prepare syscalls.
	done := make(chan struct{})
	t.Cleanup(func() {
		cancel()
		<-done
	})
	go func() {
		defer close(done)
		q2.Run(ctx)
	}()
	waitState(t, q2, j.ID, StateRunning)
}

func TestStartErrorFails(t *testing.T) {
	b := &fakeBackend{startErr: errors.New("plugin echo is unapproved")}
	q, _ := newQueue(t, b)
	j, _ := q.Add(KindUninstall, "echo", "g", "g", "G", "")
	if got := waitState(t, q, j.ID, StateFailed); got.Error != "plugin echo is unapproved" {
		t.Fatalf("%+v", got)
	}
}
