package app

import (
	"platypus/internal/runner"
	"platypus/internal/types"
)

// umuCommand builds the launch command for a game whose runner resolved to
// umu-launcher (issue #48): umu wraps the auto Proton pick (or UMU-Latest
// when no Proton build is installed) with the game's compat AppID. The
// runner stays the umu entry so cloud saves and stop hooks see the umu
// prefix layout (see cloudContext, backupRunner).
func (a *App) umuCommand(game types.Game, umu types.ProtonVersion, target runner.Target, preview bool) ([]string, map[string]string, error) {
	runners := runner.ScanProtonVersionsFor(a.homeDir())
	proton, _ := runner.UmuProton(runners)
	var compat string
	if a.settings != nil {
		compat = a.settings.CompatAppID(game.ID)
	}
	return runner.BuildUmuCommandFor(game, proton, umu.Path, compat, target, preview, a.prefixesFolder(), a.homeDir())
}
