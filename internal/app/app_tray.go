package app

import (
	"platypus/internal/types"
)

// SetMinimizeToTray saves the "Minimize to tray while a game runs" switch
// (issue #49), off by default. The shell reads it and hides the window to
// the tray while a game session runs.
func (a *App) SetMinimizeToTray(on bool) (types.LauncherSettings, error) {
	if err := a.settings.SetMinimizeToTray(on); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}
