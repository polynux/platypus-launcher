package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/chat"
	"platypus/internal/cloudsync"
	"platypus/internal/compat"
	"platypus/internal/dataplug"
	"platypus/internal/jobs"
	"platypus/internal/ledger"
	"platypus/internal/migrate"
	"platypus/internal/netstate"
	"platypus/internal/onlineart"
	"platypus/internal/paths"
	"platypus/internal/playlog"
	"platypus/internal/plugins"
	"platypus/internal/runner"
	"platypus/internal/savebackup"
	"platypus/internal/session"
	"platypus/internal/settings"
	"platypus/internal/social"
	"platypus/internal/steam"
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
	removes       coreMoves   // core-run removals (issue #43), like moves
	custom        customStore // the user's titles and art (app_customize.go)
	ledger        *ledger.Ledger
	// artwork is the online art lookup for sideloaded games (issue #36).
	// Nil means the live Steam endpoints; tests inject an httptest client.
	artwork *onlineart.Client
	// artCacheMu guards artCache.
	artCacheMu sync.Mutex
	// artCache remembers the GOG hits of recent SearchArtOnline calls by
	// "source:id", so an apply downloads exactly what the user saw: the
	// apply's game title may differ from the searched title, and GOG has no
	// lookup by product id. Bounded (artCacheMax); never leaves the process.
	artCache map[string]onlineart.Candidate
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
	// dataDir is Platypus's data folder (settings.json lives inside it);
	// the sideloaded game-info disk cache lives there too (issue #38).
	dataDir string
	// optHome/optDataDir/optCacheDir/optPluginsDir are the explicit folders
	// from Options (issue #41); empty means "resolve at startup".
	optHome       string
	optDataDir    string
	optCacheDir   string
	optPluginsDir string
	// cacheDir is the resolved cache folder (only re-fetchable data).
	cacheDir string
	// sideloadSteam fetches Steam details for sideloaded games with a compat
	// AppID (issue #38). Nil means the live store endpoint; tests inject an
	// httptest client.
	sideloadSteam *steam.StoreClient
	// infoCacheMu guards infoCache.
	infoCacheMu sync.Mutex
	// infoCache is the disk-backed GameDetails of sideloaded games by game
	// id (issue #38); infoCacheLoaded says the file was read. Never leaves
	// the data dir.
	infoCache       map[string]gameInfoEntry
	infoCacheLoaded bool
	// playlog is Platypus's own session log (issue #15): the local side of
	// the playtime and last-played numbers applyPlaytime merges.
	playlog *playlog.Log
	// sizes caches what the store says an install needs (app_installsize.go).
	sizes sizeCache
	// diskUsage caches the disk usage snapshot (app_diskusage.go, issue
	// #50). diskGames lists the games a scan measures; nil means the
	// library, and tests replace it so no plugin is needed.
	diskUsage diskUsageCache
	diskGames func() ([]types.Game, error)
	// library caches one listing per source; a finished job, a plugin change
	// or a manual rescan invalidates explicitly (app_library.go, issue #27).
	// scanMu serializes concurrent scans so the second serves the first
	// one's fresh cache instead of listing the same slow stores twice.
	library libraryCache
	scanMu  sync.Mutex
	// auth caches one login state per plugin: the Plugins view asks on every
	// visit, and asking costs the plugin a store round trip (app_auth.go,
	// issue #28). Served immediately, refreshed in the background.
	auth authCache

	gameMu    sync.Mutex
	gameLocks map[string]*sync.Mutex // by game id; see lockGame
	// localeMu guards systemLangs, the desktop languages the shell reports
	// (issue #39, phase 4). The setting lives in settings.Store; the
	// effective locale is setting else system list else English.
	localeMu    sync.Mutex
	systemLangs []string
	// migration is what the Hydra→Platypus data move did at startup
	// (internal/migrate); MigrationNotice shows it once (issue #14).
	migration migrate.Report
	// savebk backs up game saves (issue #42); nil until startup sets it
	// up, and in tests that don't need it (every method tolerates nil).
	savebk *savebackup.Service
	// backups tracks the core-run backup/restore jobs, like moves.
	backups coreBackups
	// backupGateForTests overrides backupBeforeDelete in tests (issue #43);
	// nil in production, where the stub (issue #42: the real gate) runs.
	backupGateForTests func(ctx context.Context, game types.Game, trigger string, allowNoSaves bool) (types.SaveGate, error)
	// trayMu guards deferred, the non-urgent background work waiting for
	// the last game session to end (issue #49).
	trayMu sync.Mutex
	// deferred holds background jobs put off while a game runs, in order:
	// auto artwork lookups and the save-list manifest refresh. Drained
	// when the last session ends (drainDeferred).
	deferred []func()
	// net tracks whether the machine looks online (issue #44). Every core
	// outbound request reports its outcome to it; the UI reads it through
	// GetNetState and the "netstate" event. Nil in tests that don't need
	// it (every helper tolerates nil).
	net *netstate.State
}

// Options carries the folders one App run uses (issue #41). Empty fields
// resolve from the environment once, in startup, via internal/paths; tests
// pass temp dirs directly instead of setting HOME.
type Options struct {
	DataDir    string
	CacheDir   string
	PluginsDir string
	Home       string
}

// NewApp creates a new App application struct
func NewApp() *App {
	return NewAppWithOptions(Options{})
}

// NewAppWithOptions is NewApp with explicit folders (issue #41).
func NewAppWithOptions(o Options) *App {
	a := &App{data: dataplug.NewRegistry(), host: stderrHost{}}
	a.optHome = o.Home
	a.optDataDir = o.DataDir
	a.optCacheDir = o.CacheDir
	a.optPluginsDir = o.PluginsDir
	a.stores = stores.NewRegistry(a.pluginStores)
	a.net = netstate.New()
	a.net.OnTransition(func(online bool) {
		snap := a.net.Snapshot()
		a.host.Emit("netstate", types.NetState{Online: snap.Online, Since: snap.Since})
	})
	return a
}

// homeDir is the configured Home, defaulting to os.UserHomeDir.
func (a *App) homeDir() string {
	if a == nil {
		return ""
	}
	if a.optHome != "" {
		return a.optHome
	}
	home, _ := os.UserHomeDir()
	return home
}

// resolvedDataDir is the data folder in use: the explicit field, the
// startup-resolved one, or the default for the configured home.
func (a *App) resolvedDataDir() string {
	if a == nil {
		return ""
	}
	if a.dataDir != "" {
		return a.dataDir
	}
	if a.optDataDir != "" {
		return a.optDataDir
	}
	return settings.DataDirFor(a.homeDir())
}

// resolvedCacheDir is the cache folder in use.
func (a *App) resolvedCacheDir() string {
	if a == nil {
		return ""
	}
	if a.cacheDir != "" {
		return a.cacheDir
	}
	if a.optCacheDir != "" {
		return a.optCacheDir
	}
	return paths.CacheDirFor(a.homeDir())
}

// resolvedPluginsDir is the plugins folder in use.
func (a *App) resolvedPluginsDir() string {
	if a == nil {
		return ""
	}
	if a.optPluginsDir != "" {
		return a.optPluginsDir
	}
	return paths.PluginsDirFor(a.resolvedDataDir())
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// A UI client is connected from here until shutdown: probes only run
	// while one is (issue #44).
	if a.net == nil {
		a.net = netstate.New()
		a.net.OnTransition(func(online bool) {
			snap := a.net.Snapshot()
			a.host.Emit("netstate", types.NetState{Online: snap.Online, Since: snap.Since})
		})
	}
	a.net.SetClientConnected(true)
	a.stats.changed = func() { a.host.Emit("playtime") }
	a.stats.info = func(format string, args ...any) { a.host.Infof(format, args...) }
	a.stats.warn = func(format string, args ...any) { a.host.Warnf(format, args...) }
	a.sessions = session.NewTracker(func(e types.SessionEvent) {
		a.host.Emit("session", e)
		a.socialSession(e)
		a.playSession(e)
		if e.State == "stopped" || e.State == "failed" {
			a.schedulePlaytimeRefresh(e.GameID)
			if !a.gameRunning() {
				a.drainDeferred()
			}
		}
	})

	// Before anything opens a data file: the previous data folder moves to
	// its new name, and the paths stored in it follow (internal/migrate).
	// Folders come from internal/paths once (issue #41); explicit Options
	// win over the environment.
	resolved := paths.Resolve()
	if a.optHome != "" {
		resolved.Home = a.optHome
		// Home was forced: re-resolve the env-derived folders against it,
		// unless an explicit option already pins them.
		fresh := paths.ResolveFor(resolved.Home)
		if a.optDataDir == "" {
			resolved.DataDir = fresh.DataDir
		}
		if a.optCacheDir == "" {
			resolved.CacheDir = fresh.CacheDir
		}
		resolved.ConfigDir = fresh.ConfigDir
	}
	if a.optDataDir != "" {
		resolved.DataDir = a.optDataDir
	}
	if a.optCacheDir != "" {
		resolved.CacheDir = a.optCacheDir
	}
	if a.optPluginsDir != "" {
		resolved.PluginsDir = a.optPluginsDir
	} else {
		resolved.PluginsDir = paths.PluginsDirFor(resolved.DataDir)
	}
	a.optHome = resolved.Home
	a.optDataDir = resolved.DataDir
	a.optCacheDir = resolved.CacheDir
	a.optPluginsDir = resolved.PluginsDir
	if home := resolved.Home; home != "" {
		a.migration = migrate.Run(home, migrate.Options{
			Info: func(format string, args ...any) { a.host.Infof(format, args...) },
			Warn: func(format string, args ...any) { a.host.Warnf(format, args...) },
		})
	}
	dataDir := resolved.DataDir
	a.dataDir = dataDir
	a.cacheDir = resolved.CacheDir
	a.library.snapDir = librarySnapshotDir(dataDir)
	a.sessions.SetLogDir(filepath.Join(dataDir, "logs"))
	var err error
	a.settings, err = settings.OpenWithHome(filepath.Join(dataDir, "settings.json"), resolved.Home)
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
		// Every compat fetcher's requests feed the offline tracker (issue #44).
		for _, f := range a.compat.Fetchers() {
			f.Client = a.netClient(f.Client, 15*time.Second)
		}
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

	// The session log opens after the data folder move above, so a crash
	// recovery reads the folder in use. Dangling sessions close at their
	// last heartbeat inside Open.
	a.openPlaylog(dataDir)

	// Store plugins start on demand: when the library is listed, on login,
	// or when an interrupted install resumes.
	a.plugins = plugins.NewManager(plugins.Options{
		Dir:      resolved.PluginsDir,
		LogDir:   filepath.Join(dataDir, "logs"),
		DataDir:  filepath.Join(dataDir, "plugin-data"),
		Settings: a.settings,
		Home:     resolved.Home,
		OnNotify: a.pluginNotify,
		OnExit: func(id string, err error) {
			a.host.Warnf("plugin %s exited: %v", id, err)
			// The process is gone: its cached login state died with it.
			a.invalidateAuth(id)
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
		// The bundled steam plugin installs itself when missing (issue
		// #54), before anything lists what was found.
		found = a.ensureBundledSteam(found)
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
	// The shell reports the desktop languages after initialize (see
	// SetSystemLanguages); until then the setting alone decides the locale
	// plugins start with.
	a.plugins.SetLocale(a.Locale())
	if a.stores != nil {
		a.stores.SetLocale(a.Locale())
	}
	a.startCloud(ctx, dataDir)
	a.startSavebackups(ctx)
	a.startSocial(ctx)
	a.startChat(ctx)
	a.syncSocial()
	a.syncChatUnread()

	runCtx, stop := context.WithCancel(ctx)
	a.stop = stop
	go a.jobs.Run(runCtx)
	go a.playHeartbeatLoop(runCtx)
}

// shutdown stops games Platypus started directly and every store plugin.
// Steam-launched games belong to the Steam client and keep running. Running
// installs stay "running" in jobs.json and resume on the next start.
func (a *App) shutdown(ctx context.Context) {
	if a.net != nil {
		a.net.SetClientConnected(false)
	}
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
	// Sessions still open belong to games the shutdown just killed (or to
	// Steam games that keep running): close them at their last heartbeat so
	// the next start finds nothing dangling. A crash skips this and Open
	// recovers them instead.
	if a.playlog != nil {
		a.playlog.CloseAllAtHeartbeat()
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
	return runner.ScanProtonVersionsFor(a.homeDir())
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
	savedAll := map[string]map[string]any{}
	if a.settings != nil {
		savedAll = a.settings.GameSettings()
	}
	for _, g := range a.settings.CustomGames() {
		g := types.Game{
			ID:          g.ID,
			Source:      types.SourceCustom,
			Store:       types.StoreCustom,
			StoreAppID:  types.SourceCustom,
			Title:       g.Title,
			ReleaseYear: g.ReleaseYear,
			InstallDir:  filepath.Dir(g.Executable),
			Executable:  g.Executable,
			IsInstalled: true,
			Settings:    types.GameSettings{UseGameMode: true},
		}
		// Saved per-game settings win over the defaults. Keys the backend
		// doesn't model (UI-only controls like sync_mode) are kept
		// opaquely so sideloaded games don't lose them (CORE-9).
		if saved, ok := savedAll[g.ID]; ok {
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
		g.Native = a.customNative(g.Executable, g.Settings.RunMode)
		out = append(out, g)
	}
	return out
}

// customNative resolves whether a sideloaded executable runs natively
// (issue #47): content detection, with the per-game run_mode override
// ("native"/"wine") winning over it. An unreadable file defaults to Wine,
// preserving the old behaviour until the launch surfaces the error.
func (a *App) customNative(executable, runMode string) bool {
	detected := false
	if executable != "" {
		if d, err := runner.DetectExecutable(executable); err == nil {
			detected = d.Native
		}
	}
	return runner.ResolveNative(detected, runMode)
}

// AddCustomGame saves a game the user added by hand: title plus the path of
// its executable. It shows up in the library from then on. When the "Find
// artwork online" switch is on, the cover and banner lookup starts in the
// background (issue #36) and never blocks the add.
func (a *App) AddCustomGame(title, executable string) (types.Game, error) {
	g, err := a.settings.AddCustomGame(title, executable)
	if err != nil {
		return types.Game{}, err
	}
	a.maybeAutoArtwork(g.ID, g.Title)
	return types.Game{
		ID:          g.ID,
		Source:      types.SourceCustom,
		Store:       types.StoreCustom,
		StoreAppID:  types.SourceCustom,
		Title:       g.Title,
		ReleaseYear: g.ReleaseYear,
		InstallDir:  filepath.Dir(g.Executable),
		Executable:  g.Executable,
		IsInstalled: true,
		Settings:    types.GameSettings{UseGameMode: true},
		Native:      a.customNative(g.Executable, ""),
	}, nil
}

// DetectGameExecutable reports how a sideloaded executable would run by
// default (issue #47): content detection, before the per-game override. The
// UI shows it as the default of the "Run with" choice.
func (a *App) DetectGameExecutable(path string) (types.DetectResult, error) {
	d, err := runner.DetectExecutable(path)
	if err != nil {
		return types.DetectResult{}, apperr.Wrap("sideload.executable.unreadable",
			fmt.Errorf("executable: %w", err), map[string]string{"executable": path})
	}
	return types.DetectResult{Native: d.Native, Kind: string(d.Kind)}, nil
}

// UpdateCustomGameExecutable points a sideloaded game at a new executable
// and returns it as the library shows it (re-detected, issue #47).
func (a *App) UpdateCustomGameExecutable(id, executable string) (types.Game, error) {
	g, err := a.settings.UpdateCustomGameExecutable(id, executable)
	if err != nil {
		return types.Game{}, err
	}
	for _, full := range a.customGames() {
		if full.ID == id {
			return full, nil
		}
	}
	return types.Game{
		ID: g.ID, Source: types.SourceCustom, Store: types.StoreCustom, StoreAppID: types.SourceCustom,
		Title: g.Title, ReleaseYear: g.ReleaseYear, InstallDir: filepath.Dir(g.Executable),
		Executable: g.Executable, IsInstalled: true,
		Settings: types.GameSettings{UseGameMode: true}, Native: a.customNative(g.Executable, ""),
	}, nil
}

// IsGameExecutable says whether the sideloaded game's file has the execute
// bit. The UI offers "Make it executable" when it doesn't.
func (a *App) IsGameExecutable(gameID string) (bool, error) {
	game, err := a.customGameByID(gameID)
	if err != nil {
		return false, err
	}
	ok, err := runner.IsExecutable(game.Executable)
	if err != nil {
		return false, apperr.Wrap("sideload.executable.unreadable",
			fmt.Errorf("executable: %w", err), map[string]string{"executable": game.Executable})
	}
	return ok, nil
}

// MakeGameExecutable sets the execute bit on the sideloaded game's own file
// (chmod u+x on that one file, issue #47). The UI asks for confirmation
// first; without the bit a native launch fails with a clear error.
func (a *App) MakeGameExecutable(gameID string) (types.Game, error) {
	game, err := a.customGameByID(gameID)
	if err != nil {
		return types.Game{}, err
	}
	if err := runner.MakeExecutable(game.Executable); err != nil {
		return types.Game{}, apperr.Wrap("sideload.executable.unreadable",
			fmt.Errorf("executable: %w", err), map[string]string{"executable": game.Executable})
	}
	for _, full := range a.customGames() {
		if full.ID == gameID {
			return full, nil
		}
	}
	return game, nil
}

// SetCustomGameYear saves a sideloaded game's release year (0 clears it)
// and returns the game as the library shows it.
func (a *App) SetCustomGameYear(id string, year int) (types.Game, error) {
	if err := a.settings.SetCustomGameYear(id, year); err != nil {
		return types.Game{}, err
	}
	for _, g := range a.customGames() {
		if g.ID == id {
			return g, nil
		}
	}
	return types.Game{}, apperr.New("game.notFound", fmt.Sprintf("unknown game %q", id), map[string]string{"id": id})
}

// RemoveCustomGame forgets a hand-added game; its files stay on disk.
func (a *App) RemoveCustomGame(id string) error {
	return a.settings.RemoveCustomGame(id)
}

// assembleLibrary merges what the finished sources returned with the sideloaded
// games and puts the per-game facts on them. A source that failed (logged
// out, crashed, offline) adds a warning instead of failing the scan — Steam
// included, like every other source (issue #54 groundwork): a Steam failure
// is a normal warning, with the snapshot rule. When the failed source's last
// good listing (or snapshot) stands in, the warning is marked stale.
func (a *App) assembleLibrary(all []stores.Store, results [][]types.Game, errs []error, done []bool) (types.Library, error) {
	lib := types.Library{Games: []types.Game{}, Warnings: []types.SourceWarning{}}
	for i, s := range all {
		if !done[i] {
			continue
		}
		if errs[i] != nil {
			w := sourceWarning(s.ID(), errs[i])
			if len(results[i]) > 0 {
				lib.Games = append(lib.Games, results[i]...)
				w.Stale = true
			}
			lib.Warnings = append(lib.Warnings, w)
			continue
		}
		lib.Games = append(lib.Games, results[i]...)
		a.startPlaytimeOnce(s.ID())
	}
	lib.Games = append(lib.Games, a.customGames()...)
	a.applyPlaytime(lib.Games)
	a.applyPins(&lib)
	a.applyCustomization(lib.Games)
	a.applyGameStatus(lib.Games)
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
		}, func(appID string) (compat.DeckResult, bool) {
			if a.compat == nil {
				return compat.DeckResult{}, false
			}
			return a.compat.DeckRatingCached(appID)
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
// page. Sideloaded games have no store source: theirs come from the linked
// online info source (issue #38), cached on disk.
func (a *App) GetGameDetails(game types.Game) (*types.GameDetails, error) {
	if game.Source == types.SourceCustom || (game.Source == "" && game.Store == types.StoreCustom) || a.isCustomGameID(game.ID) {
		return a.sideloadedDetails(game)
	}
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
// installed one (see runner.PickAuto). Issue #48: with no explicit runner,
// an installed umu-launcher is preferred and wraps the auto Proton pick
// (PROTONPATH is that Proton's folder); explicit "umu" picks it directly.
func (a *App) selectRunner(game types.Game) (types.ProtonVersion, error) {
	runners := runner.ScanProtonVersionsFor(a.homeDir())
	if u, _, ok := runner.SelectUmu(game.Settings.RunnerID, runners); ok {
		return u, nil
	}
	if r, ok := runner.Resolve(game.Settings.RunnerID, runners); ok {
		return r, nil
	}
	if runner.IsAuto(game.Settings.RunnerID) {
		return types.ProtonVersion{}, apperr.New("game.noRunner", "no Proton or Wine runner installed", nil)
	}
	return types.ProtonVersion{}, apperr.New("game.runnerMissing", fmt.Sprintf("runner %q is not installed", game.Settings.RunnerID), map[string]string{"id": game.Settings.RunnerID})
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
		return nil, apperr.New("plugin.notApproved", fmt.Sprintf("%s comes from %s, which isn't approved and enabled", game.Title, game.Source), map[string]string{"title": game.Title, "source": game.Source})
	}

	switch {
	case launch != nil && launch.Client != nil:
		return &prepared{args: launch.Client.Command, env: map[string]string{}, client: launch.Client}, nil
	case launch != nil && launch.Direct != nil:
		d := launch.Direct
		var r types.ProtonVersion
		if d.NeedsWine {
			var err error
			if r, err = a.selectRunner(game); err != nil {
				return nil, err
			}
			if runner.IsUmu(r) {
				args, env, err := a.umuCommand(game, r, runner.Target{Exe: d.Exe, Args: d.Args, Env: d.Env, Native: !d.NeedsWine}, preview)
				if err != nil {
					return nil, err
				}
				dir := d.Dir
				if game.Settings.WorkingDir != "" {
					dir = game.Settings.WorkingDir
				}
				return &prepared{args: args, env: env, dir: dir, redact: d.Redact, runner: r}, nil
			}
		}
		args, env, err := runner.BuildCommandFor(game, r, runner.Target{Exe: d.Exe, Args: d.Args, Env: d.Env, Native: !d.NeedsWine}, preview, a.prefixesFolder(), a.homeDir())
		if err != nil {
			return nil, err
		}
		dir := d.Dir
		if game.Settings.WorkingDir != "" {
			dir = game.Settings.WorkingDir
		}
		return &prepared{args: args, env: env, dir: dir, redact: d.Redact, runner: r}, nil
	}

	if a.isCustomLaunch(game) {
		return a.prepareSideload(game, preview)
	}
	r, err := a.selectRunner(game)
	if err != nil {
		return nil, err
	}
	if runner.IsUmu(r) {
		args, env, err := a.umuCommand(game, r, runner.Target{Exe: game.Executable}, preview)
		if err != nil {
			return nil, err
		}
		dir := game.Settings.WorkingDir
		if dir == "" && game.Executable != "" {
			dir = filepath.Dir(game.Executable)
		}
		return &prepared{args: args, env: env, dir: dir, runner: r}, nil
	}
	args, env, err := runner.BuildLaunchCommandFor(game, r, preview, a.prefixesFolder(), a.homeDir())
	if err != nil {
		return nil, err
	}
	dir := game.Settings.WorkingDir
	if dir == "" && game.Executable != "" {
		dir = filepath.Dir(game.Executable)
	}
	return &prepared{args: args, env: env, dir: dir, runner: r}, nil
}

func (a *App) prepareSideload(game types.Game, preview bool) (*prepared, error) {
	// The library sets Native, but previews from the settings draft (or any
	// hand-built game) may carry a stale flag with a fresh run_mode: resolve
	// again so the preview is honest.
	game.Native = a.customNative(game.Executable, game.Settings.RunMode)
	return a.prepareCustomLaunch(game, preview)
}

// isCustomLaunch says whether the game launches as a sideloaded executable
// (no store launch spec): source custom, or a custom id the caller passed by
// hand.
func (a *App) isCustomLaunch(game types.Game) bool {
	if game.Source == "" || game.Source == types.SourceCustom {
		return true
	}
	return a.isCustomGameID(game.ID)
}

// prepareCustomLaunch builds the launch for a sideloaded game (issue #47):
// native games run directly with no runner and no Wine prefix (GameMode,
// MangoHud and Gamescope still wrap them), Windows games go through the
// runner as before. The working directory defaults to the binary's folder.
// A .sh script without the execute bit runs through /bin/sh; any other
// native binary without it fails with a clear error instead of a raw exec
// failure (the UI offers to set the bit first).
func (a *App) prepareCustomLaunch(game types.Game, preview bool) (*prepared, error) {
	if !game.Native {
		r, err := a.selectRunner(game)
		if err != nil {
			return nil, err
		}
		var args []string
		var env map[string]string
		if runner.IsUmu(r) {
			args, env, err = a.umuCommand(game, r, runner.Target{Exe: game.Executable}, preview)
		} else {
			args, env, err = runner.BuildLaunchCommandFor(game, r, preview, a.prefixesFolder(), a.homeDir())
		}
		if err != nil {
			return nil, err
		}
		dir := game.Settings.WorkingDir
		if dir == "" && game.Executable != "" {
			dir = filepath.Dir(game.Executable)
		}
		return &prepared{args: args, env: env, dir: dir, runner: r}, nil
	}
	kind := runner.ExecUnknown
	if d, err := runner.DetectExecutable(game.Executable); err == nil {
		kind = d.Kind
	}
	exec, err := runner.IsExecutable(game.Executable)
	if err != nil {
		return nil, apperr.Wrap("sideload.executable.unreadable",
			fmt.Errorf("executable: %w", err), map[string]string{"executable": game.Executable})
	}
	target := runner.NativeTarget(game.Executable, kind, exec)
	if !exec && target.Exe == game.Executable {
		return nil, apperr.New("sideload.notExecutable",
			fmt.Sprintf("%s is not executable; make it executable or it can't start", game.Executable),
			map[string]string{"executable": game.Executable})
	}
	args, env, err := runner.BuildCommandFor(game, types.ProtonVersion{}, target, preview, a.prefixesFolder(), a.homeDir())
	if err != nil {
		return nil, err
	}
	dir := game.Settings.WorkingDir
	if dir == "" && game.Executable != "" {
		dir = filepath.Dir(game.Executable)
	}
	return &prepared{args: args, env: env, dir: dir}, nil
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
		return "", apperr.New("game.running", fmt.Sprintf("%s is already running", game.Title), map[string]string{"title": game.Title})
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
		return "", apperr.New("game.noLaunchTarget", fmt.Sprintf("game %q has no launch target", game.Title), map[string]string{"title": game.Title})
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
	// The session hooks run in order: the cloud push first, the save backup
	// second (issue #42; the backup waits, bounded, while a cloud job runs).
	opts = append(opts, a.afterSessionHook(game))
	pid, err := a.sessions.StartDirect(game.ID, cmd, p.shown(), p.redact, opts...)
	if err != nil {
		if errors.Is(err, session.ErrAlreadyRunning) {
			// Never abandon here: the record Launching wrote may belong to the game that runs.
			return "", apperr.New("game.running", fmt.Sprintf("%s is already running", game.Title), map[string]string{"title": game.Title})
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
// The after-session backup hook runs when the client-tracked session ends,
// like for direct launches.
func (a *App) startClient(game types.Game, cmd *exec.Cmd, cl *stores.ClientLaunch) (string, error) {
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("%s client: %w", strings.ToLower(cl.Client), err)
	}
	go func() { _ = cmd.Wait() }()
	if !a.sessions.WatchClient(game.ID, cl.Client, cl.Find, cl.StartTimeout, a.afterSessionHook(game)) {
		return "", apperr.New("game.running", fmt.Sprintf("%s is already running", game.Title), map[string]string{"title": game.Title})
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
		return apperr.New("game.notRunning", "that game is not running", nil)
	}
	return err
}

func (a *App) launcherSettings() types.LauncherSettings {
	return types.LauncherSettings{
		GamesFolder:           a.settings.GamesFolder(),
		DefaultGamesFolder:    settings.DefaultGamesFolderFor(a.homeDir()),
		CancelCleanupDays:     a.settings.CancelCleanupDays(),
		RatingsOnline:         a.settings.RatingsOnline(),
		ArtworkOnline:         a.settings.ArtworkOnline(),
		GameInfoOnline:        a.settings.GameInfoOnline(),
		ChatDesktopNotify:     a.settings.ChatDesktopNotify(),
		PrefixesFolder:        a.settings.PrefixesFolder(),
		DefaultPrefixesFolder: settings.DefaultPrefixesFolderFor(a.resolvedDataDir()),
		Language:              a.settings.Language(),
		MinimizeToTray:        a.settings.MinimizeToTray(),
	}
}

// SetPrefixesFolder saves where Wine prefixes are created ("" resets it to
// the default). Games whose prefix already exists in the old folder keep it:
// their wine_prefix is pinned to the existing path, so changing the folder
// never strands their saves. Nothing is moved or deleted.
func (a *App) SetPrefixesFolder(path string) (types.LauncherSettings, error) {
	old := a.settings.PrefixesFolder()
	if old == "" {
		old = settings.DefaultPrefixesFolderFor(a.resolvedDataDir())
	}
	if err := a.pinExistingPrefixes(old); err != nil {
		return a.launcherSettings(), err
	}
	if _, err := a.settings.SetPrefixesFolder(path); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}

// pinExistingPrefixes pins every game's prefix that already exists as a
// folder of the old effective prefixes folder: a game with no wine_prefix of
// its own gets one pointing at the existing path. The pins are ordinary
// per-game settings, so they show in that game's settings. Folders that are
// not games cannot be told apart, so every subfolder counts; hidden names and
// non-folders are ignored. Nothing is moved or deleted. A missing old folder
// means there is nothing to keep.
func (a *App) pinExistingPrefixes(old string) error {
	entries, err := os.ReadDir(old)
	if err != nil {
		return nil
	}
	saved := a.settings.GameSettings()
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		id := e.Name()
		if cur, ok := saved[id]["wine_prefix"]; ok {
			if s, _ := cur.(string); strings.TrimSpace(s) != "" {
				continue // the game already has a prefix of its own
			}
		}
		next := maps.Clone(saved[id])
		if next == nil {
			next = map[string]any{}
		}
		next["wine_prefix"] = filepath.Join(old, id)
		if err := a.settings.SetGameSettings(id, next); err != nil {
			return err
		}
	}
	return nil
}

// prefixesFolder is where Wine prefixes are created: the user's choice, or
// "" for the default (see runner.PrefixPath).
func (a *App) prefixesFolder() string {
	if a.settings == nil {
		return ""
	}
	return a.settings.PrefixesFolder()
}

// wineGames returns every game Platypus launches through Wine, as launches
// see them: the sideloaded games plus the installed games of the current
// library, with each game's wine_prefix pin from the settings overlaid (the
// cached listings don't carry it). Callers apply their own skips: native
// games and store-client launches have no Platypus prefix (issue #52).
func (a *App) wineGames() []types.Game {
	out := a.customGames()
	var saved map[string]map[string]any
	if a.settings != nil {
		saved = a.settings.GameSettings()
	}
	a.library.mu.Lock()
	entries := make([]libraryCacheEntry, 0, len(a.library.entries))
	for _, e := range a.library.entries {
		entries = append(entries, e)
	}
	a.library.mu.Unlock()
	for _, e := range entries {
		for _, g := range e.games {
			if !g.IsInstalled {
				continue
			}
			if m, ok := saved[g.ID]; ok {
				if s, _ := m["wine_prefix"].(string); strings.TrimSpace(s) != "" {
					g.Settings.WinePrefix = s
				}
			}
			out = append(out, g)
		}
	}
	return out
}

// PrefixUsage groups the Wine games by the prefix they resolve to (issue
// #52): the picker lists these, with the games sharing each prefix. Native
// games have no prefix and are left out, as are games their store client
// launches itself (Steam: steam.IsClientLaunch — the client owns their
// compat data, not Platypus).
func (a *App) PrefixUsage() []types.PrefixShare {
	home := a.homeDir()
	folder := a.effectivePrefixesFolder()
	byPath := map[string][]types.PrefixGame{}
	for _, g := range a.wineGames() {
		if g.Native || steam.IsClientLaunch(g) {
			continue
		}
		p := runner.PrefixPath(g, home, folder)
		byPath[p] = append(byPath[p], types.PrefixGame{ID: g.ID, Title: g.Title})
	}
	out := make([]types.PrefixShare, 0, len(byPath))
	for path, games := range byPath {
		sort.Slice(games, func(i, j int) bool { return games[i].ID < games[j].ID })
		out = append(out, types.PrefixShare{Path: path, Games: games})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// SetLanguage saves the UI language ("" for System, issue #39). When the
// effective locale changes it notifies plugins that speak protocol 1.14 and
// drops the localized store-data caches, so the next game page fetches in
// the new language.
func (a *App) SetLanguage(lang string) (types.LauncherSettings, error) {
	before := a.Locale()
	if err := a.settings.SetLanguage(lang); err != nil {
		return a.launcherSettings(), err
	}
	if after := a.Locale(); after != before {
		a.applyLocaleChange(after)
	}
	return a.launcherSettings(), nil
}

// GetLauncherSettings returns Platypus-wide settings.
func (a *App) GetLauncherSettings() types.LauncherSettings {
	return a.launcherSettings()
}

func (a *App) chooseFolder(title, start string) (string, error) {
	if _, err := os.Stat(start); err != nil {
		start = a.homeDir()
	}
	return a.host.OpenDirectory(DirectoryDialog{Title: title, Default: start, CanCreate: true})
}

// GetGameSettings returns every saved per-game settings object by game ID.
func (a *App) GetGameSettings() map[string]map[string]any {
	return a.settings.GameSettings()
}

// cleanGameSettingsPaths validates the path fields of per-game settings and
// stores them cleaned: working_dir must be an absolute path to an existing
// directory when set, wine_prefix an absolute path (it is created on demand,
// so it need not exist yet, but it must not be a file). "~" is expanded and
// the result Cleaned, so "~/x" is stored as the full path. Empty means the
// fallback. It mutates gameSettings in place.
func (a *App) cleanGameSettingsPaths(gameSettings map[string]any) error {
	home := a.homeDir()
	if v, ok := gameSettings["working_dir"]; ok {
		if s, _ := v.(string); strings.TrimSpace(s) != "" {
			clean, err := settings.CleanFolderFor(s, home)
			if err != nil {
				return fmt.Errorf("working directory: %w", err)
			}
			st, err := os.Stat(clean)
			if err != nil {
				return fmt.Errorf("working directory: %w", err)
			}
			if !st.IsDir() {
				return apperr.New("settings.workdir.notFolder", fmt.Sprintf("%s is a file, not a folder", clean), map[string]string{"path": clean})
			}
			gameSettings["working_dir"] = clean
		}
	}
	if v, ok := gameSettings["wine_prefix"]; ok {
		if s, _ := v.(string); strings.TrimSpace(s) != "" {
			clean, err := settings.CleanFolderFor(s, home)
			if err != nil {
				return fmt.Errorf("wine prefix: %w", err)
			}
			gameSettings["wine_prefix"] = clean
		}
	}
	if v, ok := gameSettings["run_mode"]; ok {
		if s, _ := v.(string); strings.TrimSpace(s) != "" && !runner.ValidRunMode(s) {
			return apperr.New("settings.runMode.invalid",
				fmt.Sprintf("run mode must be auto, native or wine, got %q", s),
				map[string]string{"value": s})
		}
		if s, _ := v.(string); strings.TrimSpace(s) == "" {
			delete(gameSettings, "run_mode")
		} else if s, _ := v.(string); strings.EqualFold(strings.TrimSpace(s), "auto") {
			gameSettings["run_mode"] = "auto"
		} else {
			gameSettings["run_mode"] = strings.ToLower(strings.TrimSpace(s))
		}
	}
	return nil
}

// SaveGameSettings persists one game's settings as the UI holds them.
func (a *App) SaveGameSettings(gameID string, gameSettings map[string]any) error {
	if err := a.cleanGameSettingsPaths(gameSettings); err != nil {
		return err
	}
	return a.settings.SetGameSettings(gameID, gameSettings)
}
