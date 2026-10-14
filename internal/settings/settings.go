// Package settings persists launcher settings, per-game settings and plugin
// trust in ~/.local/share/platypus/settings.json.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"platypus/internal/apperr"
	"platypus/internal/datadir"
	"platypus/internal/types"
)

// fileVersion 2 keeps the game folders in one list, the default first (see
// installfolders.go). Version 1 kept the default in games_folder.
const fileVersion = 2

// PluginState is what the user decided for a store plugin. Hash pins the
// plugin's executable: if it changes, the plugin stays disabled until the user
// approves it again.
type PluginState struct {
	Hash    string `json:"hash"`
	Enabled bool   `json:"enabled"`
	// InstallDir overrides <Games folder>/<install_dir> (absolute path).
	InstallDir string `json:"install_dir,omitempty"`
	// Folders are other folders the user allowed this plugin's games to be
	// launched from (games its CLI installed before Platypus).
	Folders []string `json:"folders,omitempty"`
}

type file struct {
	Version int `json:"version"`
	// GamesFolder is version 1's default game folder. Version 2 keeps it in
	// GameFolders; it is read once, by migrate.
	GamesFolder string `json:"games_folder,omitempty"`
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
	// kept before Platypus deletes it. A pointer: 0 (delete at once) is a
	// choice, unset means DefaultCancelCleanupDays.
	CancelCleanupDays *int `json:"cancel_cleanup_days,omitempty"`
	// Cloud saves: each plugin's default mode and each game's own choice
	// (see cloud.go).
	CloudPlugins map[string]string `json:"cloud_plugins,omitempty"`
	CloudGames   map[string]string `json:"cloud_games,omitempty"`
	// Pins hold a game at a version (the `versions` capability), keyed by
	// Platypus's game id. See pins.go.
	Pins map[string]types.VersionPin `json:"pins,omitempty"`
	// GameStatuses are the statuses the user gave games (issue #51), keyed
	// by Platypus's game id. See gamestatus.go.
	GameStatuses map[string]string `json:"game_statuses,omitempty"`
	// GameNotes are the user's own notes on games, plain text, keyed by
	// Platypus's game id. See gamestatus.go.
	GameNotes map[string]string `json:"game_notes,omitempty"`
	// InstallOptions are each game's last install-option choices (the
	// `install-options` capability), keyed by Platypus's game id. See options.go.
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
	// ChatDesktopNotify is the "Desktop notifications for chat" switch (issue
	// #12). A pointer: unset means on (chat.go).
	ChatDesktopNotify *bool `json:"chat_desktop_notify,omitempty"`
	// PluginSettings are each plugin's plugin-wide settings values, and
	// PluginGameSettings each game's per-game values (the `settings`
	// capability, see plugin_settings.go). Values never hold secrets.
	// GameFolders are the game folders, the default first: where installs go,
	// then the library folders (see installfolders.go). RecentInstall the
	// folders games were last installed into by hand, newest first.
	GameFolders        []string                    `json:"game_folders,omitempty"`
	RecentInstall      []string                    `json:"recent_install_folders,omitempty"`
	PluginSettings     map[string]map[string]any   `json:"plugin_settings,omitempty"`
	PluginGameSettings map[string]PluginGameValues `json:"plugin_game_settings,omitempty"`
	// FetchRatings is the "Fetch ratings online" switch. A pointer: unset
	// means on (see compat.go).
	FetchRatings *bool `json:"fetch_ratings,omitempty"`
	// FindArtwork is the "Find artwork online" switch (issue #36). A
	// pointer: unset means on, like FetchRatings.
	FindArtwork *bool `json:"find_artwork,omitempty"`
	// FindGameInfo is the "Find game info online" switch (issue #38). A
	// pointer: unset means on, like FindArtwork.
	FindGameInfo *bool `json:"find_game_info,omitempty"`
	// PrefixesFolder is where Wine prefixes are created (issue #37). Empty
	// means the default (<data>/prefixes).
	PrefixesFolder string `json:"prefixes_folder,omitempty"`
	// Language is the UI language the user picked (issue #39): "" means
	// System, resolved in the shell from the OS languages.
	Language string `json:"language,omitempty"`
	// MinimizeToTray is the "Minimize to tray while a game runs" switch
	// (issue #49). Off by default: a plain bool, unset means false.
	MinimizeToTray bool `json:"minimize_to_tray,omitempty"`
	// MigrationNoticeDismissed remembers that the user dismissed the one-time
	// Hydra→Platypus migration notice (issue #14), so it never returns.
	MigrationNoticeDismissed bool `json:"migration_notice_dismissed,omitempty"`
	// CompatAppIDs are the Steam AppIDs the user set for ratings, by Platypus's
	// game id (see compat.go).
	CompatAppIDs map[string]string `json:"compat_app_ids,omitempty"`
	// InfoSources are the online info links of sideloaded games, by
	// Platypus's game id (issue #38): "gog:<product id>" or
	// "lutris:<slug>". Steam info uses the compat AppID instead.
	InfoSources map[string]string `json:"info_sources,omitempty"`
	// BackupsFolder is the user's save-backups folder (issue #42). Empty
	// means the default (<dataDir>/backups).
	BackupsFolder string `json:"backups_folder,omitempty"`
	// KeepBackups is the versions kept per game (issue #42). 0 means the
	// default (10).
	KeepBackups int `json:"keep_backups,omitempty"`
	// BackupsMode is the global save-backup mode (issue #42): "destructive"
	// (default), "all" or "off". Empty means the default.
	BackupsMode string `json:"backups_mode,omitempty"`
	// BackupAfterSession runs a backup after each session. A pointer: unset
	// means on (backups for all games without cloud sync), false is an
	// explicit off.
	BackupAfterSession *bool `json:"backup_after_session,omitempty"`
	// GameSaveBackup is each game's backup override by Platypus game id
	// (issue #42): "" follows the global mode, "on" or "off".
	GameSaveBackup map[string]string `json:"game_save_backup,omitempty"`
	// GameSavePaths are each game's user-set save paths (issue #42).
	GameSavePaths map[string][]string `json:"game_save_paths,omitempty"`
}

// CustomGame is a game the user added by hand: an executable on disk. Platypus
// runs it through the same runner pipeline as any other game.
type CustomGame struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Executable string `json:"executable"` // absolute path
	AddedAt    int64  `json:"added_at"`   // unix ms
	// ReleaseYear is the year the game came out, 0 when unknown.
	ReleaseYear int `json:"release_year,omitempty"`
}

// ValidateReleaseYear says whether year is a usable release year: 0 clears
// it, otherwise 1970 through next year.
func ValidateReleaseYear(year int) error {
	if year == 0 {
		return nil
	}
	max := time.Now().Year() + 1
	if year < 1970 || year > max {
		return apperr.New("settings.year.invalid",
			fmt.Sprintf("release year must be between 1970 and %d, got %d", max, year),
			map[string]string{"max": strconv.Itoa(max), "year": strconv.Itoa(year)})
	}
	return nil
}

// Store is the settings file, loaded once and written through on every change.
type Store struct {
	path string
	mu   sync.Mutex
	data file
	// home expands "~" in folder settings (issue #41). Open fills it from
	// the process; tests use OpenWithHome with a temp dir.
	home string
}

// DataDir is Platypus's data folder, ~/.local/share/platypus.
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return DataDirFor(home), nil
}

// DataDirFor is the data folder for the user whose home is home (issue #41:
// tests pass temp dirs instead of setting HOME).
func DataDirFor(home string) string {
	return datadir.Path(home)
}

// Open loads the settings file at path. A missing file gives defaults. A file
// that can't be parsed is moved aside (settings.json.bad-<time>) rather than
// overwritten, and Open returns usable defaults along with the error.
func Open(path string) (*Store, error) {
	home, _ := os.UserHomeDir()
	return OpenWithHome(path, home)
}

// OpenWithHome is Open with an explicit home for "~" expansion (issue #41:
// tests pass temp dirs instead of setting HOME).
func OpenWithHome(path, home string) (*Store, error) {
	s := &Store{path: path, home: home}
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
	s.migrate()
	return s, nil
}

// DefaultGamesFolder is ~/Games.
func DefaultGamesFolder() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return DefaultGamesFolderFor(home)
}

// DefaultGamesFolderFor is ~/Games for the user whose home is home (issue
// #41: tests pass temp dirs instead of setting HOME).
func DefaultGamesFolderFor(home string) string {
	if home == "" {
		return ""
	}
	return filepath.Join(home, "Games")
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
		return apperr.New("settings.cleanupDays.invalid",
			fmt.Sprintf("keep canceled downloads between 0 and %d days, got %d", MaxCancelCleanupDays, days),
			map[string]string{"max": strconv.Itoa(MaxCancelCleanupDays), "days": strconv.Itoa(days)})
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
		return CustomGame{}, apperr.New("sideload.title.missing", "the game needs a title", nil)
	}
	if !filepath.IsAbs(executable) {
		return CustomGame{}, apperr.New("sideload.executable.notAbsolute",
			fmt.Sprintf("executable must be an absolute path, got %q", executable),
			map[string]string{"executable": executable})
	}
	real, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return CustomGame{}, apperr.Wrap("sideload.executable.unreadable",
			fmt.Errorf("executable: %w", err), map[string]string{"executable": executable})
	}
	st, err := os.Stat(real)
	if err != nil {
		return CustomGame{}, apperr.Wrap("sideload.executable.unreadable",
			fmt.Errorf("executable: %w", err), map[string]string{"executable": executable})
	}
	if !st.Mode().IsRegular() {
		return CustomGame{}, apperr.New("sideload.executable.notFile",
			fmt.Sprintf("%s is not a file", executable),
			map[string]string{"executable": executable})
	}
	executable = real

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.data.CustomGames {
		if g.Executable == executable {
			return CustomGame{}, apperr.New("sideload.duplicate",
				fmt.Sprintf("%s is already in your library as %q", executable, g.Title),
				map[string]string{"executable": executable, "title": g.Title})
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

// UpdateCustomGameExecutable points a hand-added game at a new executable:
// the same absolute, regular-file checks as AddCustomGame, re-detected on
// next read (issue #47). The duplicate check skips the game itself.
func (s *Store) UpdateCustomGameExecutable(id, executable string) (CustomGame, error) {
	executable = strings.TrimSpace(executable)
	if !filepath.IsAbs(executable) {
		return CustomGame{}, apperr.New("sideload.executable.notAbsolute",
			fmt.Sprintf("executable must be an absolute path, got %q", executable),
			map[string]string{"executable": executable})
	}
	real, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return CustomGame{}, apperr.Wrap("sideload.executable.unreadable",
			fmt.Errorf("executable: %w", err), map[string]string{"executable": executable})
	}
	st, err := os.Stat(real)
	if err != nil {
		return CustomGame{}, apperr.Wrap("sideload.executable.unreadable",
			fmt.Errorf("executable: %w", err), map[string]string{"executable": executable})
	}
	if !st.Mode().IsRegular() {
		return CustomGame{}, apperr.New("sideload.executable.notFile",
			fmt.Sprintf("%s is not a file", executable),
			map[string]string{"executable": executable})
	}
	executable = real

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.data.CustomGames {
		if g.ID != id && g.Executable == executable {
			return CustomGame{}, apperr.New("sideload.duplicate",
				fmt.Sprintf("%s is already in your library as %q", executable, g.Title),
				map[string]string{"executable": executable, "title": g.Title})
		}
	}
	for i, g := range s.data.CustomGames {
		if g.ID == id {
			prev := s.data.CustomGames[i].Executable
			s.data.CustomGames[i].Executable = executable
			if err := s.save(); err != nil {
				s.data.CustomGames[i].Executable = prev
				return CustomGame{}, err
			}
			return s.data.CustomGames[i], nil
		}
	}
	return CustomGame{}, apperr.New("sideload.unknown", fmt.Sprintf("unknown game %q", id), map[string]string{"id": id})
}

// RemoveCustomGame forgets a hand-added game; its files stay on disk. Its
// per-game rows go with it (settings, compat AppID, info source, version
// pin, install options, cloud mode, accepted terms, status and notes), so no
// orphaned entries accumulate in settings.json.
func (s *Store) RemoveCustomGame(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, g := range s.data.CustomGames {
		if g.ID == id {
			prev := s.data.CustomGames
			prevSettings, hadSettings := s.data.Games[id]
			prevCompat, hadCompat := s.data.CompatAppIDs[id]
			prevInfo, hadInfo := s.data.InfoSources[id]
			prevPin, hadPin := s.data.Pins[id]
			prevOptions, hadOptions := s.data.InstallOptions[id]
			prevCloud, hadCloud := s.data.CloudGames[id]
			prevTerms, hadTerms := s.data.Terms[id]
			prevStatus, hadStatus := s.data.GameStatuses[id]
			prevNotes, hadNotes := s.data.GameNotes[id]
			s.data.CustomGames = append(s.data.CustomGames[:i], s.data.CustomGames[i+1:]...)
			delete(s.data.Games, id)
			delete(s.data.CompatAppIDs, id)
			delete(s.data.InfoSources, id)
			delete(s.data.Pins, id)
			delete(s.data.InstallOptions, id)
			delete(s.data.CloudGames, id)
			delete(s.data.Terms, id)
			delete(s.data.GameStatuses, id)
			delete(s.data.GameNotes, id)
			if err := s.save(); err != nil {
				s.data.CustomGames = prev
				if hadSettings {
					if s.data.Games == nil {
						s.data.Games = map[string]map[string]any{}
					}
					s.data.Games[id] = prevSettings
				}
				if hadCompat {
					if s.data.CompatAppIDs == nil {
						s.data.CompatAppIDs = map[string]string{}
					}
					s.data.CompatAppIDs[id] = prevCompat
				}
				if hadInfo {
					if s.data.InfoSources == nil {
						s.data.InfoSources = map[string]string{}
					}
					s.data.InfoSources[id] = prevInfo
				}
				if hadPin {
					if s.data.Pins == nil {
						s.data.Pins = map[string]types.VersionPin{}
					}
					s.data.Pins[id] = prevPin
				}
				if hadOptions {
					if s.data.InstallOptions == nil {
						s.data.InstallOptions = map[string]types.InstallOptions{}
					}
					s.data.InstallOptions[id] = prevOptions
				}
				if hadCloud {
					if s.data.CloudGames == nil {
						s.data.CloudGames = map[string]string{}
					}
					s.data.CloudGames[id] = prevCloud
				}
				if hadTerms {
					if s.data.Terms == nil {
						s.data.Terms = map[string][]types.TermAcceptance{}
					}
					s.data.Terms[id] = prevTerms
				}
				if hadStatus {
					if s.data.GameStatuses == nil {
						s.data.GameStatuses = map[string]string{}
					}
					s.data.GameStatuses[id] = prevStatus
				}
				if hadNotes {
					if s.data.GameNotes == nil {
						s.data.GameNotes = map[string]string{}
					}
					s.data.GameNotes[id] = prevNotes
				}
				return err
			}
			return nil
		}
	}
	return apperr.New("sideload.unknown", fmt.Sprintf("unknown game %q", id), map[string]string{"id": id})
}

// SetCustomGameYear saves a hand-added game's release year (0 clears it:
// unknown). It validates the year like ValidateReleaseYear.
func (s *Store) SetCustomGameYear(id string, year int) error {
	if err := ValidateReleaseYear(year); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, g := range s.data.CustomGames {
		if g.ID == id {
			prev := s.data.CustomGames[i].ReleaseYear
			s.data.CustomGames[i].ReleaseYear = year
			if err := s.save(); err != nil {
				s.data.CustomGames[i].ReleaseYear = prev
				return err
			}
			return nil
		}
	}
	return apperr.New("sideload.unknown", fmt.Sprintf("unknown game %q", id), map[string]string{"id": id})
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
