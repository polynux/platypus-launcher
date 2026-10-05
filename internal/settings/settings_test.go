package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersistsAcrossOpen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "hydra", "settings.json")

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.GamesFolder(); got != DefaultGamesFolder() {
		t.Fatalf("default games folder = %q", got)
	}
	if _, err := s.SetGamesFolder("/mnt/fast/games/"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGameSettings("steam_620", map[string]any{"runner_id": "auto", "sync_mode": "ntsync"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPluginTrust("echo", PluginTrust{Hash: "abc", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.GamesFolder(); got != "/mnt/fast/games" {
		t.Errorf("games folder = %q", got)
	}
	if got := s.GameSettings()["steam_620"]["sync_mode"]; got != "ntsync" {
		t.Errorf("sync_mode = %v", got)
	}
	if tr, ok := s.PluginTrust("echo"); !ok || tr.Hash != "abc" || !tr.Enabled {
		t.Errorf("plugin trust = %+v, %v", tr, ok)
	}
}

func TestSetGamesFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	s, _ := Open(filepath.Join(t.TempDir(), "settings.json"))

	if got, err := s.SetGamesFolder("~/Library"); err != nil || got != filepath.Join(home, "Library") {
		t.Errorf("~ expansion: %q, %v", got, err)
	}
	if got, err := s.SetGamesFolder(""); err != nil || got != filepath.Join(home, "Games") {
		t.Errorf("reset: %q, %v", got, err)
	}
	if _, err := s.SetGamesFolder("relative/games"); err == nil {
		t.Error("relative path accepted")
	}
	f := filepath.Join(home, "file")
	os.WriteFile(f, nil, 0o644)
	if _, err := s.SetGamesFolder(f); err == nil {
		t.Error("file accepted as folder")
	}
}

func TestCorruptFileIsMovedAside(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, []byte("{not json"), 0o644)

	s, err := Open(path)
	if err == nil || s == nil {
		t.Fatalf("want defaults and an error, got %v, %v", s, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "settings.json.bad-") {
		t.Errorf("corrupt file not kept: %v", entries)
	}
}
