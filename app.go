package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

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
	cachedGames   []types.Game
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
}

// GetProtonVersions returns all discovered Proton, GE-Proton, Wine runners on the system.
func (a *App) GetProtonVersions() []types.ProtonVersion {
	return runner.ScanProtonVersions()
}

// GetGames scans all configured sources (Steam, Custom, etc.) and returns the unified library.
func (a *App) GetGames() []types.Game {
	var allGames []types.Game

	// Scan Steam library if enabled
	steamGames, err := steam.ScanSteamLibrary()
	if err == nil {
		allGames = append(allGames, steamGames...)
	}

	// Add sample custom/sideloaded games to demonstrate multi-store & fixes
	allGames = append(allGames,
		types.Game{
			ID:          "gog_witcher1",
			Store:       types.StoreGOG,
			StoreAppID:  "1207658924",
			Title:       "The Witcher: Enhanced Edition",
			CoverURL:    "https://images.igdb.com/igdb/image/upload/t_cover_big/co1r7h.jpg",
			BannerURL:   "https://images.igdb.com/igdb/image/upload/t_screenshot_big/sc8b8c.jpg",
			InstallDir:  "/home/polynux/Games/GOG/The Witcher",
			Executable:  "system/witcher.exe",
			IsInstalled: true,
			Settings: types.GameSettings{
				RunnerID:    "compat:GE-Proton11-3",
				UseGameMode: true,
			},
			FixInfo: a.fixesService.GetCuratedFix("The Witcher: Enhanced Edition"),
		},
		types.Game{
			ID:          "epic_cyberpunk",
			Store:       types.StoreEpic,
			StoreAppID:  "Ginger",
			Title:       "Cyberpunk 2077",
			CoverURL:    "https://images.igdb.com/igdb/image/upload/t_cover_big/co2m35.jpg",
			BannerURL:   "https://images.igdb.com/igdb/image/upload/t_screenshot_big/sc69n2.jpg",
			InstallDir:  "/home/polynux/Games/Epic/Cyberpunk2077",
			Executable:  "bin/x64/Cyberpunk2077.exe",
			IsInstalled: true,
			Settings: types.GameSettings{
				RunnerID:     "steam:Proton - Experimental",
				UseGameMode:  true,
				UseMangoHud:  true,
				UseGamescope: true,
				GamescopeW:   2560,
				GamescopeH:   1440,
				GamescopeR:   144,
			},
		},
		types.Game{
			ID:          "custom_fallout_london",
			Store:       types.StoreCustom,
			StoreAppID:  "fallout_london",
			Title:       "Fallout: London",
			CoverURL:    "https://images.igdb.com/igdb/image/upload/t_cover_big/co8fgv.jpg",
			BannerURL:   "https://images.igdb.com/igdb/image/upload/t_screenshot_big/scfp1a.jpg",
			InstallDir:  "/home/polynux/Games/Fallout London",
			Executable:  "f4se_loader.exe",
			IsInstalled: true,
			Settings: types.GameSettings{
				RunnerID:    "compat:GE-Proton11-3",
				UseGameMode: true,
			},
			FixInfo: a.fixesService.GetCuratedFix("Fallout: London"),
		},
	)

	a.cachedGames = allGames
	return allGames
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

// PreviewLaunchCommand returns the exact command line and environment variables that will be used.
func (a *App) PreviewLaunchCommand(game types.Game) (string, map[string]string) {
	runners := runner.ScanProtonVersions()
	var selectedRunner types.ProtonVersion

	for _, r := range runners {
		if r.ID == game.Settings.RunnerID || r.Name == game.Settings.RunnerID {
			selectedRunner = r
			break
		}
	}
	if selectedRunner.Path == "" && len(runners) > 0 {
		selectedRunner = runners[0]
	}

	cmdArgs, env, _ := runner.BuildLaunchCommand(game, selectedRunner)
	return strings.Join(cmdArgs, " "), env
}

// LaunchGame executes the game command asynchronously.
func (a *App) LaunchGame(game types.Game) (string, error) {
	runners := runner.ScanProtonVersions()
	var selectedRunner types.ProtonVersion

	for _, r := range runners {
		if r.ID == game.Settings.RunnerID || r.Name == game.Settings.RunnerID {
			selectedRunner = r
			break
		}
	}
	if selectedRunner.Path == "" && len(runners) > 0 {
		selectedRunner = runners[0]
	}

	cmdArgs, env, err := runner.BuildLaunchCommand(game, selectedRunner)
	if err != nil {
		return "", err
	}
	if len(cmdArgs) == 0 {
		return "", fmt.Errorf("no command built")
	}

	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	for k, v := range env {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("%s=%s", k, v))
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

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
