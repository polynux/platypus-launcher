// Package settings persists launcher settings, per-game settings and plugin
// trust in ~/.local/share/hydra/settings.json.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const fileVersion = 1

// PluginTrust is what the user approved for a store plugin. Hash pins the
// plugin's executable: if it changes, the plugin stays disabled until the user
// approves it again.
type PluginTrust struct {
	Hash    string `json:"hash"`
	Enabled bool   `json:"enabled"`
}

type file struct {
	Version     int    `json:"version"`
	GamesFolder string `json:"games_folder,omitempty"` // "" = DefaultGamesFolder
	// Per-game settings exactly as the UI saved them, so fields the backend
	// doesn't model (sync mode, winetricks, …) survive a restart.
	Games   map[string]map[string]any `json:"games,omitempty"`
	Plugins map[string]PluginTrust    `json:"plugins,omitempty"`
}

// Store is the settings file, loaded once and written through on every change.
type Store struct {
	path string
	mu   sync.Mutex
	data file
}

// DataDir is Hydra's data folder, ~/.local/share/hydra.
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".local/share/hydra"), nil
}

// Open loads the settings file at path. A missing file gives defaults. A file
// that can't be parsed is moved aside (settings.json.bad-<time>) rather than
// overwritten, and Open returns usable defaults along with the error.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s.data); err != nil {
		bad := fmt.Sprintf("%s.bad-%d", path, time.Now().Unix())
		_ = os.Rename(path, bad)
		s.data = file{}
		return s, fmt.Errorf("settings file is corrupt, moved to %s: %w", bad, err)
	}
	return s, nil
}

// DefaultGamesFolder is ~/Games.
func DefaultGamesFolder() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Games")
}

// GamesFolder is the base folder store plugins install into, each in its own
// subfolder.
func (s *Store) GamesFolder() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.GamesFolder == "" {
		return DefaultGamesFolder()
	}
	return s.data.GamesFolder
}

// SetGamesFolder validates and saves the Games folder. "~/" is expanded and an
// empty path resets to the default. The folder doesn't have to exist yet; it
// is created on the first install.
func (s *Store) SetGamesFolder(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	if path != "" {
		if !filepath.IsAbs(path) {
			return "", fmt.Errorf("games folder must be an absolute path, got %q", path)
		}
		path = filepath.Clean(path)
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return "", fmt.Errorf("%s is a file, not a folder", path)
		}
		if path == DefaultGamesFolder() {
			path = ""
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.GamesFolder
	s.data.GamesFolder = path
	if err := s.save(); err != nil {
		s.data.GamesFolder = prev
		return "", err
	}
	if path == "" {
		return DefaultGamesFolder(), nil
	}
	return path, nil
}

// GameSettings returns every saved per-game settings object, keyed by game ID.
func (s *Store) GameSettings() map[string]map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]map[string]any, len(s.data.Games))
	for id, g := range s.data.Games {
		out[id] = g
	}
	return out
}

// SetGameSettings saves one game's settings, replacing what was there.
func (s *Store) SetGameSettings(gameID string, settings map[string]any) error {
	if gameID == "" {
		return errors.New("game id is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Games == nil {
		s.data.Games = map[string]map[string]any{}
	}
	prev, had := s.data.Games[gameID]
	s.data.Games[gameID] = settings
	if err := s.save(); err != nil {
		if had {
			s.data.Games[gameID] = prev
		} else {
			delete(s.data.Games, gameID)
		}
		return err
	}
	return nil
}

// PluginTrust returns what the user approved for a plugin; ok is false if the
// plugin was never approved.
func (s *Store) PluginTrust(id string) (PluginTrust, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.data.Plugins[id]
	return t, ok
}

func (s *Store) SetPluginTrust(id string, t PluginTrust) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Plugins == nil {
		s.data.Plugins = map[string]PluginTrust{}
	}
	prev, had := s.data.Plugins[id]
	s.data.Plugins[id] = t
	if err := s.save(); err != nil {
		if had {
			s.data.Plugins[id] = prev
		} else {
			delete(s.data.Plugins, id)
		}
		return err
	}
	return nil
}

// save writes the file atomically (temp file + rename), so a crash mid-write
// never leaves a truncated settings file. Callers hold s.mu.
func (s *Store) save() error {
	s.data.Version = fileVersion
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".settings-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}
