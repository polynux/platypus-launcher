package app

import (
	"encoding/json"
	"fmt"
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
	// SeenSources marks the sources whose first non-empty listing was
	// already seeded silently (issue #145, A1): a store's first login must
	// not flood the feed with a row per game.
	SeenSources map[string]bool `json:"seen_sources,omitempty"`
}

// achSeenFile is the persisted unlock set apart from the feed (issue #145,
// A2): game id to achievement id to unlock time (unix seconds). The feed is
// capped and rows can be evicted, so polls must not consult it for "already
// logged".
type achSeenFile struct {
	Version int                         `json:"version"`
	Games   map[string]map[string]int64 `json:"games"`
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
	if f.Version > activityFileVersion {
		bad := fmt.Sprintf("%s.bad-%d", a.activityPath, time.Now().Unix())
		_ = os.Rename(a.activityPath, bad)
		if a.host != nil {
			a.host.Errorf("activity is from a newer Platypus, moved to %s", bad)
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

// appendActivityLocked appends e under the lock, trims the feed to the cap
// by oldest timestamp (so a backfilled old row never evicts a fresh one),
// and saves. It returns the stored row (with its id). Callers hold
// a.activityMu.
func (a *App) appendActivityLocked(e types.ActivityEvent) types.ActivityEvent {
	if a.activityNextID < 1 {
		var max int64
		for _, old := range a.activityEvents {
			if old.ID > max {
				max = old.ID
			}
		}
		a.activityNextID = max + 1
		if a.activityNextID < 1 {
			a.activityNextID = 1
		}
	}
	e.ID = a.activityNextID
	a.activityNextID++
	a.activityEvents = append(a.activityEvents, e)
	a.trimActivityLocked()
	a.saveActivity()
	return e
}

// trimActivityLocked drops rows past the cap, oldest timestamp first (ties
// by lowest id). Callers hold a.activityMu.
func (a *App) trimActivityLocked() {
	for len(a.activityEvents) > maxActivityEvents {
		victim := 0
		for i := 1; i < len(a.activityEvents); i++ {
			if a.activityEvents[i].TS < a.activityEvents[victim].TS ||
				(a.activityEvents[i].TS == a.activityEvents[victim].TS && a.activityEvents[i].ID < a.activityEvents[victim].ID) {
				victim = i
			}
		}
		a.activityEvents = append(a.activityEvents[:victim], a.activityEvents[victim+1:]...)
	}
}

// recordActivityAt appends one row with an explicit timestamp (unix ms) and
// pushes it to the UI. Callers hold no lock. Tests that never opened the
// feed use the zero value: memory only, and a nil host drops the push.
func (a *App) recordActivityAt(ts int64, kind, gameID, textKey string, args map[string]string) types.ActivityEvent {
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}
	a.activityMu.Lock()
	e := types.ActivityEvent{TS: ts, Kind: kind, GameID: gameID, TextKey: textKey, Args: args}
	e = a.appendActivityLocked(e)
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
// openAchSeen loads the persisted unlock set apart from the feed (issue
// #145, A2). Missing is empty; corrupt or newer is moved aside like the
// other data files.
func (a *App) openAchSeen(dataDir string) {
	a.achSeenPath = ""
	a.achSeen = map[string]map[string]int64{}
	if dataDir == "" {
		return
	}
	a.achSeenPath = filepath.Join(dataDir, "achievements-seen.json")
	raw, err := os.ReadFile(a.achSeenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		if a.host != nil {
			a.host.Errorf("achievements seen: %v", err)
		}
		return
	}
	var f achSeenFile
	if err := json.Unmarshal(raw, &f); err != nil || f.Version > activityFileVersion {
		bad := fmt.Sprintf("%s.bad-%d", a.achSeenPath, time.Now().Unix())
		_ = os.Rename(a.achSeenPath, bad)
		if a.host != nil {
			if err != nil {
				a.host.Errorf("achievements seen is corrupt, moved to %s: %v", bad, err)
			} else {
				a.host.Errorf("achievements seen is from a newer Platypus, moved to %s", bad)
			}
		}
		a.achSeen = map[string]map[string]int64{}
		return
	}
	if f.Games != nil {
		a.achSeen = f.Games
	}
}

// saveAchSeenLocked writes the unlock set atomically. Callers hold a.activityMu.
func (a *App) saveAchSeenLocked() {
	if a.achSeenPath == "" {
		return
	}
	f := achSeenFile{Version: activityFileVersion, Games: a.achSeen}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return
	}
	dir := filepath.Dir(a.achSeenPath)
	_ = os.MkdirAll(dir, 0o755)
	tmp, err := os.CreateTemp(dir, ".achseen-*.json")
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
	_ = os.Rename(tmp.Name(), a.achSeenPath)
}

// noteAchievementUnlocks logs newly seen unlocks to the activity feed
// (issue #145): the plugin's unlockedAt is the unlock time, so each row is
// stamped then. The first poll of a game only seeds the persisted set, so a
// library of old unlocks never floods (or recycles through) the capped feed;
// the check and the insert hold one lock, so concurrent polls can't double
// log. Callers hold no lock.
func (a *App) noteAchievementUnlocks(game types.Game, progress types.AchievementProgress) {
	a.activityMu.Lock()
	if a.achSeen == nil {
		a.achSeen = map[string]map[string]int64{}
	}
	seen, known := a.achSeen[game.ID]
	if !known {
		seen = map[string]int64{}
		a.achSeen[game.ID] = seen
	}
	type fresh struct {
		id, name string
		at       int64
	}
	var rows []fresh
	for _, ach := range progress.List {
		if !ach.Unlocked || ach.UnlockedAt <= 0 {
			continue
		}
		if _, ok := seen[ach.ID]; ok {
			continue
		}
		seen[ach.ID] = ach.UnlockedAt
		if known {
			rows = append(rows, fresh{id: ach.ID, name: ach.Name, at: ach.UnlockedAt})
		}
	}
	if known && len(rows) == 0 {
		a.activityMu.Unlock()
		return
	}
	a.saveAchSeenLocked()
	var emitted []types.ActivityEvent
	for _, r := range rows {
		emitted = append(emitted, a.appendActivityLocked(types.ActivityEvent{
			TS: r.at * 1000, Kind: "achievement", GameID: game.ID,
			TextKey: "activity:log.achievementUnlocked",
			Args:    map[string]string{"achievement": r.id, "name": r.name, "title": game.Title},
		}))
	}
	a.activityMu.Unlock()
	if a.host != nil {
		for _, e := range emitted {
			a.host.Emit("activity", e)
		}
	}
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
// are small deterministic negatives (newest first), so they never collide
// with the stored positive sequence and stay small enough for the UI.
func (a *App) sessionActivity(since int64) []types.ActivityEvent {
	if a.playlog == nil {
		return nil
	}
	recent := a.playlog.AllRecent()
	sort.Slice(recent, func(i, j int) bool {
		if recent[i].Session.End != recent[j].Session.End {
			return recent[i].Session.End > recent[j].Session.End
		}
		return recent[i].GameID > recent[j].GameID
	})
	var out []types.ActivityEvent
	for i, r := range recent {
		endMs := r.Session.End * 1000
		if since > 0 && endMs <= since {
			continue
		}
		args := map[string]string{"seconds": fmt.Sprint(r.Session.Seconds)}
		out = append(out, types.ActivityEvent{
			ID: int64(-(i + 1)), TS: endMs, Kind: "session",
			GameID: r.GameID, TextKey: "activity:log.sessionStopped", Args: args,
		})
	}
	return out
}

// openAddedAt loads the first-seen map (issue #151). Missing means this is
// the first run with the map: the next scan seeds it silently (A1), so no
// file is written here. Corrupt or newer files are moved aside like the
// other data files.
func (a *App) openAddedAt(dataDir string) {
	a.addedAtPath = ""
	a.addedAt = map[string]int64{}
	a.addedAtSeen = map[string]bool{}
	a.addedAtFresh = false
	if dataDir == "" {
		return
	}
	a.addedAtPath = filepath.Join(dataDir, "library-added.json")
	raw, err := os.ReadFile(a.addedAtPath)
	if err != nil {
		if os.IsNotExist(err) {
			a.addedAtFresh = true
			return
		}
		if a.host != nil {
			a.host.Errorf("library added: %v", err)
		}
		return
	}
	var f addedFile
	if err := json.Unmarshal(raw, &f); err != nil || f.Version > addedFileVersion {
		bad := fmt.Sprintf("%s.bad-%d", a.addedAtPath, time.Now().Unix())
		_ = os.Rename(a.addedAtPath, bad)
		if a.host != nil {
			if err != nil {
				a.host.Errorf("library added is corrupt, moved to %s: %v", bad, err)
			} else {
				a.host.Errorf("library added is from a newer Platypus, moved to %s", bad)
			}
		}
		a.addedAt = map[string]int64{}
		a.addedAtSeen = map[string]bool{}
		a.addedAtFresh = true
		return
	}
	if f.Added != nil {
		a.addedAt = f.Added
	}
	if f.SeenSources != nil {
		a.addedAtSeen = f.SeenSources
	}
}

// saveAddedAt writes the first-seen map atomically. Callers hold a.activityMu.
func (a *App) saveAddedAt() {
	if a.addedAtPath == "" {
		return
	}
	f := addedFile{Version: addedFileVersion, Added: a.addedAt, SeenSources: a.addedAtSeen}
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

// noteLibraryGames stamps every game with its first-seen time (unix ms).
// It mutates games in place. Rows are recorded only for genuinely new games:
// the very first scan with the map (an upgrade, or a store's first login)
// seeds every id silently with added_at 0 (sideloads keep their settings
// AddedAt), because those games are old, not new. One map save and one feed
// save cover the whole batch, and the new rows go out together.
func (a *App) noteLibraryGames(games []types.Game) {
	now := time.Now().UnixMilli()
	a.activityMu.Lock()
	if a.addedAt == nil {
		a.addedAt = map[string]int64{}
	}
	if a.addedAtSeen == nil {
		a.addedAtSeen = map[string]bool{}
	}
	fresh := a.addedAtFresh
	a.addedAtFresh = false
	// Which sources have a non-empty listing in this scan. A source never
	// seeded before has all its games seeded silently below: a store's
	// first login is backfill, not news.
	seedSources := map[string]bool{}
	for i := range games {
		if games[i].ID == "" {
			continue
		}
		if !a.addedAtSeen[games[i].Source] {
			seedSources[games[i].Source] = true
		}
	}
	for s := range seedSources {
		a.addedAtSeen[s] = true
	}
	changed := len(seedSources) > 0
	type row struct {
		ts     int64
		gameID string
		title  string
		store  string
	}
	var rows []row
	for i := range games {
		g := &games[i]
		if g.ID == "" {
			continue
		}
		if at, ok := a.addedAt[g.ID]; ok {
			if g.AddedAt == 0 || at < g.AddedAt {
				g.AddedAt = at
			} else if g.AddedAt != at {
				// The game knows better (a sideload's settings):
				// keep the earliest.
				if g.AddedAt < at {
					a.addedAt[g.ID] = g.AddedAt
					changed = true
				} else {
					g.AddedAt = at
				}
			}
			continue
		}
		if fresh || seedSources[g.Source] {
			// First sight of the map, or of this source (a new login):
			// old games, not new ones. No feed row.
			at := int64(0)
			if g.AddedAt != 0 {
				at = g.AddedAt
			}
			a.addedAt[g.ID] = at
			g.AddedAt = at
			changed = true
			continue
		}
		at := now
		if g.AddedAt != 0 && g.AddedAt < at {
			at = g.AddedAt
		}
		a.addedAt[g.ID] = at
		g.AddedAt = at
		changed = true
		rows = append(rows, row{ts: at, gameID: g.ID, title: g.Title, store: string(g.Store)})
	}
	if changed {
		a.saveAddedAt()
	}
	var emitted []types.ActivityEvent
	for _, r := range rows {
		emitted = append(emitted, a.appendActivityLocked(types.ActivityEvent{
			TS: r.ts, Kind: "add", GameID: r.gameID,
			TextKey: "activity:log.added",
			Args:    map[string]string{"title": r.title, "store": r.store},
		}))
	}
	a.activityMu.Unlock()
	if a.host != nil {
		for _, e := range emitted {
			a.host.Emit("activity", e)
		}
	}
}

// noteExplicitAdd records one game the user just added (issue #145): unlike
// a scan, an explicit add is always a genuine new row, even when its source
// hasn't been seen before. It mutates nothing else.
func (a *App) noteExplicitAdd(game types.Game) {
	at := time.Now().UnixMilli()
	if game.AddedAt != 0 && game.AddedAt < at {
		at = game.AddedAt
	}
	a.activityMu.Lock()
	if a.addedAt == nil {
		a.addedAt = map[string]int64{}
	}
	if a.addedAtSeen == nil {
		a.addedAtSeen = map[string]bool{}
	}
	a.addedAt[game.ID] = at
	a.addedAtSeen[game.Source] = true
	a.addedAtFresh = false
	a.saveAddedAt()
	e := a.appendActivityLocked(types.ActivityEvent{
		TS: at, Kind: "add", GameID: game.ID,
		TextKey: "activity:log.added",
		Args:    map[string]string{"title": game.Title, "store": string(game.Store)},
	})
	a.activityMu.Unlock()
	if a.host != nil {
		a.host.Emit("activity", e)
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
	// The verb is rendered from the kind in the UI's language (A5): no
	// English "action" travels in args.
	a.recordActivity(kind, j.GameID, "activity:log.jobDone", args)
}

// recordCloudActivity logs a cloud sync result (issue #145): uploaded,
// downloaded or conflict.
func (a *App) recordCloudActivity(kind, gameID, textKey string, args map[string]string) {
	a.recordActivity("cloud", gameID, textKey, args)
}
