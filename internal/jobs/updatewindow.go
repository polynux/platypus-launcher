package jobs

import (
	"time"

	"platypus/internal/settings"
	"platypus/internal/types"
)

// Update schedule (issue #207): a daily local-time window when queued
// updates may start. Installs, verifies, uninstalls and the rest are never
// held; only KindUpdate waits, since that is the download a user schedules
// for an off-peak hour.

// SetUpdateWindow installs the window the queue holds update jobs to. The
// zero Start/End (or equal times) means all day; a disabled window holds
// nothing. A job already queued starts at once when the change opens the
// window.
func (q *Queue) SetUpdateWindow(w settings.UpdateWindow) {
	q.mu.Lock()
	q.updateWindow = w
	q.mu.Unlock()
	q.signal()
}

// waitsForWindow reports whether j may not start now, and when the next
// window opens. Callers hold q.mu.
func (q *Queue) waitsForWindow(j *types.Job) (time.Time, bool) {
	if j.Kind != KindUpdate || !q.updateWindow.Enabled {
		return time.Time{}, false
	}
	start, errA := settings.ParseClock(q.updateWindow.Start)
	end, errB := settings.ParseClock(q.updateWindow.End)
	if errA != nil || errB != nil {
		return time.Time{}, false // a bad stored value holds nothing
	}
	now := q.now()
	if WindowOpen(start, end, settings.MinutesIntoDay(now)) {
		return time.Time{}, false
	}
	return NextWindowOpen(start, end, now), true
}

// WindowOpen says whether minute (since midnight) is inside the window
// start..end, inclusive start and exclusive end. start == end is all day; a
// start after the end crosses midnight (22:00–06:00).
func WindowOpen(start, end, minute int) bool {
	switch {
	case start == end:
		return true
	case start < end:
		return minute >= start && minute < end
	default:
		return minute >= start || minute < end
	}
}

// NextWindowOpen is the next time the window opens after now: today's start
// when it is still ahead, tomorrow's otherwise.
func NextWindowOpen(start, end int, now time.Time) time.Time {
	now = now.Local()
	at := time.Date(now.Year(), now.Month(), now.Day(), start/60, start%60, 0, 0, now.Location())
	if settings.MinutesIntoDay(now) >= start {
		at = at.AddDate(0, 0, 1)
	}
	return at
}
