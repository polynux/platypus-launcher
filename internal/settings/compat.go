package settings

import (
	"errors"
	"strings"
)

var errBadAppID = errors.New("a Steam AppID is a number")

// RatingsOnline says whether Platypus may fetch compatibility ratings and
// anti-cheat data from the network. On until the user switches it off.
func (s *Store) RatingsOnline() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.FetchRatings == nil || *s.data.FetchRatings
}

// SetRatingsOnline saves the "Fetch ratings online" switch.
func (s *Store) SetRatingsOnline(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.FetchRatings
	s.data.FetchRatings = &on
	if err := s.save(); err != nil {
		s.data.FetchRatings = prev
		return err
	}
	return nil
}

// ArtworkOnline says whether Platypus may look cover and banner art up
// online for sideloaded games (issue #36). On until the user switches it
// off. When off, nothing goes out: neither the background lookup after an
// add nor an on-demand search.
func (s *Store) ArtworkOnline() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.FindArtwork == nil || *s.data.FindArtwork
}

// SetArtworkOnline saves the "Find artwork online" switch.
func (s *Store) SetArtworkOnline(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.FindArtwork
	s.data.FindArtwork = &on
	if err := s.save(); err != nil {
		s.data.FindArtwork = prev
		return err
	}
	return nil
}

// CompatAppID is the Steam AppID the user set for a game's ratings, "" when
// none was set.
func (s *Store) CompatAppID(gameID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.CompatAppIDs[gameID]
}

// SetCompatAppID saves the Steam AppID the user chose for a game's ratings.
// An empty id clears it. Only digits are kept: a Steam AppID is a number.
func (s *Store) SetCompatAppID(gameID, appID string) error {
	appID = strings.TrimSpace(appID)
	for _, r := range appID {
		if r < '0' || r > '9' {
			return errBadAppID
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := s.data.CompatAppIDs[gameID]
	if appID == "" {
		delete(s.data.CompatAppIDs, gameID)
	} else {
		if s.data.CompatAppIDs == nil {
			s.data.CompatAppIDs = map[string]string{}
		}
		s.data.CompatAppIDs[gameID] = appID
	}
	if err := s.save(); err != nil {
		if had {
			s.data.CompatAppIDs[gameID] = prev
		} else {
			delete(s.data.CompatAppIDs, gameID)
		}
		return err
	}
	return nil
}
