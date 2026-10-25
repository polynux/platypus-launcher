package settings

import (
	"fmt"
	"slices"

	"platypus/internal/apperr"
)

// GameToneAccent is the "Accent" choice (issue #123): the lighter of the two,
// so a theme that opts into the game's colour needs no stored pick to show it.
const GameToneAccent = "accent"

// GameToneWhole retints the whole app with the game's hue.
const GameToneWhole = "whole"

// GameToneOff leaves the theme's own colours alone (issue #134): no game
// colour, whatever the theme allows.
const GameToneOff = "off"

// gameTones are the values the setting takes. Empty means accent.
var gameTones = []string{"", GameToneOff, GameToneAccent, GameToneWhole}

// GameTone is the game colour choice (issues #123, #134): "" means accent.
// How far it applies is the theme's color.game-tone axis, not this value: the
// pick is kept across themes.
func (s *Store) GameTone() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.GameTone
}

// SetGameTone saves the game-colour choice ("" for accent, "off" for none).
func (s *Store) SetGameTone(tone string) error {
	if !slices.Contains(gameTones, tone) {
		return apperr.New("settings.gameTone.invalid", fmt.Sprintf("game colour must be %q, %q, %q or empty, got %q", GameToneOff, GameToneAccent, GameToneWhole, tone),
			map[string]string{"tone": tone})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.GameTone
	s.data.GameTone = tone
	if err := s.save(); err != nil {
		s.data.GameTone = prev
		return err
	}
	return nil
}
