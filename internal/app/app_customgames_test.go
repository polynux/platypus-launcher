package app

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/settings"
)

// CORE-9: sideloaded games must keep settings keys the backend struct
// doesn't model, which settings.json preserves on disk.
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
	saved := map[string]any{"runner_id": "auto", "sync_mode": "ntsync", "some_future_key": "kept"}
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
	if got.Settings.SyncMode != "ntsync" {
		t.Fatalf("sync_mode = %q, want the typed ntsync", got.Settings.SyncMode)
	}
	if got.SettingsExtra["some_future_key"] != "kept" {
		t.Fatalf("an unknown key was lost: extra = %v", got.SettingsExtra)
	}
}
