package settings

import (
	"errors"
	"path/filepath"

	"platypus/internal/types"
)

// Relocation returns the folder core moved a game to, if it moved it.
func (s *Store) Relocation(gameID string) (types.Relocation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.data.Relocations[gameID]
	return r, ok
}

// SetRelocation records that a game now lives in r.To, not r.From. Both must
// be absolute.
func (s *Store) SetRelocation(gameID string, r types.Relocation) error {
	if gameID == "" {
		return errors.New("game id is empty")
	}
	if !filepath.IsAbs(r.From) || !filepath.IsAbs(r.To) {
		return errors.New("a relocation needs absolute folders")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Relocations == nil {
		s.data.Relocations = map[string]types.Relocation{}
	}
	prev, had := s.data.Relocations[gameID]
	s.data.Relocations[gameID] = r
	if err := s.save(); err != nil {
		if had {
			s.data.Relocations[gameID] = prev
		} else {
			delete(s.data.Relocations, gameID)
		}
		return err
	}
	return nil
}

// ClearRelocation forgets a relocation, once the plugin knows the new folder.
// Clearing one that isn't there is not an error.
func (s *Store) ClearRelocation(gameID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := s.data.Relocations[gameID]
	if !had {
		return nil
	}
	delete(s.data.Relocations, gameID)
	if err := s.save(); err != nil {
		s.data.Relocations[gameID] = prev
		return err
	}
	return nil
}
