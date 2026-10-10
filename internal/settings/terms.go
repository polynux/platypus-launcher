package settings

import (
	"errors"
	"slices"

	"hydra-launcher/internal/types"
)

// AcceptedTerms lists the term versions the user accepted for a game, oldest
// first. Each (term id, version) appears once.
func (s *Store) AcceptedTerms(gameID string) []types.TermAcceptance {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.data.Terms[gameID])
}

// AcceptTerms records the user's acceptance of term versions for a game. A
// version already on record keeps its first date. The caller has checked the
// versions against the store's current terms.
func (s *Store) AcceptTerms(gameID string, accepted []types.TermAcceptance) error {
	if gameID == "" {
		return errors.New("game id is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := s.data.Terms[gameID]
	list := slices.Clone(prev)
	for _, a := range accepted {
		if a.ID == "" || a.Version == "" {
			continue
		}
		if !slices.ContainsFunc(list, func(o types.TermAcceptance) bool { return o.ID == a.ID && o.Version == a.Version }) {
			list = append(list, a)
		}
	}
	if s.data.Terms == nil {
		s.data.Terms = map[string][]types.TermAcceptance{}
	}
	s.data.Terms[gameID] = list
	if err := s.save(); err != nil {
		if had {
			s.data.Terms[gameID] = prev
		} else {
			delete(s.data.Terms, gameID)
		}
		return err
	}
	return nil
}
