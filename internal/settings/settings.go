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

	"hydra-launcher/internal/types"
)

const fileVersion = 1

// PluginState is what the user decided for a store plugin. Hash pins the
// plugin's executable: if it changes, the plugin stays disabled until the user
// approves it again.
type PluginState struct {
	Hash    string `json:"hash"`
	Enabled bool   `json:"enabled"`
	// InstallDir overrides <Games folder>/<install_dir> (absolute path).
	InstallDir string `json:"install_dir,omitempty"`
	// Folders are other folders the user allowed this plugin's games to be
	// launched from (games its CLI installed before Hydra).
	Folders []string `json:"folders,omitempty"`
}

type file struct {
	Version     int    `json:"version"`
	GamesFolder string `json:"games_folder,omitempty"` // "" = DefaultGamesFolder
	// Per-game settings exactly as the UI saved them, so fields the backend
	// doesn't model (sync mode, winetricks, …) survive a restart.
	Games   map[string]map[string]any `json:"games,omitempty"`
	Plugins map[string]PluginState    `json:"plugins,omitempty"`
	// Relocations are the game folders core moved, by game id (see
	// types.Relocation): the plugin's own records still name the old folder.
	Relocations map[string]types.Relocation `json:"relocations,omitempty"`
	// Games the user added by hand (sideloaded executables), with their
	// per-game settings in Games like every other game's.
	CustomGames []CustomGame `json:"custom_games,omitempty"`
	// CancelCleanupDays is how long a canceled download's partial data is
	// kept before Hydra deletes it. A pointer: 0 (delete at once) is a
	// choice, unset means DefaultCancelCleanupDays.
	CancelCleanupDays *int `json:"cancel_cleanup_days,omitempty"`
	// Cloud saves: each plugin's default mode and each game's own choice
	// (see cloud.go).
	CloudPlugins map[string]string `json:"cloud_plugins,omitempty"`
	CloudGames   map[string]string `json:"cloud_games,omitempty"`
	// Pins hold a game at a version (the `versions` capability), keyed by
	// Hydra's game id. See pins.go.
	Pins map[string]types.VersionPin `json:"pins,omitempty"`
	// InstallOptions are each game's last install-option choices (the
	// `install-options` capability), keyed by Hydra's game id. See options.go.
	InstallOptions map[string]types.InstallOptions `json:"install_options,omitempty"`
	// Terms are the term versions the user accepted, per game id: a record
	// per (term id, version), kept as history (terms.go).
	Terms map[string][]types.TermAcceptance `json:"terms,omitempty"`
	// Social: whether each plugin's friends list is on, by plugin id. A plugin
	// that offers social is on until the user switches it off (social.go).
	SocialPlugins map[string]bool `json:"social_plugins,omitempty"`
	// SocialPlaying: whether friends see the game the user plays, per plugin.
	// A plugin is on until the user switches it off (social.go).
	SocialPlaying map[string]bool `json:"social_playing,omitempty"`
	// ChatAlerts: whether the user hears about new messages from friends, per
	// plugin. On until the user switches it off (chat.go).
	ChatAlerts map[string]bool `json:"chat_alerts,omitempty"`
	// PluginSettings are each plugin's plugin-wide settings values, and
	// PluginGameSettings each game's per-game values (the `settings`
	// capability, see plugin_settings.go). Values never hold secrets.
	// GameFolders are extra library folders to install into, besides the Games
	// folder; RecentInstall the folders games were last installed into by hand,
	// newest first (see installfolders.go).
	GameFolders        []string                    `json:"game_folders,omitempty"`
	RecentInstall      []string                    `json:"recent_install_folders,omitempty"`
	PluginSettings     map[string]map[string]any   `json:"plugin_settings,omitempty"`
	PluginGameSettings map[string]PluginGameValues `json:"plugin_game_settings,omitempty"`
}

// CustomGame is a game the user added by hand: an executable on disk. Hydra
// runs it through the same runner pipeline as any other game.
type CustomGame struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Executable string `json:"executable"` // absolute path
	AddedAt    int64  `json:"added_at"`   // unix ms
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

// Canceled downloads keep their partial data this many days by default.
const (
	DefaultCancelCleanupDays = 7
	MaxCancelCleanupDays     = 365
)

// CancelCleanupDays is how many days a canceled download's data is kept.
func (s *Store) CancelCleanupDays() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.CancelCleanupDays == nil {
		return DefaultCancelCleanupDays
	}
	return *s.data.CancelCleanupDays
}

// SetCancelCleanupDays saves the retention of canceled downloads (0 = delete
// right away).
func (s *Store) SetCancelCleanupDays(days int) error {
	if days < 0 || days > MaxCancelCleanupDays {
		return fmt.Errorf("keep canceled downloads between 0 and %d days, got %d", MaxCancelCleanupDays, days)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.CancelCleanupDays
	s.data.CancelCleanupDays = &days
	if err := s.save(); err != nil {
		s.data.CancelCleanupDays = prev
		return err
	}
	return nil
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

// CustomGames returns the games added by hand, oldest first.
func (s *Store) CustomGames() []CustomGame {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]CustomGame{}, s.data.CustomGames...)
}

// AddCustomGame saves a hand-added game and returns it. The executable must
// exist, be a regular file and be absolute.
func (s *Store) AddCustomGame(title, executable string) (CustomGame, error) {
	title = strings.TrimSpace(title)
	executable = strings.TrimSpace(executable)
	if title == "" {
		return CustomGame{}, errors.New("the game needs a title")
	}
	if !filepath.IsAbs(executable) {
		return CustomGame{}, fmt.Errorf("executable must be an absolute path, got %q", executable)
	}
	real, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return CustomGame{}, fmt.Errorf("executable: %w", err)
	}
	st, err := os.Stat(real)
	if err != nil {
		return CustomGame{}, fmt.Errorf("executable: %w", err)
	}
	if !st.Mode().IsRegular() {
		return CustomGame{}, fmt.Errorf("%s is not a file", executable)
	}
	executable = real

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.data.CustomGames {
		if g.Executable == executable {
			return CustomGame{}, fmt.Errorf("%s is already in your library as %q", executable, g.Title)
		}
	}
	g := CustomGame{
		ID:         fmt.Sprintf("custom-%d", time.Now().UnixNano()),
		Title:      title,
		Executable: executable,
		AddedAt:    time.Now().UnixMilli(),
	}
	s.data.CustomGames = append(s.data.CustomGames, g)
	if err := s.save(); err != nil {
		s.data.CustomGames = s.data.CustomGames[:len(s.data.CustomGames)-1]
		return CustomGame{}, err
	}
	return g, nil
}

// RemoveCustomGame forgets a hand-added game; its files stay on disk.
func (s *Store) RemoveCustomGame(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, g := range s.data.CustomGames {
		if g.ID == id {
			prev := s.data.CustomGames
			prevSettings, hadSettings := s.data.Games[id]
			s.data.CustomGames = append(s.data.CustomGames[:i], s.data.CustomGames[i+1:]...)
			delete(s.data.Games, id)
			if err := s.save(); err != nil {
				s.data.CustomGames = prev
				if hadSettings {
					if s.data.Games == nil {
						s.data.Games = map[string]map[string]any{}
					}
					s.data.Games[id] = prevSettings
				}
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("unknown game %q", id)
}

// PluginState returns what the user decided for a plugin; ok is false if the
// plugin was never approved.
func (s *Store) PluginState(id string) (PluginState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.data.Plugins[id]
	return t, ok
}

func (s *Store) SetPluginState(id string, t PluginState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Plugins == nil {
		s.data.Plugins = map[string]PluginState{}
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
