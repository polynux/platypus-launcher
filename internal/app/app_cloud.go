package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"platypus/internal/cloudsync"
	"platypus/internal/runner"
	"platypus/internal/session"
	"platypus/internal/stores"
	"platypus/internal/types"
	"platypus/internal/apperr"
)

// cloudShutdownGrace is how long Platypus waits for running cloud syncs when it
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
		Emit:      func(e types.CloudEvent) { a.host.Emit("cloud", e) },
		Logf:      func(f string, args ...any) { a.host.Infof("cloud: "+f, args...) },
	})
	a.cloud.Resume()
}

// cloudGame says whether the game has cloud saves Platypus can sync: it comes
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
	return cloudsync.Game{ID: game.ID, PluginID: game.Source, StoreID: game.StoreAppID, Title: game.OriginalTitle()}, true
}

// cloudContext describes a launch to the plugin. windows says the game runs
// through Wine or Proton; r is the runner then. prefixesFolder is where Wine
// prefixes are created ("" for the default); the runner package is never
// asked to read settings itself.
func (a *App) cloudContext(game types.Game, windows bool, r types.ProtonVersion, prefixesFolder string) types.CloudContext {
	prefix, user := "", ""
	if windows {
		prefix = runner.PrefixPath(game, a.homeDir(), prefixesFolder)
		if strings.HasSuffix(r.Path, "proton") {
			// Proton keeps the Wine prefix in a pfx folder of its compat data.
			prefix, user = filepath.Join(prefix, "pfx"), "steamuser"
		}
	}
	return cloudsync.ContextFor(a.homeDir(), windows, game.InstallDir, prefix, user)
}

// cloudContextOf is the context for a launch that runs with runner r: through
// Wine or Proton when the runner is set, natively otherwise. Every cloud step
// uses this one rule, so a check and a launch of a game agree on its paths.
func (a *App) cloudContextOf(game types.Game, r types.ProtonVersion, prefixesFolder string) types.CloudContext {
	return a.cloudContext(game, r.Path != "", r, prefixesFolder)
}

// cloudContextFor is the context for a game that isn't being launched (Check
// now, resolving): the launch spec's preview says which runner it would use.
// The folder is the library's (currentFolder); without one there is no context
// to send, so the plugin is never asked.
func (a *App) cloudContextFor(game types.Game) (types.CloudContext, error) {
	folder, err := a.currentFolder(game)
	if err != nil {
		return types.CloudContext{}, err
	}
	game.InstallDir = folder
	if p, err := a.prepareLaunch(game, true); err == nil && p.client == nil {
		return a.cloudContextOf(game, p.runner, a.prefixesFolder()), nil
	}
	// The launch can't be prepared: use the runner it would pick.
	var r types.ProtonVersion
	if !game.Native {
		r, _ = a.selectRunner(game)
	}
	return a.cloudContextOf(game, r, a.prefixesFolder()), nil
}

// lockGame serializes the launch and cloud steps of one game, so that a second
// Play waits for the first to start the game and then sees it running, and a
// resolve never runs beside a launch of the same game.
func (a *App) lockGame(gameID string) (unlock func()) {
	a.gameMu.Lock()
	if a.gameLocks == nil {
		a.gameLocks = map[string]*sync.Mutex{}
	}
	m := a.gameLocks[gameID]
	if m == nil {
		m = &sync.Mutex{}
		a.gameLocks[gameID] = m
	}
	a.gameMu.Unlock()
	m.Lock()
	return m.Unlock
}

// cloudLaunch is a launch whose exit is followed by a push. job is the id of
// the push record the launch wrote: its exit and its abandon find that record
// and no other.
type cloudLaunch struct {
	a   *App
	g   cloudsync.Game
	job string
}

func (c *cloudLaunch) options() []session.StartOption {
	if c == nil {
		return nil
	}
	return []session.StartOption{session.WithOnExit(func(i session.ExitInfo) { c.a.cloud.Exited(c.g.ID, c.job, i.Failed) })}
}

func (c *cloudLaunch) abandon() {
	if c != nil {
		c.a.cloud.Abandon(c.g.ID, c.job)
	}
}

// cloudBeforeLaunch syncs a game's cloud saves before Platypus starts it. anyway
// is the user's "launch anyway" for this launch only. blocked means the launch
// waits for the user (msg says on what); otherwise sync, when not nil, registers
// the post-exit push. When the launch goes on without a sync, msg says why, if
// the game has cloud saves at all (a note, not a question).
func (a *App) cloudBeforeLaunch(game types.Game, p *prepared, anyway bool) (sync *cloudLaunch, msg string, blocked bool) {
	g, ok := a.cloudGame(game)
	if !ok {
		return nil, "", false
	}
	folder, err := a.currentFolder(game)
	if err != nil {
		// No folder to read saves from: the game starts without a cloud step, and says so.
		return nil, cloudSkipNote(err), false
	}
	game.InstallDir = folder
	cc := a.cloudContextOf(game, p.runner, a.prefixesFolder())
	d := a.cloud.BeforeLaunch(a.ctx, g, cc, anyway)
	if !d.Proceed {
		if d.Message != "" {
			return nil, d.Message, true
		}
		return nil, "Waiting for your choice about " + game.Title + "'s cloud saves", true
	}
	if !d.Sync {
		return nil, "", false
	}
	job := a.cloud.Launching(g, cc)
	return &cloudLaunch{a: a, g: g, job: job}, "", false
}

// cloudSkipNote is why a launch goes without a cloud sync, in words for the user.
func cloudSkipNote(err error) string {
	return "cloud saves not synced: " + err.Error()
}

// cloudInfo is the game page's view of the game's cloud saves.
func (a *App) cloudInfo(game types.Game) types.CloudInfo {
	running := a.isRunning(game.ID)
	g, ok := a.cloudGame(game)
	if !ok {
		return types.CloudInfo{Mode: types.CloudOff, PluginMode: cloudsync.DefaultMode, Running: running}
	}
	info := a.cloud.Info(g)
	info.Running = running
	// An installed game whose folder is unknown launches without a sync: say so here too.
	if _, err := a.currentFolder(game); err != nil && game.IsInstalled {
		info.Skipped = err.Error()
	}
	return info
}

// GetCloudInfo returns what Platypus knows about the game's cloud saves without
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
		return a.cloudInfo(game), apperr.New("cloud.none", "this game has no cloud saves Platypus can sync", nil)
	}
	cc, err := a.cloudContextFor(game)
	if err != nil {
		return a.cloud.Info(g), err
	}
	if _, err := a.cloud.Check(a.ctx, g, cc); err != nil {
		return a.cloud.Info(g), err
	}
	return a.cloud.Info(g), nil
}

// ResolveCloudSaves settles conflicts: choices maps each file's path to
// "local" (keep this device's copy) or "remote" (keep the cloud copy, after
// backing up the local one). Files left out stay conflicts.
func (a *App) ResolveCloudSaves(game types.Game, choices map[string]string) (types.CloudInfo, error) {
	unlock := a.lockGame(game.ID)
	defer unlock()
	g, ok := a.cloudGame(game)
	if !ok {
		return a.cloudInfo(game), apperr.New("cloud.none", "this game has no cloud saves Platypus can sync", nil)
	}
	if a.isRunning(game.ID) {
		// The game writes its saves while it runs: settle them after it exits.
		return a.cloudInfo(game), apperr.New("cloud.running", fmt.Sprintf("%s is running; choose its saves after it exits", game.Title), map[string]string{"title": game.Title})
	}
	cc, err := a.cloudContextFor(game)
	if err != nil {
		return a.cloud.Info(g), err
	}
	if _, err := a.cloud.Resolve(a.ctx, g, cc, choices); err != nil {
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
	if g, ok := a.cloudGame(game); ok && prompt == types.CloudPromptOffline {
		a.cloud.Confirm(g.ID)
	}
	return a.launchGame(game, true)
}

// LaunchGameViaClient starts the game through the store's own client, which
// then syncs its saves itself ("Launch through Steam").
func (a *App) LaunchGameViaClient(game types.Game) (string, error) {
	unlock := a.lockGame(game.ID)
	defer unlock()
	if a.isRunning(game.ID) {
		return "", apperr.New("game.running", fmt.Sprintf("%s is already running", game.Title), map[string]string{"title": game.Title})
	}
	if err := a.heldByJob(game); err != nil {
		return "", err
	}
	s, ok := a.stores.Get(sourceOf(game))
	cl, isCl := s.(stores.ClientLauncher)
	if !ok || !isCl {
		return "", apperr.New("game.noClientLaunch", fmt.Sprintf("%s can't be started through a store client", game.Title), map[string]string{"title": game.Title})
	}
	ctx, cancel := context.WithTimeout(a.ctx, launchTimeout)
	defer cancel()
	launch, err := cl.LaunchViaClient(ctx, game)
	if err != nil {
		return "", err
	}
	if len(launch.Command) == 0 {
		return "", apperr.New("game.noCommand", fmt.Sprintf("%s returned no command", game.Title), map[string]string{"title": game.Title})
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
