package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/cloudsync"
	"platypus/internal/savebackup"
	"platypus/internal/session"
	"platypus/internal/stores"
	"platypus/internal/types"
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
		Emit:      func(e types.CloudEvent) { a.cloudEmit(e) },
		Logf:      func(f string, args ...any) { a.host.Infof("cloud: "+f, args...) },
	})
	a.cloud.Resume()
}

// cloudEmit forwards a cloud event to the UI and logs its result (uploaded,
// downloaded, conflict) to the activity feed (issue #145). Downloads come
// from the pull's Done count (A3), never from the English message text, and
// carry the file count for the UI's own wording (A5). A push-time conflict
// arrives as Failed (A7); prompt conflicts are deduped per game until the
// next push or clean status.
func (a *App) cloudEmit(e types.CloudEvent) {
	if a.host != nil {
		a.host.Emit("cloud", e)
	}
	a.activityMu.Lock()
	if a.cloudConflict == nil {
		a.cloudConflict = map[string]bool{}
	}
	switch e.Kind {
	case types.CloudEvPushed:
		delete(a.cloudConflict, e.GameID)
		a.activityMu.Unlock()
		n := e.Done
		if n < 0 {
			n = 0
		}
		a.recordCloudActivity("cloud", e.GameID, "activity:log.cloudUploaded",
			map[string]string{"files": fmt.Sprint(n)})
	case types.CloudEvPrompt:
		if e.Prompt != types.CloudPromptConflict {
			a.activityMu.Unlock()
			return
		}
		if a.cloudConflict[e.GameID] {
			a.activityMu.Unlock()
			return
		}
		a.cloudConflict[e.GameID] = true
		a.activityMu.Unlock()
		a.recordCloudActivity("cloud", e.GameID, "activity:log.cloudConflict", nil)
	case types.CloudEvStatus:
		if e.Status != nil && e.Status.State == types.CloudStateSynced {
			delete(a.cloudConflict, e.GameID)
		}
		if e.Done <= 0 {
			a.activityMu.Unlock()
			return
		}
		a.activityMu.Unlock()
		a.recordCloudActivity("cloud", e.GameID, "activity:log.cloudDownloaded",
			map[string]string{"files": fmt.Sprint(e.Done)})
	case types.CloudEvFailed:
		if e.Status == nil || e.Status.State != types.CloudStateConflict {
			a.activityMu.Unlock()
			return
		}
		if a.cloudConflict[e.GameID] {
			a.activityMu.Unlock()
			return
		}
		a.cloudConflict[e.GameID] = true
		a.activityMu.Unlock()
		a.recordCloudActivity("cloud", e.GameID, "activity:log.cloudConflict", nil)
	default:
		a.activityMu.Unlock()
	}
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

// cloudContext is the context for a game with the prefix its runner plugin
// reports: a Windows game reads its saves from that prefix, a native one from
// its own folders. Every cloud step uses this one rule, so a check and a launch
// of a game agree on its paths. The core doesn't interpret Wine/Proton prefix
// layouts.
func (a *App) cloudContext(game types.Game, prefix savebackup.Prefix) types.CloudContext {
	return cloudsync.ContextFor(a.homeDir(), prefix.Dir != "", game.InstallDir, prefix.Dir, prefix.User)
}

// cloudContextFor is the context for a game that isn't being launched (Check
// now, resolving): the runner plugin says where its prefix is, without a store
// launch. The folder is the library's (currentFolder); without one there is no
// context to send, so the plugin is never asked.
func (a *App) cloudContextFor(game types.Game) (types.CloudContext, error) {
	folder, err := a.currentFolder(game)
	if err != nil {
		return types.CloudContext{}, err
	}
	game.InstallDir = folder
	prefix, err := a.gamePrefix(game)
	if err != nil {
		return types.CloudContext{}, err
	}
	return a.cloudContext(game, prefix), nil
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

// cloudLaunchTimeout bounds the pre-launch cloud check (issue #44): the
// launch path waits at most this long for cloud.status. Longer, and an
// offline store would stall every Play.
const cloudLaunchTimeout = 5 * time.Second

// cloudBeforeLaunch syncs a game's cloud saves before Platypus starts it. anyway
// is the user's "launch anyway" for this launch only. blocked means the launch
// waits for the user (msg says on what); otherwise sync, when not nil, registers
// the post-exit push. When the launch goes on without a sync, msg says why, if
// the game has cloud saves at all (a note, not a question).
//
// Offline is never in the way: while offline cloud.status is skipped and the
// launch proceeds with the note "cloud saves not checked: offline". While
// online the check is bounded by cloudLaunchTimeout; a network-class failure
// (the bound ran out, or the machine went offline during the check) proceeds
// with the same note. Anything else (a conflict, the store client running)
// still waits for the user: only the network never blocks.
func (a *App) cloudBeforeLaunch(game types.Game, anyway bool) (sync *cloudLaunch, msg string, blocked bool) {
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
	if !a.netOnline() {
		return nil, "cloud saves not checked: offline", false
	}
	prefix, err := a.gamePrefix(game)
	if err != nil {
		return nil, cloudSkipNote(err), false
	}
	cc := a.cloudContext(game, prefix)
	ctx, cancel := context.WithTimeout(a.ctx, cloudLaunchTimeout)
	defer cancel()
	d := a.cloud.BeforeLaunch(ctx, g, cc, anyway)
	if !d.Proceed {
		if d.Prompt == types.CloudPromptOffline && (ctx.Err() != nil || !a.netOnline()) {
			return nil, "cloud saves not checked: offline", false
		}
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
