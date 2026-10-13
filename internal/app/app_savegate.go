package app

import (
	"context"

	"platypus/internal/types"
)

// backupBeforeDelete is the shared seam with #43: before a destructive action
// deletes anything, back up the game's saves (all of them: a superset of what
// the action deletes).
// A failed backup returns an error and the caller deletes nothing. With no
// known save paths it returns NoPaths with a savebackup.noPaths error, unless
// allowNoSaves is set. Helpers live in app_backups.go; this file holds only
// the seam.
//
// backupGateForTests overrides the gate in tests (nil in production): the
// removal tests fail it to check that nothing was deleted.
func (a *App) backupBeforeDelete(ctx context.Context, game types.Game, trigger string, allowNoSaves bool) (types.SaveGate, error) {
	if a != nil && a.backupGateForTests != nil {
		return a.backupGateForTests(ctx, game, trigger, allowNoSaves)
	}
	return a.backupGate(ctx, game, trigger, allowNoSaves)
}
