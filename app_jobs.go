package main

import (
	"context"
	"fmt"
	"slices"

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

func (a *App) CancelJob(id string) error {
	return a.jobs.Cancel(a.ctx, id)
}

func (a *App) RetryJob(id string) error {
	return a.jobs.Retry(id)
}

func (a *App) ClearFinishedJobs() {
	a.jobs.ClearFinished()
}
