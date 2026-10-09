package main

import (
	"fmt"
	"slices"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

// Verify files: the `verify` capability (protocol 1.4). The plugin checks the
// files of an installed game against the store's own copies. A check first
// reports the damage; the repair is a second run the user asks for, and Hydra
// only queues it after a check found something (docs/plugins/protocol.md,
// "Verify and move").

// verifyGate says whether the game's plugin can check this game right now:
// the plugin is ready, offers verify (which needs protocol 1.4), and the game's
// own features keep it. Hydra sends nothing when this fails.
func (a *App) verifyGate(game types.Game) (plugins.Plugin, error) {
	p, ok := a.plugins.Plugin(game.Source)
	if !ok {
		return plugins.Plugin{}, fmt.Errorf("%s doesn't come from a store plugin", game.Title)
	}
	if p.Status != plugins.StatusReady {
		return p, fmt.Errorf("%s is %s", p.Manifest.Name, p.Status)
	}
	if !a.plugins.Has(game.Source, "verify") {
		return p, fmt.Errorf("%s can't verify files", p.Manifest.Name)
	}
	if game.Features != nil && !slices.Contains(game.Features, "verify") {
		return p, fmt.Errorf("%s can't verify %s", p.Manifest.Name, game.Title)
	}
	return p, nil
}

// VerifyGame queues a check of an installed game's files. It only reports;
// nothing is changed until RepairGame.
func (a *App) VerifyGame(game types.Game) (types.Job, error) {
	if !game.IsInstalled || game.InstallDir == "" {
		return types.Job{}, fmt.Errorf("%s is not installed", game.Title)
	}
	if _, err := a.verifyGate(game); err != nil {
		return types.Job{}, err
	}
	if a.sessions.Running(game.ID) {
		return types.Job{}, fmt.Errorf("%s is running", game.Title)
	}
	return a.jobs.AddVerify(game.Source, game.ID, game.StoreAppID, game.Title, game.InstallDir, false)
}

// RepairGame queues the second run of a check, which restores the files the
// last check found damaged or missing. Hydra refuses it unless that check
// reported some, so a repair always follows a report the user saw.
func (a *App) RepairGame(game types.Game) (types.Job, error) {
	if !game.IsInstalled || game.InstallDir == "" {
		return types.Job{}, fmt.Errorf("%s is not installed", game.Title)
	}
	if _, err := a.verifyGate(game); err != nil {
		return types.Job{}, err
	}
	if a.sessions.Running(game.ID) {
		return types.Job{}, fmt.Errorf("%s is running", game.Title)
	}
	last, ok := a.lastVerify(game.ID)
	if !ok || last.Verify == nil || last.Verify.Damaged+last.Verify.Missing == 0 {
		return types.Job{}, fmt.Errorf("check %s first: there is no damage report to repair from", game.Title)
	}
	return a.jobs.AddVerify(game.Source, game.ID, game.StoreAppID, game.Title, game.InstallDir, true)
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
