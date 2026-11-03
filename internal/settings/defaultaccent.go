package settings

import (
	"regexp"

	"platypus/internal/apperr"
)

var accentHex = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// DefaultAccent is the default accent colour; empty follows the theme.
func (s *Store) DefaultAccent() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !accentHex.MatchString(s.data.DefaultAccent) {
		return ""
	}
	return s.data.DefaultAccent
}

// SetDefaultAccent saves a #RRGGBB accent, or empty to follow the theme.
func (s *Store) SetDefaultAccent(color string) error {
	if color != "" && !accentHex.MatchString(color) {
		return apperr.New("settings.defaultAccent.invalid", "default accent must be #RRGGBB or empty", map[string]string{"color": color})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.DefaultAccent
	s.data.DefaultAccent = color
	if err := s.save(); err != nil {
		s.data.DefaultAccent = prev
		return err
	}
	return nil
}
