package app

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/settings"
)

// CORE-9: sideloaded games must keep settings keys the backend struct
// doesn't model (e.g. sync_mode), which settings.json preserves on disk.
func TestCustomGamesPreserveUnknownSettings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "game.sh")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	g, err := st.AddCustomGame("My Game", exe)
	if err != nil {
		t.Fatal(err)
	}
	saved := map[string]any{"runner_id": "auto", "sync_mode": "ntsync"}
	if err := st.SetGameSettings(g.ID, saved); err != nil {
		t.Fatal(err)
	}

	a := &App{settings: st}
	games := a.customGames()
	if len(games) != 1 {
		t.Fatalf("customGames = %d games, want 1", len(games))
	}
	got := games[0]
	if got.Settings.RunnerID != "auto" {
		t.Fatalf("runner_id = %q, want auto", got.Settings.RunnerID)
	}
	if got.SettingsExtra["sync_mode"] != "ntsync" {
		t.Fatalf("sync_mode lost: extra = %v", got.SettingsExtra)
	}
}
