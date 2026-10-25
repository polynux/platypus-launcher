package settings

import (
	"fmt"
	"slices"

	"platypus/internal/apperr"
)

// Colour schemes (issue #134): the dark, light and black looks of a theme
// that declares them. Empty means dark. Themes without schemes are dark only.
const (
	ColorSchemeDark  = "dark"
	ColorSchemeLight = "light"
	ColorSchemeBlack = "black"
)

// colorSchemes are the values the setting takes. Empty means dark.
var colorSchemes = []string{"", ColorSchemeDark, ColorSchemeLight, ColorSchemeBlack}

// ColorScheme is the colour scheme choice: "" means dark.
func (s *Store) ColorScheme() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.ColorScheme
}

// SetColorScheme saves the colour-scheme choice ("" for dark).
func (s *Store) SetColorScheme(scheme string) error {
	if !slices.Contains(colorSchemes, scheme) {
		return apperr.New("settings.colorScheme.invalid", fmt.Sprintf("colour scheme must be %q, %q, %q or empty, got %q", ColorSchemeDark, ColorSchemeLight, ColorSchemeBlack, scheme),
			map[string]string{"scheme": scheme})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.ColorScheme
	s.data.ColorScheme = scheme
	if err := s.save(); err != nil {
		s.data.ColorScheme = prev
		return err
	}
	return nil
}
