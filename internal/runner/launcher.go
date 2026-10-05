package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"hydra-launcher/internal/types"
)

// BuildLaunchCommand constructs, for games Hydra starts itself (see
// steam.IsClientLaunch for the other case), the full Linux gaming command pipeline (gamescope, mangohud, gamemode, proton/wine).
func BuildLaunchCommand(game types.Game, runner types.ProtonVersion) ([]string, map[string]string, error) {
	var cmdArgs []string
	env := make(map[string]string)

	// 1. Inherit or set environment variables
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	prefixPath := game.Settings.WinePrefix
	if prefixPath == "" {
		prefixPath = filepath.Join(home, ".local/share/hydra/prefixes", game.ID)
	}

	env["WINEPREFIX"] = prefixPath
	env["STEAM_COMPAT_DATA_PATH"] = prefixPath
	env["STEAM_COMPAT_CLIENT_INSTALL_PATH"] = filepath.Join(home, ".local/share/Steam")

	// Anti-cheat runtime bindings
	if game.Settings.EACRuntime {
		eacPath := filepath.Join(home, ".local/share/Steam/steamapps/common/Proton EasyAntiCheat Runtime")
		if _, err := os.Stat(eacPath); err == nil {
			env["PROTON_EAC_RUNTIME"] = eacPath
		}
	}
	if game.Settings.BattlEyeRuntime {
		bePath := filepath.Join(home, ".local/share/Steam/steamapps/common/Proton BattlEye Runtime")
		if _, err := os.Stat(bePath); err == nil {
			env["PROTON_BATTLEYE_RUNTIME"] = bePath
		}
	}

	// Custom user env vars
	for k, v := range game.Settings.EnvVars {
		env[k] = v
	}

	// 2. Build Wrapper Stack: gamemoderun -> gamescope -> mangohud -> runner -> executable
	if game.Settings.UseGameMode {
		if _, err := exec.LookPath("gamemoderun"); err == nil {
			cmdArgs = append(cmdArgs, "gamemoderun")
		}
	}

	if game.Settings.UseGamescope {
		if _, err := exec.LookPath("gamescope"); err == nil {
			cmdArgs = append(cmdArgs, "gamescope")
			if game.Settings.GamescopeW > 0 {
				cmdArgs = append(cmdArgs, "-w", fmt.Sprintf("%d", game.Settings.GamescopeW))
			}
			if game.Settings.GamescopeH > 0 {
				cmdArgs = append(cmdArgs, "-h", fmt.Sprintf("%d", game.Settings.GamescopeH))
			}
			if game.Settings.GamescopeR > 0 {
				cmdArgs = append(cmdArgs, "-r", fmt.Sprintf("%d", game.Settings.GamescopeR))
			}
			if game.Settings.GamescopeArgs != "" {
				cmdArgs = append(cmdArgs, strings.Fields(game.Settings.GamescopeArgs)...)
			}
			cmdArgs = append(cmdArgs, "--")
		}
	}

	if game.Settings.UseMangoHud {
		if _, err := exec.LookPath("mangohud"); err == nil {
			cmdArgs = append(cmdArgs, "mangohud")
		}
	}

	// 3. Runner invocation (Proton vs Wine vs Native)
	if runner.Path != "" {
		if strings.HasSuffix(runner.Path, "proton") {
			cmdArgs = append(cmdArgs, runner.Path, "run")
		} else {
			cmdArgs = append(cmdArgs, runner.Path)
		}
	}

	// 4. Executable target
	if game.Executable == "" {
		return nil, nil, fmt.Errorf("game %q has no executable configured", game.Title)
	}
	cmdArgs = append(cmdArgs, game.Executable)

	// 5. Game launch arguments
	if game.Settings.LaunchArguments != "" {
		cmdArgs = append(cmdArgs, strings.Fields(game.Settings.LaunchArguments)...)
	}

	return cmdArgs, env, nil
}
