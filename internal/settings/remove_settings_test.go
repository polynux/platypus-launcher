package settings

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/types"
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
	// Every per-game row the game may hold goes with it.
	if err := s.SetPin(g.ID, types.VersionPin{Kind: types.PinChannel, ID: "beta"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetOptions(g.ID, types.InstallOptions{"language": "fr"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCloudGameMode(g.ID, types.CloudOff); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGameStatus(g.ID, types.GameStatusBacklog); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGameNotes(g.ID, "cave behind the waterfall"); err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptTerms(g.ID, []types.TermAcceptance{{ID: "eula", Version: "3", At: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveCustomGame(g.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.GameSettings()[g.ID]; ok {
		t.Fatalf("settings for removed game %s survived", g.ID)
	}
	if _, ok := s.Pin(g.ID); ok {
		t.Fatalf("pin for removed game %s survived", g.ID)
	}
	if _, ok := s.Options(g.ID); ok {
		t.Fatalf("install options for removed game %s survived", g.ID)
	}
	if mode := s.CloudGameMode(g.ID); mode != "" {
		t.Fatalf("cloud mode for removed game %s survived: %q", g.ID, mode)
	}
	if status := s.GameStatus(g.ID); status != "" {
		t.Fatalf("status for removed game %s survived: %q", g.ID, status)
	}
	if notes := s.GameNotes(g.ID); notes != "" {
		t.Fatalf("notes for removed game %s survived: %q", g.ID, notes)
	}
	if got := s.AcceptedTerms(g.ID); len(got) != 0 {
		t.Fatalf("accepted terms for removed game %s survived: %v", g.ID, got)
	}
}
