package app

import (
	"path/filepath"
	"reflect"
	"testing"
)

// A game saved by the old settings UI (its choices folded into env_vars and
// gamescope_args) gets the same env through the runner as it had: the migration
// moves the choices into typed keys, and the runner writes the env back out.
func TestLegacyGameSettingsRunWithTheSameEnv(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	exe := writeExe(t, filepath.Join(home, "game.exe"), []byte("MZ\x90\x00"), true)
	cg, err := a.settings.AddCustomGame("Legacy", exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetGameSettings(cg.ID, map[string]any{
		"runner_id":      "auto",
		"use_mangohud":   true,
		"proton_flags":   []any{"PROTON_ENABLE_NVAPI"},
		"gamescope_args": "-f --extra",
		"env_vars": map[string]any{
			"PROTON_USE_NTSYNC": "1",
			"MANGOHUD_CONFIG":   "preset=2",
			"FOO":               "bar",
		},
	}); err != nil {
		t.Fatal(err)
	}
	g := a.customGames()[0]
	if g.Settings.SyncMode != "ntsync" || g.Settings.MangoHudPreset != "minimal" {
		t.Fatalf("migrated settings = sync %q, mangohud %q", g.Settings.SyncMode, g.Settings.MangoHudPreset)
	}
	got, err := a.GetRunnerEnv(g)
	if err != nil {
		t.Fatal(err)
	}
	// What the old UI wrote for these choices: its derived env plus the user's own.
	want := map[string]string{
		"PROTON_ENABLE_NVAPI": "1",
		"PROTON_USE_NTSYNC":   "1",
		"MANGOHUD_CONFIG":     "preset=2",
		"FOO":                 "bar",
	}
	if !reflect.DeepEqual(got.Env, want) {
		t.Errorf("env = %v, want %v", got.Env, want)
	}
	// GameMode is on by default for a game that never saved it, as before.
	if !reflect.DeepEqual(got.Wrappers, []string{"gamemode", "mangohud"}) {
		t.Errorf("wrappers = %v, want [gamemode mangohud]", got.Wrappers)
	}
}
