package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersistsAcrossOpen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "platypus", "settings.json")

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
	if err := s.SetPluginState("echo", PluginState{Hash: "abc", Enabled: true}); err != nil {
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
	if tr, ok := s.PluginState("echo"); !ok || tr.Hash != "abc" || !tr.Enabled {
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

func TestCustomGames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(t.TempDir(), "settings.json")
	s, _ := Open(path)

	if _, err := s.AddCustomGame("My Game", "game.exe"); err == nil {
		t.Error("relative path accepted")
	}
	missing := filepath.Join(home, "nope.bin")
	if _, err := s.AddCustomGame("My Game", missing); err == nil {
		t.Error("missing executable accepted")
	}
	exe := filepath.Join(home, "game", "start.sh")
	os.MkdirAll(filepath.Dir(exe), 0o755)
	os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755)
	g, err := s.AddCustomGame("  My Game ", exe)
	if err != nil {
		t.Fatal(err)
	}
	if g.Title != "My Game" || g.Executable != exe || g.ID == "" {
		t.Fatalf("added: %+v", g)
	}
	if _, err := s.AddCustomGame("Again", exe); err == nil {
		t.Error("duplicate executable accepted")
	}
	// A symlink to the same file is the same game.
	link := filepath.Join(home, "game", "link.sh")
	os.Symlink(exe, link)
	if _, err := s.AddCustomGame("Link", link); err == nil {
		t.Error("symlink duplicate accepted")
	}

	if err := s.SetGameSettings(g.ID, map[string]any{"runner_id": "auto"}); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	list := s.CustomGames()
	if len(list) != 1 || list[0].ID != g.ID || list[0].Title != "My Game" {
		t.Fatalf("after reopen: %+v", list)
	}
	if s.GameSettings()[g.ID]["runner_id"] != "auto" {
		t.Error("custom game's settings not kept")
	}
	if err := s.RemoveCustomGame(g.ID); err != nil {
		t.Fatal(err)
	}
	if len(s.CustomGames()) != 0 {
		t.Error("not removed")
	}
	if err := s.RemoveCustomGame("nope"); err == nil {
		t.Error("unknown id removed")
	}
}
