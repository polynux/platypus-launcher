package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

// jobBackend runs queue jobs through the owning plugin.
type jobBackend struct{ a *App }

func (b jobBackend) Size(ctx context.Context, j types.Job) (plugins.InstallSize, error) {
	var s plugins.InstallSize
	err := b.a.plugins.Call(ctx, j.Source, "game.installSize", map[string]any{"id": j.StoreGameID}, &s)
	return s, err
}

func (b jobBackend) Start(ctx context.Context, j types.Job) error {
	return b.a.plugins.Call(ctx, j.Source, "game."+j.Kind, installParams(j), nil)
}

// installParams builds the params for game.install/game.update/game.uninstall
// from a job. Plugins may key their own records on `title` (GOG keeps it in
// its installed index, steam-standalone names its dl request), so Hydra sends
// the title the job already has. Branches and their passwords are plugin-side
// choices: Hydra sends no branch key at all — an empty one must not leak into
// the params. installDir only makes sense where files land.
func installParams(j types.Job) map[string]any {
	params := map[string]any{"id": j.StoreGameID, "jobId": j.ID}
	if j.Kind == jobs.KindUninstall {
		return params
	}
	params["installDir"] = j.InstallDir
	if j.Title != "" {
		params["title"] = j.Title
	}
	// branch/branchPassword would go here per-plugin; none is set, so the
	// params carry none: a plugin defaults to its own branch.
	return params
}

func (b jobBackend) Cancel(ctx context.Context, j types.Job) error {
	return b.a.plugins.Call(ctx, j.Source, "job.cancel", map[string]any{"jobId": j.ID}, nil)
}

// queueJob checks the game's plugin can run the job and queues it. Hydra picks
// the install folder; plugins never choose paths.
func (a *App) queueJob(kind string, game types.Game) (types.Job, error) {
	p, ok := a.plugins.Plugin(game.Source)
	if !ok {
		return types.Job{}, fmt.Errorf("%s doesn't come from a store plugin", game.Title)
	}
	if p.Status != plugins.StatusReady {
		return types.Job{}, fmt.Errorf("%s is %s", p.Manifest.Name, p.Status)
	}
	if !slices.Contains(p.Manifest.Capabilities, "install") {
		return types.Job{}, fmt.Errorf("%s can't install games", p.Manifest.Name)
	}
	folder, _ := a.plugins.InstallFolder(game.Source, a.settings.GamesFolder())
	return a.jobs.Add(kind, game.Source, game.ID, game.StoreAppID, game.Title, folder)
}

// InstallGame queues an install into the plugin's install folder.
func (a *App) InstallGame(game types.Game) (types.Job, error) {
	if game.IsInstalled {
		return types.Job{}, fmt.Errorf("%s is already installed", game.Title)
	}
	if !game.Installable {
		return types.Job{}, fmt.Errorf("%s can't be installed by Hydra", game.Title)
	}
	return a.queueJob(jobs.KindInstall, game)
}

// UpdateGame queues an update of an installed plugin game.
func (a *App) UpdateGame(game types.Game) (types.Job, error) {
	if !game.IsInstalled {
		return types.Job{}, fmt.Errorf("%s is not installed", game.Title)
	}
	return a.queueJob(jobs.KindUpdate, game)
}

// UninstallGame queues removal of an installed plugin game.
func (a *App) UninstallGame(game types.Game) (types.Job, error) {
	if !game.IsInstalled {
		return types.Job{}, fmt.Errorf("%s is not installed", game.Title)
	}
	if a.sessions.Running(game.ID) {
		return types.Job{}, fmt.Errorf("%s is running", game.Title)
	}
	return a.queueJob(jobs.KindUninstall, game)
}

// ListJobs returns the download queue and recent history.
func (a *App) ListJobs() []types.Job {
	return a.jobs.List()
}

// CancelJob stops a download. Its data is deleted after the cleanup
// threshold (Settings → Downloads) unless it is resumed first.
func (a *App) CancelJob(id string) error {
	return a.jobs.Cancel(a.ctx, id)
}

// PauseJob stops a download and keeps everything downloaded so far.
func (a *App) PauseJob(id string) error {
	return a.jobs.Pause(a.ctx, id)
}

// ResumeJob continues a paused or canceled download where it stopped.
func (a *App) ResumeJob(id string) error {
	return a.jobs.Resume(id)
}

// DiscardJob deletes the downloaded data of a canceled download right now.
func (a *App) DiscardJob(id string) error {
	return a.jobs.Discard(a.ctx, id)
}

// PauseAllJobs pauses every queued and running download. Queued ones first,
// so pausing the running download doesn't start the next one for a moment.
func (a *App) PauseAllJobs() error {
	var errs []error
	for _, state := range []string{jobs.StateQueued, jobs.StateRunning} {
		for _, j := range a.jobs.List() {
			if j.State == state {
				errs = append(errs, a.jobs.Pause(a.ctx, j.ID))
			}
		}
	}
	return errors.Join(errs...)
}

// ResumeAllJobs resumes every paused download, oldest first.
func (a *App) ResumeAllJobs() error {
	var errs []error
	for _, j := range a.jobs.List() {
		if j.State == jobs.StatePaused {
			errs = append(errs, a.jobs.Resume(j.ID))
		}
	}
	return errors.Join(errs...)
}

// SetCancelCleanupDays sets how many days a canceled download's data is kept
// before Hydra deletes it (0 = right away). It applies to downloads that are
// already canceled too.
func (a *App) SetCancelCleanupDays(days int) (types.LauncherSettings, error) {
	if err := a.settings.SetCancelCleanupDays(days); err != nil {
		return a.launcherSettings(), err
	}
	a.jobs.SetCleanupAfter(cleanupAfter(days))
	return a.launcherSettings(), nil
}

func cleanupAfter(days int) time.Duration {
	return time.Duration(days) * 24 * time.Hour
}

func (a *App) RetryJob(id string) error {
	return a.jobs.Retry(id)
}

func (a *App) ClearFinishedJobs() {
	a.jobs.ClearFinished()
}
