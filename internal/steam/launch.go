package steam

import (
	"os/exec"
	"strings"

	"hydra-launcher/internal/types"
)

// IsClientLaunch reports whether the game must be started by the Steam client
// (an installed Steam app with no executable override).
func IsClientLaunch(game types.Game) bool {
	return game.Store == types.StoreSteam && game.Executable == "" && game.StoreAppID != ""
}

// Command asks the Steam client to start the app. The client picks the
// Proton version and applies the game's Launch Options itself, so no runner or
// wrapper may be prepended here: `proton run steam` or `gamemoderun steam`
// would wrap the client, not the game.
func Command(game types.Game) []string {
	var args []string
	switch {
	case lookPath("steam"):
		args = []string{"steam", "-applaunch", game.StoreAppID}
	case flatpakSteam():
		args = []string{"flatpak", "run", "com.valvesoftware.Steam", "-applaunch", game.StoreAppID}
	default:
		// The steam:// handler can't forward launch arguments.
		return []string{"xdg-open", "steam://rungameid/" + game.StoreAppID}
	}
	if game.Settings.LaunchArguments != "" {
		args = append(args, strings.Fields(game.Settings.LaunchArguments)...)
	}
	return args
}

func lookPath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

func flatpakSteam() bool {
	if !lookPath("flatpak") {
		return false
	}
	return exec.Command("flatpak", "info", "com.valvesoftware.Steam").Run() == nil
}
