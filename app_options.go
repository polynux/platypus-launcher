package main

import (
	"context"
	"fmt"
	"slices"
	"time"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

// Install options: the `install-options` capability (protocol 1.5,
// docs/plans/launcher-parity.md §2.1). The plugin lists the groups (language,
// add-on content, packs, toggles) and installs the choices it is sent; core
// remembers each game's choices, checks them against the store's groups before
// anything is queued, and passes them on every later install and update. An
// update with new choices is a game.update with the new `options`.

const optionsTimeout = 30 * time.Second

// optionsOffered says whether the game's plugin offers install options for
// it right now: the capability is on at runtime (needs protocol 1.5) and the
// game's own `features`, when it lists some, keep it.
func (a *App) optionsOffered(game types.Game) bool {
	if !a.plugins.Has(game.Source, "install-options") {
		return false
	}
	return game.Features == nil || slices.Contains(game.Features, "install-options")
}

// optionsGate is optionsOffered with the reasons the UI can show. Hydra sends
// no game.options or option choices when it fails.
func (a *App) optionsGate(game types.Game) (plugins.Plugin, error) {
	p, ok := a.plugins.Plugin(game.Source)
	if !ok {
		return plugins.Plugin{}, fmt.Errorf("%s doesn't come from a store plugin", game.Title)
	}
	if p.Status != plugins.StatusReady {
		return p, fmt.Errorf("%s is %s", p.Manifest.Name, p.Status)
	}
	if !a.optionsOffered(game) {
		return p, fmt.Errorf("%s doesn't offer install options for %s", p.Manifest.Name, game.Title)
	}
	return p, nil
}

// fetchOptions asks the plugin for the groups of a game, normalized for core.
func (a *App) fetchOptions(game types.Game) ([]types.OptionGroup, error) {
	ctx, cancel := context.WithTimeout(a.ctx, optionsTimeout)
	defer cancel()
	var res struct {
		Groups []types.OptionGroup `json:"groups"`
	}
	if err := a.plugins.Call(ctx, game.Source, "game.options", map[string]any{"id": game.StoreAppID}, &res); err != nil {
		return nil, err
	}
	return types.NormalizeGroups(res.Groups), nil
}

// savedOptions is what a plain install or update of the game sends: the
// choices the user saved for it, re-checked leniently against the store's
// groups as they are now. Nil means the store's defaults (nothing to send).
func (a *App) savedOptions(game types.Game) (types.InstallOptions, error) {
	saved, ok := a.settings.Options(game.ID)
	if !ok || !a.optionsOffered(game) {
		return nil, nil
	}
	groups, err := a.fetchOptions(game)
	if err != nil {
		return nil, fmt.Errorf("%s: its install options can't be read right now: %w", game.Title, err)
	}
	return types.ResolveOptions(groups, saved, false)
}

// GetGameOptions lists what the install dialog asks for a game, and the
// choices Hydra would use now (the saved ones, or the defaults).
func (a *App) GetGameOptions(game types.Game) (types.GameOptions, error) {
	if _, err := a.optionsGate(game); err != nil {
		return types.GameOptions{}, err
	}
	groups, err := a.fetchOptions(game)
	if err != nil {
		return types.GameOptions{}, err
	}
	saved, _ := a.settings.Options(game.ID)
	chosen, err := types.ResolveOptions(groups, saved, false)
	if err != nil {
		return types.GameOptions{}, err
	}
	return types.GameOptions{Groups: groups, Chosen: chosen}, nil
}

// InstallGameWithOptions queues an install with the choices the user made in
// the install dialog. A choice the store doesn't offer, or DLC the user doesn't
// own, refuses the install; the choices are remembered once it is queued.
// The choices are a plain map: Wails binds named map types as nothing in the
// generated TypeScript.
func (a *App) InstallGameWithOptions(game types.Game, options map[string]any) (types.Job, error) {
	if game.IsInstalled {
		return types.Job{}, fmt.Errorf("%s is already installed", game.Title)
	}
	if !game.Installable {
		return types.Job{}, fmt.Errorf("%s can't be installed by Hydra", game.Title)
	}
	return a.queueWithOptions(jobs.KindInstall, game, options)
}

// UpdateGameWithOptions changes the add-ons or options of an installed game.
// It is a game.update with the new options; the plugin installs what was added,
// and removes what was switched off only where the store allows it.
func (a *App) UpdateGameWithOptions(game types.Game, options map[string]any) (types.Job, error) {
	if !game.IsInstalled {
		return types.Job{}, fmt.Errorf("%s is not installed", game.Title)
	}
	return a.queueWithOptions(jobs.KindUpdate, game, options)
}

// queueWithOptions checks the user's choices strictly against the store's
// groups, keeps the game's version pin, queues the job with the resolved
// options and remembers them for later installs and updates.
func (a *App) queueWithOptions(kind string, game types.Game, choices types.InstallOptions) (types.Job, error) {
	if _, err := a.optionsGate(game); err != nil {
		return types.Job{}, err
	}
	groups, err := a.fetchOptions(game)
	if err != nil {
		return types.Job{}, err
	}
	resolved, err := types.ResolveOptions(groups, choices, true)
	if err != nil {
		return types.Job{}, err
	}
	target, err := a.pinnedTarget(game)
	if err != nil {
		return types.Job{}, err
	}
	job, err := a.queueJobWith(kind, game, target, resolved)
	if err != nil {
		return job, err
	}
	if err := a.settings.SetOptions(game.ID, resolved); err != nil {
		a.warnf("install options: %v", err)
	}
	return job, nil
}

// GetInstallFreeSpace is the free space, in bytes, where the game would be
// installed. The dialog compares it with the size of the chosen options.
func (a *App) GetInstallFreeSpace(game types.Game) (int64, error) {
	folder, _ := a.plugins.InstallFolder(game.Source, a.settings.GamesFolder())
	free, err := jobs.FreeSpace(folder)
	return int64(free), err
}
