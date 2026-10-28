package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultRunnerPersistsAndDefaultsEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenWithHome(filepath.Join(dir, "settings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.DefaultRunnerID() != "" {
		t.Fatalf("DefaultRunnerID = %q, want the automatic pick (empty)", s.DefaultRunnerID())
	}
	if err := s.SetDefaultRunnerID("steam:Proton 9.0"); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenWithHome(filepath.Join(dir, "settings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.DefaultRunnerID() != "steam:Proton 9.0" {
		t.Fatalf("reopened DefaultRunnerID = %q, want steam:Proton 9.0", reopened.DefaultRunnerID())
	}
	// Setting it back to the automatic pick clears it.
	if err := reopened.SetDefaultRunnerID(""); err != nil {
		t.Fatal(err)
	}
	again, err := OpenWithHome(filepath.Join(dir, "settings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if again.DefaultRunnerID() != "" {
		t.Fatalf("cleared DefaultRunnerID = %q", again.DefaultRunnerID())
	}
}

// A settings file written before the setting existed still loads: the missing
// key means the automatic pick, and the next save keeps it that way.
func TestOldSettingsFileHasNoDefaultRunner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	old := `{"version": 2, "game_folders": ["/games"], "theme": "atlas"}` + "\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := OpenWithHome(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.DefaultRunnerID() != "" {
		t.Fatalf("DefaultRunnerID = %q, want empty", s.DefaultRunnerID())
	}
	if s.Theme() != "atlas" {
		t.Fatalf("Theme = %q, want atlas (the old file must still load)", s.Theme())
	}
}
