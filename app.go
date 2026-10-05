package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"hydra-launcher/internal/fixes"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/runner"
	"hydra-launcher/internal/session"
	"hydra-launcher/internal/settings"
	"hydra-launcher/internal/stores"
	"hydra-launcher/internal/types"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx          context.Context
	catalog      *plugins.Catalog
	plugins      *plugins.Manager
	settings     *settings.Store
	fixesService *fixes.Service
	stores       *stores.Registry
	sessions     *session.Tracker

	tweaksOnce sync.Once
	tweaks     []types.Tweak
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		catalog:      plugins.NewCatalog(),
		fixesService: fixes.NewService(),
		stores:       stores.NewRegistry(stores.NewSteam()),
	}
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

	// Store plugins are discovered here but only start when something calls
	// them; nothing does until the Epic plugin (milestone 3).
	a.plugins = plugins.NewManager(filepath.Join(dataDir, "plugins"), filepath.Join(dataDir, "logs"), a.settings, nil)
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
}

// shutdown stops games Hydra started directly and every store plugin.
// Steam-launched games belong to the Steam client and keep running.
func (a *App) shutdown(ctx context.Context) {
	if a.sessions != nil {
		a.sessions.StopAll()
	}
	if a.plugins != nil {
		a.plugins.Shutdown()
	}
}

// GetProtonVersions returns all discovered Proton, GE-Proton, Wine runners on the system.
func (a *App) GetProtonVersions() []types.ProtonVersion {
	return runner.ScanProtonVersions()
}

// GetGames scans every store and returns the unified library.
func (a *App) GetGames() ([]types.Game, error) {
	var allGames []types.Game
	for _, s := range a.stores.All() {
		games, err := s.Library()
		if err != nil {
			return nil, fmt.Errorf("%s library scan failed: %w", s.ID(), err)
		}
		allGames = append(allGames, games...)
	}
	return allGames, nil
}

// GetGameFix fetches live ProtonDB or curated Lutris fix data for a game.
func (a *App) GetGameFix(storeAppID string, title string) *types.GameFixInfo {
	if fix := a.fixesService.GetCuratedFix(title); fix != nil {
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
	s, ok := a.stores.Get(game.Store)
	if !ok {
		return &types.GameDetails{}, nil
	}
	return s.Details(game)
}

// GetTweaks returns hardware-based tweaks offered for every game.
func (a *App) GetTweaks() []types.Tweak {
	a.tweaksOnce.Do(func() { a.tweaks = fixes.SuggestTweaks(fixes.DetectSystem()) })
	return a.tweaks
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

// clientLaunch returns the store-client launch for the game, or nil when Hydra
// starts the game itself.
func (a *App) clientLaunch(game types.Game) *stores.ClientLaunch {
	if s, ok := a.stores.Get(game.Store); ok {
		return s.ClientLaunch(game)
	}
	return nil
}

func (a *App) launchCommand(game types.Game) ([]string, map[string]string, error) {
	if cl := a.clientLaunch(game); cl != nil {
		return cl.Command, map[string]string{}, nil
	}
	r, err := selectRunner(game)
	if err != nil {
		return nil, nil, err
	}
	return runner.BuildLaunchCommand(game, r)
}

// PreviewLaunchCommand returns the exact command line that will be used.
func (a *App) PreviewLaunchCommand(game types.Game) (string, error) {
	cmdArgs, env, err := a.launchCommand(game)
	if err != nil {
		return "", err
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys)+len(cmdArgs))
	for _, k := range keys {
		parts = append(parts, k+"="+env[k])
	}
	return strings.Join(append(parts, cmdArgs...), " "), nil
}

// LaunchGame starts the game. Progress (running, stopped, failed) arrives on
// the "session" event; the returned string is only a status line.
func (a *App) LaunchGame(game types.Game) (string, error) {
	if a.sessions.Running(game.ID) {
		return "", fmt.Errorf("%s is already running", game.Title)
	}
	cmdArgs, env, err := a.launchCommand(game)
	if err != nil {
		return "", err
	}
	filtered := make([]string, 0, len(cmdArgs))
	for _, arg := range cmdArgs {
		if strings.TrimSpace(arg) != "" {
			filtered = append(filtered, arg)
		}
	}
	if len(filtered) == 0 {
		return "", fmt.Errorf("game %q has no launch target", game.Title)
	}

	cmd := exec.Command(filtered[0], filtered[1:]...)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if game.Settings.WorkingDir != "" {
		cmd.Dir = game.Settings.WorkingDir
	} else if game.Executable != "" {
		cmd.Dir = filepath.Dir(game.Executable)
	}

	if cl := a.clientLaunch(game); cl != nil {
		// With the client running, the command forwards the request and
		// exits; if not, it becomes the client and keeps running. Either way,
		// follow the game itself (Steam: its reaper process), not this PID.
		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("%s client: %w", strings.ToLower(cl.Client), err)
		}
		go func() { _ = cmd.Wait() }()
		a.sessions.WatchClient(game.ID, cl.Client, cl.Find)
		return "Asked " + cl.Client + " to start " + game.Title, nil
	}
	pid, err := a.sessions.StartDirect(game.ID, cmd)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Started %s (PID %d)", game.Title, pid), nil
}

// ListPlugins returns the Plugins view's entries (still the static catalog).
func (a *App) ListPlugins() []types.PluginInfo {
	return a.catalog.ListPlugins()
}

// TogglePlugin toggles a plugin state.
func (a *App) TogglePlugin(id string, enabled bool) bool {
	return a.catalog.TogglePlugin(id, enabled)
}

func (a *App) launcherSettings() types.LauncherSettings {
	return types.LauncherSettings{
		GamesFolder:        a.settings.GamesFolder(),
		DefaultGamesFolder: settings.DefaultGamesFolder(),
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
	start := a.settings.GamesFolder()
	if _, err := os.Stat(start); err != nil {
		start, _ = os.UserHomeDir()
	}
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:                "Games folder",
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
