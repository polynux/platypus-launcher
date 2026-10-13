package settings

import (
	"fmt"
	"regexp"
	"strings"
)

var langRe = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// Language is the UI language the user picked (issue #39): "" means System,
// the default, resolved in the shell from the OS languages. Otherwise a BCP
// 47 tag like "fr" or "fr-CA"; the UI falls back to the base ("fr") and then
// to English when the tag isn't shipped.
func (s *Store) Language() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Language
}

// SetLanguage saves the UI language ("" for System). Tags are trimmed; the
// UI resolves the best supported match, so any well-formed tag is accepted.
func (s *Store) SetLanguage(lang string) error {
	lang = strings.TrimSpace(lang)
	if lang != "" && !langRe.MatchString(lang) {
		return fmt.Errorf("language must be empty (System) or a BCP 47 tag like fr, got %q", lang)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.Language
	s.data.Language = lang
	if err := s.save(); err != nil {
		s.data.Language = prev
		return err
	}
	return nil
}
