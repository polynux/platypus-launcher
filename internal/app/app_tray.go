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

// gameRunning reports whether any game session is currently tracked. The
// shell hides the window on this while the minimize-to-tray switch is on,
// and the core defers non-urgent background work on it (issue #49).
func (a *App) gameRunning() bool {
	if a == nil || a.sessions == nil {
		return false
	}
	return len(a.sessions.Snapshot()) > 0
}

// deferWhileGameRuns queues fn to run when the last game session ends and
// reports true. When no game runs it reports false and the caller runs the
// work now. Only automatic background work uses it (the auto artwork lookup
// after an add, the save-list manifest refresh); anything the user asked for
// — a rescan, a search, an apply, opening a game page — runs right away.
// Library rescans need no gate: there is no automatic rescan timer, every
// listing is an explicit read or a manual rescan.
func (a *App) deferWhileGameRuns(fn func()) bool {
	if !a.gameRunning() {
		return false
	}
	a.trayMu.Lock()
	a.deferred = append(a.deferred, fn)
	a.trayMu.Unlock()
	return true
}

// drainDeferred runs the background work put off while a game ran, in order.
// Called when a session ends and no game is left running; each job runs in
// its own goroutine so one slow fetch never blocks the next.
func (a *App) drainDeferred() {
	a.trayMu.Lock()
	queued := a.deferred
	a.deferred = nil
	a.trayMu.Unlock()
	for _, fn := range queued {
		go fn()
	}
}
