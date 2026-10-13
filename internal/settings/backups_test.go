package settings

import (
	"path/filepath"
	"testing"
)

func TestBackupSettingsRoundTrip(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	s, err := OpenWithHome(filepath.Join(t.TempDir(), "settings.json"), home)
	if err != nil {
		t.Fatal(err)
	}
	if s.BackupsMode() != BackupsDestructive {
		t.Errorf("mode = %q, want destructive", s.BackupsMode())
	}
	if s.KeepBackups() != DefaultKeepBackups {
		t.Errorf("keep = %d, want %d", s.KeepBackups(), DefaultKeepBackups)
	}
	if !s.BackupAfterSession() {
		t.Error("after-session defaults on (unset means on)")
	}
	if s.BackupAfterSessionSet() {
		t.Error("after-session should be unset initially")
	}
	if got := s.ResolvedBackupsDir(filepath.Join(home, "data")); got != filepath.Join(home, "data", "backups") {
		t.Errorf("backups dir = %q", got)
	}
	if _, err := s.SetBackupsFolder(filepath.Join(home, "Sync", "backups")); err != nil {
		t.Fatal(err)
	}
	if err := s.SetKeepBackups(5); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBackupsMode(BackupsAll); err != nil {
		t.Fatal(err)
	}
	if err := s.SetBackupAfterSession(true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGameSaveBackup("g", "off"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGameSavePaths("g", []string{"/saves/a", "", "/saves/b"}); err != nil {
		t.Fatal(err)
	}
	if s.GameSaveBackup("g") != "off" {
		t.Errorf("per-game = %q", s.GameSaveBackup("g"))
	}
	if got := s.GameSavePaths("g"); len(got) != 2 {
		t.Errorf("paths = %v", got)
	}
	if _, err := s.SetBackupsFolder("relative/path"); err == nil {
		t.Error("relative backups folder accepted")
	}
	if err := s.SetKeepBackups(0); err == nil {
		t.Error("keep 0 accepted")
	}
	if err := s.SetKeepBackups(101); err == nil {
		t.Error("keep 101 accepted")
	}
	if err := s.SetBackupsMode("sometimes"); err == nil {
		t.Error("bad mode accepted")
	}
	if err := s.SetGameSaveBackup("g", "sometimes"); err == nil {
		t.Error("bad per-game mode accepted")
	}
}
