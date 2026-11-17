package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"platypus/internal/apperr"
	"platypus/internal/jobs"
	"platypus/internal/plugins"
	"platypus/internal/settings"
	"platypus/internal/stores"
	"platypus/internal/types"
	"slices"
	"time"
)

// jobBackend runs queue jobs through the owning plugin.
type jobBackend struct{ a *App }

func (b jobBackend) Size(ctx context.Context, j types.Job) (plugins.InstallSize, error) {
	var s plugins.InstallSize
	if j.Kind == jobs.KindWorkshop {
		// A Workshop sync's download is the subscribed items' current sizes:
		// the store's own listing is the estimate, checked before it starts.
		var items []plugins.WorkshopItem
		if err := b.a.plugins.Call(ctx, j.Source, "workshop.list", map[string]any{"id": j.StoreGameID}, &items); err != nil {
			return s, err
		}
		for _, it := range items {
			if it.SizeBytes > 0 {
				s.DiskBytes += it.SizeBytes
			}
		}
		s.DownloadBytes = s.DiskBytes
		return s, nil
	}
	options, err := b.a.optionsForJob(ctx, j)
	if err != nil {
		return s, err
	}
	params := map[string]any{"id": j.StoreGameID}
	if j.Target != nil {
		if err := b.a.jobTargetGate(j); err != nil {
			return s, err
		}
		params["target"] = j.Target // the size of that version, not the latest
	}
	if options != nil {
		params["options"] = options // the size of the chosen add-ons and packs
	}
	err = b.a.plugins.Call(ctx, j.Source, "game.installSize", params, &s)
	return s, err
}

func (b jobBackend) Start(ctx context.Context, j types.Job) error {
	switch j.Kind {
	case jobs.KindVerify:
		params := verifyParams(j)
		b.a.attachDownloadLimit(j, params) // a repair downloads the damaged files
		return b.a.plugins.Call(ctx, j.Source, "game.verify", params, nil)
	case jobs.KindMove:
		if !b.a.plugins.Has(j.Source, "move") {
			return b.a.startCoreMove(j) // runs on its own; reports through the queue
		}
		return b.a.plugins.Call(ctx, j.Source, "game.move", moveParams(j), nil)
	case jobs.KindBackup:
		return b.a.startCoreBackup(j) // runs on its own; reports through the queue
	case jobs.KindRestore:
		return b.a.startCoreRestore(j) // runs on its own; reports through the queue
	case jobs.KindUninstall:
		// The save backup runs before game.uninstall: a failed backup
		// fails the job and the plugin never deletes anything.
		game := types.Game{ID: j.GameID, Source: j.Source, StoreAppID: j.StoreGameID, Title: j.Title, InstallDir: j.InstallDir}
		if _, err := b.a.backupBeforeDelete(ctx, game, triggerUninstall, j.AllowNoSaves); err != nil {
			return err
		}
		return b.a.startInstallLike(ctx, j, installParams(j, nil))
	case jobs.KindRemove:
		return b.a.startCoreRemove(j) // core deletes; reports through the queue
	case jobs.KindRunnerInstall:
		return b.a.startCoreRunnerInstall(j) // core downloads; reports through the queue
	case jobs.KindClientSetup:
		return b.a.startClientSetup(j) // core runs the installer's spec; reports through the queue
	case jobs.KindWorkshop:
		return b.a.startWorkshopSync(ctx, j) // the plugin syncs; reports through the queue
	}
	if j.Target != nil {
		if err := b.a.jobTargetGate(j); err != nil {
			return err
		}
	}
	options, err := b.a.optionsForJob(ctx, j)
	if err != nil {
		return err
	}
	params := installParams(j, options)
	b.a.attachDownloadLimit(j, params)
	return b.a.startInstallLike(ctx, j, params)
}

// startInstallLike calls game.install/game.update/game.uninstall and either
// leaves the job to the plugin (a {} answer) or runs the answered clientRun
// command in the pinned client prefix as part of the same job (protocol
// 1.23, issue #212). A repeated jobId starts no second copy.
func (a *App) startInstallLike(ctx context.Context, j types.Job, params map[string]any) error {
	if a.clientRuns.active(j.ID) {
		return nil
	}
	var answer plugins.InstallAnswer
	if err := a.plugins.Call(ctx, j.Source, "game."+j.Kind, params, &answer); err != nil {
		return err
	}
	if answer.ClientRun == nil {
		return nil // the plugin does the work; it ends the job itself
	}
	if !a.plugins.Runtime(j.Source).Speaks(plugins.ClientRunVersion) {
		return fmt.Errorf("%s speaks an older protocol and can't answer clientRun (needs 1.23)", j.Source)
	}
	if answer.Size < 0 {
		return fmt.Errorf("%s: clientRun size %d must not be negative", j.Source, answer.Size)
	}
	p, ok := a.plugins.Plugin(j.Source)
	if !ok {
		return fmt.Errorf("unknown plugin %q", j.Source)
	}
	clientDir := stores.ClientPrefixDir(j.Source, a.effectivePrefixesFolder())
	folders := append(a.plugins.Folders(j.Source, a.settings.GamesFolder()), clientDir)
	if !answer.ClientOwnsLocation && j.InstallDir != "" {
		folders = append(folders, j.InstallDir)
	}
	if answer.ClientOwnsLocation {
		// The client owns installs: drop the job's install folder when
		// the queue created it and nothing landed in it.
		a.dropEmptyInstallDir(j)
	}
	run, err := stores.ValidateClientRun(*answer.ClientRun, p.Manifest, folders, clientDir)
	if err != nil {
		return err
	}
	return a.startClientRun(j, answer, run)
}

// dropEmptyInstallDir removes a fresh install's folder when the plugin's
// clientOwnsLocation answer says the client owns installs (protocol 1.23):
// the queue already created it, and the client installs elsewhere. A
// non-empty folder, or one that predates the job, is left alone; os.Remove
// only removes empty dirs anyway.
func (a *App) dropEmptyInstallDir(j types.Job) {
	if j.Kind != jobs.KindInstall || j.InstallDir == "" {
		return
	}
	st, err := os.Stat(j.InstallDir)
	if err != nil || !st.IsDir() {
		return
	}
	if st.ModTime().Before(time.UnixMilli(j.CreatedAt)) {
		return
	}
	entries, err := os.ReadDir(j.InstallDir)
	if err != nil || len(entries) > 0 {
		return
	}
	_ = os.Remove(j.InstallDir)
}

// installing the latest while the UI says the game is pinned.
func (a *App) jobTargetGate(j types.Job) error {
	game := types.Game{ID: j.GameID, Source: j.Source, StoreAppID: j.StoreGameID, Title: j.Title}
	if _, err := a.versionsGate(game); err != nil {
		return apperr.New("jobs.pinnedStale", fmt.Sprintf("%s was queued for a chosen version, which the store can't install now (%v); cancel the job or unpin the game to install the latest", j.Title, err), map[string]string{"title": j.Title})
	}
	return nil
}

// installParams builds the params for game.install/game.update/game.uninstall
// from a job. Plugins may key their own records on `title` (GOG keeps it in
// its installed index, steam-standalone names its dl request), so Platypus sends
// the title the job already has. Branches and their passwords are plugin-side
// choices: Platypus sends no branch key at all — an empty one must not leak into
// the params. installDir only makes sense where files land. options are the
// install-option choices as optionsForJob checked them (nil sends none).
func installParams(j types.Job, options types.InstallOptions) map[string]any {
	params := map[string]any{"id": j.StoreGameID, "jobId": j.ID}
	if j.Kind == jobs.KindUninstall {
		return params
	}
	params["installDir"] = j.InstallDir
	if j.Title != "" {
		params["title"] = j.Title
	}
	// branch/branchPassword would go here per-plugin; none is set, so the
	// params carry none: a plugin defaults to its own branch. A version target
	// (the `versions` capability) names a channel or build, never a password.
	if j.Target != nil {
		params["target"] = j.Target
	}
	if options != nil {
		params["options"] = options
	}
	return params
}

// Cancel stops a job. A core move stops and puts the game back; it is waited
// for, so the folders are free when Cancel returns. A core backup or restore
// stops the same way.
func (b jobBackend) Cancel(ctx context.Context, j types.Job) error {
	if j.Kind == jobs.KindMove && !b.a.plugins.Has(j.Source, "move") {
		b.a.moves.stop(ctx, j.ID)
		return nil
	}
	if j.Kind == jobs.KindBackup || j.Kind == jobs.KindRestore {
		b.a.backups.stop(ctx, j.ID)
		return nil
	}
	if j.Kind == jobs.KindRemove {
		b.a.removes.stop(ctx, j.ID)
		return nil
	}
	if j.Kind == jobs.KindRunnerInstall {
		b.a.runnerInstalls.stop(ctx, j.ID)
		return nil
	}
	if j.Kind == jobs.KindClientSetup {
		// Core runs the installer itself: canceling kills it and stops the
		// prefix's wineserver, and the run's unwind does both.
		b.a.clientSetups.stop(ctx, j.ID)
		return nil
	}
	if b.a.clientRuns.active(j.ID) {
		// Core runs the client command itself: stop it and its wineserver,
		// then forward the cancel so the plugin ends the job too.
		b.a.clientRuns.stop(ctx, j.ID)
	}
	return b.a.plugins.Call(ctx, j.Source, "job.cancel", map[string]any{"jobId": j.ID}, nil)
}

// RunsInCore says whether core is running the job itself, in a goroutine of
// its own (see CoreRunner in the queue): core's own moves and removals, its
func (b jobBackend) RunsInCore(j types.Job) bool {
	if j.Kind == jobs.KindBackup || j.Kind == jobs.KindRestore {
		return true
	}
	return b.a.moves.active(j.ID) || b.a.removes.active(j.ID) || b.a.runnerInstalls.active(j.ID) || b.a.clientSetups.active(j.ID) || b.a.clientRuns.active(j.ID)
}

// heldByJob refuses a launch while a move, a check, a backup, a restore or a
// removal holds the game: the game would start from files that are being
// moved, checked, rewritten or deleted. A game its store client launches
// also waits for the client's own setup (protocol 1.20); wine-client
// launches without that flag are checked again after the launch spec is
// known (see launchGame).
func (a *App) heldByJob(game types.Game) error {
	if game.ClientLaunch {
		if _, ok := a.jobs.Holding(stores.ClientGameID(game.Source), jobs.KindClientSetup); ok {
			return apperr.New("game.launchBusy", fmt.Sprintf("%s is being set up; launch it once that finishes", game.Title), map[string]string{"title": game.Title, "what": "set up"})
		}
		if kind := a.clientRunKind(game.Source); kind != "" {
			enVerb, frVerb := clientRunVerb(kind)
			p, ok := a.plugins.Plugin(game.Source)
			client := game.Source
			if ok {
				client = p.Manifest.Name
			}
			return apperr.New("game.clientRunBusy",
				fmt.Sprintf("%s is %s in %s; launch it once that finishes", game.Title, enVerb, client),
				map[string]string{"title": game.Title, "client": client, "action": enVerb, "actionFr": frVerb})
		}
	}
	j, ok := a.jobs.Holding(game.ID, jobs.KindMove, jobs.KindVerify, jobs.KindBackup, jobs.KindRestore, jobs.KindRemove, jobs.KindWorkshop)
	if !ok {
		return nil
	}
	what := "checked"
	if j.Kind == jobs.KindWorkshop {
		return apperr.New("game.launchBusyWorkshop", fmt.Sprintf("%s is syncing Workshop items; launch it once that finishes", game.Title), map[string]string{"title": game.Title})
	}
	switch j.Kind {
	case jobs.KindMove:
		what = "moved"
	case jobs.KindBackup:
		what = "backed up"
	case jobs.KindRestore:
		what = "restored"
	case jobs.KindRemove:
		what = "removed"
	}
	return apperr.New("game.launchBusy", fmt.Sprintf("%s is being %s; launch it once that finishes", game.Title, what), map[string]string{"title": game.Title, "what": what})
}

// jobFinished drops the finished job's source from the library cache and keeps
// the relocation record true to the folders. The queue runs it before the job
// reads as done, so the rescan the jobs event triggers re-lists the source
// instead of serving what it listed before the install. A finished uninstall
// leaves nothing at the moved folder for the record to name, and a finished
// install puts the game where the plugin says it belongs: either way an old
// record must not send the library and launches to the stale copy. A move
// keeps its record: the plugin still reports the old folder, which maps to
// the new one.
func (a *App) jobFinished(j types.Job) {
	a.recordJobActivity(j)
	// A verify only reports: it changes no bytes and lists nothing new, so
	// the library and disk caches stay (issue #145 records it, nothing more).
	if j.Kind == jobs.KindVerify {
		return
	}
	// A Workshop sync writes mod content beside the game, not the game or its
	// library listing: only its size on disk changed.
	if j.Kind == jobs.KindWorkshop {
		a.invalidateDiskGame(j.GameID)
		return
	}
	a.invalidateLibrary(j.Source)
	a.invalidateDiskGame(j.GameID) // the job moved its game's bytes: installs, updates, uninstalls, moves, removals
	if j.Kind != jobs.KindInstall && j.Kind != jobs.KindUninstall {
		return
	}
	if _, ok := a.settings.Relocation(j.GameID); !ok {
		return
	}
	if err := a.settings.ClearRelocation(j.GameID); err != nil {
		log.Printf("moves: clearing the record for %s: %v", j.Title, err)
	}
}

// queueJob checks the game's plugin can run the job and queues it, for the
// version in target (nil = the plugin's own default). Platypus picks the install
// folder; plugins never choose paths.
func (a *App) queueJob(kind string, game types.Game, target *types.VersionTarget) (types.Job, error) {
	return a.queueJobWith(kind, game, target, nil)
}

// queueJobWith is queueJob for an install or update that also carries the
// install-option choices (nil means the store's defaults).
func (a *App) queueJobWith(kind string, game types.Game, target *types.VersionTarget, options types.InstallOptions) (types.Job, error) {
	return a.queueJobIn(kind, game, target, options, "")
}

// queueJobIn is queueJobWith for a fresh install into a folder the user picked
// ("" is the plugin's own install folder). The folder is checked first and,
// once the job is queued, remembered and allowed for the plugin's launches.
// A job the user asked for replaces a queued (not running) automatic update of
// the game (issue #209), which never keeps the user waiting.
func (a *App) queueJobIn(kind string, game types.Game, target *types.VersionTarget, options types.InstallOptions, picked string) (types.Job, error) {
	a.dropAutoForApply(game.ID)
	return a.queueJobInAuto(kind, game, target, options, picked, false)
}

// queueJobInAuto is queueJobIn for a job Platypus starts by itself (an
// automatic update, issue #209): auto marks the job, so the queue holds it to
// the update window.
func (a *App) queueJobInAuto(kind string, game types.Game, target *types.VersionTarget, options types.InstallOptions, picked string, auto bool) (types.Job, error) {
	p, ok := a.plugins.Plugin(game.Source)
	if !ok {
		return types.Job{}, apperr.New("game.notFromStore", fmt.Sprintf("%s doesn't come from a store plugin", game.Title), map[string]string{"title": game.Title})
	}
	if p.Status != plugins.StatusReady {
		return types.Job{}, apperr.New("plugin.notReady", fmt.Sprintf("%s is %s", p.Manifest.Name, p.Status), map[string]string{"name": p.Manifest.Name, "status": string(p.Status)})
	}
	if !a.plugins.Has(game.Source, "install") {
		return types.Job{}, apperr.New("install.notSupported", fmt.Sprintf("%s can't install games", p.Manifest.Name), map[string]string{"name": p.Manifest.Name})
	}
	if termsKind(kind) {
		if err := a.termsGate(game); err != nil {
			return types.Job{}, err
		}
	}
	folder, _ := a.plugins.InstallFolder(game.Source, a.settings.GamesFolder())
	chosen, custom, err := a.installFolderFor(game, picked)
	if err != nil {
		return types.Job{}, err
	}
	if chosen != "" {
		folder = chosen
	}
	if game.IsInstalled && game.InstallDir != "" {
		folder = game.InstallDir // an installed game updates where it is, moved or not
	}
	var job types.Job
	if auto {
		job, err = a.jobs.AddAuto(game.Source, game.ID, game.StoreAppID, game.OriginalTitle(), folder, target, options)
	} else {
		job, err = a.jobs.AddWithOptions(kind, game.Source, game.ID, game.StoreAppID, game.OriginalTitle(), folder, target, options)
	}
	if err == nil && custom {
		a.rememberInstallFolder(game.Source, folder)
	}
	return job, err
}

// InstallGame queues an install into the plugin's install folder. A game held
// at a version (pinned) installs that version.
func (a *App) InstallGame(game types.Game) (types.Job, error) {
	if game.IsInstalled {
		return types.Job{}, apperr.New("install.alreadyInstalled", fmt.Sprintf("%s is already installed", game.Title), map[string]string{"title": game.Title})
	}
	if !game.Installable {
		return types.Job{}, apperr.New("install.cannotInstall", fmt.Sprintf("%s can't be installed by Platypus", game.Title), map[string]string{"title": game.Title})
	}
	target, err := a.pinnedTarget(game)
	if err != nil {
		return types.Job{}, err
	}
	return a.queueJobWith(jobs.KindInstall, game, target, a.savedOptions(game))
}

// UpdateGame queues an update of an installed plugin game. A pinned game is
// updated to its pin, not to the latest version.
func (a *App) UpdateGame(game types.Game) (types.Job, error) {
	if !game.IsInstalled {
		return types.Job{}, apperr.New("game.notInstalled", fmt.Sprintf("%s is not installed", game.Title), map[string]string{"title": game.Title})
	}
	target, err := a.pinnedTarget(game)
	if err != nil {
		return types.Job{}, err
	}
	return a.queueJobWith(jobs.KindUpdate, game, target, a.savedOptions(game))
}

// UninstallGame queues removal of an installed plugin game.
func (a *App) UninstallGame(game types.Game) (types.Job, error) {
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
	game.InstallDir = folder // the library's folder, as Verify and Move use
	return a.queueJob(jobs.KindUninstall, game, nil)
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

// ResumeJob continues a paused or canceled download where it stopped. A
// canceled install or update meets the terms gate first (see requeueGate);
// a paused one keeps going: it was gated when it was queued.
func (a *App) ResumeJob(id string) error {
	if err := a.requeueGate(id, jobs.StateCanceled); err != nil {
		return err
	}
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

// attachDownloadLimit adds the global speed cap to the jobs that download
// (install, update, and a verify that repairs), for a plugin that speaks
// protocol 1.21 (the revision that added the param). The plugin passes it to
// its own downloader; a plugin that doesn't speak it gets nothing and
// downloads at its own pace.
func (a *App) attachDownloadLimit(j types.Job, params map[string]any) {
	switch {
	case j.Kind == jobs.KindInstall, j.Kind == jobs.KindUpdate:
	case j.Kind == jobs.KindVerify && j.Repair:
	default:
		return
	}
	limit := a.settings.DownloadLimit()
	if limit <= 0 || !a.plugins.OffersProtocol(j.Source, plugins.DownloadLimitVersion) {
		return
	}
	params["downloadLimit"] = limit
}

// SetDownloadLimit saves the global download speed cap in bytes per second
// (0 = unlimited). It applies to the next install or update each downloader
// starts; a running one keeps its pace.
func (a *App) SetDownloadLimit(bytesPerSec int64) (types.LauncherSettings, error) {
	if err := a.settings.SetDownloadLimit(bytesPerSec); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}

// SetUpdateWindow saves the daily window when queued updates may start. The
// queue reads it from now on: an update queued outside the window waits,
// installs never do.
func (a *App) SetUpdateWindow(enabled bool, start, end string) (types.LauncherSettings, error) {
	w := settings.UpdateWindow{Enabled: enabled, Start: start, End: end}
	if err := a.settings.SetUpdateWindow(w); err != nil {
		return a.launcherSettings(), err
	}
	a.jobs.SetUpdateWindow(w)
	return a.launcherSettings(), nil
}

// autoUpdateEvery is how often the automatic-update check re-lists the
// library (issue #209), besides every library refresh.
const autoUpdateEvery = 2 * time.Hour

// autoRetryAfter is how long a failed automatic update waits before the game
// is tried again (issue #209): a store outage must not retry every check.
const autoRetryAfter = 6 * time.Hour

// The outcomes of an automatic update attempt (autoTry.outcome). "" means it
// was queued and hasn't ended yet.
const (
	autoCanceled = "canceled"
	autoFailed   = "failed"
)

// autoTry is one game's last automatic update attempt (issue #209), kept in
// memory: the installed version the store reported then, when it was tried,
// and how it ended. A canceled attempt isn't retried while the same update is
// offered; a failed one waits out autoRetryAfter.
type autoTry struct {
	version string
	at      time.Time
	outcome string
}

// SetAutoUpdate saves the "Automatic updates" switch (issue #209). Turning it
// on checks the library right away, so a game already waiting for an update
// is queued without another manual refresh; turning it off cancels the
// automatic updates that haven't started yet (a running one may finish).
func (a *App) SetAutoUpdate(on bool) (types.LauncherSettings, error) {
	if err := a.settings.SetAutoUpdate(on); err != nil {
		return a.launcherSettings(), err
	}
	if on {
		a.autoChecks.Add(1)
		go func() {
			defer a.autoChecks.Done()
			a.checkAutoUpdates()
		}()
	} else {
		a.cancelQueuedAuto("")
	}
	return a.launcherSettings(), nil
}

// GetGameAutoUpdate reports whether one game may be updated automatically:
// false only when the user excluded it.
func (a *App) GetGameAutoUpdate(gameID string) (bool, error) {
	return a.settings.GameAutoUpdate(gameID), nil
}

// SetGameAutoUpdate saves one game's "Don't update automatically" override
// (on returns the game to following the global switch). Excluding a game
// cancels its automatic update that hasn't started yet.
func (a *App) SetGameAutoUpdate(gameID string, on bool) error {
	if err := a.settings.SetGameAutoUpdate(gameID, on); err != nil {
		return err
	}
	if !on {
		a.cancelQueuedAuto(gameID)
	}
	return nil
}

// autoUpdateCheck queues an automatic update for each installed game whose
// store reports one (issue #209). A game is skipped when it is pinned, when
// the user excluded it, when it can't be installed by its store, and while it
// runs: its update waits for the next check instead. A game that already has
// a job is skipped too (the queue would refuse a second one), and so is one
// whose canceled or failed attempt still stands.
func (a *App) autoUpdateCheck(lib types.Library) {
	if a.settings == nil || !a.settings.AutoUpdate() {
		return
	}
	a.autoUpdateMu.Lock()
	defer a.autoUpdateMu.Unlock()
	pins := a.settings.Pins()
	now := time.Now()
	for _, g := range lib.Games {
		if !g.IsInstalled || !g.UpdateAvailable || g.Pinned != nil || !g.Installable {
			continue
		}
		if _, pinned := pins[g.ID]; pinned {
			continue
		}
		if !a.settings.GameAutoUpdate(g.ID) || a.isRunning(g.ID) {
			continue
		}
		if wait, ok := a.autoTry(g.ID); ok {
			switch wait.outcome {
			case autoCanceled:
				// The user canceled this update; wait for the store to offer
				// a different one (a new installed version, or the update
				// itself going away).
				if wait.version == "" || wait.version == g.Version {
					continue
				}
			case autoFailed:
				if now.Sub(wait.at) < autoRetryAfter {
					continue
				}
			}
		}
		// The saved choices go along unchecked: the job checks them against
		// the store when it starts, so this read makes no plugin call (S4).
		options, _ := a.settings.Options(g.ID)
		if _, err := a.queueJobInAuto(jobs.KindUpdate, g, nil, options, "", true); err != nil {
			continue // a job already holds the game, or the store can't update it now
		}
		a.setAutoTry(g.ID, autoTry{version: g.Version, at: now})
	}
}

// autoEnded remembers how an automatic update ended (issue #209): a canceled
// one isn't tried again while the store offers the same update, a failed one
// waits autoRetryAfter, and a finished one clears the record.
func (a *App) autoEnded(j types.Job) {
	if j.Kind != jobs.KindUpdate || !j.Auto {
		return
	}
	now := time.Now()
	a.autoMu.Lock()
	defer a.autoMu.Unlock()
	if j.State != jobs.StateDone && a.autoTries == nil {
		a.autoTries = map[string]autoTry{}
	}
	try := a.autoTries[j.GameID]
	switch j.State {
	case jobs.StateDone:
		delete(a.autoTries, j.GameID)
	case jobs.StateCanceled:
		try.outcome, try.at = autoCanceled, now
		a.autoTries[j.GameID] = try
	case jobs.StateFailed:
		try.outcome, try.at = autoFailed, now
		a.autoTries[j.GameID] = try
	}
}

func (a *App) autoTry(gameID string) (autoTry, bool) {
	a.autoMu.Lock()
	defer a.autoMu.Unlock()
	try, ok := a.autoTries[gameID]
	return try, ok
}

func (a *App) setAutoTry(gameID string, try autoTry) {
	a.autoMu.Lock()
	defer a.autoMu.Unlock()
	if a.autoTries == nil {
		a.autoTries = map[string]autoTry{}
	}
	a.autoTries[gameID] = try
}

func (a *App) forgetAutoTry(gameID string) {
	a.autoMu.Lock()
	defer a.autoMu.Unlock()
	delete(a.autoTries, gameID)
}

// cancelQueuedAuto cancels automatic updates that haven't started, for one
// game or every game (""): switching automatic updates off stops what is
// still waiting (issue #209). A running one may finish.
func (a *App) cancelQueuedAuto(gameID string) {
	for _, j := range a.jobs.List() {
		if j.Auto && j.Kind == jobs.KindUpdate && j.State == jobs.StateQueued && (gameID == "" || j.GameID == gameID) {
			if err := a.jobs.Cancel(a.ctx, j.ID); err != nil {
				a.warnf("automatic updates: canceling %s: %v", j.Title, err)
			}
		}
	}
}

// dropAutoForApply cancels a queued automatic update of the game before an
// action the user asked for takes it (issue #209): a waiting auto update never
// blocks verify, move, remove or the user's own install or update. Its
// canceled attempt is forgotten: the user didn't cancel the update itself.
func (a *App) dropAutoForApply(gameID string) {
	a.cancelQueuedAuto(gameID)
	a.forgetAutoTry(gameID)
}

// pruneAutoUpdateOff forgets the automatic-update exclusions of games that
// left the library (issue #209), like PruneGames does for plugin settings.
func (a *App) pruneAutoUpdateOff(lib types.Library) {
	if a.settings == nil {
		return
	}
	keep := make(map[string]bool, len(lib.Games))
	for _, g := range lib.Games {
		keep[g.ID] = true
	}
	if err := a.settings.PruneAutoUpdateOff(keep); err != nil {
		a.warnf("automatic updates: forgetting removed games: %v", err)
	}
}

// scheduleAutoUpdateCheck runs the check after a library read returned: the
// queueing it does (terms, install options) never holds the scan. The check
// is tracked so shutdown waits for it.
func (a *App) scheduleAutoUpdateCheck(lib types.Library) {
	if a.settings == nil || !a.settings.AutoUpdate() {
		return
	}
	a.autoChecks.Add(1)
	go func() {
		defer a.autoChecks.Done()
		a.autoUpdateCheck(lib)
	}()
}

// checkAutoUpdates lists the library for the periodic check; a library
// refresh schedules the check itself.
func (a *App) checkAutoUpdates() {
	if a == nil || a.settings == nil || a.ctx == nil || !a.settings.AutoUpdate() {
		return
	}
	// The library carries a warning for a source that is logged out or
	// unreachable, so this adds no log line of its own (S4).
	_, _ = a.GetLibrary()
}

// autoUpdateLoop re-checks the library every autoUpdateEvery (issue #209).
// Every library refresh checks too; this only makes sure one happens when no
// window is opened and no rescan is asked for.
func (a *App) autoUpdateLoop(ctx context.Context) {
	t := time.NewTicker(autoUpdateEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.checkAutoUpdates()
		}
	}
}

// SetCancelCleanupDays sets how many days a canceled download's data is kept
// before Platypus deletes it (0 = right away). It applies to downloads that are
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
	// A job saved by an older build may predate its terms acceptance: a
	// failed or canceled install or update meets the terms gate before it
	// is re-queued, the same gate a new install would meet. Anything else
	// (an uninstall, a verify, a move, a job that is gone, a retry that
	// would be a no-op) retries as before.
	if err := a.requeueGate(id, jobs.StateFailed, jobs.StateCanceled); err != nil {
		return err
	}
	return a.jobs.Retry(id)
}

// requeueGate refuses re-queuing an install or update whose terms aren't
// accepted, when the job is in one of the given states (a retry that would
// be a no-op, and a resume that only continues paused data, are not gated).
// A job whose game can't be resolved anymore skips the gate: it fails when
// it starts either way.
func (a *App) requeueGate(id string, gated ...string) error {
	for _, j := range a.jobs.List() {
		if j.ID != id || !termsKind(j.Kind) || !slices.Contains(gated, j.State) {
			continue
		}
		game, err := a.findGame(j.GameID)
		if err != nil {
			return nil
		}
		return a.termsGate(game)
	}
	return nil
}

// MoveJobToFront moves a queued or paused download to the top of the queue,
// so it starts next. The running download keeps running. Moving a running
// or finished job is a no-op returning nil.
func (a *App) MoveJobToFront(id string) error {
	return a.jobs.MoveToFront(id)
}

func (a *App) ClearFinishedJobs() {
	a.jobs.ClearFinished()
}
