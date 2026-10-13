package app

import (
	"context"

	"platypus/internal/types"
)

// Save backups (issue #42): the gate that backs up saves before a
// destructive action deletes them. #43 creates this file with a stub, which
// #42 replaces with the real gate; the removal job already calls it, so a
// failed backup deletes nothing.
//
// backupGateForTests overrides the gate in tests (nil in production): it
// lets the removal tests fail the gate and check that nothing was deleted,
// without implementing backups here.
func (a *App) backupBeforeDelete(ctx context.Context, game types.Game, trigger string, allowNoSaves bool) (types.SaveGate, error) {
	if a != nil && a.backupGateForTests != nil {
		return a.backupGateForTests(ctx, game, trigger, allowNoSaves)
	}
	return types.SaveGate{}, nil
}
