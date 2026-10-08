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
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"hydra-launcher/internal/dataplug"
	"hydra-launcher/internal/fixes"
	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/jsonrpc"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/runner"
	"hydra-launcher/internal/session"
	"hydra-launcher/internal/settings"
	"hydra-launcher/internal/stores"
	"hydra-launcher/internal/types"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// How long a store plugin gets for library.list (legendary refreshes its
// catalogue over the network) and for a launch spec.
const (
	libraryTimeout = 2 * time.Minute
	launchTimeout  = time.Minute
)

// App struct
type App struct {
	ctx          context.Context
	stop         context.CancelFunc
	plugins      *plugins.Manager
	settings     *settings.Store
	jobs         *jobs.Queue
	fixesService *fixes.Service
	stores       *stores.Registry
	sessions     *session.Tracker
	// data holds the built-in fixes and tweaks plus every valid data plugin's.
	data *dataplug.Registry

	tweaksMu     sync.Mutex
	tweaksCached bool
	tweaks       []types.Tweak
}

// NewApp creates a new App application struct
func NewApp() *App {
	a := &App{fixesService: fixes.NewService(), data: dataplug.NewRegistry()}
	a.stores = stores.NewRegistry(a.pluginStores, stores.NewSteam())
	return a
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.sessions = session.NewTracker(func(e types.SessionEvent) {
		wruntime.EventsEmit(ctx, "session", e)
	})

	dataDir, err := settings.DataDir()
	if err != nil {
		wruntime.LogErrorf(ctx, "settings: %v", err)
	}
	a.settings, err = settings.Open(filepath.Join(dataDir, "settings.json"))
	if err != nil {
		// Open still returns usable defaults.
		wruntime.LogErrorf(ctx, "settings: %v", err)
	}

	a.jobs, err = jobs.Open(filepath.Join(dataDir, "jobs.json"), jobBackend{a}, func(list []types.Job) {
		wruntime.EventsEmit(ctx, "jobs", list)
	})
	if err != nil {
		wruntime.LogErrorf(ctx, "jobs: %v", err)
	}
	a.jobs.SetCleanupAfter(cleanupAfter(a.settings.CancelCleanupDays()))

	// Store plugins start on demand: when the library is listed, on login,
	// or when an interrupted install resumes.
	a.plugins = plugins.NewManager(plugins.Options{
		Dir:      filepath.Join(dataDir, "plugins"),
		LogDir:   filepath.Join(dataDir, "logs"),
		DataDir:  filepath.Join(dataDir, "plugin-data"),
		Settings: a.settings,
		OnNotify: a.pluginNotify,
		OnExit: func(id string, err error) {
			wruntime.LogWarningf(ctx, "plugin %s exited: %v", id, err)
			a.jobs.PluginExited(id, err)
		},
	})
	found, err := a.plugins.Discover()
	if err != nil {
		wruntime.LogErrorf(ctx, "plugins: %v", err)
	}
	for _, p := range found {
		msg := fmt.Sprintf("plugin %s: %s", p.Manifest.ID, p.Status)
		if p.Problem != "" {
			msg += " (" + p.Problem + ")"
		}
		wruntime.LogInfo(ctx, msg)
	}
	// Data plugins feed the fixes/tweaks registry; rebuilt here and on
	// every rescan so installs and removals apply without a restart.
	a.syncDataPlugins(found)

	runCtx, stop := context.WithCancel(ctx)
	a.stop = stop
	go a.jobs.Run(runCtx)
}

// shutdown stops games Hydra started directly and every store plugin.
// Steam-launched games belong to the Steam client and keep running. Running
// installs stay "running" in jobs.json and resume on the next start.
func (a *App) shutdown(ctx context.Context) {
	if a.stop != nil {
		a.stop()
	}
	if a.sessions != nil {
		a.sessions.StopAll()
	}
	if a.plugins != nil {
		a.plugins.Shutdown()
	}
}

// pluginNotify routes plugin notifications: job updates to the queue.
func (a *App) pluginNotify(id, method string, params json.RawMessage) {
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
		if p.Status != plugins.StatusReady || !slices.Contains(p.Manifest.Capabilities, "library") {
			continue
		}
		id := p.Manifest.ID
		out = append(out, stores.NewPlugin(a.plugins, p, func() []string {
			return a.plugins.Folders(id, a.settings.GamesFolder())
		}))
	}
	return out
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
func (a *App) GetLibrary() (types.Library, error) {
	all := a.stores.All()
	results := make([][]types.Game, len(all))
	errs := make([]error, len(all))
	var wg sync.WaitGroup
	for i, s := range all {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(a.ctx, libraryTimeout)
			defer cancel()
			results[i], errs[i] = s.Library(ctx)
		}()
	}
	wg.Wait()

	lib := types.Library{Games: []types.Game{}, Warnings: []types.SourceWarning{}}
	for i, s := range all {
		if errs[i] != nil {
			if s.ID() == types.SourceSteam {
				return types.Library{}, fmt.Errorf("%s library scan failed: %w", s.ID(), errs[i])
			}
			var rpcErr *jsonrpc.Error
			needsAuth := errors.As(errs[i], &rpcErr) && rpcErr.Code == plugins.CodeAuthRequired
			lib.Warnings = append(lib.Warnings, types.SourceWarning{Source: s.ID(), Message: errs[i].Error(), NeedsAuth: needsAuth})
			continue
		}
		lib.Games = append(lib.Games, results[i]...)
	}
	lib.Games = append(lib.Games, a.customGames()...)
	return lib, nil
}

// GetGameFix returns the curated fix from data plugins, or a live ProtonDB
// summary for a game without one.
func (a *App) GetGameFix(storeAppID string, title string) *types.GameFixInfo {
	if fix := a.data.Fix(title, storeAppID); fix != nil {
		return fix
	}
	if fix, err := a.fixesService.FetchProtonDBFix(storeAppID); err == nil {
		return fix
	}
	return nil
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
// the "session" event; the returned string is only a status line.
func (a *App) LaunchGame(game types.Game) (string, error) {
	if a.sessions.Running(game.ID) {
		return "", fmt.Errorf("%s is already running", game.Title)
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

	cmd := exec.Command(filtered[0], filtered[1:]...)
	cmd.Env = os.Environ()
	for k, v := range p.env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Dir = p.dir

	if cl := p.client; cl != nil {
		// With the client running, the command forwards the request and
		// exits; if not, it becomes the client and keeps running. Either way,
		// follow the game itself (Steam: its reaper process), not this PID.
		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("%s client: %w", strings.ToLower(cl.Client), err)
		}
		go func() { _ = cmd.Wait() }()
		if !a.sessions.WatchClient(game.ID, cl.Client, cl.Find, cl.StartTimeout) {
			return "", fmt.Errorf("%s is already running", game.Title)
		}
		return "Asked " + cl.Client + " to start " + game.Title, nil
	}
	pid, err := a.sessions.StartDirect(game.ID, cmd, p.shown(), p.redact, session.WithStopHook(p.stopHook()))
	if err != nil {
		if errors.Is(err, session.ErrAlreadyRunning) {
			return "", fmt.Errorf("%s is already running", game.Title)
		}
		return "", err
	}
	return fmt.Sprintf("Started %s (PID %d)", game.Title, pid), nil
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
	}
}

// GetLauncherSettings returns Hydra-wide settings.
func (a *App) GetLauncherSettings() types.LauncherSettings {
	return a.launcherSettings()
}

// SetGamesFolder saves the base folder store plugins install into. An empty
// path resets it to ~/Games.
func (a *App) SetGamesFolder(path string) (types.LauncherSettings, error) {
	if _, err := a.settings.SetGamesFolder(path); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}

// ChooseGamesFolder opens a folder picker at the current Games folder.
// Returns "" if the user cancels.
func (a *App) ChooseGamesFolder() (string, error) {
	return a.chooseFolder("Games folder", a.settings.GamesFolder())
}

func (a *App) chooseFolder(title, start string) (string, error) {
	if _, err := os.Stat(start); err != nil {
		start, _ = os.UserHomeDir()
	}
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:                title,
		DefaultDirectory:     start,
		CanCreateDirectories: true,
	})
}

// GetGameSettings returns every saved per-game settings object by game ID.
func (a *App) GetGameSettings() map[string]map[string]any {
	return a.settings.GameSettings()
}

// SaveGameSettings persists one game's settings as the UI holds them.
func (a *App) SaveGameSettings(gameID string, gameSettings map[string]any) error {
	return a.settings.SetGameSettings(gameID, gameSettings)
}
