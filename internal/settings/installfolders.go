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

// MaxGameFolders caps the library folders, not counting the default one.
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

// folderListLocked is every game folder: the default one first, where installs
// go, then the library folders in the order they were added. Until the user
// changes anything the list is just ~/Games. Callers hold s.mu.
func (s *Store) folderListLocked() []string {
	if len(s.data.GameFolders) == 0 {
		return []string{DefaultGamesFolder()}
	}
	return s.data.GameFolders
}

// saveFoldersLocked stores a new game folder list and rolls it back if the file
// can't be written. Callers hold s.mu.
func (s *Store) saveFoldersLocked(list []string) error {
	prev := s.data.GameFolders
	s.data.GameFolders = list
	if err := s.save(); err != nil {
		s.data.GameFolders = prev
		return err
	}
	return nil
}

// GamesFolder is the default game folder: where a store's games install, each
// in its own subfolder.
func (s *Store) GamesFolder() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.folderListLocked()[0]
}

// GameFolders are the library folders: every game folder except the default
// one, in the order they were added.
func (s *Store) GameFolders() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.folderListLocked()[1:])
}

// SetGamesFolder makes a folder the default game folder. A folder not in the
// list yet is added; the old default stays in the list as a library folder. An
// empty path means ~/Games. The folder doesn't have to exist yet; it is created
// on the first install. Returns the default folder.
func (s *Store) SetGamesFolder(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultGamesFolder()
	}
	path, err := CleanFolder(path)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list := append([]string{path}, slices.DeleteFunc(slices.Clone(s.folderListLocked()), func(p string) bool { return p == path })...)
	if len(list)-1 > MaxGameFolders {
		return "", fmt.Errorf("at most %d game folders", MaxGameFolders)
	}
	if err := s.saveFoldersLocked(list); err != nil {
		return "", err
	}
	return path, nil
}

// AddGameFolder adds a library folder. A folder already in the list is not
// added twice; the library folders are returned either way.
func (s *Store) AddGameFolder(path string) ([]string, error) {
	path, err := CleanFolder(path)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.folderListLocked()
	if slices.Contains(list, path) {
		return slices.Clone(list[1:]), nil
	}
	if len(list)-1 >= MaxGameFolders {
		return nil, fmt.Errorf("at most %d game folders", MaxGameFolders)
	}
	next := append(slices.Clone(list), path)
	if err := s.saveFoldersLocked(next); err != nil {
		return nil, err
	}
	return slices.Clone(next[1:]), nil
}

// RemoveGameFolder takes a library folder off the list. The default folder
// can't be removed: make another one the default first. The folder and the
// games in it stay on disk; a game installed there keeps working.
func (s *Store) RemoveGameFolder(path string) ([]string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.folderListLocked()
	if path == list[0] {
		return nil, errors.New("the default game folder can't be removed; make another folder the default first")
	}
	i := slices.Index(list, path)
	if i < 0 {
		return slices.Clone(list[1:]), nil
	}
	next := slices.Delete(slices.Clone(list), i, i+1)
	if err := s.saveFoldersLocked(next); err != nil {
		return nil, err
	}
	return slices.Clone(next[1:]), nil
}

// migrate moves a version 1 file's folders into the list: the default folder
// (games_folder, or ~/Games when unset) comes first, then the library folders.
// The next save writes the list.
func (s *Store) migrate() {
	if s.data.Version >= 2 {
		return
	}
	def := s.data.GamesFolder
	if def == "" {
		def = DefaultGamesFolder()
	}
	list := []string{def}
	for _, p := range s.data.GameFolders {
		if !slices.Contains(list, p) {
			list = append(list, p)
		}
	}
	s.data.GamesFolder = ""
	s.data.GameFolders = list
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
