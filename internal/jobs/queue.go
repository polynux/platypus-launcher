// Package jobs runs store-plugin installs, updates and uninstalls one at a
// time, and persists the queue in ~/.local/share/hydra/jobs.json so an
// interrupted install resumes after a crash or restart.
package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"hydra-launcher/internal/jsonrpc"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

const (
	KindInstall   = "install"
	KindUpdate    = "update"
	KindUninstall = "uninstall"

	StateQueued   = "queued"
	StateRunning  = "running"
	StateDone     = "done"
	StateFailed   = "failed"
	StateCanceled = "canceled"
)

// Backend talks to the plugin that owns a job.
type Backend interface {
	// Size returns download and disk sizes for an install or update.
	Size(ctx context.Context, job types.Job) (plugins.InstallSize, error)
	// Start asks the plugin to begin the job (game.install, …). The plugin
	// reports progress and the outcome through notifications (Update).
	Start(ctx context.Context, job types.Job) error
	// Cancel asks the plugin to stop the job.
	Cancel(ctx context.Context, job types.Job) error
}

const (
	maxAttempts  = 4               // plugin crashes survived by one job
	keepFinished = 50              // finished jobs kept in history
	saveEvery    = 5 * time.Second // progress persistence interval
	emitEvery    = 250 * time.Millisecond
	// sizeMargin is extra free space required on top of the reported size.
	sizeMargin = 1.02
)

// Queue is safe for concurrent use.
type Queue struct {
	path      string
	b         Backend
	emit      func([]types.Job)
	freeSpace func(path string) (uint64, error)

	mu       sync.Mutex
	jobs     []*types.Job
	lastSave time.Time
	lastEmit time.Time
	wake     chan struct{}
	pending  chan []types.Job // latest snapshot for the emitter
}

type file struct {
	Version int          `json:"version"`
	Jobs    []*types.Job `json:"jobs"`
}

// Open loads the queue. Jobs that were running when Hydra stopped are queued
// again; plugins resume their downloads where the files left off.
func Open(path string, b Backend, emit func([]types.Job)) (*Queue, error) {
	q := &Queue{path: path, b: b, emit: emit, freeSpace: freeSpace, wake: make(chan struct{}, 1), pending: make(chan []types.Job, 1)}
	if emit != nil {
		// One emitter keeps events in order; a slow consumer only ever
		// misses intermediate snapshots.
		go func() {
			for jobs := range q.pending {
				emit(jobs)
			}
		}()
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return q, nil
	}
	if err != nil {
		return q, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		bad := fmt.Sprintf("%s.bad-%d", path, time.Now().Unix())
		_ = os.Rename(path, bad)
		return q, fmt.Errorf("jobs file is corrupt, moved to %s: %w", bad, err)
	}
	for _, j := range f.Jobs {
		if j == nil || j.ID == "" {
			continue
		}
		if j.State == StateRunning {
			j.State, j.Phase = StateQueued, "resuming"
		}
		q.jobs = append(q.jobs, j)
	}
	return q, nil
}

// Run processes the queue until ctx is done.
func (q *Queue) Run(ctx context.Context) {
	q.signal()
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.wake:
		}
		q.next(ctx)
	}
}

func (q *Queue) signal() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// next starts the first queued job unless one is already running.
func (q *Queue) next(ctx context.Context) {
	q.mu.Lock()
	var job *types.Job
	for _, j := range q.jobs {
		if j.State == StateRunning {
			q.mu.Unlock()
			return
		}
		if job == nil && j.State == StateQueued {
			job = j
		}
	}
	if job == nil {
		q.mu.Unlock()
		return
	}
	job.State = StateRunning
	job.Attempts++
	job.Error = ""
	if job.Phase == "" {
		job.Phase = "starting"
	}
	job.UpdatedAt = time.Now().UnixMilli()
	snapshot := *job
	q.changed(true)
	q.mu.Unlock()

	if err := q.prepare(ctx, snapshot); err != nil {
		q.finish(snapshot.ID, StateFailed, err.Error())
		return
	}
	startCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := q.b.Start(startCtx, snapshot); err != nil {
		if errors.Is(err, plugins.ErrExited) {
			q.PluginExited(snapshot.Source, err)
			return
		}
		q.finish(snapshot.ID, StateFailed, err.Error())
	}
}

// prepare checks free space and creates the install folder.
func (q *Queue) prepare(ctx context.Context, job types.Job) error {
	if job.Kind == KindUninstall {
		return nil
	}
	sizeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	size, err := q.b.Size(sizeCtx, job)
	var rpcErr *jsonrpc.Error
	switch {
	case errors.As(err, &rpcErr) && rpcErr.Code == jsonrpc.CodeMethodNotFound:
		// The plugin can't tell; install without the check.
	case err != nil:
		return fmt.Errorf("size check: %w", err)
	default:
		need := size.DiskBytes
		if job.Kind == KindUpdate {
			need = size.DownloadBytes
		}
		free, err := q.freeSpace(job.InstallDir)
		if err != nil {
			return fmt.Errorf("free space check: %w", err)
		}
		if float64(need)*sizeMargin > float64(free) {
			return fmt.Errorf("not enough space in %s: needs %s, %s free", job.InstallDir, human(need), human(int64(free)))
		}
		q.mu.Lock()
		if j := q.find(job.ID); j != nil && j.Total == 0 {
			j.Total = size.DownloadBytes
		}
		q.mu.Unlock()
	}
	if job.Kind == KindInstall {
		if err := os.MkdirAll(job.InstallDir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// Add queues a job. A game has at most one active job.
func (q *Queue) Add(kind, source, gameID, storeGameID, title, installDir string) (types.Job, error) {
	switch kind {
	case KindInstall, KindUpdate, KindUninstall:
	default:
		return types.Job{}, fmt.Errorf("unknown job kind %q", kind)
	}
	if kind != KindUninstall && !filepath.IsAbs(installDir) {
		return types.Job{}, fmt.Errorf("install folder %q is not an absolute path", installDir)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, j := range q.jobs {
		if j.GameID == gameID && (j.State == StateQueued || j.State == StateRunning) {
			return types.Job{}, fmt.Errorf("%s already has a %s queued", title, j.Kind)
		}
	}
	now := time.Now().UnixMilli()
	j := &types.Job{
		ID: newID(), Kind: kind, Source: source, GameID: gameID, StoreGameID: storeGameID,
		Title: title, State: StateQueued, InstallDir: installDir, Percent: -1,
		CreatedAt: now, UpdatedAt: now,
	}
	q.jobs = append(q.jobs, j)
	q.changed(true)
	q.signal()
	return *j, nil
}

// Cancel stops a queued or running job.
func (q *Queue) Cancel(ctx context.Context, id string) error {
	q.mu.Lock()
	j := q.find(id)
	if j == nil {
		q.mu.Unlock()
		return fmt.Errorf("no job %s", id)
	}
	state, snapshot := j.State, *j
	if state != StateQueued && state != StateRunning {
		q.mu.Unlock()
		return nil
	}
	j.State, j.Phase, j.Speed = StateCanceled, "", 0
	j.UpdatedAt = time.Now().UnixMilli()
	q.changed(true)
	q.mu.Unlock()
	if state == StateRunning {
		// Best effort: the job is canceled for Hydra either way, and later
		// notifications for it are ignored.
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_ = q.b.Cancel(cctx, snapshot)
	}
	q.signal()
	return nil
}

// Retry queues a failed or canceled job again.
func (q *Queue) Retry(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	j := q.find(id)
	if j == nil {
		return fmt.Errorf("no job %s", id)
	}
	if j.State != StateFailed && j.State != StateCanceled {
		return nil
	}
	for _, o := range q.jobs {
		if o != j && o.GameID == j.GameID && (o.State == StateQueued || o.State == StateRunning) {
			return fmt.Errorf("%s already has a %s queued", j.Title, o.Kind)
		}
	}
	j.State, j.Error, j.Attempts, j.Phase = StateQueued, "", 0, ""
	j.UpdatedAt = time.Now().UnixMilli()
	q.changed(true)
	q.signal()
	return nil
}

// ClearFinished drops done, failed and canceled jobs from the list.
func (q *Queue) ClearFinished() {
	q.mu.Lock()
	defer q.mu.Unlock()
	kept := q.jobs[:0]
	for _, j := range q.jobs {
		if j.State == StateQueued || j.State == StateRunning {
			kept = append(kept, j)
		}
	}
	q.jobs = kept
	q.changed(true)
}

func (q *Queue) List() []types.Job {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.list()
}

func (q *Queue) list() []types.Job {
	out := make([]types.Job, len(q.jobs))
	for i, j := range q.jobs {
		out[i] = *j
	}
	return out
}

// Notify applies a job.progress, job.done or job.failed notification from
// source. Notifications for another plugin's jobs, or for jobs that aren't
// running, are ignored.
func (q *Queue) Notify(source, method string, u plugins.JobUpdate) {
	switch method {
	case "job.done":
		q.finishFrom(source, u.JobID, StateDone, "")
		return
	case "job.failed":
		msg := u.Error
		if msg == "" {
			msg = "failed"
		}
		q.finishFrom(source, u.JobID, StateFailed, msg)
		return
	case "job.progress":
	default:
		return
	}
	q.mu.Lock()
	j := q.find(u.JobID)
	if j == nil || j.Source != source || j.State != StateRunning {
		q.mu.Unlock()
		return
	}
	if u.Phase != "" {
		j.Phase = u.Phase
	}
	if u.Total > 0 {
		j.Total = u.Total
	}
	if u.Bytes > 0 {
		j.Bytes = u.Bytes
	}
	j.Speed = u.Speed
	switch {
	case u.Percent != nil:
		j.Percent = min(max(*u.Percent, 0), 100)
	case j.Total > 0:
		j.Percent = min(float64(j.Bytes)/float64(j.Total)*100, 100)
	}
	j.UpdatedAt = time.Now().UnixMilli()
	q.changed(false)
	q.mu.Unlock()
}

// PluginExited restarts the running job of a plugin that crashed. The plugin
// manager waits out its restart backoff when the job starts again.
func (q *Queue) PluginExited(source string, err error) {
	q.mu.Lock()
	for _, j := range q.jobs {
		if j.Source != source || j.State != StateRunning {
			continue
		}
		if j.Attempts >= maxAttempts {
			j.State, j.Error = StateFailed, fmt.Sprintf("the plugin crashed %d times: %v", j.Attempts, err)
		} else {
			j.State, j.Phase = StateQueued, "plugin restarted, resuming"
		}
		j.Speed = 0
		j.UpdatedAt = time.Now().UnixMilli()
	}
	q.changed(true)
	q.mu.Unlock()
	q.signal()
}

// FailSource fails the running job of a plugin that was stopped on purpose
// (disabled, changed, removed), which won't report back.
func (q *Queue) FailSource(source, msg string) {
	q.mu.Lock()
	var ids []string
	for _, j := range q.jobs {
		if j.Source == source && j.State == StateRunning {
			ids = append(ids, j.ID)
		}
	}
	q.mu.Unlock()
	for _, id := range ids {
		q.finish(id, StateFailed, msg)
	}
}

func (q *Queue) finishFrom(source, id, state, msg string) {
	q.mu.Lock()
	j := q.find(id)
	ok := j != nil && j.Source == source && j.State == StateRunning
	q.mu.Unlock()
	if ok {
		q.finish(id, state, msg)
	}
}

func (q *Queue) finish(id, state, msg string) {
	q.mu.Lock()
	if j := q.find(id); j != nil && j.State == StateRunning {
		j.State, j.Error, j.Speed = state, msg, 0
		if state == StateDone {
			j.Percent, j.Phase = 100, ""
			if j.Total > 0 {
				j.Bytes = j.Total
			}
		}
		j.UpdatedAt = time.Now().UnixMilli()
	}
	q.trim()
	q.changed(true)
	q.mu.Unlock()
	q.signal()
}

// trim drops the oldest finished jobs beyond keepFinished. Callers hold q.mu.
func (q *Queue) trim() {
	finished := 0
	for _, j := range q.jobs {
		if j.State != StateQueued && j.State != StateRunning {
			finished++
		}
	}
	if finished <= keepFinished {
		return
	}
	drop := finished - keepFinished
	kept := q.jobs[:0]
	for _, j := range q.jobs {
		if drop > 0 && j.State != StateQueued && j.State != StateRunning {
			drop--
			continue
		}
		kept = append(kept, j)
	}
	q.jobs = kept
}

func (q *Queue) find(id string) *types.Job {
	for _, j := range q.jobs {
		if j.ID == id {
			return j
		}
	}
	return nil
}

// changed saves and emits. State changes do both at once; progress is
// throttled. Callers hold q.mu.
func (q *Queue) changed(state bool) {
	now := time.Now()
	if state || now.Sub(q.lastSave) >= saveEvery {
		q.lastSave = now
		_ = q.save()
	}
	if q.emit != nil && (state || now.Sub(q.lastEmit) >= emitEvery) {
		q.lastEmit = now
		select {
		case <-q.pending:
		default:
		}
		q.pending <- q.list()
	}
}

func (q *Queue) save() error {
	data, err := json.MarshalIndent(file{Version: 1, Jobs: q.jobs}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(q.path), 0o755); err != nil {
		return err
	}
	tmp := q.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, q.path)
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// freeSpace returns the bytes available to the user on the filesystem that
// holds path, or its nearest existing parent.
func freeSpace(path string) (uint64, error) {
	for {
		var st syscall.Statfs_t
		err := syscall.Statfs(path, &st)
		if err == nil {
			return st.Bavail * uint64(st.Bsize), nil
		}
		parent := filepath.Dir(path)
		if !errors.Is(err, syscall.ENOENT) || parent == path {
			return 0, err
		}
		path = parent
	}
}

// human formats bytes in decimal units, like the UI and file managers.
func human(b int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	v, i := float64(b), 0
	for v >= 1000 && i < len(units)-1 {
		v /= 1000
		i++
	}
	if i == 0 || v >= 100 {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}
