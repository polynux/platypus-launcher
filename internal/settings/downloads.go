package settings

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"platypus/internal/apperr"
)

// Downloads settings (issue #207): a global speed cap and the daily window
// when automatic updates may start, plus the automatic-update switch and its
// per-game exclusions (issue #209). All are store-agnostic: the cap reaches
// the plugins that declare protocol 1.21, the window is enforced by the job
// queue for updates Platypus started by itself.

// UpdateWindow is a daily local-time window, "HH:MM" to "HH:MM", inclusive
// start and exclusive end. An end before the start crosses midnight
// (22:00–06:00); equal times mean all day.
type UpdateWindow struct {
	Enabled bool   `json:"enabled"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

const (
	DefaultUpdateWindowStart = "03:00"
	DefaultUpdateWindowEnd   = "06:00"
	// MaxDownloadLimit bounds the speed cap, in bytes per second (1 GiB/s).
	MaxDownloadLimit = 1 << 30
)

// DownloadLimit is the global download speed cap in bytes per second; 0
// means unlimited (unset in the file).
func (s *Store) DownloadLimit() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.DownloadLimit
}

// SetDownloadLimit saves the speed cap (0 = unlimited).
func (s *Store) SetDownloadLimit(bytesPerSec int64) error {
	if bytesPerSec < 0 || bytesPerSec > MaxDownloadLimit {
		return apperr.New("settings.downloadLimit.invalid",
			fmt.Sprintf("the download limit must be between 0 and %d bytes per second, got %d", MaxDownloadLimit, bytesPerSec),
			map[string]string{"max": strconv.FormatInt(MaxDownloadLimit, 10), "limit": strconv.FormatInt(bytesPerSec, 10)})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.DownloadLimit
	s.data.DownloadLimit = bytesPerSec
	if err := s.save(); err != nil {
		s.data.DownloadLimit = prev
		return err
	}
	return nil
}

// UpdateWindow is the daily window when queued updates may start. Unset in
// the file means the default (off, 03:00–06:00).
func (s *Store) UpdateWindow() UpdateWindow {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.UpdateWindow == nil {
		return UpdateWindow{Start: DefaultUpdateWindowStart, End: DefaultUpdateWindowEnd}
	}
	return *s.data.UpdateWindow
}

// SetUpdateWindow saves the update window, checking both clock times.
func (s *Store) SetUpdateWindow(w UpdateWindow) error {
	if _, err := ParseClock(w.Start); err != nil {
		return err
	}
	if _, err := ParseClock(w.End); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.UpdateWindow
	s.data.UpdateWindow = &w
	if err := s.save(); err != nil {
		s.data.UpdateWindow = prev
		return err
	}
	return nil
}

// ParseClock reads a local "HH:MM" time into minutes since midnight.
func ParseClock(s string) (int, error) {
	hh, mm, ok := strings.Cut(s, ":")
	h, errH := strconv.Atoi(hh)
	m, errM := strconv.Atoi(mm)
	if !ok || len(hh) != 2 || len(mm) != 2 || errH != nil || errM != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, apperr.New("settings.clock.invalid",
			fmt.Sprintf("time must look like HH:MM, got %q", s), map[string]string{"time": s})
	}
	return h*60 + m, nil
}

// MinutesIntoDay is a time's local clock reading in minutes, for window math.
func MinutesIntoDay(t time.Time) int {
	t = t.Local()
	return t.Hour()*60 + t.Minute()
}

// AutoUpdate is the "Automatic updates" switch (issue #209). Unset in the
// file means on, like the Steam client.
func (s *Store) AutoUpdate() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.AutoUpdate == nil || *s.data.AutoUpdate
}

// SetAutoUpdate saves the "Automatic updates" switch.
func (s *Store) SetAutoUpdate(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.AutoUpdate
	s.data.AutoUpdate = &on
	if err := s.save(); err != nil {
		s.data.AutoUpdate = prev
		return err
	}
	return nil
}

// GameAutoUpdate says whether one game is updated automatically. Only an
// explicit exclusion (the game's "Don't update automatically") turns it off.
func (s *Store) GameAutoUpdate(gameID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.data.AutoUpdateOff[gameID]
}

// SetGameAutoUpdate saves one game's automatic-update exclusion: off keeps
// the game out of automatic updates, on returns it to following the global
// switch.
func (s *Store) SetGameAutoUpdate(gameID string, on bool) error {
	if gameID == "" {
		return fmt.Errorf("game id is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.AutoUpdateOff
	prevHad := prev[gameID]
	if on {
		if prev == nil || !prev[gameID] {
			return nil
		}
		delete(prev, gameID)
	} else {
		next := make(map[string]bool, len(prev)+1)
		for id, off := range prev {
			next[id] = off
		}
		next[gameID] = true
		s.data.AutoUpdateOff = next
	}
	if err := s.save(); err != nil {
		if on && prevHad {
			prev[gameID] = true
		} else if !on {
			s.data.AutoUpdateOff = prev
		}
		return err
	}
	return nil
}
