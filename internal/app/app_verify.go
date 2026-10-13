package app

import (
	"fmt"
	"slices"

	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/plugins"
	"platypus/internal/types"
)

// Verify files: the `verify` capability (protocol 1.4). The plugin checks the
// files of an installed game against the store's own copies. A check first
// reports the damage; the repair is a second run the user asks for, and Platypus
// only queues it after a check found something (docs/plugins/protocol.md,
// "Verify and move").

// verifyGate says whether the game's plugin can check this game right now:
// the plugin is ready, offers verify (which needs protocol 1.4), and the game's
// own features keep it. Platypus sends nothing when this fails.
func (a *App) verifyGate(game types.Game) (plugins.Plugin, error) {
	p, ok := a.plugins.Plugin(game.Source)
	if !ok {
		return plugins.Plugin{}, apperr.New("game.notFromStore", fmt.Sprintf("%s doesn't come from a store plugin", game.Title), map[string]string{"title": game.Title})
	}
	if p.Status != plugins.StatusReady {
		return p, apperr.New("plugin.notReady", fmt.Sprintf("%s is %s", p.Manifest.Name, p.Status), map[string]string{"name": p.Manifest.Name, "status": string(p.Status)})
	}
	if !a.plugins.Has(game.Source, "verify") {
		return p, apperr.New("verify.unsupported", fmt.Sprintf("%s can't verify files", p.Manifest.Name), map[string]string{"name": p.Manifest.Name})
	}
	if game.Features != nil && !slices.Contains(game.Features, "verify") {
		return p, apperr.New("verify.gameUnsupported", fmt.Sprintf("%s can't verify %s", p.Manifest.Name, game.Title), map[string]string{"name": p.Manifest.Name, "title": game.Title})
	}
	return p, nil
}

// VerifyGame queues a check of an installed game's files. It only reports;
// nothing is changed until RepairGame.
func (a *App) VerifyGame(game types.Game) (types.Job, error) {
	folder, err := a.verifyFolder(game)
	if err != nil {
		return types.Job{}, err
	}
	return a.jobs.AddVerify(game.Source, game.ID, game.StoreAppID, game.Title, folder, false)
}

// RepairGame queues the second run of a check, which restores the files the
// last check found damaged or missing. Platypus refuses it unless that check
// reported some, so a repair always follows a report the user saw.
func (a *App) RepairGame(game types.Game) (types.Job, error) {
	folder, err := a.verifyFolder(game)
	if err != nil {
		return types.Job{}, err
	}
	last, ok := a.lastVerify(game.ID)
	if !ok || last.Verify == nil || last.Verify.Damaged+last.Verify.Missing == 0 {
		return types.Job{}, apperr.New("verify.noReport", fmt.Sprintf("check %s first: there is no damage report to repair from", game.Title), map[string]string{"title": game.Title})
	}
	return a.jobs.AddVerify(game.Source, game.ID, game.StoreAppID, game.Title, folder, true)
}

// verifyFolder checks a check or repair of game can run now, and returns the
// folder it runs in: the one the library reports, not the UI's copy.
func (a *App) verifyFolder(game types.Game) (string, error) {
	if !game.IsInstalled {
		return "", apperr.New("game.notInstalled", fmt.Sprintf("%s is not installed", game.Title), map[string]string{"title": game.Title})
	}
	if _, err := a.verifyGate(game); err != nil {
		return "", err
	}
	if a.isRunning(game.ID) {
		return "", apperr.New("game.running", fmt.Sprintf("%s is running", game.Title), map[string]string{"title": game.Title})
	}
	folder, err := a.currentFolder(game)
	if err != nil {
		return "", err
	}
	if err := checkFolderFor(folder, a.settings.GamesFolder(), "check", a.homeDir()); err != nil {
		return "", err
	}
	return folder, nil
}

// lastVerify is the most recent finished check of a game (the queue's list
// is newest last).
func (a *App) lastVerify(gameID string) (types.Job, bool) {
	list := a.jobs.List()
	for i := len(list) - 1; i >= 0; i-- {
		j := list[i]
		if j.GameID == gameID && j.Kind == jobs.KindVerify && !j.Repair && j.State == jobs.StateDone {
			return j, true
		}
	}
	return types.Job{}, false
}

// verifyParams are the params of game.verify for a job.
func verifyParams(j types.Job) map[string]any {
	return map[string]any{"id": j.StoreGameID, "installDir": j.InstallDir, "jobId": j.ID, "repair": j.Repair}
}
