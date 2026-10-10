package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"platypus/internal/chat"
	"platypus/internal/cloudsync"
	"platypus/internal/compat"
	"platypus/internal/dataplug"
	"platypus/internal/jobs"
	"platypus/internal/ledger"
	"platypus/internal/migrate"
	"platypus/internal/plugins"
	"platypus/internal/runner"
	"platypus/internal/session"
	"platypus/internal/settings"
	"platypus/internal/social"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// How long a store plugin gets for library.list (legendary refreshes its
// catalogue over the network) and for a launch spec.
const (
	libraryTimeout = 2 * time.Minute
	launchTimeout  = time.Minute
)

// App struct
type App struct {
	// host is how the App reaches the shell it runs in: events, log, dialogs (host.go).
	host     Host
	ctx      context.Context
	stop     context.CancelFunc
	plugins  *plugins.Manager
	settings *settings.Store
	jobs     *jobs.Queue
	// compat is the ProtonDB, anti-cheat and install-marker metadata (app_compat.go).
	compat      *compat.Service
	compatHints compatHintCache
	stores      *stores.Registry
	sessions    *session.Tracker
	cloud       *cloudsync.Orchestrator
	social      *social.Manager // friends lists of plugins with the social capability
	chat        *chat.Manager   // conversations with friends (the chat capability, protocol 1.9)
	socialMu    sync.Mutex
	socialGames map[string]socialGame // launched store games by Platypus game id, for "playing"
	// familyEmit delivers family.warning notifications to the UI (the
	// "family" event). Nil until the window exists; tests replace it.
	familyEmit func(types.FamilyWarning)
	// libraryEmit delivers a library scan's partial results to the UI (the
	// "library-partial" event). Nil until the window exists.
	libraryEmit func(types.Library)
	// settingsLocks orders the settings saves and starts of each plugin.
	settingsLocks settingsLocks
	moves         coreMoves
	custom        customStore // the user's titles and art (app_customize.go)
	ledger        *ledger.Ledger
	// sessionRunning says whether a game has a session open. Nil asks the
	// session tracker; tests replace it.
	sessionRunning func(gameID string) bool
	// data holds the built-in fixes and tweaks plus every valid data plugin's.
	data *dataplug.Registry

	tweaksMu     sync.Mutex
	tweaksCached bool
	tweaks       []types.Tweak
	// stats caches the store playtime and the achievement progress (protocol 1.7).
	stats statsCache
	// sizes caches what the store says an install needs (app_installsize.go).
	sizes sizeCache

	gameMu    sync.Mutex
	gameLocks map[string]*sync.Mutex // by game id; see lockGame
}

// NewApp creates a new App application struct
func NewApp() *App {
	a := &App{data: dataplug.NewRegistry(), host: stderrHost{}}
	a.stores = stores.NewRegistry(a.pluginStores, stores.NewSteam())
	return a
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.stats.changed = func() { a.host.Emit("playtime") }
	a.stats.info = func(format string, args ...any) { a.host.Infof(format, args...) }
	a.stats.warn = func(format string, args ...any) { a.host.Warnf(format, args...) }
	a.sessions = session.NewTracker(func(e types.SessionEvent) {
		a.host.Emit("session", e)
		a.socialSession(e)
		if e.State == "stopped" || e.State == "failed" {
			a.schedulePlaytimeRefresh(e.GameID)
		}
	})

	// Before anything opens a data file: the previous data folder moves to
	// its new name, and the paths stored in it follow (internal/migrate).
	if home, err := os.UserHomeDir(); err == nil {
		migrate.Run(home, migrate.Options{
			Info: func(format string, args ...any) { a.host.Infof(format, args...) },
			Warn: func(format string, args ...any) { a.host.Warnf(format, args...) },
		})
	}
	dataDir, err := settings.DataDir()
	if err != nil {
		a.host.Errorf("settings: %v", err)
	}
	a.settings, err = settings.Open(filepath.Join(dataDir, "settings.json"))
	if err != nil {
		// Open still returns usable defaults.
		a.host.Errorf("settings: %v", err)
	}

	a.compat, err = newCompatService(dataDir, a.settings)
	if err != nil {
		a.host.Errorf("compat: %v", err)
	}
	if a.compat != nil {
		a.compat.Warn = func(format string, args ...any) { a.host.Warnf("compat: "+format, args...) }
	}

	if err := a.openCustomization(dataDir); err != nil {
		a.host.Errorf("customization: %v", err)
	}

	a.jobs, err = jobs.Open(filepath.Join(dataDir, "jobs.json"), jobBackend{a}, func(list []types.Job) {
		a.host.Emit("jobs", list)
	})
	if err != nil {
		a.host.Errorf("jobs: %v", err)
	}
	a.jobs.SetCleanupAfter(cleanupAfter(a.settings.CancelCleanupDays()))

	a.ledger, err = ledger.Open(filepath.Join(dataDir, "install-history.json"))
	if err != nil {
		a.host.Errorf("install history: %v", err)
	}
	a.jobs.SetOnInstalled(a.recordInstall)
	a.jobs.SetOnFinished(a.jobFinished)

	// Store plugins start on demand: when the library is listed, on login,
	// or when an interrupted install resumes.
	a.plugins = plugins.NewManager(plugins.Options{
		Dir:      filepath.Join(dataDir, "plugins"),
		LogDir:   filepath.Join(dataDir, "logs"),
		DataDir:  filepath.Join(dataDir, "plugin-data"),
		Settings: a.settings,
		OnNotify: a.pluginNotify,
		OnExit: func(id string, err error) {
			a.host.Warnf("plugin %s exited: %v", id, err)
			a.jobs.PluginExited(id, err)
			if a.social != nil {
				a.social.Exited(id)
			}
		},
		OnReady: a.pluginReady,
	})
	a.familyEmit = func(w types.FamilyWarning) { a.host.Emit("family", w) }
	a.libraryEmit = func(l types.Library) { a.host.Emit("library-partial", l) }
	found, err := a.plugins.Discover()
	if err != nil {
		a.host.Errorf("plugins: %v", err)
	}
	if err == nil {
		a.forgetSettingsOfRemoved(found)
	}
	for _, p := range found {
		msg := fmt.Sprintf("plugin %s: %s", p.Manifest.ID, p.Status)
		if p.Problem != "" {
			msg += " (" + p.Problem + ")"
		}
		a.host.Infof("%s", msg)
	}
	// Data plugins feed the fixes/tweaks registry; rebuilt here and on
	// every rescan so installs and removals apply without a restart.
	a.syncDataPlugins(found)
	a.startCloud(ctx, dataDir)
	a.startSocial(ctx)
	a.startChat(ctx)
	a.syncSocial()

	runCtx, stop := context.WithCancel(ctx)
	a.stop = stop
	go a.jobs.Run(runCtx)
}

// shutdown stops games Platypus started directly and every store plugin.
// Steam-launched games belong to the Steam client and keep running. Running
// installs stay "running" in jobs.json and resume on the next start.
func (a *App) shutdown(ctx context.Context) {
	if a.stop != nil {
		a.stop()
	}
	// Syncs get a few seconds to finish; a push cut short stays owed and runs
	// at the next start.
	if a.cloud != nil {
		a.cloud.Shutdown(cloudShutdownGrace)
	}
	if a.sessions != nil {
		a.sessions.StopAll()
	}
	if a.social != nil {
		a.social.StopAll()
	}
	if a.plugins != nil {
		a.plugins.Shutdown()
	}
}

// pluginNotify routes plugin notifications: job updates to the queue.
func (a *App) pluginNotify(id, method string, params json.RawMessage) {
	if method == "cloud.progress" && a.cloud != nil {
		a.cloud.Progress(id, params)
		return
	}
	if a.social != nil && strings.HasPrefix(method, "social.") {
		a.social.Notify(id, method, params)
		return
	}
	if a.chatNotify(id, method, params) {
		return
	}
	if method == "family.warning" {
		a.familyWarning(id, params)
		return
	}
	if !strings.HasPrefix(method, "job.") {
		return
	}
	var u plugins.JobUpdate
	if err := json.Unmarshal(params, &u); err != nil {
		return
	}
	a.jobs.Notify(id, method, u)
}

// pluginStores are the approved, enabled plugins that provide a library.
func (a *App) pluginStores() []stores.Store {
	if a.plugins == nil {
		return nil
	}
	var out []stores.Store
	for _, p := range a.plugins.Plugins() {
		if p.Status != plugins.StatusReady || !a.plugins.Has(p.Manifest.ID, "library") {
			continue
		}
		out = append(out, a.pluginStore(p))
	}
	return out
}

// pluginStore is the library of one store plugin, with core's moves applied.
func (a *App) pluginStore(p plugins.Plugin) *stores.Plugin {
	id := p.Manifest.ID
	return stores.NewPlugin(a.plugins, p, func() []string {
		return a.plugins.Folders(id, a.settings.GamesFolder())
	}).WithRelocations(a.settings.Relocation)
}

// isRunning says whether the game has a session open.
func (a *App) isRunning(gameID string) bool {
	if a.sessionRunning != nil {
		return a.sessionRunning(gameID)
	}
	return a.sessions.Running(gameID)
}

// GetProtonVersions returns all discovered Proton, GE-Proton, Wine runners on the system.
func (a *App) GetProtonVersions() []types.ProtonVersion {
	return runner.ScanProtonVersions()
}

// gameSettingsFields are the settings keys types.GameSettings models.
// Anything else in a saved settings map rides along in Game.SettingsExtra.
var gameSettingsFields = func() map[string]struct{} {
	out := map[string]struct{}{}
	t := reflect.TypeOf(types.GameSettings{})
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if name := strings.Split(tag, ",")[0]; name != "" && name != "-" {
			out[name] = struct{}{}
		}
	}
	return out
}()

// customGames returns the games the user added by hand, saved in settings.json.
func (a *App) customGames() []types.Game {
	out := []types.Game{}
	for _, g := range a.settings.CustomGames() {
		g := types.Game{
			ID:          g.ID,
			Source:      types.SourceCustom,
			Store:       types.StoreCustom,
			StoreAppID:  types.SourceCustom,
			Title:       g.Title,
			InstallDir:  filepath.Dir(g.Executable),
			Executable:  g.Executable,
			IsInstalled: true,
			Settings:    types.GameSettings{UseGameMode: true},
		}
		// Saved per-game settings win over the defaults. Keys the backend
		// doesn't model (frontend-only controls like sync_mode) are kept
		// opaquely so sideloaded games don't lose them (CORE-9).
		if saved, ok := a.settings.GameSettings()[g.ID]; ok {
			if data, err := json.Marshal(saved); err == nil {
				_ = json.Unmarshal(data, &g.Settings)
			}
			for k, v := range saved {
				if _, known := gameSettingsFields[k]; !known {
					if g.SettingsExtra == nil {
						g.SettingsExtra = map[string]any{}
					}
					g.SettingsExtra[k] = v
				}
			}
		}
		out = append(out, g)
	}
	return out
}

// AddCustomGame saves a game the user added by hand: title plus the path of
// its executable. It shows up in the library from then on.
func (a *App) AddCustomGame(title, executable string) (types.Game, error) {
	g, err := a.settings.AddCustomGame(title, executable)
	if err != nil {
		return types.Game{}, err
	}
	return types.Game{
		ID:          g.ID,
		Source:      types.SourceCustom,
		Store:       types.StoreCustom,
		StoreAppID:  types.SourceCustom,
		Title:       g.Title,
		InstallDir:  filepath.Dir(g.Executable),
		Executable:  g.Executable,
		IsInstalled: true,
		Settings:    types.GameSettings{UseGameMode: true},
	}, nil
}

// RemoveCustomGame forgets a hand-added game; its files stay on disk.
func (a *App) RemoveCustomGame(id string) error {
	return a.settings.RemoveCustomGame(id)
}

// GetLibrary lists every source and returns the merged library. Steam failing
// fails the scan, as before; a plugin failing (logged out, crashed) only adds
// a warning, so one broken plugin can't hide the rest of the library.
//
// Sources answer at their own pace (Steam reads local files, a store plugin
// may wait on its store): each time one finishes while others are still out,
// the library of those finished is sent as a "library-partial" event, so the
// window can show it before the slowest plugin answers.
func (a *App) GetLibrary() (types.Library, error) {
	all := a.stores.All()
	results := make([][]types.Game, len(all))
	errs := make([]error, len(all))
	finished := make(chan int, len(all))
	for i, s := range all {
		go func() {
			ctx, cancel := context.WithTimeout(a.ctx, libraryTimeout)
			defer cancel()
			results[i], errs[i] = s.Library(ctx)
			finished <- i
		}()
	}
	done := make([]bool, len(all))
	for left := len(all); left > 0; left-- {
		done[<-finished] = true // the channel hands over the writes of results[i] and errs[i]
		if left > 1 && a.libraryEmit != nil {
			if lib, err := a.assembleLibrary(all, results, errs, done); err == nil && len(lib.Games) > 0 {
				a.libraryEmit(lib)
			}
		}
	}
	lib, err := a.assembleLibrary(all, results, errs, done)
	if err != nil {
		return types.Library{}, err
	}
	a.forgetRemovedGameSettings(all, results, errs)
	return lib, nil
}

// assembleLibrary merges what the finished sources returned with the sideloaded
// games and puts the per-game facts on them.
func (a *App) assembleLibrary(all []stores.Store, results [][]types.Game, errs []error, done []bool) (types.Library, error) {
	lib := types.Library{Games: []types.Game{}, Warnings: []types.SourceWarning{}}
	for i, s := range all {
		if !done[i] {
			continue
		}
		if errs[i] != nil {
			if s.ID() == types.SourceSteam {
				return types.Library{}, fmt.Errorf("%s library scan failed: %w", s.ID(), errs[i])
			}
			lib.Warnings = append(lib.Warnings, sourceWarning(s.ID(), errs[i]))
			continue
		}
		lib.Games = append(lib.Games, results[i]...)
		a.startPlaytimeOnce(s.ID())
	}
	lib.Games = append(lib.Games, a.customGames()...)
	a.stats.applyPlaytime(lib.Games)
	a.applyPins(&lib)
	a.applyCustomization(lib.Games)
	a.applyFixes(lib.Games)
	return lib, nil
}

// applyFixes puts each game's curated fix or ProtonDB tier on it (the library
// cards and filters read it). Ratings come from the cache: a game with none yet
// gets one on a later scan, when its fetch has finished.
func (a *App) applyFixes(games []types.Game) {
	for i := range games {
		hints, _ := a.cachedPluginHints(games[i]) // never a plugin call per game here
		games[i].FixInfo = a.gameFix(games[i], hints, func(appID string) (compat.Rating, bool) {
			if a.compat == nil {
				return compat.Rating{}, false
			}
			return a.compat.ProtonDBCached(appID)
		})
	}
}

// sourceWarning describes a source that failed to list. The plugin's message
// is shown as is; its error code only tells a login problem (-32001) from a
// temporary "unavailable right now" (-32002).
func sourceWarning(source string, err error) types.SourceWarning {
	return types.SourceWarning{
		Source: source, Message: err.Error(),
		NeedsAuth:   plugins.IsAuthRequired(err),
		Unavailable: plugins.IsUnavailable(err),
	}
}

// GetGameDetails returns summary, screenshots and achievements for the game
// page. Sideloaded games have no metadata source.
func (a *App) GetGameDetails(game types.Game) (*types.GameDetails, error) {
	s, ok := a.stores.Get(sourceOf(game))
	if !ok {
		return &types.GameDetails{}, nil
	}
	ctx, cancel := context.WithTimeout(a.ctx, launchTimeout)
	defer cancel()
	return s.Details(ctx, game)
}

// sourceOf tolerates games from before Game.Source existed.
func sourceOf(game types.Game) string {
	if game.Source == "" && game.Store == types.StoreSteam {
		return types.SourceSteam
	}
	return game.Source
}

// syncDataPlugins rebuilds the fixes/tweaks registry from the discovered
// data plugins plus the built-in data, and invalidates the cached tweaks
// (CORE-5). Call after every Discover.
func (a *App) syncDataPlugins(found []plugins.Plugin) {
	a.data.Reset()
	for _, p := range found {
		if p.Data != nil {
			a.data.Add(p.Data)
		}
	}
	// The built-in fixes and tweaks are data plugin files too, applied
	// through the same registry.
	a.data.Add(dataplug.Builtin())
	a.tweaksMu.Lock()
	a.tweaksCached = false
	a.tweaksMu.Unlock()
}

// GetTweaks returns hardware-based tweaks offered for every game, from the
// built-in data plugin and any user-installed ones.
func (a *App) GetTweaks() []types.Tweak {
	a.tweaksMu.Lock()
	defer a.tweaksMu.Unlock()
	if !a.tweaksCached {
		a.tweaks = a.data.Tweaks(dataplug.DetectSystem())
		a.tweaksCached = true
	}
	return a.tweaks
}

// ListSessions returns the currently tracked game sessions (starting or
// running) so windows opened after a launch can hydrate without waiting for
// the next session event (CORE-1). It never returns nil.
func (a *App) ListSessions() []types.SessionEvent {
	if a.sessions == nil {
		return []types.SessionEvent{}
	}
	return a.sessions.Snapshot()
}

// selectRunner resolves the game's runner; "auto" or empty picks the best
// installed one (see runner.PickAuto).
func selectRunner(game types.Game) (types.ProtonVersion, error) {
	runners := runner.ScanProtonVersions()
	if r, ok := runner.Resolve(game.Settings.RunnerID, runners); ok {
		return r, nil
	}
	if runner.IsAuto(game.Settings.RunnerID) {
		return types.ProtonVersion{}, fmt.Errorf("no Proton or Wine runner installed")
	}
	return types.ProtonVersion{}, fmt.Errorf("runner %q is not installed", game.Settings.RunnerID)
}

// prepared is a launch ready to run.
type prepared struct {
	args   []string
	env    map[string]string
	dir    string
	client *stores.ClientLaunch // set when a store client starts the game
	redact []string             // argument prefixes hidden in previews and logs
	runner types.ProtonVersion  // the Wine/Proton runner, when the game uses one
}

// prepareLaunch asks the game's store how to launch it and builds the command.
// preview must not cost the store anything (no launch tokens).
func (a *App) prepareLaunch(game types.Game, preview bool) (*prepared, error) {
	var launch *stores.Launch
	if s, ok := a.stores.Get(sourceOf(game)); ok {
		ctx, cancel := context.WithTimeout(a.ctx, launchTimeout)
		defer cancel()
		l, err := s.Launch(ctx, game, preview)
		if err != nil {
			return nil, err
		}
		launch = l
	} else if game.Source != "" && game.Source != types.SourceCustom {
		return nil, fmt.Errorf("%s comes from %s, which isn't approved and enabled", game.Title, game.Source)
	}

	switch {
	case launch != nil && launch.Client != nil:
		return &prepared{args: launch.Client.Command, env: map[string]string{}, client: launch.Client}, nil
	case launch != nil && launch.Direct != nil:
		d := launch.Direct
		var r types.ProtonVersion
		if d.NeedsWine {
			var err error
			if r, err = selectRunner(game); err != nil {
				return nil, err
			}
		}
		args, env, err := runner.BuildCommand(game, r, runner.Target{Exe: d.Exe, Args: d.Args, Env: d.Env, Native: !d.NeedsWine}, preview)
		if err != nil {
			return nil, err
		}
		dir := d.Dir
		if game.Settings.WorkingDir != "" {
			dir = game.Settings.WorkingDir
		}
		return &prepared{args: args, env: env, dir: dir, redact: d.Redact, runner: r}, nil
	}

	r, err := selectRunner(game)
	if err != nil {
		return nil, err
	}
	args, env, err := runner.BuildLaunchCommand(game, r, preview)
	if err != nil {
		return nil, err
	}
	dir := game.Settings.WorkingDir
	if dir == "" && game.Executable != "" {
		dir = filepath.Dir(game.Executable)
	}
	return &prepared{args: args, env: env, dir: dir, runner: r}, nil
}

// shown is the command line with env, redacted for previews and logs.
func (p *prepared) shown() string {
	keys := make([]string, 0, len(p.env))
	for k := range p.env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys)+len(p.args))
	for _, k := range keys {
		parts = append(parts, k+"="+p.env[k])
	}
	return strings.Join(append(parts, stores.RedactArgs(p.args, p.redact)...), " ")
}

// stopHook asks the game's wineserver to terminate when the game is stopped;
// wineserver daemonizes, so it outlives the launch's process group. Native
// games have none.
func (p *prepared) stopHook() func(force bool) {
	return func(force bool) {
		cmd := runner.WineserverStopCommand(p.runner, p.env, force)
		if cmd == nil {
			return
		}
		done := make(chan struct{})
		if err := cmd.Start(); err != nil {
			return
		}
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
		}
	}
}

// PreviewLaunchCommand returns the exact command line that will be used.
func (a *App) PreviewLaunchCommand(game types.Game) (string, error) {
	p, err := a.prepareLaunch(game, true)
	if err != nil {
		return "", err
	}
	return p.shown(), nil
}

// LaunchGame starts the game. Progress (running, stopped, failed) arrives on
// the "session" event; the returned string is only a status line. For a game
// with cloud saves it first syncs them: when the user has to decide (a
// conflict, an unreachable store, the store's client running) nothing is
// launched, the "cloud" event carries the prompt, and the answer is another
// launch (LaunchGameAnyway, LaunchGameViaClient) or a resolve and a retry.
func (a *App) LaunchGame(game types.Game) (string, error) {
	return a.launchGame(game, false)
}

// launchGame starts a game. anyway is the user's "launch anyway" for this one
// launch: it skips the cloud step and nothing after it.
func (a *App) launchGame(game types.Game, anyway bool) (string, error) {
	if _, ok := a.plugins.Plugin(game.Source); ok {
		a.stats.noteSource(game.ID, game.Source)
	}
	unlock := a.lockGame(game.ID)
	defer unlock()
	if a.isRunning(game.ID) {
		return "", fmt.Errorf("%s is already running", game.Title)
	}
	if err := a.heldByJob(game); err != nil {
		return "", err
	}
	p, err := a.prepareLaunch(game, false)
	if err != nil {
		return "", err
	}
	filtered := make([]string, 0, len(p.args))
	for _, arg := range p.args {
		if strings.TrimSpace(arg) != "" {
			filtered = append(filtered, arg)
		}
	}
	if len(filtered) == 0 {
		return "", fmt.Errorf("game %q has no launch target", game.Title)
	}

	a.noteSocialGame(game)
	cmd := exec.Command(filtered[0], filtered[1:]...)
	cmd.Env = os.Environ()
	for k, v := range p.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Dir = p.dir

	if cl := p.client; cl != nil {
		return a.startClient(game, cmd, cl)
	}
	opts := []session.StartOption{session.WithStopHook(p.stopHook())}
	push, note, blocked := a.cloudBeforeLaunch(game, p, anyway)
	if blocked {
		return note, nil
	}
	opts = append(opts, push.options()...)
	pid, err := a.sessions.StartDirect(game.ID, cmd, p.shown(), p.redact, opts...)
	if err != nil {
		if errors.Is(err, session.ErrAlreadyRunning) {
			// Never abandon here: the record Launching wrote may belong to the game that runs.
			return "", fmt.Errorf("%s is already running", game.Title)
		}
		push.abandon()
		return "", err
	}
	if note != "" {
		return fmt.Sprintf("Started %s (PID %d) · %s", game.Title, pid, note), nil
	}
	return fmt.Sprintf("Started %s (PID %d)", game.Title, pid), nil
}

// startClient runs the command that asks the store's client to start the game
// and follows the game itself. With the client running, the command forwards
// the request and exits; if not, it becomes the client and keeps running.
// Either way, follow the game itself (Steam: its reaper process), not this PID.
func (a *App) startClient(game types.Game, cmd *exec.Cmd, cl *stores.ClientLaunch) (string, error) {
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("%s client: %w", strings.ToLower(cl.Client), err)
	}
	go func() { _ = cmd.Wait() }()
	if !a.sessions.WatchClient(game.ID, cl.Client, cl.Find, cl.StartTimeout) {
		return "", fmt.Errorf("%s is already running", game.Title)
	}
	return "Asked " + cl.Client + " to start " + game.Title, nil
}

// StopGame asks a running game to quit: SIGTERM to its processes (and the
// Wine prefix's wineserver), then SIGKILL after a few seconds. It returns once
// the request is delivered; the "session" event reports "stopping", then
// "stopped". Stopping a game that is already stopping does nothing.
func (a *App) StopGame(gameID string) error {
	return a.stopGame(gameID, false)
}

// ForceStopGame kills the game's processes right away.
func (a *App) ForceStopGame(gameID string) error {
	return a.stopGame(gameID, true)
}

func (a *App) stopGame(gameID string, force bool) error {
	err := a.sessions.Stop(gameID, force)
	if errors.Is(err, session.ErrNotRunning) {
		return errors.New("that game is not running")
	}
	return err
}

func (a *App) launcherSettings() types.LauncherSettings {
	return types.LauncherSettings{
		GamesFolder:        a.settings.GamesFolder(),
		DefaultGamesFolder: settings.DefaultGamesFolder(),
		CancelCleanupDays:  a.settings.CancelCleanupDays(),
		RatingsOnline:      a.settings.RatingsOnline(),
	}
}

// GetLauncherSettings returns Platypus-wide settings.
func (a *App) GetLauncherSettings() types.LauncherSettings {
	return a.launcherSettings()
}

func (a *App) chooseFolder(title, start string) (string, error) {
	if _, err := os.Stat(start); err != nil {
		start, _ = os.UserHomeDir()
	}
	return a.host.OpenDirectory(DirectoryDialog{Title: title, Default: start, CanCreate: true})
}

// GetGameSettings returns every saved per-game settings object by game ID.
func (a *App) GetGameSettings() map[string]map[string]any {
	return a.settings.GameSettings()
}

// SaveGameSettings persists one game's settings as the UI holds them.
func (a *App) SaveGameSettings(gameID string, gameSettings map[string]any) error {
	return a.settings.SetGameSettings(gameID, gameSettings)
}
