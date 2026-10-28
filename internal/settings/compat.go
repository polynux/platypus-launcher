package settings

import (
	"strings"

	"platypus/internal/apperr"
)

func errBadAppID() *apperr.Error {
	return apperr.New("steam.appid.invalid", "a Steam AppID is a number", nil)
}

// RatingsOnline says whether Platypus may fetch ProtonDB compatibility
// ratings from the network. On until the user switches it off. Protection
// facts (PCGamingWiki, AreWeAntiCheatYet) have their own switch since #150.
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

// ProtectionOnline says whether Platypus may fetch protection facts
// (PCGamingWiki and AreWeAntiCheatYet) from the network (issue #150). On
// until the user switches it off.
func (s *Store) ProtectionOnline() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.FetchProtection == nil || *s.data.FetchProtection
}

// SetProtectionOnline saves the "Fetch protection data online" switch.
func (s *Store) SetProtectionOnline(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.FetchProtection
	s.data.FetchProtection = &on
	if err := s.save(); err != nil {
		s.data.FetchProtection = prev
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

// GameInfoOnline says whether Platypus may look a sideloaded game's
// description and details up online (issue #38). On until the user switches
// it off. When off, nothing goes out: neither GetGameDetails for sideloaded
// games nor an explicit link.
func (s *Store) GameInfoOnline() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.FindGameInfo == nil || *s.data.FindGameInfo
}

// SetGameInfoOnline saves the "Find game info online" switch.
func (s *Store) SetGameInfoOnline(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.FindGameInfo
	s.data.FindGameInfo = &on
	if err := s.save(); err != nil {
		s.data.FindGameInfo = prev
		return err
	}
	return nil
}

// InfoSource is the online info link of a sideloaded game (issue #38):
// "gog:<product id>" or "lutris:<slug>", "" when none is set.
func (s *Store) InfoSource(gameID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.InfoSources[gameID]
}

// SetInfoSource saves the online info link of a sideloaded game. An empty
// source clears it. Otherwise source is "gog" or "lutris" and id the GOG
// product id or the Lutris slug.
func (s *Store) SetInfoSource(gameID, source, id string) error {
	source = strings.TrimSpace(strings.ToLower(source))
	id = strings.TrimSpace(id)
	var link string
	switch {
	case source == "" && id == "":
		link = ""
	case source == "gog" && id != "":
		link = "gog:" + id
	case source == "lutris" && id != "":
		link = "lutris:" + id
	default:
		return apperr.New("info.source.invalid", "an info source is gog or lutris", nil)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := s.data.InfoSources[gameID]
	if link == "" {
		delete(s.data.InfoSources, gameID)
	} else {
		if s.data.InfoSources == nil {
			s.data.InfoSources = map[string]string{}
		}
		s.data.InfoSources[gameID] = link
	}
	if err := s.save(); err != nil {
		if had {
			if s.data.InfoSources == nil {
				s.data.InfoSources = map[string]string{}
			}
			s.data.InfoSources[gameID] = prev
		} else {
			delete(s.data.InfoSources, gameID)
		}
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
			return errBadAppID()
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
