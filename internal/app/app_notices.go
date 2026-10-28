package app

import (
	"platypus/internal/types"
)

// SetClientNotices saves the "Store client notices" switch (issue #149). Off,
// the App stops reporting what a store's client is doing while it starts a
// game: no "Steam is updating" status, and no poller behind the launch.
func (a *App) SetClientNotices(on bool) (types.LauncherSettings, error) {
	if err := a.settings.SetClientNotices(on); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}
