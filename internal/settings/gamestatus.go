package settings

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"platypus/internal/types"
)

// MaxGameNotes caps one game's notes in bytes: notes live in settings.json,
// which is read whole on every load, so an unbounded field would bloat it.
const MaxGameNotes = 8 << 10

// GameStatus is the status the user gave a game, "" when none was set.
func (s *Store) GameStatus(gameID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.GameStatuses[gameID]
}

// SetGameStatus saves a game's status; "" clears it (back to none).
func (s *Store) SetGameStatus(gameID, status string) error {
	if gameID == "" {
		return errors.New("game id is empty")
	}
	if status != "" && !types.ValidGameStatus(status) {
		return fmt.Errorf("game status must be backlog, playing, finished or abandoned, got %q", status)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := s.data.GameStatuses[gameID]
	if status == "" {
		delete(s.data.GameStatuses, gameID)
	} else {
		if s.data.GameStatuses == nil {
			s.data.GameStatuses = map[string]string{}
		}
		s.data.GameStatuses[gameID] = status
	}
	if err := s.save(); err != nil {
		if had {
			if s.data.GameStatuses == nil {
				s.data.GameStatuses = map[string]string{}
			}
			s.data.GameStatuses[gameID] = prev
		} else {
			delete(s.data.GameStatuses, gameID)
		}
		return err
	}
	return nil
}

// GameNotes are the user's own notes on a game, plain text ("" when none).
func (s *Store) GameNotes(gameID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.GameNotes[gameID]
}

// SetGameNotes saves a game's notes; blank clears them. Notes are plain
// text: control characters other than newline and tab are refused.
func (s *Store) SetGameNotes(gameID, notes string) error {
	if gameID == "" {
		return errors.New("game id is empty")
	}
	notes = strings.TrimSpace(notes)
	if notes != "" {
		if len(notes) > MaxGameNotes {
			return fmt.Errorf("notes are longer than %d bytes", MaxGameNotes)
		}
		for _, r := range notes {
			if unicode.IsControl(r) && r != '\n' && r != '\t' {
				return errors.New("notes have a control character")
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := s.data.GameNotes[gameID]
	if notes == "" {
		delete(s.data.GameNotes, gameID)
	} else {
		if s.data.GameNotes == nil {
			s.data.GameNotes = map[string]string{}
		}
		s.data.GameNotes[gameID] = notes
	}
	if err := s.save(); err != nil {
		if had {
			if s.data.GameNotes == nil {
				s.data.GameNotes = map[string]string{}
			}
			s.data.GameNotes[gameID] = prev
		} else {
			delete(s.data.GameNotes, gameID)
		}
		return err
	}
	return nil
}

// GameStatusNotes returns every saved status and notes by game id, for
// decorating a whole library without taking the lock per game.
func (s *Store) GameStatusNotes() (statuses, notes map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	statuses = make(map[string]string, len(s.data.GameStatuses))
	for id, st := range s.data.GameStatuses {
		statuses[id] = st
	}
	notes = make(map[string]string, len(s.data.GameNotes))
	for id, n := range s.data.GameNotes {
		notes[id] = n
	}
	return statuses, notes
}
