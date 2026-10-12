package settings

import (
	"path/filepath"
	"strings"
)

// DefaultPrefixesFolder is where Wine prefixes are created unless the user
// chose another folder: <data>/prefixes, one folder per game inside it.
func DefaultPrefixesFolder() string {
	data, err := DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(data, "prefixes")
}

// PrefixesFolder is where Wine prefixes are created: the user's choice, or
// "" for the default (DefaultPrefixesFolder).
func (s *Store) PrefixesFolder() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.PrefixesFolder
}

// SetPrefixesFolder saves where Wine prefixes are created. An empty path
// resets it to the default. Anything else must be an absolute path; it is
// created on demand at launch, so it need not exist yet. Changing the folder
// never moves or deletes existing prefixes (App.SetPrefixesFolder pins the
// ones already there to their paths first). Returns the saved path.
func (s *Store) SetPrefixesFolder(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		prev := s.data.PrefixesFolder
		s.data.PrefixesFolder = ""
		if err := s.save(); err != nil {
			s.data.PrefixesFolder = prev
			return "", err
		}
		return "", nil
	}
	clean, err := CleanFolder(path)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.PrefixesFolder
	s.data.PrefixesFolder = clean
	if err := s.save(); err != nil {
		s.data.PrefixesFolder = prev
		return "", err
	}
	return clean, nil
}
