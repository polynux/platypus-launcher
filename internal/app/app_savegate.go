package app

import (
	"context"

	"platypus/internal/types"
)

// backupBeforeDelete is the shared seam with #43: before a destructive action
// deletes anything, back up exactly the save paths the action would delete.
// A failed backup returns an error and the caller deletes nothing. With no
// known save paths it returns NoPaths with a savebackup.noPaths error, unless
// allowNoSaves is set. Helpers live in app_backups.go; this file holds only
// the seam.
func (a *App) backupBeforeDelete(ctx context.Context, game types.Game, trigger string, allowNoSaves bool) (types.SaveGate, error) {
	return a.backupGate(ctx, game, trigger, allowNoSaves, "")
}
