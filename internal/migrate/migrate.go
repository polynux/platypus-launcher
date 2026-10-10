// Package migrate brings a user's data over the rename of the project from
// Hydra to Platypus.
//
// Run is called once at startup, before anything opens a data file:
//
//   - ~/.local/share/hydra does not exist: nothing to do (a fresh install, or
//     already migrated).
//   - only the old folder exists: it is renamed to ~/.local/share/platypus
//     (atomic, same filesystem). If the rename fails (another filesystem, a
//     mount point) the folder is copied beside the new name, verified, and
//     only then put in place; the old folder is kept untouched. If the copy
//     fails, the partial copy is removed and the app keeps working in the old
//     folder (Report.Root), so nothing is lost and the next start retries.
//   - both exist: the new one is used, a warning is logged, nothing is
//     merged and nothing is deleted.
//
// Hydra kept everything in that one folder (no XDG config, cache or state
// folders), so there is nothing else to move.
//
// After a move, paths stored inside the old folder are rewritten to the new
// one (paths.go) and plugin builds that still speak the old environment
// variables are set aside (plugins.go). Both steps are driven by a marker file
// that travels with the folder, so a crash between the move and the rewrite is
// finished at the next start.
package migrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"platypus/internal/datadir"
)

const (
	// legacyName is the previous data folder's name under ~/.local/share.
	legacyName = "hydra"
	// pendingMarker lives in the data folder from just before the move until
	// the follow-up steps are done.
	pendingMarker = ".platypus-migration-pending"
	// keptNote is left in the old folder after a copy, so later starts know
	// why two folders exist.
	keptNote = ".migrated-to-platypus"
)

// Options are the log hooks and a test seam.
type Options struct {
	Info func(format string, args ...any)
	Warn func(format string, args ...any)

	// rename stands in for os.Rename in tests (to force the cross-filesystem
	// fallback).
	rename func(oldpath, newpath string) error
}

// Report says what Run did.
type Report struct {
	// Root is the data folder the app must use.
	Root string
	// Moved: the old folder was renamed. Copied: it was copied and the old
	// one kept. Both false: nothing was moved.
	Moved, Copied bool
	// BothExist: the old folder exists next to the new one and was left alone.
	BothExist bool
	// Rewritten lists the files (relative to Root) whose stored paths changed.
	Rewritten []string
	// SetAside lists plugin builds that predate the rename, moved out of
	// plugins/ until they are reinstalled.
	SetAside []string
	// Err is set when the move failed; Root is then the old folder.
	Err error
}

func (o *Options) info(format string, args ...any) {
	if o.Info != nil {
		o.Info(format, args...)
	}
}

func (o *Options) warn(format string, args ...any) {
	if o.Warn != nil {
		o.Warn(format, args...)
	}
}

// Run migrates the data folder of the user whose home is home. It never
// deletes the old folder after a copy and never merges two folders.
func Run(home string, o Options) Report {
	oldRoot := filepath.Join(home, ".local/share", legacyName)
	newRoot := datadir.Default(home)
	rep := Report{Root: newRoot}
	rename := o.rename
	if rename == nil {
		rename = os.Rename
	}

	switch {
	case exists(newRoot):
		if _, err := os.Lstat(filepath.Join(newRoot, pendingMarker)); err == nil {
			o.info("migration: finishing the move to %s", newRoot)
			o.finish(&rep, oldRoot, newRoot)
			return rep
		}
		if isDir(oldRoot) {
			rep.BothExist = true
			if _, err := os.Lstat(filepath.Join(oldRoot, keptNote)); err == nil {
				o.info("migration: %s is a kept copy of an earlier move; using %s", oldRoot, newRoot)
			} else {
				o.warn("migration: both %s and %s exist; using %s and leaving the old folder alone (nothing is merged)", oldRoot, newRoot, newRoot)
			}
		}
		return rep
	case !isDir(oldRoot):
		return rep // a fresh install
	}

	// The marker goes into the old folder first so it moves with it. If it
	// can't be written the marker is added after the move instead.
	markerWritten := os.WriteFile(filepath.Join(oldRoot, pendingMarker), []byte(oldRoot+"\n"), 0o600) == nil

	if err := rename(oldRoot, newRoot); err == nil {
		rep.Moved = true
		o.info("migration: moved %s to %s", oldRoot, newRoot)
	} else {
		o.warn("migration: renaming %s failed (%v); copying it instead and keeping the old folder", oldRoot, err)
		if cerr := copyVerified(oldRoot, newRoot); cerr != nil {
			if markerWritten {
				_ = os.Remove(filepath.Join(oldRoot, pendingMarker))
			}
			datadir.Use(oldRoot)
			rep.Root, rep.Err = oldRoot, fmt.Errorf("could not move %s to %s: %w (the old folder is kept and used as it is)", oldRoot, newRoot, cerr)
			o.warn("migration: %v", rep.Err)
			return rep
		}
		rep.Copied = true
		_ = os.Remove(filepath.Join(oldRoot, pendingMarker))
		_ = os.WriteFile(filepath.Join(oldRoot, keptNote), []byte("This folder was copied to "+newRoot+" and is no longer used. You can delete it once everything works.\n"), 0o644)
		o.info("migration: copied %s to %s; the old folder is kept", oldRoot, newRoot)
	}
	if !markerWritten {
		if err := os.WriteFile(filepath.Join(newRoot, pendingMarker), []byte(oldRoot+"\n"), 0o600); err != nil {
			// Not fatal: the steps below run now; they just can't be retried.
			o.warn("migration: could not mark the migration as pending: %v", err)
		}
	}
	o.finish(&rep, oldRoot, newRoot)
	return rep
}

// finish rewrites the stored paths and sets old plugin builds aside, then
// removes the marker. A failure keeps the marker so the next start retries.
func (o *Options) finish(rep *Report, oldRoot, newRoot string) {
	changed, err := RewritePaths(newRoot, oldRoot, o.warn)
	rep.Rewritten = changed
	if err != nil {
		rep.Err = errors.Join(rep.Err, fmt.Errorf("rewriting stored paths: %w", err))
		o.warn("migration: %v (will retry at the next start)", err)
		return
	}
	aside, err := SetAsideLegacyPlugins(newRoot)
	rep.SetAside = aside
	if err != nil {
		rep.Err = errors.Join(rep.Err, fmt.Errorf("setting old plugins aside: %w", err))
		o.warn("migration: %v (will retry at the next start)", err)
		return
	}
	for _, a := range aside {
		o.warn("migration: plugin %s predates the rename and was moved to plugins-before-rename/; reinstall it with plugins/install.sh and approve it again", a)
	}
	if len(changed) > 0 {
		o.info("migration: updated stored paths in %d file(s)", len(changed))
	}
	if err := os.Remove(filepath.Join(newRoot, pendingMarker)); err != nil && !errors.Is(err, os.ErrNotExist) {
		o.warn("migration: could not remove the pending marker: %v", err)
	}
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// isDir follows symlinks: a data folder symlinked to a bigger disk counts.
func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
