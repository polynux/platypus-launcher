// Package playlog is Platypus's own session log: when each game ran and for
// how long, in ~/.local/share/platypus/play-sessions.json.
//
// The store's playtime report (the `playtime` capability) is the only playtime
// Platypus had, so plugin and sideloaded games showed the store's minutes or
// nothing (issue #15). This log records sessions from the session tracker:
// Start when a game reaches "running", Finish at "stopped"/"failed".
//
// A session shorter than MinSessionSec (a crash on launch) counts nothing
// toward the total but still sets last_played. A session still open when the
// core exits or crashes is closed at its last heartbeat: the App writes a
// heartbeat every HeartbeatEvery while a game runs, so at most a minute is
// lost. Open loads the file and closes any dangling session at its last
// heartbeat.
//
// The file can't grow forever: per-game totals aggregate every closed
// session, and only the last MaxRecentPerGame sessions per game are kept in
// detail. Totals of forgotten games are dropped past MaxGames (oldest
// last-played first); the cap is high enough to never matter in practice.
package playlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	fileVersion = 1
	// MinSessionSec bounds a session that counts toward the total: a shorter
	// run (a crash on launch) sets last_played but adds no minutes.
	MinSessionSec = 10
	// HeartbeatEvery is how often the App refreshes an open session's
	// heartbeat while a game runs, so a crash loses at most this much.
	HeartbeatEvery = 60 * time.Second
	// MaxRecentPerGame bounds the session detail kept per game; older
	// sessions live on only in the per-game total.
	MaxRecentPerGame = 20
	// MaxGames bounds the games kept; the game played longest ago goes
	// first. Each entry is tiny, so the cap is only a backstop.
	MaxGames = 5000
)

// Session is one closed run of a game. Start and End are unix seconds;
// Seconds is End-Start.
type Session struct {
	Start   int64 `json:"start"`
	End     int64 `json:"end"`
	Seconds int64 `json:"seconds"`
}

// open is a session that hasn't ended yet.
type open struct {
	Start     int64 `json:"start"`
	Heartbeat int64 `json:"heartbeat"`
}

type record struct {
	TotalSec   int64     `json:"total_sec"`
	LastPlayed int64     `json:"last_played"`
	Recent     []Session `json:"recent"`
	Open       *open     `json:"open,omitempty"`
}

type file struct {
	Version int              `json:"version"`
	Games   map[string]record `json:"games"`
}

// Log is safe for concurrent use and writes through on every change.
type Log struct {
	path string
	now  func() time.Time

	maxRecent int
	maxGames  int
	// readErr is set when the file exists but couldn't be read. The log in
	// memory is then empty and not the file's, so writes are refused and the
	// real file is left as it is.
	readErr error
	// recovered counts the dangling sessions Open closed at their last
	// heartbeat (a previous core that exited or crashed mid-session).
	recovered int

	mu   sync.Mutex
	data file
}

// Open loads the log at path. A missing file is an empty log; a corrupt one
// is moved aside (…bad-<time>) and Open returns an empty log with the error,
// so sessions are never silently overwritten. A file that can't be read at
// all also gives an empty log with the error, and that log is read-only:
// Start/Heartbeat/Finish refuse, so the real file is left as it is.
//
// Sessions still open from the previous run (the core exited or crashed
// mid-session) are closed at their last heartbeat before Open returns, so
// the totals already include them.
func Open(path string) (*Log, error) {
	l := &Log{
		path: path, now: time.Now,
		maxRecent: MaxRecentPerGame, maxGames: MaxGames,
		data: file{Games: map[string]record{}},
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		l.readErr = fmt.Errorf("play sessions can't be read: %w", err)
		return l, l.readErr
	}
	var f file
	if err := json.Unmarshal(raw, &f); err != nil {
		bad := fmt.Sprintf("%s.bad-%d", path, time.Now().Unix())
		_ = os.Rename(path, bad)
		return l, fmt.Errorf("play sessions are corrupt, moved to %s: %w", bad, err)
	}
	if f.Games != nil {
		l.data.Games = f.Games
	}
	l.recovered = l.closeDangling()
	if l.recovered > 0 {
		if err := l.save(); err != nil {
			return l, err
		}
	}
	return l, nil
}

// Recovered counts the dangling sessions Open closed at their last
// heartbeat: the sessions a previous core left open by exiting or crashing.
func (l *Log) Recovered() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.recovered
}

// closeDangling closes every open session at its last heartbeat. Callers hold
// l.mu only during Open, before the log is shared; it saves nothing.
func (l *Log) closeDangling() int {
	n := 0
	for id, r := range l.data.Games {
		if r.Open == nil {
			continue
		}
		end := r.Open.Heartbeat
		if end < r.Open.Start {
			end = r.Open.Start
		}
		l.finishAt(id, time.Unix(end, 0))
		n++
	}
	return n
}

// Start records that gameID reached "running" now. A session already open
// for the game is left alone (its start is the earlier one).
func (l *Log) Start(gameID string) {
	l.StartAt(gameID, l.now())
}

// StartAt records that gameID reached "running" at t.
func (l *Log) StartAt(gameID string, t time.Time) {
	if gameID == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.readErr != nil {
		return
	}
	r := l.data.Games[gameID]
	if r.Open != nil {
		return
	}
	ts := t.Unix()
	r.Open = &open{Start: ts, Heartbeat: ts}
	l.data.Games[gameID] = r
	_ = l.save()
}

// Heartbeat refreshes gameID's open session to now, so a crash loses at most
// HeartbeatEvery. Games with no open session are ignored.
func (l *Log) Heartbeat(gameID string) {
	l.HeartbeatAt(gameID, l.now())
}

// HeartbeatAt refreshes gameID's open session to t.
func (l *Log) HeartbeatAt(gameID string, t time.Time) {
	if gameID == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.readErr != nil {
		return
	}
	r, ok := l.data.Games[gameID]
	if !ok || r.Open == nil {
		return
	}
	if ts := t.Unix(); ts > r.Open.Heartbeat {
		r.Open.Heartbeat = ts
		l.data.Games[gameID] = r
		_ = l.save()
	}
}

// Finish closes gameID's open session now. It reports the session's length
// and whether one was open. Sessions under MinSessionSec add nothing to the
// total but still set last_played.
func (l *Log) Finish(gameID string) (time.Duration, bool) {
	return l.FinishAt(gameID, l.now())
}

// FinishAt closes gameID's open session at t.
func (l *Log) FinishAt(gameID string, t time.Time) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.readErr != nil {
		return 0, false
	}
	return l.finishAt(gameID, t)
}

// finishAt closes the session and saves. Callers hold l.mu.
func (l *Log) finishAt(gameID string, t time.Time) (time.Duration, bool) {
	r, ok := l.data.Games[gameID]
	if !ok || r.Open == nil {
		return 0, false
	}
	end := t.Unix()
	dur := end - r.Open.Start
	if dur < 0 {
		dur = 0
	}
	if end > r.LastPlayed {
		r.LastPlayed = end
	}
	if dur >= MinSessionSec {
		r.TotalSec += dur
		r.Recent = append(r.Recent, Session{Start: r.Open.Start, End: end, Seconds: dur})
		if len(r.Recent) > l.maxRecent {
			r.Recent = r.Recent[len(r.Recent)-l.maxRecent:]
		}
	}
	r.Open = nil
	if r.TotalSec == 0 && len(r.Recent) == 0 && r.LastPlayed == 0 {
		delete(l.data.Games, gameID)
	} else {
		l.data.Games[gameID] = r
	}
	l.pruneGames()
	_ = l.save()
	return time.Duration(dur) * time.Second, true
}

// CloseAllAtHeartbeat closes every open session at its last heartbeat, for a
// clean shutdown: what is on disk already includes everything, and the next
// start finds nothing dangling.
func (l *Log) CloseAllAtHeartbeat() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.readErr != nil {
		return
	}
	changed := false
	for id, r := range l.data.Games {
		if r.Open == nil {
			continue
		}
		end := r.Open.Heartbeat
		if end < r.Open.Start {
			end = r.Open.Start
		}
		dur := end - r.Open.Start
		if end > r.LastPlayed {
			r.LastPlayed = end
		}
		if dur >= MinSessionSec {
			r.TotalSec += dur
			r.Recent = append(r.Recent, Session{Start: r.Open.Start, End: end, Seconds: dur})
			if len(r.Recent) > l.maxRecent {
				r.Recent = r.Recent[len(r.Recent)-l.maxRecent:]
			}
		}
		r.Open = nil
		if r.TotalSec == 0 && len(r.Recent) == 0 && r.LastPlayed == 0 {
			delete(l.data.Games, id)
		} else {
			l.data.Games[id] = r
		}
		changed = true
	}
	if changed {
		l.pruneGames()
		_ = l.save()
	}
}

// Minutes is gameID's local total in whole minutes.
func (l *Log) Minutes(gameID string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return int(l.data.Games[gameID].TotalSec / 60)
}

// TotalSec is gameID's local total in seconds.
func (l *Log) TotalSec(gameID string) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.data.Games[gameID].TotalSec
}

// LastPlayed is when gameID last ran (a session started or ended there),
// unix seconds, or 0 when it never did.
func (l *Log) LastPlayed(gameID string) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.data.Games[gameID].LastPlayed
}

// Recent returns up to n of gameID's last closed sessions, oldest first.
// Sessions under MinSessionSec were never kept.
func (l *Log) Recent(gameID string, n int) []Session {
	l.mu.Lock()
	defer l.mu.Unlock()
	recent := l.data.Games[gameID].Recent
	if n <= 0 || n >= len(recent) {
		return append([]Session{}, recent...)
	}
	return append([]Session{}, recent[len(recent)-n:]...)
}

// pruneGames drops the games played longest ago past maxGames. Callers hold l.mu.
func (l *Log) pruneGames() {
	if len(l.data.Games) <= l.maxGames {
		return
	}
	ids := make([]string, 0, len(l.data.Games))
	for id, r := range l.data.Games {
		if r.Open != nil {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return l.data.Games[ids[i]].LastPlayed < l.data.Games[ids[j]].LastPlayed
	})
	for len(l.data.Games) > l.maxGames && len(ids) > 0 {
		delete(l.data.Games, ids[0])
		ids = ids[1:]
	}
}

// save writes the file atomically. Callers hold l.mu.
func (l *Log) save() error {
	l.data.Version = fileVersion
	raw, err := json.MarshalIndent(l.data, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".sessions-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), l.path)
}
