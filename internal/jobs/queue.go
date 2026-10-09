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
	"slices"
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
	// KindVerify checks an installed game's files (game.verify); with Repair
	// set it restores the damaged ones. KindMove takes an installed game to
	// another folder (game.move, or core's own move when the plugin has none).
	KindVerify = "verify"
	KindMove   = "move"

	StateQueued   = "queued"
	StateRunning  = "running"
	StatePaused   = "paused"
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
	// Cancel asks the plugin to stop the job. It keeps everything downloaded
	// so far: pausing is the same call, and a canceled job resumes with the
	// same job id until it is discarded.
	Cancel(ctx context.Context, job types.Job) error
}

// Discarder is the optional half of a Backend: it deletes the partial data of
// a job that was canceled and will not be resumed (staging chunks, checkpoints
// and, for a fresh install, the half-written install folder). A backend whose
// plugin can't do that returns ErrDiscardUnsupported, or doesn't implement
// Discarder at all; the queue then only forgets the job and notes what may
// remain.
type Discarder interface {
	Discard(ctx context.Context, job types.Job) error
}

// ErrDiscardUnsupported means the job's plugin has no way to delete the data.
var ErrDiscardUnsupported = errors.New("the plugin can't delete downloaded data")

// DefaultCleanupAfter is how long a canceled job's partial data is kept.
const DefaultCleanupAfter = 7 * 24 * time.Hour

// cleanupEvery is how often Run looks for canceled jobs past their threshold.
const cleanupEvery = time.Hour

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
	// onInstalled is called, outside the queue's lock, when an install or
	// update reports the version it put on disk (job.done's `installed`).
	onInstalled func(job types.Job, installed types.Installed)

	mu  sync.Mutex
	now func() time.Time // injected by tests
	// cleanupAfter is how long a canceled job's data is kept. discarding
	// marks the jobs a cleanup is working on, so a resume can't race it.
	cleanupAfter time.Duration
	discarding   map[string]bool
	cleanupWake  chan struct{}
	jobs         []*types.Job
	lastSave     time.Time
	lastEmit     time.Time
	wake         chan struct{}
	pending      chan []types.Job // latest snapshot for the emitter
	// saveErrs counts failed persistence attempts and lastSaveErr is the
	// latest save error, so a full disk or crash can't silently lose the
	// queue (CORE-12). The queue keeps emitting live state regardless.
	saveErrs    int
	lastSaveErr string
}

type file struct {
	Version int          `json:"version"`
	Jobs    []*types.Job `json:"jobs"`
}

// Open loads the queue. Jobs that were running when Hydra stopped are queued
// again; plugins resume their downloads where the files left off.
func Open(path string, b Backend, emit func([]types.Job)) (*Queue, error) {
	q := &Queue{
		path: path, b: b, emit: emit, freeSpace: freeSpace, now: time.Now,
		wake: make(chan struct{}, 1), pending: make(chan []types.Job, 1),
		cleanupAfter: DefaultCleanupAfter, discarding: map[string]bool{}, cleanupWake: make(chan struct{}, 1),
	}
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
		// Files written before canceled_at existed: the last update is
		// when the job was canceled.
		if j.State == StateCanceled && j.CanceledAt == 0 {
			j.CanceledAt = j.UpdatedAt
		}
		q.jobs = append(q.jobs, j)
	}
	return q, nil
}

// Run processes the queue until ctx is done.
func (q *Queue) Run(ctx context.Context) {
	cleaned := make(chan struct{})
	go func() {
		defer close(cleaned)
		q.cleanupLoop(ctx)
	}()
	defer func() { <-cleaned }()
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

// cleanupLoop deletes the data of canceled jobs past the threshold: once at
// startup, hourly, and whenever a cancel or a threshold change asks for it.
// It runs apart from the job loop because a plugin can take a while to wind a
// job down before it deletes anything.
func (q *Queue) cleanupLoop(ctx context.Context) {
	tick := time.NewTicker(cleanupEvery)
	defer tick.Stop()
	for {
		q.Cleanup(ctx)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-q.cleanupWake:
		}
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
	if job.Kind == KindUninstall || job.Kind == KindVerify || job.Kind == KindMove {
		return nil // no download, so nothing to size; a move checks its space when it is queued
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
		// The probe just ran, so its size is newer than any total kept
		// from an earlier attempt (CORE-15). Progress totals arriving
		// later in this run still win via Notify.
		if j := q.find(job.ID); j != nil && size.DownloadBytes > 0 {
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

// SetOnInstalled registers the callback for finished installs and updates that
// report the version they installed (the install ledger).
func (q *Queue) SetOnInstalled(f func(job types.Job, installed types.Installed)) {
	q.mu.Lock()
	q.onInstalled = f
	q.mu.Unlock()
}

// Add queues a job. A game has at most one active job.
func (q *Queue) Add(kind, source, gameID, storeGameID, title, installDir string) (types.Job, error) {
	return q.AddTargeted(kind, source, gameID, storeGameID, title, installDir, nil)
}

// AddTargeted queues an install or update of a specific version (the
// `versions` capability). The target is kept with the job, so a resumed or
// retried job installs the same version; it names a channel or build and never
// holds a password.
func (q *Queue) AddTargeted(kind, source, gameID, storeGameID, title, installDir string, target *types.VersionTarget) (types.Job, error) {
	switch kind {
	case KindInstall, KindUpdate, KindUninstall:
	default:
		return types.Job{}, fmt.Errorf("unknown job kind %q", kind)
	}
	if kind != KindUninstall && !filepath.IsAbs(installDir) {
		return types.Job{}, fmt.Errorf("install folder %q is not an absolute path", installDir)
	}
	if target != nil && kind == KindUninstall {
		return types.Job{}, errors.New("an uninstall has no version target")
	}
	if target != nil {
		t := *target
		target = &t
	}
	return q.enqueue(&types.Job{
		Kind: kind, Source: source, GameID: gameID, StoreGameID: storeGameID,
		Title: title, InstallDir: installDir, Target: target,
	})
}

// AddVerify queues a check of an installed game's files in installDir. With
// repair set, the plugin restores the files it found damaged; the report of
// the check is kept on the job (Verify).
func (q *Queue) AddVerify(source, gameID, storeGameID, title, installDir string, repair bool) (types.Job, error) {
	if !filepath.IsAbs(installDir) {
		return types.Job{}, fmt.Errorf("install folder %q is not an absolute path", installDir)
	}
	return q.enqueue(&types.Job{
		Kind: KindVerify, Source: source, GameID: gameID, StoreGameID: storeGameID,
		Title: title, InstallDir: installDir, Repair: repair,
	})
}

// AddMove queues moving an installed game from installDir to destination.
// Both must be absolute; the move itself checks what the folders hold.
func (q *Queue) AddMove(source, gameID, storeGameID, title, installDir, destination string) (types.Job, error) {
	if !filepath.IsAbs(installDir) || !filepath.IsAbs(destination) {
		return types.Job{}, errors.New("a move needs absolute folders")
	}
	return q.enqueue(&types.Job{
		Kind: KindMove, Source: source, GameID: gameID, StoreGameID: storeGameID,
		Title: title, InstallDir: installDir, Destination: destination,
	})
}

// enqueue queues a new job. A game has at most one job holding it.
func (q *Queue) enqueue(j *types.Job) (types.Job, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, o := range q.jobs {
		if o.GameID == j.GameID && holdsGame(o) {
			return types.Job{}, fmt.Errorf("%s already has a %s queued", j.Title, o.Kind)
		}
	}
	now := time.Now().UnixMilli()
	j.ID, j.State, j.Percent = newID(), StateQueued, -1
	j.CreatedAt, j.UpdatedAt = now, now
	q.jobs = append(q.jobs, j)
	q.changed(true)
	q.signal()
	return *j, nil
}

// holdsGame is true for jobs that keep a game from getting another job: the
// ones in the queue and the paused ones, which resume later.
func holdsGame(j *types.Job) bool {
	return j.State == StateQueued || j.State == StateRunning || j.State == StatePaused
}

// Cancel stops a queued, running or paused job. Its downloaded data stays
// until the cleanup threshold passes (or Discard); until then Resume picks it
// up again.
func (q *Queue) Cancel(ctx context.Context, id string) error {
	q.mu.Lock()
	j := q.find(id)
	if j == nil {
		q.mu.Unlock()
		return fmt.Errorf("no job %s", id)
	}
	state, snapshot := j.State, *j
	if !holdsGame(j) {
		q.mu.Unlock()
		return nil
	}
	j.State, j.Phase, j.Speed = StateCanceled, "", 0
	j.CanceledAt, j.CleanedAt, j.Leftover = q.now().UnixMilli(), 0, ""
	j.UpdatedAt = j.CanceledAt
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
	q.wakeCleanup() // threshold 0 deletes right away
	return nil
}

// Pause stops a queued or running job and keeps all its data. The plugin is
// asked to stop like a cancel; the job stays paused, across restarts, until
// Resume. It doesn't hold up the rest of the queue.
func (q *Queue) Pause(ctx context.Context, id string) error {
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
	j.State, j.Phase, j.Speed = StatePaused, "", 0
	j.UpdatedAt = q.now().UnixMilli()
	q.changed(true)
	q.mu.Unlock()
	if state == StateRunning {
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_ = q.b.Cancel(cctx, snapshot)
	}
	q.signal()
	return nil
}

// Resume queues a paused or canceled job again, with the same job id so the
// plugin continues from what is on disk. A canceled job whose data was already
// deleted starts over.
func (q *Queue) Resume(id string) error {
	return q.requeue(id, StatePaused, StateCanceled)
}

// Retry queues a failed or canceled job again.
func (q *Queue) Retry(id string) error {
	return q.requeue(id, StateFailed, StateCanceled)
}

func (q *Queue) requeue(id string, from ...string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	j := q.find(id)
	if j == nil {
		return fmt.Errorf("no job %s", id)
	}
	if !slices.Contains(from, j.State) {
		return nil
	}
	if q.discarding[id] {
		return fmt.Errorf("%s is being cleaned up; try again in a moment", j.Title)
	}
	for _, o := range q.jobs {
		if o != j && o.GameID == j.GameID && holdsGame(o) {
			return fmt.Errorf("%s already has a %s queued", j.Title, o.Kind)
		}
	}
	// A resumed run starts counting crashes afresh. Pausing or canceling and
	// resuming shows "resuming" until the plugin reports.
	was := j.State
	j.State, j.Error, j.Phase, j.Attempts = StateQueued, "", "", 0
	j.CanceledAt, j.CleanedAt, j.Leftover = 0, 0, ""
	if was != StateFailed {
		j.Phase = "resuming"
	}
	j.UpdatedAt = q.now().UnixMilli()
	q.changed(true)
	q.signal()
	return nil
}

// ClearFinished drops done, failed and canceled jobs from the list. A canceled
// job whose data hasn't been deleted yet stays, so clearing the list can't
// orphan downloaded data nobody tracks anymore.
func (q *Queue) ClearFinished() {
	q.mu.Lock()
	defer q.mu.Unlock()
	kept := q.jobs[:0]
	for _, j := range q.jobs {
		if holdsGame(j) || q.holdsData(j) {
			kept = append(kept, j)
		}
	}
	q.jobs = kept
	q.changed(true)
}

// holdsData is true for a canceled download whose partial data is still on
// disk. Callers hold q.mu.
func (q *Queue) holdsData(j *types.Job) bool {
	return j.State == StateCanceled && j.CleanedAt == 0 && (j.Kind == KindInstall || j.Kind == KindUpdate)
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
		if u.Verify != nil {
			q.setVerify(source, u.JobID, *u.Verify)
		}
		job, ok := q.finishFrom(source, u.JobID, StateDone, "")
		if !ok {
			return
		}
		q.mu.Lock()
		installed := q.onInstalled
		q.mu.Unlock()
		if u.Installed != nil && job.Kind != KindUninstall && installed != nil {
			installed(job, *u.Installed)
		}
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
	if u.Bytes >= 0 {
		j.Bytes = u.Bytes
		if j.Total > 0 && j.Bytes > j.Total {
			j.Bytes = j.Total
		}
	}
	j.Speed = u.Speed
	switch {
	case u.Percent != nil:
		j.Percent = min(max(*u.Percent, 0), 100)
		// A plugin that reports only a percentage still gets a byte readout,
		// estimated from the size Hydra queued the job with.
		if u.Bytes == 0 && j.Total > 0 {
			j.Bytes = int64(j.Percent / 100 * float64(j.Total))
		}
	case j.Total > 0:
		j.Percent = min(float64(j.Bytes)/float64(j.Total)*100, 100)
	}
	j.UpdatedAt = time.Now().UnixMilli()
	q.changed(false)
	q.mu.Unlock()
}

// setVerify keeps what a verify job found, for the job's running source.
func (q *Queue) setVerify(source, id string, report types.VerifyReport) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if j := q.find(id); j != nil && j.Source == source && j.State == StateRunning {
		j.Verify = &report
	}
}

// SetLeftover notes what a finished move could not clean up: the old folder,
// still on disk after the game was moved. The job stays done.
func (q *Queue) SetLeftover(id, msg string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if j := q.find(id); j != nil {
		j.Leftover = msg
		q.changed(true)
	}
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

// finishFrom ends a running job that source owns and returns it (as it was
// when it ended).
func (q *Queue) finishFrom(source, id, state, msg string) (types.Job, bool) {
	q.mu.Lock()
	j := q.find(id)
	ok := j != nil && j.Source == source && j.State == StateRunning
	var snapshot types.Job
	if ok {
		snapshot = *j
	}
	q.mu.Unlock()
	if ok {
		q.finish(id, state, msg)
	}
	return snapshot, ok
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
		if !holdsGame(j) {
			finished++
		}
	}
	if finished <= keepFinished {
		return
	}
	drop := finished - keepFinished
	kept := q.jobs[:0]
	for _, j := range q.jobs {
		if drop > 0 && !holdsGame(j) && !q.holdsData(j) {
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
// throttled. A failed state save is counted and remembered (see SaveErrors)
// but never blocks the live UI. Callers hold q.mu.
func (q *Queue) changed(state bool) {
	now := time.Now()
	if state || now.Sub(q.lastSave) >= saveEvery {
		q.lastSave = now
		if err := q.save(); err != nil {
			q.saveErrs++
			q.lastSaveErr = err.Error()
		}
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
	dir := filepath.Dir(q.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Temp file + Sync + rename, mirroring settings.save, so a crash
	// mid-write never leaves a truncated jobs file (CORE-12).
	tmp, err := os.CreateTemp(dir, ".jobs-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), q.path)
}

// SetClock replaces the time source (tests).
func (q *Queue) SetClock(now func() time.Time) {
	q.mu.Lock()
	q.now = now
	q.mu.Unlock()
}

// SetCleanupAfter sets how long a canceled job's data is kept before it is
// deleted; 0 deletes it as soon as the job is canceled. Canceled jobs already
// older than the new threshold are cleaned right away.
func (q *Queue) SetCleanupAfter(d time.Duration) {
	q.mu.Lock()
	q.cleanupAfter = max(d, 0)
	q.mu.Unlock()
	q.wakeCleanup()
}

func (q *Queue) wakeCleanup() {
	select {
	case q.cleanupWake <- struct{}{}:
	default:
	}
}

// Cleanup deletes the data of every canceled job older than the threshold.
// Paused jobs, and jobs in the queue, are never touched.
func (q *Queue) Cleanup(ctx context.Context) {
	q.mu.Lock()
	cutoff := q.now().Add(-q.cleanupAfter).UnixMilli()
	var ids []string
	for _, j := range q.jobs {
		if q.holdsData(j) && j.CanceledAt <= cutoff {
			ids = append(ids, j.ID)
		}
	}
	// Uninstalls have no download: just stop tracking their leftovers.
	for _, j := range q.jobs {
		if j.State == StateCanceled && j.CleanedAt == 0 && j.Kind == KindUninstall && j.CanceledAt <= cutoff {
			j.CleanedAt = q.now().UnixMilli()
			q.changed(true)
		}
	}
	q.mu.Unlock()
	for _, id := range ids {
		if ctx.Err() != nil {
			return
		}
		_ = q.Discard(ctx, id)
	}
}

// Discard deletes the downloaded data of a canceled job now. Only canceled
// jobs qualify: pausing keeps data for good, and a queued or running job is
// still using it. A plugin that can't delete anything leaves the job
// forgotten, with a note about what may remain on disk.
func (q *Queue) Discard(ctx context.Context, id string) error {
	q.mu.Lock()
	j := q.find(id)
	if j == nil {
		q.mu.Unlock()
		return fmt.Errorf("no job %s", id)
	}
	if j.State != StateCanceled {
		q.mu.Unlock()
		return fmt.Errorf("%s isn't canceled; cancel it before discarding its data", j.Title)
	}
	if j.CleanedAt != 0 || q.discarding[id] {
		q.mu.Unlock()
		return nil
	}
	snapshot := *j
	q.discarding[id] = true
	q.mu.Unlock()

	err := ErrDiscardUnsupported
	if d, ok := q.b.(Discarder); ok && snapshot.Kind != KindUninstall {
		dctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		err = d.Discard(dctx, snapshot)
		cancel()
	}
	var rpcErr *jsonrpc.Error
	unsupported := errors.Is(err, ErrDiscardUnsupported) || (errors.As(err, &rpcErr) && rpcErr.Code == jsonrpc.CodeMethodNotFound)

	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.discarding, id)
	j = q.find(id)
	if j == nil || j.State != StateCanceled {
		return nil
	}
	switch {
	case err == nil:
		j.CleanedAt, j.Leftover = q.now().UnixMilli(), ""
	case unsupported:
		j.CleanedAt = q.now().UnixMilli()
		j.Leftover = fmt.Sprintf("The %s plugin can't delete a partial download. Files may remain in %s.", j.Source, j.InstallDir)
	default:
		j.Leftover = "Couldn't delete the downloaded data: " + err.Error()
	}
	j.UpdatedAt = q.now().UnixMilli()
	q.changed(true)
	if err != nil && !unsupported {
		return err
	}
	return nil
}

// SaveErrors counts failed queue persistence attempts since Open.
func (q *Queue) SaveErrors() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.saveErrs
}

// LastSaveError is the latest queue persistence error, or "".
func (q *Queue) LastSaveError() string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.lastSaveErr
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// freeSpace returns the bytes available to the user on the filesystem that
// holds path, or its nearest existing parent.
// FreeSpace is the free bytes on the filesystem of path, for a move's check.
func FreeSpace(path string) (uint64, error) { return freeSpace(path) }

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
