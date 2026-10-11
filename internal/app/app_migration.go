package app

import (
	"strings"

	"platypus/internal/migrate"
	"platypus/internal/types"
)

// MigrationNotice is the one-time notice shown after the Hydra→Platypus data
// move (issue #14): what happened (the data folder moved from the old Hydra
// name) and what to do (reinstall and re-approve the plugin builds that were
// set aside). It returns nil when there is nothing to show: no migration ran,
// or the user already dismissed it. DismissMigrationNotice persists the
// dismissal so the notice never returns.
func (a *App) MigrationNotice() *types.MigrationNotice {
	if a.settings != nil && a.settings.MigrationNoticeDismissed() {
		return nil
	}
	return migrationNotice(a.migration)
}

// DismissMigrationNotice persists the dismissal of the one-time migration
// notice (in settings.json), so it never returns: not after dismissal, and
// not on later launches where no migration ran (those have nothing to show
// anyway).
func (a *App) DismissMigrationNotice() error {
	if a.settings == nil {
		return nil
	}
	return a.settings.SetMigrationNoticeDismissed(true)
}

// migrationNotice maps a migrate Report to the UI's notice. Nil means nothing
// to show: no move happened and nothing warns. Warnings carry the report's
// failures (failed copies, paths not rewritten) in plain words.
func migrationNotice(rep migrate.Report) *types.MigrationNotice {
	if !rep.Moved && !rep.Copied && len(rep.SetAside) == 0 && rep.Err == nil {
		return nil
	}
	n := &types.MigrationNotice{
		Moved:    rep.Moved,
		Copied:   rep.Copied,
		SetAside: append([]string{}, rep.SetAside...),
	}
	if n.SetAside == nil {
		n.SetAside = []string{}
	}
	if rep.Err != nil {
		for _, e := range strings.Split(rep.Err.Error(), "\n") {
			if e = strings.TrimSpace(e); e != "" {
				n.Warnings = append(n.Warnings, "Something did not finish ("+e+"). It will be retried at the next start.")
			}
		}
	}
	if n.Warnings == nil {
		n.Warnings = []string{}
	}
	return n
}
