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
	"hydra-launcher/internal/steam"
	"hydra-launcher/internal/types"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx           context.Context
	pluginManager *plugins.Manager
	fixesService  *fixes.Service
	store         *steam.StoreClient
	sessions      *session.Tracker

	tweaksOnce sync.Once
	tweaks     []types.Tweak
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		pluginManager: plugins.NewManager(),
		fixesService:  fixes.NewService(),
		store:         steam.NewStoreClient(),
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.sessions = session.NewTracker(func(e types.SessionEvent) {
		wruntime.EventsEmit(ctx, "session", e)
	})
}

// shutdown stops games Hydra started directly. Steam-launched games belong to
// the Steam client and keep running.
func (a *App) shutdown(ctx context.Context) {
	if a.sessions != nil {
		a.sessions.StopAll()
	}
}

// GetProtonVersions returns all discovered Proton, GE-Proton, Wine runners on the system.
func (a *App) GetProtonVersions() []types.ProtonVersion {
	return runner.ScanProtonVersions()
}

// GetGames scans all configured sources (Steam, Custom, etc.) and returns the unified library.
func (a *App) GetGames() ([]types.Game, error) {
	var allGames []types.Game

	// Scan Steam library if enabled
	steamGames, err := steam.ScanSteamLibrary()
	if err != nil {
		return nil, fmt.Errorf("steam library scan failed: %w", err)
	}
	allGames = append(allGames, steamGames...)

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
// page. Only Steam titles have a metadata source for now.
func (a *App) GetGameDetails(game types.Game) (*types.GameDetails, error) {
	if game.Store != types.StoreSteam || game.StoreAppID == "" {
		return &types.GameDetails{}, nil
	}
	return a.store.Details(steam.Root(), game.StoreAppID)
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

func launchCommand(game types.Game) ([]string, map[string]string, error) {
	if runner.IsSteamClientLaunch(game) {
		return runner.SteamCommand(game), map[string]string{}, nil
	}
	r, err := selectRunner(game)
	if err != nil {
		return nil, nil, err
	}
	return runner.BuildLaunchCommand(game, r)
}

// PreviewLaunchCommand returns the exact command line that will be used.
func (a *App) PreviewLaunchCommand(game types.Game) (string, error) {
	cmdArgs, env, err := launchCommand(game)
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
	cmdArgs, env, err := launchCommand(game)
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

	if runner.IsSteamClientLaunch(game) {
		// With Steam running, the command forwards the request and exits; if
		// not, it becomes the client and keeps running. Either way, follow the
		// game itself through Steam's reaper process, not this PID.
		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("steam client: %w", err)
		}
		go func() { _ = cmd.Wait() }()
		a.sessions.WatchSteam(game.ID, game.StoreAppID)
		return "Asked Steam to start " + game.Title, nil
	}
	pid, err := a.sessions.StartDirect(game.ID, cmd)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Started %s (PID %d)", game.Title, pid), nil
}

// ListPlugins returns installed plugins.
func (a *App) ListPlugins() []types.PluginInfo {
	return a.pluginManager.ListPlugins()
}

// TogglePlugin toggles a plugin state.
func (a *App) TogglePlugin(id string, enabled bool) bool {
	return a.pluginManager.TogglePlugin(id, enabled)
}
