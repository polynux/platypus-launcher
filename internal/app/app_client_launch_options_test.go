package app

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"platypus/internal/types"
)

// A game the store client launches (issue #88) is offered only the runner
// choices the generated Steam launch-options line carries: the client picks
// the Proton version and owns the prefix, so the runner, sync and anti-cheat
// groups don't apply. A directly launched game keeps the whole schema.
func TestClientLaunchRunnerOptionsHideClientDecidedGroups(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	a.ctx = context.Background()

	ids := func(schema types.SettingsSchema) []string {
		out := make([]string, 0, len(schema.Groups))
		for _, g := range schema.Groups {
			out = append(out, g.ID)
		}
		return out
	}

	direct := types.Game{ID: "steam_440", Source: types.SourceSteam, Store: types.StoreSteam,
		StoreAppID: "440", Title: "Direct game", Executable: "/games/direct/game.exe"}
	got, err := a.GetRunnerOptions(direct)
	if err != nil {
		t.Fatal(err)
	}
	wantDirect := []string{"runner", "wrappers", "graphics", "sync", "proton", "gamescope", "anticheat"}
	if !reflect.DeepEqual(ids(got), wantDirect) {
		t.Fatalf("direct-launch groups = %v, want %v", ids(got), wantDirect)
	}

	client := direct
	client.ID = "steam_322170"
	client.StoreAppID = "322170"
	client.Title = "Client game"
	client.Executable = ""
	client.ClientLaunch = true
	got, err = a.GetRunnerOptions(client)
	if err != nil {
		t.Fatal(err)
	}
	wantClient := []string{"wrappers", "graphics", "proton", "gamescope"}
	if !reflect.DeepEqual(ids(got), wantClient) {
		t.Fatalf("client-launch groups = %v, want %v", ids(got), wantClient)
	}
}

// The hidden choices stay saved: a game can switch between client and direct
// launch (steam-standalone), and saving the visible choices must not drop
// what the panel hides.
func TestClientLaunchSaveKeepsHiddenChoices(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	a.ctx = context.Background()

	exe := writeExe(t, filepath.Join(home, "game.exe"), []byte("MZ\x90\x00"), true)
	cg, err := a.settings.AddCustomGame("Switchable", exe)
	if err != nil {
		t.Fatal(err)
	}
	saved := map[string]any{
		"runner_id": "proton-ge-11", "sync_mode": "esync", "winetricks": []any{"vcrun2022"},
		"proton_enable_nvapi": true, "env_vars": map[string]any{"FOO": "bar"},
	}
	if err := a.SaveGameSettings(cg.ID, saved); err != nil {
		t.Fatal(err)
	}

	// The client-launch form edits only what it shows; the save carries the
	// whole record, hidden values included.
	next := map[string]any{
		"runner_id": "proton-ge-11", "sync_mode": "esync", "winetricks": []any{"vcrun2022"},
		"proton_enable_nvapi": false, "proton_log": true, "env_vars": map[string]any{"FOO": "bar"},
	}
	if err := a.SaveGameSettings(cg.ID, next); err != nil {
		t.Fatal(err)
	}
	got := a.settings.GameSettings()[cg.ID]
	if got["runner_id"] != "proton-ge-11" || got["sync_mode"] != "esync" {
		t.Fatalf("hidden choices dropped: %v", got)
	}
	if !reflect.DeepEqual(got["winetricks"], []any{"vcrun2022"}) {
		t.Fatalf("hidden winetricks dropped: %v", got["winetricks"])
	}
	if got["proton_enable_nvapi"] != false || got["proton_log"] != true {
		t.Fatalf("visible choices not saved: %v", got)
	}
}

// The choices the panel keeps for a client launch still reach the generated
// Steam launch-options line (runner.env): Proton flags and the user's env.
func TestClientLaunchKeptChoicesReachRunnerEnv(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	a.ctx = context.Background()

	game := types.Game{ID: "steam_322170", Source: types.SourceSteam, Store: types.StoreSteam,
		StoreAppID: "322170", Title: "Client game", ClientLaunch: true,
		Settings:      types.GameSettings{UseGameMode: true, EnvVars: map[string]string{"FOO": "bar"}},
		SettingsExtra: map[string]any{"proton_enable_nvapi": true}}
	got, err := a.GetRunnerEnv(game)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"PROTON_ENABLE_NVAPI": "1", "FOO": "bar"}
	if !reflect.DeepEqual(got.Env, want) {
		t.Errorf("env = %v, want %v", got.Env, want)
	}
	if !reflect.DeepEqual(got.Wrappers, []string{"gamemode"}) {
		t.Errorf("wrappers = %v, want [gamemode]", got.Wrappers)
	}
}
