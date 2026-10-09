package settings

import (
	"errors"
	"fmt"

	"hydra-launcher/internal/types"
)

// maxPinID bounds a pinned channel or build id: they come from a plugin, and
// end up in settings.json and in later install requests.
const maxPinID = 256

// Pin returns the version a game is held at.
func (s *Store) Pin(gameID string) (types.VersionPin, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.data.Pins[gameID]
	return p, ok
}

// Pins returns every pin by game id.
func (s *Store) Pins() map[string]types.VersionPin {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]types.VersionPin, len(s.data.Pins))
	for id, p := range s.data.Pins {
		out[id] = p
	}
	return out
}

// SetPin holds a game at a build or channel.
func (s *Store) SetPin(gameID string, pin types.VersionPin) error {
	if gameID == "" {
		return errors.New("game id is empty")
	}
	if pin.Kind != types.PinBuild && pin.Kind != types.PinChannel {
		return fmt.Errorf("a pin is a %q or a %q, got %q", types.PinBuild, types.PinChannel, pin.Kind)
	}
	if pin.ID == "" || len(pin.ID) > maxPinID {
		return fmt.Errorf("a pin needs an id of at most %d characters", maxPinID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Pins == nil {
		s.data.Pins = map[string]types.VersionPin{}
	}
	prev, had := s.data.Pins[gameID]
	s.data.Pins[gameID] = pin
	if err := s.save(); err != nil {
		if had {
			s.data.Pins[gameID] = prev
		} else {
			delete(s.data.Pins, gameID)
		}
		return err
	}
	return nil
}

// ClearPin returns a game to following the latest version. Clearing a pin that
// isn't there is not an error.
func (s *Store) ClearPin(gameID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := s.data.Pins[gameID]
	if !had {
		return nil
	}
	delete(s.data.Pins, gameID)
	if err := s.save(); err != nil {
		s.data.Pins[gameID] = prev
		return err
	}
	return nil
}
