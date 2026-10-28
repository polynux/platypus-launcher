package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/compat"
	"platypus/internal/jobs"
	"platypus/internal/plugins"
	"platypus/internal/runner"
	"platypus/internal/types"
)

// Runner releases (issue #142): installable GE-Proton builds, downloaded
// through the existing downloader, verified against the release checksum,
// unpacked safely and run as a job with progress. Removal only deletes
// user-installed builds in the runner folder.

// runnerReleasesURL overrides the GitHub endpoint in tests (httptest).
func (a *App) runnerReleasesURL() string {
	if v := os.Getenv("PLATYPUS_GE_RELEASES_URL"); v != "" {
		return v
	}
	return ""
}

// ListAvailableRunners lists installable GE-Proton releases.
func (a *App) ListAvailableRunners() ([]types.AvailableRunner, error) {
	if !a.netOnline() {
		return nil, apperr.New("runner.offline", "You're offline; connect to list installable runners", nil)
	}
	fetcher := compat.NewFetcher("platypus")
	fetcher.Client = a.netClient(nil, 15*time.Second)
	ctx, cancel := context.WithTimeout(a.ctxOrBackground(), 30*time.Second)
	defer cancel()
	return runner.FetchAvailable(ctx, fetcher, a.runnerReleasesURL())
}

// runnerByID finds an installed runner by id.
func (a *App) runnerByID(id string) (types.ProtonVersion, bool) {
	for _, r := range a.GetProtonVersions() {
		if r.ID == id {
			return r, true
		}
	}
	return types.ProtonVersion{}, false
}

// runnerUsedBy lists saved games whose runner_id names id (or its name).
func (a *App) runnerUsedBy(id, name string) []string {
	var out []string
	if a.settings == nil {
		return out
	}
	for gameID, saved := range a.settings.GameSettings() {
		if v, ok := saved["runner_id"]; ok {
			if s, ok := v.(string); ok && (s == id || (name != "" && s == name)) {
				out = append(out, gameID)
			}
		}
	}
	return out
}

// CheckRemoveRunner says whether a runner can be removed and who uses it.
func (a *App) CheckRemoveRunner(id string) (types.RunnerRemoveCheck, error) {
	r, ok := a.runnerByID(id)
	if !ok {
		return types.RunnerRemoveCheck{}, apperr.New("game.runnerMissing",
			fmt.Sprintf("Runner %s is not installed", id), map[string]string{"id": id})
	}
	removable := runner.Removable(r, a.homeDir())
	used := a.runnerUsedBy(r.ID, r.Name)
	return types.RunnerRemoveCheck{ID: id, SizeBytes: r.SizeBytes, UsedBy: used, Removable: removable}, nil
}

// RemoveRunner deletes a user-installed build. System Wine and Steam-managed
// Proton are never removed. Without force, a build games still use is kept
// so the caller can warn first.
func (a *App) RemoveRunner(id string, force bool) error {
	r, ok := a.runnerByID(id)
	if !ok {
		return apperr.New("game.runnerMissing",
			fmt.Sprintf("Runner %s is not installed", id), map[string]string{"id": id})
	}
	dir, err := runner.BuildDirForRemovable(r, a.homeDir())
	if err != nil {
		return apperr.New("runner.notRemovable", err.Error(), map[string]string{"id": id})
	}
	if used := a.runnerUsedBy(r.ID, r.Name); len(used) > 0 && !force {
		return apperr.New("runner.inUse",
			fmt.Sprintf("%s is still used by %d game(s); switch them first or remove anyway", r.Name, len(used)),
			map[string]string{"id": id})
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return nil
}

// InstallRunner queues installing one GE-Proton release as a job with
// progress. Tag is the release tag (e.g. "GE-Proton10-4").
func (a *App) InstallRunner(tag string) (types.Job, error) {
	if tag == "" {
		return types.Job{}, apperr.New("runner.unknownVersion", "Choose a runner version to install", nil)
	}
	if r, ok := a.runnerByID("compat:" + tag); ok && r.ID != "" {
		return types.Job{}, apperr.New("runner.alreadyInstalled",
			fmt.Sprintf("%s is already installed", r.Name), map[string]string{"id": r.ID})
	}
	// Resolve the display name without the network when it is already
	// listed; the job resolves the URL itself at start.
	title := tag
	dir := runner.UserRunnerDir(a.homeDir())
	gameID := "runner:" + tag
	if _, err := a.jobs.AddRunnerInstall("runner", gameID, tag, title, dir); err != nil {
		return types.Job{}, err
	}
	// Return the queued job.
	for _, j := range a.jobs.List() {
		if j.GameID == gameID && (j.State == jobs.StateQueued || j.State == jobs.StateRunning || j.State == jobs.StatePaused) {
			return j, nil
		}
	}
	return types.Job{}, fmt.Errorf("runner install of %s did not queue", tag)
}

// startCoreRunnerInstall runs a runner install in its own goroutine.
func (a *App) startCoreRunnerInstall(j types.Job) error {
	ctx, cancel := context.WithCancel(a.ctx)
	run, err := a.runnerInstalls.begin(j.ID, cancel)
	if err != nil {
		cancel()
		return err
	}
	go func() {
		defer a.runnerInstalls.end(j.ID, run)
		defer cancel()
		a.runCoreRunnerInstall(ctx, j)
	}()
	return nil
}

// runCoreRunnerInstall downloads, verifies and unpacks one release, reporting
// progress through the queue. Staging lives next to the runner folder so a
// cancel or failure leaves no partial build behind.
func (a *App) runCoreRunnerInstall(ctx context.Context, j types.Job) {
	tag := j.StoreGameID
	notifyProgress := func(done, total int64) {
		a.jobs.Notify(j.Source, "job.progress", plugins.JobUpdate{JobID: j.ID, Phase: "downloading", Bytes: done, Total: total})
	}
	fetcher := compat.NewFetcher("platypus")
	fetcher.Client = a.netClient(nil, 30*time.Second)
	listCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	avail, err := runner.FetchAvailable(listCtx, fetcher, a.runnerReleasesURL())
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: err.Error()})
		return
	}
	var picked *types.AvailableRunner
	for i := range avail {
		if avail[i].Tag == tag {
			picked = &avail[i]
			break
		}
	}
	if picked == nil {
		if ctx.Err() != nil {
			return
		}
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: fmt.Sprintf("runner version %q is not available", tag)})
		return
	}
	destDir := j.InstallDir
	if destDir == "" {
		destDir = runner.UserRunnerDir(a.homeDir())
	}
	parent := filepath.Dir(filepath.Clean(destDir))
	stage, err := os.MkdirTemp(parent, ".runner-install-*")
	if err != nil {
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: err.Error()})
		return
	}
	defer os.RemoveAll(stage)
	tmpPath := filepath.Join(stage, "runner.tar.gz")
	out, err := os.Create(tmpPath)
	if err != nil {
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: err.Error()})
		return
	}
	dlErr := runner.Download(ctx, a.netClient(nil, 0), picked.URL, out, notifyProgress)
	_ = out.Close()
	if dlErr != nil {
		if ctx.Err() != nil {
			return
		}
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: dlErr.Error()})
		return
	}
	if picked.ChecksumURL != "" {
		// Fetch the checksum file through the downloader (small file).
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		sumBody, ferr := fetcher.Get(cctx, picked.ChecksumURL)
		cancel()
		if ferr != nil {
			if ctx.Err() != nil {
				return
			}
			a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: ferr.Error()})
			return
		}
		if verr := runner.VerifyChecksumFile(tmpPath, string(sumBody)); verr != nil {
			if ctx.Err() != nil {
				return
			}
			a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: verr.Error()})
			return
		}
	}
	f, err := os.Open(tmpPath)
	if err != nil {
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: err.Error()})
		return
	}
	top, err := runner.UnpackTarGz(f, stage, 0)
	_ = f.Close()
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: err.Error()})
		return
	}
	final := filepath.Join(destDir, top)
	if _, err := os.Lstat(final); err == nil {
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: fmt.Sprintf("%s is already installed", top)})
		return
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: err.Error()})
		return
	}
	if err := os.Rename(filepath.Join(stage, top), final); err != nil {
		if ctx.Err() != nil {
			return
		}
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: err.Error()})
		return
	}
	a.noteNetSuccess()
	a.jobs.Notify(j.Source, "job.progress", plugins.JobUpdate{JobID: j.ID, Phase: "done", Bytes: 1, Total: 1})
	a.jobs.Notify(j.Source, "job.done", plugins.JobUpdate{JobID: j.ID})
}
