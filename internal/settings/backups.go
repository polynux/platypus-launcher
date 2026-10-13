package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"platypus/internal/apperr"
)

// Save-backup settings (issue #42). The backups live under <dataDir>/backups
// by default, never the cache dir; BackupsFolder may point at a synced folder.

const (
	// DefaultKeepBackups is the versions kept per game.
	DefaultKeepBackups = 10
	// MaxKeepBackups bounds the per-game history.
	MaxKeepBackups = 100
	// BackupWarnBytes warns above 1 GiB of saves; BackupRefuseBytes refuses above 10 GiB.
	BackupWarnBytes   = int64(1) << 30
	BackupRefuseBytes = int64(10) << 30
)

// Backups modes: "destructive" (default, before destructive actions only),
// "all" (also after sessions) and "off".
const (
	BackupsDestructive = "destructive"
	BackupsAll         = "all"
	BackupsOff         = "off"
)

// DefaultBackupsFolderFor is <dataDir>/backups for the data dir in use.
func DefaultBackupsFolderFor(dataDir string) string {
	return filepath.Join(dataDir, "backups")
}

// BackupsFolder is the user's backups folder, or "" for the default.
func (s *Store) BackupsFolder() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.BackupsFolder
}

// SetBackupsFolder saves the backups folder ("" restores the default). It is
// validated like the other folders: a clean absolute path.
func (s *Store) SetBackupsFolder(path string) (string, error) {
	clean := strings.TrimSpace(path)
	if clean == "" {
		s.mu.Lock()
		defer s.mu.Unlock()
		prev := s.data.BackupsFolder
		s.data.BackupsFolder = ""
		if err := s.save(); err != nil {
			s.data.BackupsFolder = prev
			return "", err
		}
		return "", nil
	}
	if clean == "~" || strings.HasPrefix(clean, "~/") {
		if s.home == "" {
			return "", apperr.New("settings.folder.noHome", "cannot expand ~ without a home folder", nil)
		}
		clean = filepath.Join(s.home, strings.TrimPrefix(clean, "~/"))
	}
	if !filepath.IsAbs(clean) || filepath.Clean(clean) != clean {
		return "", apperr.New("settings.folder.notAbsolute",
			fmt.Sprintf("folder %q is not a clean absolute path", path), map[string]string{"path": path})
	}
	if clean == "/" {
		return "", apperr.New("settings.folder.isRoot", "the backups folder cannot be the filesystem root", nil)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.BackupsFolder
	s.data.BackupsFolder = clean
	if err := s.save(); err != nil {
		s.data.BackupsFolder = prev
		return "", err
	}
	return clean, nil
}

// KeepBackups is the versions kept per game (default 10).
func (s *Store) KeepBackups() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.KeepBackups <= 0 {
		return DefaultKeepBackups
	}
	return s.data.KeepBackups
}

// SetKeepBackups saves how many versions are kept per game (1-100).
func (s *Store) SetKeepBackups(n int) error {
	if n < 1 || n > MaxKeepBackups {
		return apperr.New("settings.keepBackups.invalid",
			fmt.Sprintf("keep between 1 and %d backups, got %d", MaxKeepBackups, n),
			map[string]string{"keep": fmt.Sprintf("%d", n)})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.KeepBackups
	s.data.KeepBackups = n
	if err := s.save(); err != nil {
		s.data.KeepBackups = prev
		return err
	}
	return nil
}

// BackupsMode is the global backups mode (default "destructive").
func (s *Store) BackupsMode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.data.BackupsMode {
	case BackupsAll, BackupsOff:
		return s.data.BackupsMode
	default:
		return BackupsDestructive
	}
}

// SetBackupsMode saves the global backups mode.
func (s *Store) SetBackupsMode(mode string) error {
	switch mode {
	case "", BackupsDestructive, BackupsAll, BackupsOff:
	default:
		return apperr.New("settings.backupsMode.invalid",
			fmt.Sprintf("unknown backups mode %q", mode), map[string]string{"mode": mode})
	}
	if mode == "" {
		mode = BackupsDestructive
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.BackupsMode
	s.data.BackupsMode = mode
	if err := s.save(); err != nil {
		s.data.BackupsMode = prev
		return err
	}
	return nil
}

// BackupAfterSession says whether a backup runs after each session: on by
// default (unset means on), explicit false switches it off.
func (s *Store) BackupAfterSession() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.BackupAfterSession == nil {
		return true
	}
	return *s.data.BackupAfterSession
}

// BackupAfterSessionSet reports whether the user explicitly chose the
// after-session switch (unset means the default on).
func (s *Store) BackupAfterSessionSet() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.BackupAfterSession != nil
}

// SetBackupAfterSession saves the after-session switch.
func (s *Store) SetBackupAfterSession(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.BackupAfterSession
	v := on
	s.data.BackupAfterSession = &v
	if err := s.save(); err != nil {
		s.data.BackupAfterSession = prev
		return err
	}
	return nil
}

// GameSaveBackup is one game's backup override: "" follows the global mode,
// "on" always backs up, "off" never does.
func (s *Store) GameSaveBackup(gameID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.GameSaveBackup[gameID]
}

// SetGameSaveBackup saves one game's backup override ("" follows the global mode).
func (s *Store) SetGameSaveBackup(gameID, mode string) error {
	switch mode {
	case "", "on", "off":
	default:
		return apperr.New("settings.saveBackup.invalid",
			fmt.Sprintf("unknown per-game backup mode %q", mode), map[string]string{"mode": mode})
	}
	if gameID == "" {
		return apperr.New("settings.game.missing", "game id is empty", nil)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := s.data.GameSaveBackup[gameID]
	if s.data.GameSaveBackup == nil && mode != "" {
		s.data.GameSaveBackup = map[string]string{}
	}
	if mode == "" {
		if s.data.GameSaveBackup != nil {
			delete(s.data.GameSaveBackup, gameID)
		}
	} else {
		s.data.GameSaveBackup[gameID] = mode
	}
	if err := s.save(); err != nil {
		if s.data.GameSaveBackup == nil {
			s.data.GameSaveBackup = map[string]string{}
		}
		if had {
			s.data.GameSaveBackup[gameID] = prev
		} else {
			delete(s.data.GameSaveBackup, gameID)
		}
		return err
	}
	return nil
}

// GameSavePaths are the user-set save paths of one game (per-game override).
func (s *Store) GameSavePaths(gameID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.data.GameSavePaths[gameID]...)
}

// SetGameSavePaths saves one game's user save paths (raw strings, resolved later).
func (s *Store) SetGameSavePaths(gameID string, paths []string) error {
	if gameID == "" {
		return apperr.New("settings.game.missing", "game id is empty", nil)
	}
	cleaned := make([]string, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		cleaned = append(cleaned, p)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := s.data.GameSavePaths[gameID]
	if s.data.GameSavePaths == nil && len(cleaned) > 0 {
		s.data.GameSavePaths = map[string][]string{}
	}
	if len(cleaned) == 0 {
		if s.data.GameSavePaths != nil {
			delete(s.data.GameSavePaths, gameID)
		}
	} else {
		s.data.GameSavePaths[gameID] = cleaned
	}
	if err := s.save(); err != nil {
		if s.data.GameSavePaths == nil {
			s.data.GameSavePaths = map[string][]string{}
		}
		if had {
			s.data.GameSavePaths[gameID] = prev
		} else {
			delete(s.data.GameSavePaths, gameID)
		}
		return err
	}
	return nil
}

// resolvedBackupsDir is the backups folder in use: the user's choice or the default.
func (s *Store) resolvedBackupsDir(dataDir string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.BackupsFolder != "" {
		return s.data.BackupsFolder
	}
	return DefaultBackupsFolderFor(dataDir)
}

// ResolvedBackupsDir is the backups folder in use (exported for app/tests).
func (s *Store) ResolvedBackupsDir(dataDir string) string {
	return s.resolvedBackupsDir(dataDir)
}

var _ = os.Getenv // keep os import if unused in future edits
