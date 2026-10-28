package app

import (
	"platypus/internal/types"
)

// SetClientNotices saves the "Store client notices" switch (issue #149). Off,
// the App stops reporting what a store's client is doing while it starts a
// game: the "Steam is updating" status events stop, but the launch still polls
// the client, so a long update keeps its deadline.
func (a *App) SetClientNotices(on bool) (types.LauncherSettings, error) {
	if err := a.settings.SetClientNotices(on); err != nil {
		return a.launcherSettings(), err
	}
	return a.launcherSettings(), nil
}
