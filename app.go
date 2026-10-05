package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"hydra-launcher/internal/fixes"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/runner"
	"hydra-launcher/internal/steam"
	"hydra-launcher/internal/types"
)

// App struct
type App struct {
	ctx           context.Context
	pluginManager *plugins.Manager
	fixesService  *fixes.Service

	mu         sync.Mutex
	pendingCmd *exec.Cmd
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		pluginManager: plugins.NewManager(),
		fixesService:  fixes.NewService(),
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Reap launched game processes so they don't linger, and stop the
	// most recent game when the app (and its lifecycle context) shuts down.
	go func() {
		<-ctx.Done()
		a.mu.Lock()
		cmd := a.pendingCmd
		a.mu.Unlock()
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()
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

// selectRunner resolves the configured runner for a game, falling back to the
// first available runner only when nothing matches and only for runners that
// exist on the system.
func selectRunner(game types.Game) (types.ProtonVersion, bool) {
	runners := runner.ScanProtonVersions()
	for _, r := range runners {
		if r.ID == game.Settings.RunnerID || r.Name == game.Settings.RunnerID {
			return r, true
		}
	}
	// Only fall back if the game never had a runner configured.
	if game.Settings.RunnerID == "" && len(runners) > 0 {
		return runners[0], true
	}
	return types.ProtonVersion{}, false
}

// PreviewLaunchCommand returns the exact command line that will be used.
func (a *App) PreviewLaunchCommand(game types.Game) (string, error) {
	selectedRunner, ok := selectRunner(game)
	if !ok {
		return "", fmt.Errorf("runner %q not found", game.Settings.RunnerID)
	}
	cmdArgs, _, err := runner.BuildLaunchCommand(game, selectedRunner)
	if err != nil {
		return "", err
	}
	return strings.Join(cmdArgs, " "), nil
}

// LaunchGame executes the game command asynchronously.
func (a *App) LaunchGame(game types.Game) (string, error) {
	selectedRunner, ok := selectRunner(game)
	if !ok {
		return "", fmt.Errorf("runner %q not found", game.Settings.RunnerID)
	}

	cmdArgs, env, err := runner.BuildLaunchCommand(game, selectedRunner)
	if err != nil {
		return "", err
	}
	// Filter empty argv entries (e.g. a missing executable must abort, not exec "").
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
	for k, v := range env {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("%s=%s", k, v))
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	// Track the pending process: reap it on exit to avoid zombies, and keep a
	// handle so shutdown can stop it.
	a.mu.Lock()
	a.pendingCmd = cmd
	a.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		a.mu.Lock()
		if a.pendingCmd == cmd {
			a.pendingCmd = nil
		}
		a.mu.Unlock()
	}()

	return fmt.Sprintf("Game launched with PID %d", cmd.Process.Pid), nil
}

// ListPlugins returns installed plugins.
func (a *App) ListPlugins() []types.PluginInfo {
	return a.pluginManager.ListPlugins()
}

// TogglePlugin toggles a plugin state.
func (a *App) TogglePlugin(id string, enabled bool) bool {
	return a.pluginManager.TogglePlugin(id, enabled)
}
