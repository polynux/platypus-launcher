package app

import (
	"context"
	"path/filepath"
	"time"

	"platypus/internal/playlog"
	"platypus/internal/stats"
	"platypus/internal/types"
)

// Local play sessions (issue #15): the session log behind the playtime and
// last-played numbers. Sessions are recorded from the session tracker:
// Start when a game reaches "running", Finish at "stopped"/"failed". The
// merge into the library lives in applyPlaytime below, the one place local
// and store numbers meet, so the UI needs no per-game calls.

// playSession records one session event in the log. "starting" and
// "stopping" carry no time: only "running" opens a session and only
// "stopped"/"failed" close one. A failure without a running state (the
// store's client never started the game) closes nothing.
func (a *App) playSession(e types.SessionEvent) {
	if a.playlog == nil || e.GameID == "" {
		return
	}
	switch e.State {
	case "running":
		a.playlog.Start(e.GameID)
	case "stopped", "failed":
		a.playlog.Finish(e.GameID)
	}
}

// playHeartbeatLoop refreshes the open sessions while games run, so a core
// that exits or crashes loses at most playlog.HeartbeatEvery per game. It
// ends with ctx (the jobs run context startup created).
func (a *App) playHeartbeatLoop(ctx context.Context) {
	t := time.NewTicker(playlog.HeartbeatEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.playHeartbeat()
		}
	}
}

// playHeartbeat refreshes every running session once.
func (a *App) playHeartbeat() {
	if a.playlog == nil || a.sessions == nil {
		return
	}
	for _, s := range a.sessions.Snapshot() {
		if s.State == "running" {
			a.playlog.Heartbeat(s.GameID)
		}
	}
}

// openPlaylog loads the session log from the data dir, closing at their last
// heartbeat the sessions a previous core left open by exiting or crashing.
// It reports how many it recovered, for the startup log.
func (a *App) openPlaylog(dataDir string) {
	l, err := playlog.Open(filepath.Join(dataDir, "play-sessions.json"))
	if err != nil {
		a.host.Errorf("play sessions: %v", err)
	}
	a.playlog = l
	if n := l.Recovered(); n > 0 {
		a.host.Infof("play sessions: recovered %d session(s) left open by the last run", n)
	}
}

// applyPlaytime merges the cached store playtime and the local session log
// into every game, whatever its source (plugin, sideloaded, Steam
// installed), keyed by game id. What shows is max(local, store) as before,
// and last_played_at is max(local, store). It is the one place the merge
// happens; the UI reads the numbers off the library.
func (a *App) applyPlaytime(games []types.Game) {
	a.stats.applyPlaytime(games)
	if a.playlog == nil {
		return
	}
	for i := range games {
		g := &games[i]
		if last := a.playlog.LastPlayed(g.ID); last > g.LastPlayed {
			g.LastPlayed = last
		}
		local := a.playlog.Minutes(g.ID)
		if local <= 0 {
			continue
		}
		if g.PlaytimeStoreMin != nil {
			// A store report already merged by the stats cache: the
			// local side is whatever the listing carried, if anything,
			// or this log, whichever is larger.
			store := *g.PlaytimeStoreMin
			if g.PlaytimeLocalMin != nil && *g.PlaytimeLocalMin > local {
				local = *g.PlaytimeLocalMin
			}
			v := local
			g.PlaytimeLocalMin = &v
			g.PlaytimeMin = stats.MergePlaytime(local, store)
			continue
		}
		// No store report (an unreported plugin game, sideloaded): what
		// the listing carried, if anything, is local too.
		if g.PlaytimeMin > local {
			local = g.PlaytimeMin
		}
		v := local
		g.PlaytimeLocalMin = &v
		g.PlaytimeMin = local
	}
}
