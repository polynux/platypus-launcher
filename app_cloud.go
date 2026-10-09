package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"hydra-launcher/internal/cloudsync"
	"hydra-launcher/internal/runner"
	"hydra-launcher/internal/session"
	"hydra-launcher/internal/stores"
	"hydra-launcher/internal/types"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// cloudShutdownGrace is how long Hydra waits for running cloud syncs when it
// quits. A push cut short stays owed (cloud.json) and runs at the next start.
const cloudShutdownGrace = 3 * time.Second

// cloudBackend reaches plugins for the cloud orchestrator.
type cloudBackend struct{ a *App }

// Offers: the plugin runs, has initialized, and offers cloud-saves right now.
// A plugin that settled on a protocol older than 1.2 never does (the runtime
// capability set already leaves it out).
func (b cloudBackend) Offers(ctx context.Context, pluginID string) bool {
	if _, err := b.a.plugins.Initialize(ctx, pluginID); err != nil {
		return false
	}
	return b.a.plugins.Has(pluginID, "cloud-saves")
}

func (b cloudBackend) Call(ctx context.Context, pluginID, method string, params, result any) error {
	return b.a.plugins.Call(ctx, pluginID, method, params, result)
}

// startCloud creates the cloud orchestrator and resumes the pushes an earlier
// run left owed.
func (a *App) startCloud(ctx context.Context, dataDir string) {
	a.cloud = cloudsync.New(cloudsync.Options{
		Backend:   cloudBackend{a},
		Settings:  a.settings,
		StatePath: filepath.Join(dataDir, "cloud.json"),
		Emit:      func(e types.CloudEvent) { wruntime.EventsEmit(ctx, "cloud", e) },
		Logf:      func(f string, args ...any) { wruntime.LogInfof(ctx, "cloud: "+f, args...) },
	})
	a.cloud.Resume()
}

// cloudGame says whether the game has cloud saves Hydra can sync: it comes
// from a plugin that offers cloud-saves and the game's features include it.
func (a *App) cloudGame(game types.Game) (cloudsync.Game, bool) {
	if a.cloud == nil || a.plugins == nil || game.Source == "" || game.StoreAppID == "" {
		return cloudsync.Game{}, false
	}
	if _, ok := a.plugins.Plugin(game.Source); !ok || !a.plugins.Has(game.Source, "cloud-saves") {
		return cloudsync.Game{}, false
	}
	if game.Features != nil && !slices.Contains(game.Features, "cloud-saves") {
		return cloudsync.Game{}, false
	}
	return cloudsync.Game{ID: game.ID, PluginID: game.Source, StoreID: game.StoreAppID, Title: game.Title}, true
}

// cloudContext describes a launch to the plugin. windows says the game runs
// through Wine or Proton; r is the runner then.
func cloudContext(game types.Game, windows bool, r types.ProtonVersion) types.CloudContext {
	prefix, user := "", ""
	if windows {
		home, _ := os.UserHomeDir()
		prefix = runner.PrefixPath(game, home)
		if strings.HasSuffix(r.Path, "proton") {
			// Proton keeps the Wine prefix in a pfx folder of its compat data.
			prefix, user = filepath.Join(prefix, "pfx"), "steamuser"
		}
	}
	return cloudsync.Context(windows, game.InstallDir, prefix, user)
}

// cloudContextFor is the context for a game that isn't being launched (Check
// now, resolving): the launch spec's preview says whether Wine is involved.
func (a *App) cloudContextFor(game types.Game) types.CloudContext {
	if p, err := a.prepareLaunch(game, true); err == nil && p.client == nil {
		return cloudContext(game, p.runner.Path != "", p.runner)
	}
	windows := !game.Native
	var r types.ProtonVersion
	if windows {
		r, _ = selectRunner(game)
	}
	return cloudContext(game, windows, r)
}

// cloudLaunch is a launch whose exit is followed by a push.
type cloudLaunch struct {
	a *App
	g cloudsync.Game
}

func (c *cloudLaunch) options() []session.StartOption {
	if c == nil {
		return nil
	}
	return []session.StartOption{session.WithOnExit(func(i session.ExitInfo) { c.a.cloud.Exited(c.g.ID, i.Failed) })}
}

func (c *cloudLaunch) abandon() {
	if c != nil {
		c.a.cloud.Abandon(c.g.ID)
	}
}

// cloudBeforeLaunch syncs a game's cloud saves before Hydra starts it. blocked
// means the launch waits for the user (msg says on what); otherwise sync, when
// not nil, registers the post-exit push.
func (a *App) cloudBeforeLaunch(game types.Game, p *prepared) (sync *cloudLaunch, msg string, blocked bool) {
	g, ok := a.cloudGame(game)
	if !ok {
		return nil, "", false
	}
	cc := cloudContext(game, p.runner.Path != "", p.runner)
	d := a.cloud.BeforeLaunch(a.ctx, g, cc)
	if !d.Proceed {
		if d.Message != "" {
			return nil, d.Message, true
		}
		return nil, "Waiting for your choice about " + game.Title + "'s cloud saves", true
	}
	if !d.Sync {
		return nil, "", false
	}
	a.cloud.Launching(g, cc)
	return &cloudLaunch{a: a, g: g}, "", false
}

// cloudInfo is the game page's view of the game's cloud saves.
func (a *App) cloudInfo(game types.Game) types.CloudInfo {
	g, ok := a.cloudGame(game)
	if !ok {
		return types.CloudInfo{Mode: types.CloudOff, PluginMode: cloudsync.DefaultMode}
	}
	return a.cloud.Info(g)
}

// GetCloudInfo returns what Hydra knows about the game's cloud saves without
// asking the plugin: the modes and the last status of this session.
func (a *App) GetCloudInfo(game types.Game) types.CloudInfo {
	return a.cloudInfo(game)
}

// CheckCloudSaves compares the game's saves with the cloud (read-only) and
// returns the result. A store that can't be reached comes back as an error
// status, not an error.
func (a *App) CheckCloudSaves(game types.Game) (types.CloudInfo, error) {
	g, ok := a.cloudGame(game)
	if !ok {
		return a.cloudInfo(game), errors.New("this game has no cloud saves Hydra can sync")
	}
	if _, err := a.cloud.Check(a.ctx, g, a.cloudContextFor(game)); err != nil {
		return a.cloud.Info(g), err
	}
	return a.cloud.Info(g), nil
}

// ResolveCloudSaves settles conflicts: choices maps each file's path to
// "local" (keep this device's copy) or "remote" (keep the cloud copy, after
// backing up the local one). Files left out stay conflicts.
func (a *App) ResolveCloudSaves(game types.Game, choices map[string]string) (types.CloudInfo, error) {
	g, ok := a.cloudGame(game)
	if !ok {
		return a.cloudInfo(game), errors.New("this game has no cloud saves Hydra can sync")
	}
	if _, err := a.cloud.Resolve(a.ctx, g, a.cloudContextFor(game), choices); err != nil {
		return a.cloud.Info(g), err
	}
	return a.cloud.Info(g), nil
}

// CancelCloudSync stops the game's running cloud sync, if any.
func (a *App) CancelCloudSync(gameID string) bool {
	return a.cloud != nil && a.cloud.Cancel(gameID)
}

// LaunchGameAnyway launches a game its cloud step stopped: "Launch anyway"
// after a conflict the user will settle later, an unreachable store, or the
// store client running. prompt is the prompt being answered; confirming an
// unreachable store lets `auto` mode stop asking.
func (a *App) LaunchGameAnyway(game types.Game, prompt string) (string, error) {
	if g, ok := a.cloudGame(game); ok {
		a.cloud.Allow(g.ID, prompt == types.CloudPromptOffline)
	}
	return a.LaunchGame(game)
}

// LaunchGameViaClient starts the game through the store's own client, which
// then syncs its saves itself ("Launch through Steam").
func (a *App) LaunchGameViaClient(game types.Game) (string, error) {
	if a.isRunning(game.ID) {
		return "", fmt.Errorf("%s is already running", game.Title)
	}
	if err := a.heldByJob(game); err != nil {
		return "", err
	}
	s, ok := a.stores.Get(sourceOf(game))
	cl, isCl := s.(stores.ClientLauncher)
	if !ok || !isCl {
		return "", fmt.Errorf("%s can't be started through a store client", game.Title)
	}
	ctx, cancel := context.WithTimeout(a.ctx, launchTimeout)
	defer cancel()
	launch, err := cl.LaunchViaClient(ctx, game)
	if err != nil {
		return "", err
	}
	if len(launch.Command) == 0 {
		return "", fmt.Errorf("%s returned no command", game.Title)
	}
	cmd := exec.Command(launch.Command[0], launch.Command[1:]...)
	cmd.Env = os.Environ()
	return a.startClient(game, cmd, launch)
}

// GetCloudSettings returns the saved sync modes: each plugin's default and
// each game's own choice. Absent means the default (ask).
func (a *App) GetCloudSettings() types.CloudSettings {
	plugins, games := a.settings.CloudModes()
	return types.CloudSettings{Plugins: plugins, Games: games}
}

// SetCloudPluginMode saves a plugin's default sync mode (off, ask or auto);
// "" restores the default.
func (a *App) SetCloudPluginMode(pluginID, mode string) error {
	return a.settings.SetCloudPluginMode(pluginID, mode)
}

// SetCloudGameMode saves one game's sync mode; "" makes it follow its plugin.
func (a *App) SetCloudGameMode(gameID, mode string) error {
	return a.settings.SetCloudGameMode(gameID, mode)
}
