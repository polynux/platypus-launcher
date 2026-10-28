package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"platypus/internal/datadir"
	"platypus/internal/types"
)

// Target is what the pipeline runs: the game's own executable, or what a
// store plugin's launch spec asked for.
type Target struct {
	Exe    string
	Args   []string          // before the user's launch arguments
	Env    map[string]string // below Platypus's and the user's env
	Native bool              // a Linux binary: no runner, no prefix
}

// BuildLaunchCommand runs the game's configured executable through the
// pipeline (see BuildCommand).
func BuildLaunchCommand(game types.Game, runner types.ProtonVersion, preview bool, prefixesFolder string) ([]string, map[string]string, error) {
	return BuildCommand(game, runner, Target{Exe: game.Executable}, preview, prefixesFolder)
}

// BuildLaunchCommandFor is BuildLaunchCommand with an explicit home (issue #41).
func BuildLaunchCommandFor(game types.Game, runner types.ProtonVersion, preview bool, prefixesFolder, home string) ([]string, map[string]string, error) {
	return BuildCommandFor(game, runner, Target{Exe: game.Executable}, preview, prefixesFolder, home)
}

// PrefixPath is the game's Wine prefix (Proton's compat data folder): the one
// the user set for it, else a folder of its own under the prefixes folder
// (the user's choice, or Platypus's data when unset).
func PrefixPath(game types.Game, home, prefixesFolder string) string {
	if game.Settings.WinePrefix != "" {
		return game.Settings.WinePrefix
	}
	if prefixesFolder != "" {
		return filepath.Join(prefixesFolder, game.ID)
	}
	return filepath.Join(datadir.Path(home), "prefixes", game.ID)
}

// FreshMarker is the file ResetPrefix leaves in a recreated prefix: its
// modification time says when the prefix was recreated. prefixDamaged treats
// a marker newer than the last local session as a fresh prefix awaiting its
// first run, not damage.
const FreshMarker = ".platypus-fresh"

// PrefixDamaged reports whether a Wine prefix's layout looks broken (issue
// #144): no Windows side, neither drive_c nor pfx/drive_c, as a failed
// creation leaves it, or a file blocks the path. A missing folder is not
// damage: the launch creates it. Freshness (a prefix Platypus just recreated,
// see FreshMarker) is the caller's call: it needs the play history.
func PrefixDamaged(prefixPath string) bool {
	if st, err := os.Stat(prefixPath); err != nil || !st.IsDir() {
		return err == nil
	}
	if st, err := os.Stat(filepath.Join(prefixPath, "drive_c")); err == nil && st.IsDir() {
		return false
	}
	if st, err := os.Stat(filepath.Join(prefixPath, "pfx", "drive_c")); err == nil && st.IsDir() {
		return false
	}
	return true
}

// BuildCommand constructs, for games Platypus starts itself (see
// steam.IsClientLaunch for the other case), the full Linux gaming command pipeline (gamescope, mangohud, gamemode, proton/wine).
// When preview is true, no on-disk state is created: the Proton prefix is
// resolved (so the preview is honest) but not created (CORE-8).
func BuildCommand(game types.Game, runner types.ProtonVersion, target Target, preview bool, prefixesFolder string) ([]string, map[string]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	return BuildCommandFor(game, runner, target, preview, prefixesFolder, home)
}

// BuildCommandFor is BuildCommand with an explicit home (issue #41: the App
// passes its configured Home so tests need no process environment).
func BuildCommandFor(game types.Game, runner types.ProtonVersion, target Target, preview bool, prefixesFolder, home string) ([]string, map[string]string, error) {
	var cmdArgs []string
	env := make(map[string]string)
	for k, v := range target.Env {
		env[k] = v
	}

	// 1. Inherit or set environment variables
	if home == "" {
		return nil, nil, fmt.Errorf("cannot determine home directory")
	}
	prefixPath := PrefixPath(game, home, prefixesFolder)

	if !target.Native {
		// Proton assumes Steam created the compat data dir; it crashes with a
		// FileNotFoundError on <prefix>/pfx.lock when the folder is missing.
		// Previews resolve the path without creating it (CORE-8).
		if !preview {
			if err := os.MkdirAll(prefixPath, 0o755); err != nil {
				return nil, nil, fmt.Errorf("cannot create Proton prefix %s: %w", prefixPath, err)
			}
		}
		env["WINEPREFIX"] = prefixPath
		env["STEAM_COMPAT_DATA_PATH"] = prefixPath
		env["STEAM_COMPAT_CLIENT_INSTALL_PATH"] = filepath.Join(home, ".local/share/Steam")
	}

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
	if runner.Path != "" && !target.Native {
		if strings.HasSuffix(runner.Path, "proton") {
			cmdArgs = append(cmdArgs, runner.Path, "run")
		} else {
			cmdArgs = append(cmdArgs, runner.Path)
		}
	}

	// 4. Executable target
	if target.Exe == "" {
		return nil, nil, fmt.Errorf("game %q has no executable configured", game.Title)
	}
	cmdArgs = append(cmdArgs, target.Exe)
	cmdArgs = append(cmdArgs, target.Args...)

	// 5. Game launch arguments
	if game.Settings.LaunchArguments != "" {
		cmdArgs = append(cmdArgs, strings.Fields(game.Settings.LaunchArguments)...)
	}

	return cmdArgs, env, nil
}
