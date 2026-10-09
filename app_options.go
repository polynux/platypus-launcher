package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
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

// optionsVersion is the first protocol revision with install options.
var optionsVersion = plugins.Version{Major: 1, Minor: 5}

// optionsAllowed says whether the plugin offers install options right now: it
// has initialized (which settles the protocol it speaks), speaks 1.5 or later,
// and the capability is on. Initializing starts it if needed, so the answer
// is never the manifest's guess.
func (a *App) optionsAllowed(ctx context.Context, source string) bool {
	ctx, cancel := context.WithTimeout(ctx, optionsTimeout)
	defer cancel()
	if _, err := a.plugins.Initialize(ctx, source); err != nil {
		return false
	}
	rt := a.plugins.Runtime(source)
	v, err := plugins.ParseVersion(rt.Protocol)
	return err == nil && !v.Less(optionsVersion) && slices.Contains(rt.Capabilities, "install-options")
}

// optionsOffered is optionsAllowed for a game: the game's own `features`, when
// it lists some, must keep the capability too.
func (a *App) optionsOffered(ctx context.Context, game types.Game) bool {
	if !a.optionsAllowed(ctx, game.Source) {
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
	if !a.optionsOffered(a.ctx, game) {
		return p, fmt.Errorf("%s doesn't offer install options for %s", p.Manifest.Name, game.Title)
	}
	return p, nil
}

// fetchGroups asks the plugin for the groups of a game, normalized for core.
func (a *App) fetchGroups(ctx context.Context, source, storeAppID string) ([]types.OptionGroup, error) {
	ctx, cancel := context.WithTimeout(ctx, optionsTimeout)
	defer cancel()
	var res struct {
		Groups []types.OptionGroup `json:"groups"`
	}
	if err := a.plugins.Call(ctx, source, "game.options", map[string]any{"id": storeAppID}, &res); err != nil {
		return nil, err
	}
	return types.NormalizeGroups(res.Groups), nil
}

// savedOptions is what a plain install or update of the game queues with: the
// choices the user saved for it, re-checked leniently against the store's
// groups as they are now. If the store can't be asked, the saved choices are
// queued as they were saved; the job checks them again when it starts, and
// fails with a retryable message if the store is still unreachable then. Nil
// means the store's defaults.
func (a *App) savedOptions(game types.Game) types.InstallOptions {
	saved, ok := a.settings.Options(game.ID)
	if !ok || !a.optionsOffered(a.ctx, game) {
		return nil
	}
	groups, err := a.fetchGroups(a.ctx, game.Source, game.StoreAppID)
	if err != nil {
		a.warnf("%s: saved install options queued unchecked: %v", game.Title, err)
		return saved
	}
	resolved, err := types.ResolveOptions(groups, saved, types.Check{Installed: game.IsInstalled})
	if err != nil {
		a.warnf("%s: saved install options queued unchecked: %v", game.Title, err)
		return saved
	}
	return resolved
}

// optionsForJob is what a job sends as its install options when it is sized
// or started. Choices are checked now, not when the job was queued: the store
// may have dropped a DLC (refunded while paused) or a value since. The options
// are sent only while the plugin still offers them; otherwise the job installs
// the store's defaults. A store that can't be asked fails the job, retryably:
// the install needs the store anyway.
func (a *App) optionsForJob(ctx context.Context, j types.Job) (types.InstallOptions, error) {
	if j.Options == nil || j.Kind == jobs.KindUninstall {
		return nil, nil
	}
	if !a.optionsAllowed(ctx, j.Source) {
		a.warnf("%s: install options not sent, the store no longer offers them", j.Title)
		return nil, nil
	}
	groups, err := a.fetchGroups(ctx, j.Source, j.StoreGameID)
	if err != nil {
		return nil, fmt.Errorf("couldn't check add-ons with the store; retry: %w", err)
	}
	resolved, err := types.ResolveOptions(groups, j.Options, types.Check{Installed: j.Kind == jobs.KindUpdate})
	if err != nil {
		return nil, err
	}
	if changed := types.ChangedOptions(j.Options, resolved); len(changed) > 0 {
		a.warnf("%s: install options changed since they were chosen, sending the current ones; changed: %s", j.Title, strings.Join(changed, ", "))
	}
	return resolved, nil
}

// GetGameOptions lists what the install dialog asks for a game, and the
// choices Hydra would use now (the saved ones, or the defaults).
func (a *App) GetGameOptions(game types.Game) (types.GameOptions, error) {
	if _, err := a.optionsGate(game); err != nil {
		return types.GameOptions{}, err
	}
	groups, err := a.fetchGroups(a.ctx, game.Source, game.StoreAppID)
	if err != nil {
		return types.GameOptions{}, err
	}
	saved, _ := a.settings.Options(game.ID)
	chosen, err := types.ResolveOptions(groups, saved, types.Check{Installed: game.IsInstalled})
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
	groups, err := a.fetchGroups(a.ctx, game.Source, game.StoreAppID)
	if err != nil {
		return types.Job{}, err
	}
	resolved, err := types.ResolveOptions(groups, choices, types.Check{Strict: true, Installed: kind == jobs.KindUpdate})
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

// GetInstallSize is the disk space a fresh install of the game needs with the
// store's default choices, the figure the free-space check compares with. The
// dialog adds the chosen add-ons to it. An error means the store can't say.
func (a *App) GetInstallSize(game types.Game) (int64, error) {
	target, err := a.pinnedTarget(game)
	if err != nil {
		return 0, err
	}
	s, err := jobBackend{a}.Size(a.ctx, types.Job{Kind: jobs.KindInstall, Source: game.Source, StoreGameID: game.StoreAppID, Target: target})
	return s.DiskBytes, err
}

// GetInstallFreeSpace is the free space, in bytes, where the game would be
// installed. The dialog compares it with the size of the chosen options.
func (a *App) GetInstallFreeSpace(game types.Game) (int64, error) {
	folder, _ := a.plugins.InstallFolder(game.Source, a.settings.GamesFolder())
	free, err := jobs.FreeSpace(folder)
	return int64(free), err
}
