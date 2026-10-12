package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// CORE-11: removing a hand-added game must also drop its per-game settings,
// or orphaned entries accumulate in settings.json.
func TestRemoveCustomGameDropsSettings(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	s, err := OpenWithHome(filepath.Join(t.TempDir(), "settings.json"), home)
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(home, "game", "start.sh")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	g, err := s.AddCustomGame("My Game", exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetGameSettings(g.ID, map[string]any{"runner_id": "auto"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveCustomGame(g.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.GameSettings()[g.ID]; ok {
		t.Fatalf("settings for removed game %s survived", g.ID)
	}
}
