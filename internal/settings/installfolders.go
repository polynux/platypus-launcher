package settings

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// MaxRecentInstall is how many folders Platypus remembers from "Install to…".
const MaxRecentInstall = 5

// MaxGameFolders caps the list of extra library folders.
const MaxGameFolders = 32

// CleanFolder expands "~/", requires an absolute path and cleans it. It does
// not touch the disk beyond refusing a path that is a file.
func CleanFolder(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("a folder must be an absolute path, got %q", path)
	}
	path = filepath.Clean(path)
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		return "", fmt.Errorf("%s is a file, not a folder", path)
	}
	return path, nil
}

// GameFolders are the extra library folders, in the order they were added.
// The Games folder itself is not in the list; it is always a library.
func (s *Store) GameFolders() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.data.GameFolders)
}

// AddGameFolder adds a library folder. The Games folder and one already in the
// list are not added twice; the list is returned either way.
func (s *Store) AddGameFolder(path string) ([]string, error) {
	path, err := CleanFolder(path)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if path == s.gamesFolderLocked() || slices.Contains(s.data.GameFolders, path) {
		return slices.Clone(s.data.GameFolders), nil
	}
	if len(s.data.GameFolders) >= MaxGameFolders {
		return nil, fmt.Errorf("at most %d game folders", MaxGameFolders)
	}
	prev := s.data.GameFolders
	s.data.GameFolders = append(slices.Clone(prev), path)
	if err := s.save(); err != nil {
		s.data.GameFolders = prev
		return nil, err
	}
	return slices.Clone(s.data.GameFolders), nil
}

// RemoveGameFolder takes a library folder off the list. The folder and the
// games in it stay on disk; a game installed there keeps working.
func (s *Store) RemoveGameFolder(path string) ([]string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.Index(s.data.GameFolders, path)
	if i < 0 {
		if path == s.gamesFolderLocked() {
			return nil, errors.New("the Games folder is the default one; change it in Settings instead")
		}
		return slices.Clone(s.data.GameFolders), nil
	}
	prev := s.data.GameFolders
	s.data.GameFolders = slices.Delete(slices.Clone(prev), i, i+1)
	if err := s.save(); err != nil {
		s.data.GameFolders = prev
		return nil, err
	}
	return slices.Clone(s.data.GameFolders), nil
}

func (s *Store) gamesFolderLocked() string {
	if s.data.GamesFolder == "" {
		return DefaultGamesFolder()
	}
	return s.data.GamesFolder
}

// RecentInstallFolders are the folders games were installed into by hand,
// newest first.
func (s *Store) RecentInstallFolders() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.data.RecentInstall)
}

// AddRecentInstallFolder remembers a folder as the newest, without a repeat.
// The folder is an absolute clean path the caller already checked.
func (s *Store) AddRecentInstallFolder(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("a recent folder must be a clean absolute path, got %q", path)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.RecentInstall
	next := append([]string{path}, slices.DeleteFunc(slices.Clone(prev), func(p string) bool { return p == path })...)
	if len(next) > MaxRecentInstall {
		next = next[:MaxRecentInstall]
	}
	if slices.Equal(prev, next) {
		return nil
	}
	s.data.RecentInstall = next
	if err := s.save(); err != nil {
		s.data.RecentInstall = prev
		return err
	}
	return nil
}

// ForgetRecentInstallFolder drops a folder from the recent list (it is gone, or
// the user doesn't want it offered).
func (s *Store) ForgetRecentInstallFolder(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.RecentInstall
	next := slices.DeleteFunc(slices.Clone(prev), func(p string) bool { return p == path })
	if len(next) == len(prev) {
		return nil
	}
	s.data.RecentInstall = next
	if err := s.save(); err != nil {
		s.data.RecentInstall = prev
		return err
	}
	return nil
}
