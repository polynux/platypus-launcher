package app

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"platypus/internal/stores"
	"platypus/internal/types"
)

// legacyRecord is a game saved by the old settings UI: its choices folded into
// env_vars and gamescope_args, with a key of its own the core doesn't model.
func legacyRecord() map[string]any {
	return map[string]any{
		"runner_id":      "auto",
		"use_mangohud":   true,
		"proton_flags":   []any{"PROTON_ENABLE_NVAPI"},
		"gamescope_args": "-f --extra",
		"env_vars": map[string]any{
			"PROTON_USE_NTSYNC": "1",
			"MANGOHUD_CONFIG":   "preset=2",
			"FOO":               "bar",
		},
	}
}

// A game saved by the old UI runs with the env the old UI gave it: the runner
// reads the saved keys and migrates the legacy ones in memory.
func TestLegacyGameSettingsRunWithTheSameEnv(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	a.ctx = context.Background()
	exe := writeExe(t, filepath.Join(home, "game.exe"), []byte("MZ\x90\x00"), true)
	cg, err := a.settings.AddCustomGame("Legacy", exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetGameSettings(cg.ID, legacyRecord()); err != nil {
		t.Fatal(err)
	}
	g := a.customGames()[0]
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

// At startup the runner plugin migrates each saved game and saves only the ones
// it changed. A second run changes nothing.
func TestStartupMigrationSavesOnlyChangedGames(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	exe := writeExe(t, filepath.Join(home, "game.exe"), []byte("MZ\x90\x00"), true)
	legacy, err := a.settings.AddCustomGame("Legacy", exe)
	if err != nil {
		t.Fatal(err)
	}
	other := writeExe(t, filepath.Join(home, "other.exe"), []byte("MZ\x90\x00"), true)
	current, err := a.settings.AddCustomGame("Current", other)
	if err != nil {
		t.Fatal(err)
	}
	currentRecord := map[string]any{"runner_id": "auto", "sync_mode": "esync", "env_vars": map[string]any{"FOO": "bar"}}
	if err := a.settings.SetGameSettings(legacy.ID, legacyRecord()); err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetGameSettings(current.ID, currentRecord); err != nil {
		t.Fatal(err)
	}

	a.migrateRunnerSettings(context.Background())
	got := a.settings.GameSettings()
	if got[legacy.ID]["sync_mode"] != "ntsync" || got[legacy.ID]["proton_enable_nvapi"] != true {
		t.Fatalf("legacy game after startup = %v, want its choices under their keys", got[legacy.ID])
	}
	if _, ok := got[legacy.ID]["proton_flags"]; ok {
		t.Error("the legacy proton_flags list is still saved")
	}
	if env := got[legacy.ID]["env_vars"].(map[string]any); env["FOO"] != "bar" || env["PROTON_USE_NTSYNC"] != nil {
		t.Errorf("env_vars after migration = %v, want only the user's FOO", env)
	}
	if !reflect.DeepEqual(got[current.ID], currentRecord) {
		t.Errorf("a game nothing changed in was saved: %v", got[current.ID])
	}

	before := a.settings.GameSettings()[legacy.ID]
	a.migrateRunnerSettings(context.Background())
	if !reflect.DeepEqual(a.settings.GameSettings()[legacy.ID], before) {
		t.Error("a second migration changed the game again")
	}
}

// Without the runner plugin nothing is migrated: the saved values wait for it.
func TestNoRunnerPluginMigratesNothing(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	exe := writeExe(t, filepath.Join(home, "game.exe"), []byte("MZ\x90\x00"), true)
	cg, err := a.settings.AddCustomGame("Legacy", exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetGameSettings(cg.ID, legacyRecord()); err != nil {
		t.Fatal(err)
	}
	a.plugins = nil
	a.migrateRunnerSettings(context.Background())
	if !reflect.DeepEqual(a.settings.GameSettings()[cg.ID], legacyRecord()) {
		t.Fatalf("the saved game changed without the runner plugin: %v", a.settings.GameSettings()[cg.ID])
	}
}

// A save keeps the keys the core doesn't model, so a plugin's choices persist.
func TestSaveGameSettingsKeepsUnmodeledKeys(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	exe := writeExe(t, filepath.Join(home, "game.exe"), []byte("MZ\x90\x00"), true)
	cg, err := a.settings.AddCustomGame("Saved", exe)
	if err != nil {
		t.Fatal(err)
	}
	saved := map[string]any{"runner_id": "auto", "proton_log": true, "sync_mode": "off", "gamescope_filter": "fsr"}
	if err := a.SaveGameSettings(cg.ID, saved); err != nil {
		t.Fatal(err)
	}
	if got := a.settings.GameSettings()[cg.ID]; !reflect.DeepEqual(got, saved) {
		t.Fatalf("saved = %v, want every key kept: %v", got, saved)
	}
	// And the game reads them back as its extra settings.
	if g := a.customGames()[0]; g.SettingsExtra["proton_log"] != true || g.SettingsExtra["sync_mode"] != "off" {
		t.Fatalf("extra settings = %v", g.SettingsExtra)
	}
}

func TestPluginGameSavedSettingsReachRunnerAndLibrary(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	a.ctx = context.Background()
	game := types.Game{ID: "echo_123", Source: "echo", Store: "echo", StoreAppID: "123", Title: "Plugin game",
		Executable: "/bin/true", Settings: types.GameSettings{UseGameMode: true}}
	a.stores = stores.NewRegistry(func() []stores.Store { return []stores.Store{&fakeSource{id: "echo", games: []types.Game{game}}} })
	runnerDir := filepath.Join(home, ".local/share/Steam/compatibilitytools.d/FakeProton")
	writeExe(t, filepath.Join(runnerDir, "proton"), []byte("#!/bin/sh\n"), true)
	saved := map[string]any{"runner_id": "compat:FakeProton", "env_vars": map[string]any{"PLUGIN_SETTING": "saved"}, "proton_log": true}
	if err := a.SaveGameSettings(game.ID, saved); err != nil {
		t.Fatal(err)
	}
	lib, err := a.GetLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Games) != 1 || lib.Games[0].Settings.RunnerID != saved["runner_id"] || lib.Games[0].SettingsExtra["proton_log"] != true {
		t.Fatalf("library game settings = %+v extra=%v", lib.Games, lib.Games[0].SettingsExtra)
	}

	// A stale game object must use settings.json at wrap time. The resulting
	// runner and environment prove both modeled and opaque saved values reached
	// runner.wrap.
	p, err := a.prepareLaunch(game, true)
	if err != nil {
		t.Fatalf("prepareLaunch: %v", err)
	}
	if p.runner.ID != "compat:FakeProton" {
		t.Fatalf("wrapped runner = %q, want compat:FakeProton", p.runner.ID)
	}
	if p.env["PLUGIN_SETTING"] != "saved" || p.env["PROTON_LOG"] != "1" {
		t.Fatalf("wrapped env = %v, want saved custom env and runner option", p.env)
	}
}
