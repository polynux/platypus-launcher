package settings

import (
	"errors"

	"hydra-launcher/internal/types"
)

// Options returns the install-option choices the user made for a game, as
// saved. A game never asked has none.
func (s *Store) Options(gameID string) (types.InstallOptions, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.data.InstallOptions[gameID]
	return o, ok
}

// SetOptions remembers the choices for a game's next install or update.
// Options are checked by the caller against the store's groups first.
func (s *Store) SetOptions(gameID string, options types.InstallOptions) error {
	if gameID == "" {
		return errors.New("game id is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.InstallOptions == nil {
		s.data.InstallOptions = map[string]types.InstallOptions{}
	}
	prev, had := s.data.InstallOptions[gameID]
	if options = types.NormalizeChoices(options); options == nil {
		delete(s.data.InstallOptions, gameID)
	} else {
		s.data.InstallOptions[gameID] = options
	}
	if err := s.save(); err != nil {
		if had {
			s.data.InstallOptions[gameID] = prev
		} else {
			delete(s.data.InstallOptions, gameID)
		}
		return err
	}
	return nil
}
