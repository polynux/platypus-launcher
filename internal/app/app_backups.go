package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/cloudsync"
	"platypus/internal/jobs"
	"platypus/internal/plugins"
	"platypus/internal/savebackup"
	"platypus/internal/session"
	"platypus/internal/settings"
	"platypus/internal/steam"
	"platypus/internal/types"
)

// Save backups (issue #42): per-game save files are backed up before any
// destructive action, optionally after each session, and by hand from the
// game page. The gate itself is the shared seam in app_savegate.go; this
// file holds the service, the API, the core-run jobs and the session hook.

// backupTriggers name why a backup runs.
const (
	triggerManual        = "manual"
	triggerUninstall     = "uninstall"
	triggerAfterSession  = "after-session"
	triggerBeforeRestore = "before-restore"
)

// afterSessionWait bounds how long an after-session backup waits for a cloud
// job of the same game.
const afterSessionWait = 60 * time.Second

// coreBackups tracks the core-run backup/restore jobs, like coreMoves: one
// run per job until its goroutine has unwound.
type coreBackups struct {
	mu      sync.Mutex
	runs    map[string]*coreBackupRun
	stopped map[string]bool
}

// coreBackupRun is one backup core runs. done closes once its goroutine ends.
type coreBackupRun struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func (m *coreBackups) begin(id string, cancel context.CancelFunc) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped[id] {
		delete(m.stopped, id)
		return fmt.Errorf("the backup was canceled before it started")
	}
	if _, busy := m.runs[id]; busy {
		return fmt.Errorf("the earlier backup of this job is still stopping; try again in a moment")
	}
	if m.runs == nil {
		m.runs = map[string]*coreBackupRun{}
	}
	m.runs[id] = &coreBackupRun{cancel: cancel, done: make(chan struct{})}
	return nil
}

func (m *coreBackups) end(id string) {
	m.mu.Lock()
	r, ok := m.runs[id]
	if ok {
		delete(m.runs, id)
	}
	m.mu.Unlock()
	if ok {
		close(r.done)
	}
}

func (m *coreBackups) stop(ctx context.Context, id string) {
	m.mu.Lock()
	r, ok := m.runs[id]
	if !ok {
		if m.stopped == nil {
			m.stopped = map[string]bool{}
		}
		m.stopped[id] = true
	}
	m.mu.Unlock()
	if !ok {
		return
	}
	r.cancel()
	select {
	case <-r.done:
	case <-ctx.Done():
	}
}

func (m *coreBackups) active(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.runs[id]
	return ok
}

// saveService returns the backup service, creating it on first use from the
// configured folders so tests that build an App by hand get one too.
func (a *App) saveService() *savebackup.Service {
	if a.savebk != nil {
		return a.savebk
	}
	home := a.homeDir()
	dataDir := a.resolvedDataDir()
	cacheDir := a.optCacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join(home, ".cache", "platypus")
	}
	a.savebk = savebackup.New(savebackup.Options{
		Settings: a.settings,
		DataDir:  dataDir,
		CacheDir: cacheDir,
		Home:     home,
		Emit: func(e savebackup.Event) {
			a.host.Emit("backups", e)
		},
		Lookup: func(gameID string) (types.Game, bool) {
			g, err := a.findGame(gameID)
			return g, err == nil
		},
	})
	return a.savebk
}

// startSavebackups sets the backup service up at startup and refreshes the
// manifest in the background when it is older than 7 days.
func (a *App) startSavebackups(ctx context.Context) {
	svc := a.saveService()
	go func() {
		// Let the startup finish first; the refresh is background work.
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
		svc.MaybeRefresh(ctx)
	}()
}

// backupRunner is the runner path the game would launch with ("", when the
// game runs natively or no runner is known).
func (a *App) backupRunner(game types.Game) string {
	if game.Native {
		return ""
	}
	if r, err := a.selectRunner(game); err == nil {
		return r.Path
	}
	return ""
}

// backupIDs are the game's Steam/GOG ids for manifest matching: the compat
// AppID or info_source gog:<id> for sideloaded games, the store id otherwise.
func (a *App) backupIDs(game types.Game) (string, string) {
	var compat, info string
	if a.settings != nil {
		compat = a.settings.CompatAppID(game.ID)
		info = a.settings.InfoSource(game.ID)
	}
	return savebackup.GameIDs(game, compat, info)
}

// backupSteamRoot is the Steam client folder for save resolution: the real
// install when found, else "" for the default under home.
func (a *App) backupSteamRoot() string {
	return steam.RootFor(a.homeDir())
}

// backupCtx builds the resolution context for a game.
func (a *App) backupCtx(game types.Game, steamID, gogID string) savebackup.Context {
	folder, err := a.currentFolder(game)
	if err == nil {
		game.InstallDir = folder
	}
	ctx := savebackup.NewContextFor(game, a.homeDir(), a.prefixesFolder(), a.resolvedDataDir(), a.backupRunner(game), a.backupSteamRoot())
	ctx.SteamID, ctx.GogID = steamID, gogID
	return ctx
}

// backupGate backs up every save path the game resolves, a superset of what
// a destructive action deletes. A failed backup returns an error and the caller deletes nothing. With no
// known save paths it returns NoPaths with a savebackup.noPaths error,
// unless allowNoSaves is set.
func (a *App) backupGate(ctx context.Context, game types.Game, trigger string, allowNoSaves bool) (types.SaveGate, error) {
	if a.settings != nil && !savebackup.BackupsApplies(a.settings, game.ID) {
		return types.SaveGate{}, nil
	}
	if a.isRunning(game.ID) {
		return types.SaveGate{}, apperr.New("savebackup.running",
			fmt.Sprintf("%s is running; back up its saves after it exits", game.Title),
			map[string]string{"title": game.Title})
	}
	if err := a.backupCloudGate(game); err != nil {
		return types.SaveGate{}, err
	}
	steamID, gogID := a.backupIDs(game)
	var userPaths []string
	if a.settings != nil {
		userPaths = a.settings.GameSavePaths(game.ID)
	}
	svc := a.saveService()
	folder, ferr := a.currentFolder(game)
	if ferr == nil {
		game.InstallDir = folder
	}
	_, patterns, _, _, _ := svc.ResolveFor(game, steamID, gogID, a.backupRunner(game), a.prefixesFolder(), a.backupSteamRoot(), userPaths)
	if len(patterns) == 0 {
		// Cloud games with no known save paths pass without the
		// "delete without a save backup" confirmation: the cloud holds
		// the saves. Games without cloud still ask.
		if a.gameHasCloudSync(game) {
			return types.SaveGate{Cloud: true}, nil
		}
		g := types.SaveGate{NoPaths: true}
		if allowNoSaves {
			return g, nil
		}
		return g, apperr.New("savebackup.noPaths",
			fmt.Sprintf("Platypus knows no save paths for %s; add some or delete without a save backup", game.Title),
			map[string]string{"title": game.Title})
	}
	// The backup takes every known save of the game, a superset of what the
	// action deletes: simpler, and never less safe.
	b, warned, skippedOut, unresolvedOut, err := svc.Backup(game, trigger, savebackup.BackupOptions{
		RunnerPath: a.backupRunner(game), PrefixesFolder: a.prefixesFolder(),
		SteamRoot: a.backupSteamRoot(),
		UserPaths: userPaths, SteamID: steamID, GogID: gogID,
	})
	_ = skippedOut
	_ = unresolvedOut
	if errors.Is(err, savebackup.ErrNothingFound) {
		// Known paths, nothing on disk: nothing to protect.
		return types.SaveGate{}, nil
	}
	if err != nil {
		if ctx.Err() != nil {
			return types.SaveGate{}, ctx.Err()
		}
		return types.SaveGate{}, apperr.Wrap("savebackup.failed", err, map[string]string{"title": game.Title})
	}
	if warned {
		a.host.Warnf("savebackup: %s's saves are above 1 GiB", game.ID)
	}
	return types.SaveGate{BackupID: b.ID, Files: b.Files, Bytes: b.Bytes, Unchanged: b.Unchanged}, nil
}

// savesPreview is what the gate would do, without backing anything up: off,
// no known save paths, cloud (a cloud game with no known paths passes without
// confirmation), or how many save files exist now (CheckRemove).
func (a *App) savesPreview(game types.Game) types.SaveGate {
	if a.settings != nil && !savebackup.BackupsApplies(a.settings, game.ID) {
		return types.SaveGate{Off: true}
	}
	steamID, gogID := a.backupIDs(game)
	var userPaths []string
	if a.settings != nil {
		userPaths = a.settings.GameSavePaths(game.ID)
	}
	if folder, err := a.currentFolder(game); err == nil {
		game.InstallDir = folder
	}
	_, patterns, _, rctx, _ := a.saveService().ResolveFor(game, steamID, gogID, a.backupRunner(game), a.prefixesFolder(), a.backupSteamRoot(), userPaths)
	if len(patterns) == 0 {
		if a.gameHasCloudSync(game) {
			return types.SaveGate{Cloud: true}
		}
		return types.SaveGate{NoPaths: true}
	}
	files, _, _, _, _ := savebackup.Enumerate(patterns, rctx)
	g := types.SaveGate{Files: len(files)}
	for _, f := range files {
		g.Bytes += f.Size
	}
	return g
}

// gameHasCloudSync reports whether the game's store syncs its saves to the
// cloud, so automatic backups skip it by default. It is true when the game's
// plugin offers cloud-saves for this game (the capability, and the game's
// features where the protocol says so) and the effective cloud mode isn't
// off; or, for built-in Steam games, when the cached store details say the
// game has Steam Cloud (category 23, no new request per game). Unknown means
// false.
func (a *App) gameHasCloudSync(game types.Game) bool {
	if g, ok := a.cloudGame(game); ok {
		mode := ""
		if a.cloud != nil {
			mode = a.cloud.Mode(g)
		} else if a.settings != nil {
			if m := a.settings.CloudGameMode(game.ID); types.ValidCloudMode(m) {
				mode = m
			} else if m := a.settings.CloudPluginMode(game.Source); types.ValidCloudMode(m) {
				mode = m
			} else {
				mode = cloudsync.DefaultMode
			}
		} else {
			mode = cloudsync.DefaultMode
		}
		if mode != types.CloudOff {
			return true
		}
	}
	// Only the built-in Steam source launches through the Steam client, which
	// syncs Steam Cloud; a plugin game with store "steam" (steam-standalone)
	// is covered by the plugin's own cloud-saves capability above.
	if game.Source == types.SourceSteam {
		if game.StoreAppID == "" {
			return false
		}
		if a.stores != nil {
			if s, ok := a.stores.Get(types.SourceSteam); ok {
				type cloudCacher interface {
					HasSteamCloudCached(string) (bool, bool)
				}
				if st, ok := s.(cloudCacher); ok {
					if has, cached := st.HasSteamCloudCached(game.StoreAppID); cached {
						return has
					}
				}
			}
		}
	}
	return false
}

// autoBackupState is whether after-session automatic backups run for a game:
// on by default for games without cloud sync, skipped for games with cloud
// sync unless the per-game override forces it on. The per-game "off" always
// disables.
func (a *App) autoBackupState(game types.Game) types.AutoBackupState {
	if a.settings != nil {
		if a.settings.GameSaveBackup(game.ID) == "off" {
			return types.AutoBackupState{On: false, Reason: "off"}
		}
		if a.settings.BackupsMode() == settings.BackupsOff && a.settings.GameSaveBackup(game.ID) != "on" {
			return types.AutoBackupState{On: false, Reason: "off"}
		}
	}
	cloud := a.gameHasCloudSync(game)
	if cloud && (a.settings == nil || a.settings.GameSaveBackup(game.ID) != "on") {
		return types.AutoBackupState{On: false, Reason: "cloud"}
	}
	if a.settings != nil && !a.settings.BackupAfterSession() && a.settings.GameSaveBackup(game.ID) != "on" {
		return types.AutoBackupState{On: false, Reason: "off"}
	}
	return types.AutoBackupState{On: true, Reason: "on"}
}

// GetAutoBackupState returns whether after-session automatic backups run for
// a game and why, for the Maintenance section's effective-state line.
func (a *App) GetAutoBackupState(game types.Game) (types.AutoBackupState, error) {
	return a.autoBackupState(game), nil
}

// backupCloudGate refuses a destructive backup while a cloud job holds the
// game's saves, or the last cloud status saw the store's own client running
// ("close <client> first"). It uses only cached state: no store round trip.
func (a *App) backupCloudGate(game types.Game) error {
	if a.cloud == nil {
		return nil
	}
	g, ok := a.cloudGame(game)
	if !ok {
		return nil
	}
	info := a.cloud.Info(g)
	if info.Busy != "" {
		return apperr.New("savebackup.cloudBusy",
			fmt.Sprintf("cloud sync is busy for %s; back up once it finishes", game.Title),
			map[string]string{"title": game.Title})
	}
	if st := a.cloud.Last(g.ID); st != nil && st.Client != nil && st.Client.Running {
		return apperr.New("savebackup.clientRunning",
			fmt.Sprintf("close %s first: it is running and would write the same saves", st.Client.Name),
			map[string]string{"client": st.Client.Name, "title": game.Title})
	}
	return nil
}

// findGame resolves a game id through the library (sideloaded and store).
// Test harnesses without a data registry resolve nothing: callers fall back
// to the job's own copy of the game.
func (a *App) findGame(gameID string) (types.Game, error) {
	if a.data == nil {
		return types.Game{}, apperr.New("game.notInLibrary", fmt.Sprintf("%s isn't in your library", gameID), map[string]string{"title": gameID})
	}
	lib, err := a.GetLibrary()
	if err != nil {
		return types.Game{}, err
	}
	for _, g := range lib.Games {
		if g.ID == gameID {
			return g, nil
		}
	}
	return types.Game{}, apperr.New("game.notFound", fmt.Sprintf("unknown game %s", gameID), map[string]string{"id": gameID})
}

// ---- API (gen-ts) ----

// GetSavePaths returns a game's save locations with each path's origin
// (manifest/user) and whether it exists.
func (a *App) GetSavePaths(game types.Game) ([]types.SavePath, error) {
	steamID, gogID := a.backupIDs(game)
	var userPaths []string
	if a.settings != nil {
		userPaths = a.settings.GameSavePaths(game.ID)
	}
	folder, err := a.currentFolder(game)
	if err == nil {
		game.InstallDir = folder
	}
	roots, _, _, _, _ := a.saveService().ResolveFor(game, steamID, gogID, a.backupRunner(game), a.prefixesFolder(), a.backupSteamRoot(), userPaths)
	if roots == nil {
		return []types.SavePath{}, nil
	}
	return roots, nil
}

// ListSaveBackups lists a game's backup versions, newest first.
func (a *App) ListSaveBackups(gameID string) ([]types.SaveBackup, error) {
	if out := a.saveService().List(gameID); out != nil {
		return out, nil
	}
	return []types.SaveBackup{}, nil
}

// CreateSaveBackup queues a manual backup job ("Back up now"), always
// available. It is refused while the game runs.
func (a *App) CreateSaveBackup(game types.Game) (types.Job, error) {
	if a.isRunning(game.ID) {
		return types.Job{}, apperr.New("savebackup.running",
			fmt.Sprintf("%s is running; back up its saves after it exits", game.Title),
			map[string]string{"title": game.Title})
	}
	if _, ok := a.jobs.Holding(game.ID, jobs.KindBackup, jobs.KindRestore); ok {
		return types.Job{}, apperr.New("savebackup.queued",
			fmt.Sprintf("%s already has a save backup queued", game.Title),
			map[string]string{"title": game.Title})
	}
	folder, err := a.currentFolder(game)
	if err != nil {
		folder = game.InstallDir
	}
	return a.jobs.AddBackup(game.Source, game.ID, game.StoreAppID, game.Title, folder, triggerManual)
}

// RestoreSaveBackup queues a restore of one backup version. The restore
// takes a before-restore backup first; if that fails the restore is blocked.
// It is refused while the game runs.
func (a *App) RestoreSaveBackup(game types.Game, backupID string) (types.Job, error) {
	if backupID == "" {
		return types.Job{}, apperr.New("savebackup.unknown", "unknown backup version", nil)
	}
	if a.isRunning(game.ID) {
		return types.Job{}, apperr.New("savebackup.running",
			fmt.Sprintf("%s is running; restore its saves after it exits", game.Title),
			map[string]string{"title": game.Title})
	}
	if _, ok := a.jobs.Holding(game.ID, jobs.KindBackup, jobs.KindRestore); ok {
		return types.Job{}, apperr.New("savebackup.queued",
			fmt.Sprintf("%s already has a save backup queued", game.Title),
			map[string]string{"title": game.Title})
	}
	folder, err := a.currentFolder(game)
	if err != nil {
		folder = game.InstallDir
	}
	return a.jobs.AddRestore(game.Source, game.ID, game.StoreAppID, game.Title, folder, backupID)
}

// PreviewRestoreSaveBackup says what a restore would do: overwritten,
// added and kept counts. Only files in the backup are considered.
func (a *App) PreviewRestoreSaveBackup(game types.Game, backupID string) (types.RestorePreview, error) {
	steamID, gogID := a.backupIDs(game)
	pv, err := a.saveService().RestorePreview(game.ID, backupID, a.backupCtx(game, steamID, gogID))
	if err != nil {
		return types.RestorePreview{}, apperr.Wrap("savebackup.unknown", err, nil)
	}
	return pv, nil
}

// DeleteSaveBackup deletes one backup version.
func (a *App) DeleteSaveBackup(gameID, backupID string) error {
	return a.saveService().Delete(gameID, backupID)
}

// GetBackupSettings returns the global save-backup switches and the manifest state.
func (a *App) GetBackupSettings() (types.BackupSettings, error) {
	svc := a.saveService()
	def := settings.DefaultBackupsFolderFor(a.resolvedDataDir())
	folder := def
	if a.settings != nil {
		if f := a.settings.BackupsFolder(); f != "" {
			folder = f
		}
		return types.BackupSettings{
			Folder: folder, DefaultFolder: def, Keep: a.settings.KeepBackups(),
			Mode: a.settings.BackupsMode(), AfterSession: a.settings.BackupAfterSession(),
			Manifest: svc.ManifestStateV(),
		}, nil
	}
	return types.BackupSettings{Folder: folder, DefaultFolder: def, Keep: 10, Mode: "destructive", Manifest: svc.ManifestStateV()}, nil
}

// SetBackupSettings saves the global save-backup switches. Empty folder
// restores the default; keep <= 0 leaves the count unchanged.
func (a *App) SetBackupSettings(folder string, keep int, mode string, afterSession bool) (types.BackupSettings, error) {
	if a.settings == nil {
		return types.BackupSettings{}, apperr.New("settings.unavailable", "settings are not ready", nil)
	}
	if _, err := a.settings.SetBackupsFolder(folder); err != nil {
		return types.BackupSettings{}, err
	}
	if keep != 0 {
		if err := a.settings.SetKeepBackups(keep); err != nil {
			return types.BackupSettings{}, err
		}
	}
	if mode != "" {
		if err := a.settings.SetBackupsMode(mode); err != nil {
			return types.BackupSettings{}, err
		}
	}
	if err := a.settings.SetBackupAfterSession(afterSession); err != nil {
		return types.BackupSettings{}, err
	}
	return a.GetBackupSettings()
}

// SetGameSavePaths saves one game's user save paths (the Maintenance editor).
func (a *App) SetGameSavePaths(gameID string, paths []string) error {
	if a.settings == nil {
		return apperr.New("settings.unavailable", "settings are not ready", nil)
	}
	return a.settings.SetGameSavePaths(gameID, paths)
}

// SetGameSaveBackup saves one game's backup override ("" follows the global
// mode, "on" or "off").
func (a *App) SetGameSaveBackup(gameID, mode string) error {
	if a.settings == nil {
		return apperr.New("settings.unavailable", "settings are not ready", nil)
	}
	return a.settings.SetGameSaveBackup(gameID, mode)
}

// GetGameSaveBackup returns one game's backup override ("" follows the
// global mode, "on" or "off").
func (a *App) GetGameSaveBackup(gameID string) (string, error) {
	if a.settings == nil {
		return "", apperr.New("settings.unavailable", "settings are not ready", nil)
	}
	return a.settings.GameSaveBackup(gameID), nil
}

// RefreshSavePaths refreshes the save list in the background and returns the
// game's current save locations.
func (a *App) RefreshSavePaths(game types.Game) ([]types.SavePath, error) {
	go a.saveService().Refresh(a.ctx)
	return a.GetSavePaths(game)
}

// RefreshSaveManifest refreshes the save list now (the Settings "Update save
// list" button).
func (a *App) RefreshSaveManifest() error {
	return a.saveService().Refresh(a.ctx)
}

// UninstallGameWithoutBackup queues an uninstall that skips the save backup:
// the user checked "delete without a save backup" after the gate found no
// known save paths.
func (a *App) UninstallGameWithoutBackup(game types.Game) (types.Job, error) {
	if !game.IsInstalled {
		return types.Job{}, apperr.New("game.notInstalled", fmt.Sprintf("%s is not installed", game.Title), map[string]string{"title": game.Title})
	}
	if a.isRunning(game.ID) {
		return types.Job{}, apperr.New("game.running", fmt.Sprintf("%s is running", game.Title), map[string]string{"title": game.Title})
	}
	folder, err := a.currentFolder(game)
	if err != nil {
		return types.Job{}, err
	}
	game.InstallDir = folder
	j, err := a.queueJob(jobs.KindUninstall, game, nil)
	if err != nil {
		return types.Job{}, err
	}
	if err := a.jobs.SetAllowNoSaves(j.ID); err != nil {
		return types.Job{}, err
	}
	j.AllowNoSaves = true
	return j, nil
}

// ---- core-run jobs ----

// startCoreBackup runs a backup job in its own goroutine and reports through
// the queue like a plugin job.
func (a *App) startCoreBackup(j types.Job) error {
	ctx, cancel := context.WithCancel(a.ctx)
	if err := a.backups.begin(j.ID, cancel); err != nil {
		cancel()
		return err
	}
	go func() {
		defer a.backups.end(j.ID)
		defer cancel()
		a.runCoreBackup(ctx, j)
	}()
	return nil
}

func (a *App) runCoreBackup(ctx context.Context, j types.Job) {
	game := types.Game{ID: j.GameID, Source: j.Source, StoreAppID: j.StoreGameID, Title: j.Title, InstallDir: j.InstallDir}
	if full, err := a.findGame(j.GameID); err == nil {
		if j.InstallDir != "" {
			full.InstallDir = j.InstallDir
		}
		game = full
	}
	// A manual backup waits briefly while a cloud job holds the game's
	// saves, then skips with an event when it is still busy.
	if g, ok := a.cloudGame(game); ok && a.cloud != nil {
		deadline := time.Now().Add(afterSessionWait)
		for a.cloud.Info(g).Busy != "" && time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
		if a.cloud.Info(g).Busy != "" {
			a.host.Emit("backups", savebackup.Event{Kind: savebackup.EvFailed, GameID: game.ID, Trigger: j.Destination, Message: "skipped: cloud sync is busy for this game"})
			a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: "skipped: cloud sync is busy for this game"})
			return
		}
	}
	trigger := j.Destination
	if trigger == "" {
		trigger = triggerManual
	}
	steamID, gogID := a.backupIDs(game)
	var userPaths []string
	if a.settings != nil {
		userPaths = a.settings.GameSavePaths(game.ID)
	}
	_, _, _, _, err := a.saveService().Backup(game, trigger, savebackup.BackupOptions{
		RunnerPath: a.backupRunner(game), PrefixesFolder: a.prefixesFolder(),
		SteamRoot: a.backupSteamRoot(),
		UserPaths: userPaths, SteamID: steamID, GogID: gogID,
	})
	if errors.Is(err, savebackup.ErrNothingFound) {
		err = fmt.Errorf("no save files of %s were found on disk; nothing was backed up", game.Title)
	}
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: err.Error()})
		return
	}
	a.jobs.Notify(j.Source, "job.done", plugins.JobUpdate{JobID: j.ID})
}

// startCoreRestore runs a restore job in its own goroutine.
func (a *App) startCoreRestore(j types.Job) error {
	ctx, cancel := context.WithCancel(a.ctx)
	if err := a.backups.begin(j.ID, cancel); err != nil {
		cancel()
		return err
	}
	go func() {
		defer a.backups.end(j.ID)
		defer cancel()
		a.runCoreRestore(ctx, j)
	}()
	return nil
}

func (a *App) runCoreRestore(ctx context.Context, j types.Job) {
	game := types.Game{ID: j.GameID, Source: j.Source, StoreAppID: j.StoreGameID, Title: j.Title, InstallDir: j.InstallDir}
	if full, err := a.findGame(j.GameID); err == nil {
		if j.InstallDir != "" {
			full.InstallDir = j.InstallDir
		}
		game = full
	}
	if a.isRunning(game.ID) {
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: fmt.Sprintf("%s is running", game.Title)})
		return
	}
	steamID, gogID := a.backupIDs(game)
	rctx := a.backupCtx(game, steamID, gogID)
	svc := a.saveService()
	// Every restore takes a before-restore backup first; when that fails
	// the restore is blocked and nothing is written.
	safety := func() error {
		var userPaths []string
		if a.settings != nil {
			userPaths = a.settings.GameSavePaths(game.ID)
		}
		_, _, _, _, err := svc.Backup(game, triggerBeforeRestore, savebackup.BackupOptions{
			RunnerPath: a.backupRunner(game), PrefixesFolder: a.prefixesFolder(),
			SteamRoot: a.backupSteamRoot(),
			UserPaths: userPaths, SteamID: steamID, GogID: gogID,
		})
		if errors.Is(err, savebackup.ErrNothingFound) {
			return nil // nothing on disk to keep before restoring
		}
		return err
	}
	if _, _, err := svc.Restore(game, j.Destination, rctx, safety); err != nil {
		if ctx.Err() != nil {
			return
		}
		a.jobs.Notify(j.Source, "job.failed", plugins.JobUpdate{JobID: j.ID, Error: err.Error()})
		return
	}
	a.jobs.Notify(j.Source, "job.done", plugins.JobUpdate{JobID: j.ID})
}

// backupAfterSession runs after a session for games without cloud sync, on
// by default (backup_after_session unset means on; the per-game "on" forces
// it for cloud games, "off" disables it). Cloud push runs first, backup
// second. It waits, bounded, while a cloud job for the game runs, then skips
// with an event when still busy.
func (a *App) backupAfterSession(game types.Game) {
	if !a.autoBackupState(game).On {
		return
	}
	if g, ok := a.cloudGame(game); ok && a.cloud != nil {
		deadline := time.Now().Add(afterSessionWait)
		for a.cloud.Info(g).Busy != "" && time.Now().Before(deadline) {
			select {
			case <-a.ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
		if a.cloud.Info(g).Busy != "" {
			a.host.Emit("backups", savebackup.Event{Kind: savebackup.EvFailed, GameID: game.ID, Trigger: triggerAfterSession, Message: "skipped: cloud sync is busy for this game"})
			return
		}
	}
	steamID, gogID := a.backupIDs(game)
	var userPaths []string
	if a.settings != nil {
		userPaths = a.settings.GameSavePaths(game.ID)
	}
	_, _, _, _, err := a.saveService().Backup(game, triggerAfterSession, savebackup.BackupOptions{
		RunnerPath: a.backupRunner(game), PrefixesFolder: a.prefixesFolder(),
		SteamRoot: a.backupSteamRoot(),
		UserPaths: userPaths, SteamID: steamID, GogID: gogID,
	})
	if err != nil && !errors.Is(err, savebackup.ErrNothingFound) {
		a.host.Emit("backups", savebackup.Event{Kind: savebackup.EvFailed, GameID: game.ID, Trigger: triggerAfterSession, Message: err.Error()})
	}
}

// afterSessionHook is the session onExit hook after the cloud push: it runs
// off the tracker's goroutine, so real work starts in its own goroutine.
func (a *App) afterSessionHook(game types.Game) session.StartOption {
	return session.WithOnExit(func(session.ExitInfo) {
		go a.backupAfterSession(game)
	})
}
