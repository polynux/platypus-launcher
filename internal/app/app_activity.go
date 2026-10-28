package app

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"time"

	"platypus/internal/types"
)

// Activity feed (issues #145, #151): a small append-only log under the data
// dir plus the playlog merge. Stored events cover finished jobs, cloud sync
// results, achievement unlocks and library adds; sessions are merged live
// from the playlog so yesterday's runs survive a restart without a second
// store. The UI renders TextKey with Args in its own language and links
// GameID back to the library.

const (
	activityFileVersion = 1
	addedFileVersion    = 1
	// maxActivityEvents bounds the persisted feed; older rows fall off the
	// end. Each row is tiny, so the cap is only a backstop.
	maxActivityEvents = 500
)

type activityFile struct {
	Version int                  `json:"version"`
	NextID  int64                `json:"next_id"`
	Events  []types.ActivityEvent `json:"events"`
}

type addedFile struct {
	Version int              `json:"version"`
	Added   map[string]int64 `json:"added"`
}

// openActivity loads the persisted feed. A missing file is empty; a corrupt
// one is moved aside and Open returns empty with the error, like the session
// log, so history is never silently overwritten.
func (a *App) openActivity(dataDir string) {
	a.activityPath = ""
	a.activityEvents = nil
	a.activityNextID = 1
	if dataDir == "" {
		return
	}
	a.activityPath = filepath.Join(dataDir, "activity.json")
	raw, err := os.ReadFile(a.activityPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		if a.host != nil {
			a.host.Errorf("activity: %v", err)
		}
		return
	}
	var f activityFile
	if err := json.Unmarshal(raw, &f); err != nil {
		bad := fmt.Sprintf("%s.bad-%d", a.activityPath, time.Now().Unix())
		_ = os.Rename(a.activityPath, bad)
		if a.host != nil {
			a.host.Errorf("activity is corrupt, moved to %s: %v", bad, err)
		}
		a.activityEvents = nil
		a.activityNextID = 1
		return
	}
	a.activityEvents = f.Events
	if a.activityEvents == nil {
		a.activityEvents = []types.ActivityEvent{}
	}
	a.activityNextID = f.NextID
	if a.activityNextID < 1 {
		var max int64
		for _, e := range a.activityEvents {
			if e.ID > max {
				max = e.ID
			}
		}
		a.activityNextID = max + 1
		if a.activityNextID < 1 {
			a.activityNextID = 1
		}
	}
}

// saveActivity writes the feed atomically. Callers hold a.activityMu.
func (a *App) saveActivity() {
	if a.activityPath == "" {
		return
	}
	f := activityFile{Version: activityFileVersion, NextID: a.activityNextID, Events: a.activityEvents}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return
	}
	dir := filepath.Dir(a.activityPath)
	_ = os.MkdirAll(dir, 0o755)
	tmp, err := os.CreateTemp(dir, ".activity-*.json")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	_ = os.Rename(tmp.Name(), a.activityPath)
}

// recordActivityAt appends one row with an explicit timestamp (unix ms) and
// pushes it to the UI. Callers hold no lock. Tests that never opened the
// feed use the zero value: memory only, and a nil host drops the push.
func (a *App) recordActivityAt(ts int64, kind, gameID, textKey string, args map[string]string) types.ActivityEvent {
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}
	a.activityMu.Lock()
	if a.activityNextID < 1 {
		var max int64
		for _, e := range a.activityEvents {
			if e.ID > max {
				max = e.ID
			}
		}
		a.activityNextID = max + 1
		if a.activityNextID < 1 {
			a.activityNextID = 1
		}
	}
	e := types.ActivityEvent{ID: a.activityNextID, TS: ts, Kind: kind, GameID: gameID, TextKey: textKey, Args: args}
	a.activityNextID++
	a.activityEvents = append(a.activityEvents, e)
	if len(a.activityEvents) > maxActivityEvents {
		a.activityEvents = a.activityEvents[len(a.activityEvents)-maxActivityEvents:]
	}
	a.saveActivity()
	a.activityMu.Unlock()
	if a.host != nil {
		a.host.Emit("activity", e)
	}
	return e
}

// recordActivity appends one row stamped now and pushes it to the UI.
func (a *App) recordActivity(kind, gameID, textKey string, args map[string]string) types.ActivityEvent {
	return a.recordActivityAt(time.Now().UnixMilli(), kind, gameID, textKey, args)
}

// hasAchievementActivity reports whether an unlock row for the game and
// achievement is already stored, so achievement polls don't duplicate it.
func (a *App) hasAchievementActivity(gameID, achID string) bool {
	a.activityMu.Lock()
	defer a.activityMu.Unlock()
	for _, e := range a.activityEvents {
		if e.Kind != "achievement" || e.GameID != gameID {
			continue
		}
		if e.Args != nil && e.Args["achievement"] == achID {
			return true
		}
	}
	return false
}

// ListActivity returns the feed newest first: stored rows plus the playlog's
// closed sessions. Since (unix ms) keeps only rows newer than it; limit caps
// the rows (<=0 means 200). It never returns nil.
func (a *App) ListActivity(since int64, limit int) []types.ActivityEvent {
	if limit <= 0 {
		limit = 200
	}
	if limit > maxActivityEvents {
		limit = maxActivityEvents
	}
	a.activityMu.Lock()
	stored := make([]types.ActivityEvent, 0, len(a.activityEvents))
	for _, e := range a.activityEvents {
		if since > 0 && e.TS <= since {
			continue
		}
		stored = append(stored, e)
	}
	a.activityMu.Unlock()
	out := append(stored, a.sessionActivity(since)...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].TS != out[j].TS {
			return out[i].TS > out[j].TS
		}
		return out[i].ID > out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	if out == nil {
		out = []types.ActivityEvent{}
	}
	return out
}

// sessionActivity converts the playlog's closed sessions to feed rows. IDs
// are deterministic negatives from the session end and game, so they never
// collide with the stored positive sequence.
func (a *App) sessionActivity(since int64) []types.ActivityEvent {
	if a.playlog == nil {
		return nil
	}
	var out []types.ActivityEvent
	for _, r := range a.playlog.AllRecent() {
		endMs := r.Session.End * 1000
		if since > 0 && endMs <= since {
			continue
		}
		h := fnv.New32a()
		_, _ = h.Write([]byte(r.GameID))
		id := -((r.Session.End << 32) | int64(h.Sum32()))
		if id >= 0 {
			id = -(id + 1)
			if id >= 0 {
				id = -1
			}
		}
		args := map[string]string{"seconds": fmt.Sprint(r.Session.Seconds)}
		out = append(out, types.ActivityEvent{
			ID: id, TS: endMs, Kind: "session",
			GameID: r.GameID, TextKey: "activity:log.sessionStopped", Args: args,
		})
	}
	return out
}

// openAddedAt loads the first-seen map (issue #151). Missing is empty;
// corrupt is moved aside like the other data files.
func (a *App) openAddedAt(dataDir string) {
	a.addedAtPath = ""
	a.addedAt = map[string]int64{}
	if dataDir == "" {
		return
	}
	a.addedAtPath = filepath.Join(dataDir, "library-added.json")
	raw, err := os.ReadFile(a.addedAtPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		if a.host != nil {
			a.host.Errorf("library added: %v", err)
		}
		return
	}
	var f addedFile
	if err := json.Unmarshal(raw, &f); err != nil {
		bad := fmt.Sprintf("%s.bad-%d", a.addedAtPath, time.Now().Unix())
		_ = os.Rename(a.addedAtPath, bad)
		if a.host != nil {
			a.host.Errorf("library added is corrupt, moved to %s: %v", bad, err)
		}
		a.addedAt = map[string]int64{}
		return
	}
	if f.Added != nil {
		a.addedAt = f.Added
	}
}

// saveAddedAt writes the first-seen map atomically. Callers hold a.activityMu.
func (a *App) saveAddedAt() {
	if a.addedAtPath == "" {
		return
	}
	f := addedFile{Version: addedFileVersion, Added: a.addedAt}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return
	}
	dir := filepath.Dir(a.addedAtPath)
	_ = os.MkdirAll(dir, 0o755)
	tmp, err := os.CreateTemp(dir, ".added-*.json")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	_ = os.Rename(tmp.Name(), a.addedAtPath)
}

// noteLibraryGames stamps every game with its first-seen time (unix ms),
// records an add row for each new id, and persists new entries. It mutates
// games in place. Custom games already carry their settings AddedAt, which
// wins over now for their first sighting.
func (a *App) noteLibraryGames(games []types.Game) {
	now := time.Now().UnixMilli()
	a.activityMu.Lock()
	if a.addedAt == nil {
		a.addedAt = map[string]int64{}
	}
	var added []types.Game
	for i := range games {
		g := &games[i]
		if g.ID == "" {
			continue
		}
		if at, ok := a.addedAt[g.ID]; ok {
			if g.AddedAt == 0 || at < g.AddedAt {
				g.AddedAt = at
			} else if g.AddedAt != at && g.AddedAt != 0 {
				// The game knows better (a sideload's settings):
				// keep the earliest.
				if g.AddedAt < at {
					a.addedAt[g.ID] = g.AddedAt
				} else {
					g.AddedAt = at
				}
			}
			continue
		}
		at := now
		if g.AddedAt != 0 && g.AddedAt < at {
			at = g.AddedAt
		}
		a.addedAt[g.ID] = at
		g.AddedAt = at
		added = append(added, *g)
	}
	changed := len(added) > 0
	if changed {
		a.saveAddedAt()
	}
	// Assign add rows outside the map update but under the same lock order:
	// recordActivityAt takes the lock, so copy first, release, then record.
	a.activityMu.Unlock()
	for _, g := range added {
		a.recordActivityAt(g.AddedAt, "add", g.ID, "activity:log.added",
			map[string]string{"title": g.Title, "store": string(g.Store)})
	}
	if !changed {
		return
	}
}

// recordJobActivity logs a finished job (issue #145): install, update,
// uninstall, verify and move, with store, size and version.
func (a *App) recordJobActivity(j types.Job) {
	kind := j.Kind
	switch kind {
	case "install", "update", "uninstall", "verify", "move":
	default:
		return
	}
	args := map[string]string{"title": j.Title, "store": j.Source}
	if j.Total > 0 {
		args["size"] = fmt.Sprint(j.Total)
	} else if j.Bytes > 0 {
		args["size"] = fmt.Sprint(j.Bytes)
	}
	if j.Target != nil {
		if j.Target.Channel != "" {
			args["version"] = j.Target.Channel
		} else if j.Target.Build != "" {
			args["version"] = j.Target.Build
		}
	}
	verb := map[string]string{
		"install": "installed", "update": "updated", "uninstall": "uninstalled",
		"verify": "verified", "move": "moved",
	}[kind]
	if verb == "" {
		verb = kind
	}
	if args == nil {
		args = map[string]string{}
	}
	args["action"] = verb
	a.recordActivity(kind, j.GameID, "activity:log.jobDone", args)
}

// recordCloudActivity logs a cloud sync result (issue #145): uploaded,
// downloaded or conflict.
func (a *App) recordCloudActivity(kind, gameID, textKey string, args map[string]string) {
	a.recordActivity("cloud", gameID, textKey, args)
}
