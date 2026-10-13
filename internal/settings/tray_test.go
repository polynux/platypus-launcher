package settings

import (
	"path/filepath"
	"testing"
)

func TestMinimizeToTrayDefaultsOffAndPersists(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenWithHome(filepath.Join(dir, "settings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.MinimizeToTray() {
		t.Fatal("MinimizeToTray defaults to on, want off")
	}
	if err := s.SetMinimizeToTray(true); err != nil {
		t.Fatal(err)
	}
	if !s.MinimizeToTray() {
		t.Fatal("MinimizeToTray = false after setting on")
	}
	reopened, err := OpenWithHome(filepath.Join(dir, "settings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.MinimizeToTray() {
		t.Fatal("MinimizeToTray did not persist across reopen")
	}
	if err := reopened.SetMinimizeToTray(false); err != nil {
		t.Fatal(err)
	}
	if reopened.MinimizeToTray() {
		t.Fatal("MinimizeToTray = true after setting off")
	}
}
