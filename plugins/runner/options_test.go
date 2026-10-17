package main

import (
	"reflect"
	"testing"

	"platypus/internal/types"
)

// The env and gamescope arguments the old UI wrote for the same choices (its
// derivedEnv and composeGamescopeArgs). The runner now writes them, so these
// expectations pin the behaviour users had.
func TestDerivedEnvMatchesWhatTheOldUIWrote(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		s    types.GameSettings
		env  map[string]string
	}{
		{"defaults write nothing", types.GameSettings{}, map[string]string{}},
		{"proton flag", types.GameSettings{ProtonEnableNvapi: true}, map[string]string{"PROTON_ENABLE_NVAPI": "1"}},
		{"esync", types.GameSettings{SyncMode: "esync"}, map[string]string{"PROTON_NO_FSYNC": "1"}},
		{"ntsync", types.GameSettings{SyncMode: "ntsync"}, map[string]string{"PROTON_USE_NTSYNC": "1"}},
		{"sync off writes both", types.GameSettings{SyncMode: "off"}, map[string]string{"PROTON_NO_ESYNC": "1", "PROTON_NO_FSYNC": "1"}},
		{"wined3d", types.GameSettings{DxLayer: "wined3d"}, map[string]string{"PROTON_USE_WINED3D": "1"}},
		{"fsr", types.GameSettings{Upscaler: "fsr1"}, map[string]string{"WINE_FULLSCREEN_FSR": "1"}},
		{"fps only", types.GameSettings{MangoHudPreset: "fps only"}, map[string]string{"MANGOHUD_CONFIG": "preset=1"}},
		{"minimal", types.GameSettings{MangoHudPreset: "minimal"}, map[string]string{"MANGOHUD_CONFIG": "preset=2"}},
		{"full", types.GameSettings{MangoHudPreset: "full"}, map[string]string{"MANGOHUD_CONFIG": "preset=4"}},
		{"shader cache off", types.GameSettings{ShaderCache: "off"}, map[string]string{"DXVK_STATE_CACHE": "0"}},
		{"a saved use_mangohud with no preset is full", types.GameSettings{UseMangoHud: true}, map[string]string{"MANGOHUD_CONFIG": "preset=4"}},
	}
	for _, c := range cases {
		if got := derivedEnv(c.s); !reflect.DeepEqual(got, c.env) {
			t.Errorf("%s: env = %v, want %v", c.name, got, c.env)
		}
	}
}

// The gamescope arguments are the switches in the old UI's order, then its -F
// filter, then the user's own arguments.
func TestGamescopeArgsMatchTheOldUIOrder(t *testing.T) {
	t.Parallel()
	s := types.GameSettings{
		GamescopeVRR: true, GamescopeFullscreen: true, GamescopeWayland: true,
		GamescopeFilter: "fsr", GamescopeArgs: "--foo  --bar=1",
	}
	if got, want := gamescopeArgs(s), "-f --adaptive-sync --expose-wayland -F fsr --foo --bar=1"; got != want {
		t.Fatalf("gamescope args = %q, want %q", got, want)
	}
}

// The launch uses the choices: size and rate from the selects, the switches as
// arguments, and the user's env_vars win over a derived value.
func TestEffectiveSettingsAppliesTheChoices(t *testing.T) {
	t.Parallel()
	s := effectiveSettings(types.GameSettings{
		UseGamescope: true, GamescopeResolution: "1920x1080", GamescopeRate: "144",
		MangoHudPreset: "full", DxLayer: "wined3d",
		EnvVars: map[string]string{"MANGOHUD_CONFIG": "cpu_temp", "FOO": "bar"},
	})
	if s.GamescopeW != 1920 || s.GamescopeH != 1080 || s.GamescopeR != 144 {
		t.Errorf("gamescope size/rate = %d×%d@%d", s.GamescopeW, s.GamescopeH, s.GamescopeR)
	}
	if !s.UseMangoHud {
		t.Error("a full preset must wrap the game in mangohud")
	}
	want := map[string]string{"PROTON_USE_WINED3D": "1", "MANGOHUD_CONFIG": "cpu_temp", "FOO": "bar"}
	if !reflect.DeepEqual(s.EnvVars, want) {
		t.Errorf("env = %v, want %v (the user's own MANGOHUD_CONFIG wins)", s.EnvVars, want)
	}
}

// runner.env gives the env and wrappers the Steam launch options spell out.
func TestRunnerEnvReportsEnvAndWrappers(t *testing.T) {
	t.Parallel()
	got := handle(request("runner.env", map[string]any{"game": map[string]any{"id": "g", "settings": map[string]any{
		"use_gamemode": true, "proton_log": true, "mangohud_preset": "minimal"}}}))
	if got.Error != nil {
		t.Fatal(got.Error)
	}
	out := got.Result.(map[string]any)
	env := out["env"].(map[string]string)
	if env["PROTON_LOG"] != "1" || env["MANGOHUD_CONFIG"] != "preset=2" {
		t.Errorf("env = %v", env)
	}
	if want := []string{"gamemode", "mangohud"}; !reflect.DeepEqual(out["wrappers"], want) {
		t.Errorf("wrappers = %v, want %v", out["wrappers"], want)
	}
}

// The schema and the catalogue speak the caller's language, falling back to English.
func TestRunnerOptionsFollowTheLocale(t *testing.T) {
	defer setLocale("en")
	setLocale("fr-FR")
	if got := text("sync_mode.label"); got != "Synchro" {
		t.Fatalf("fr label = %q", got)
	}
	setLocale("de")
	if got := text("sync_mode.label"); got != "Sync" {
		t.Fatalf("unknown locale label = %q, want the English one", got)
	}
}

// Every label the schema shows is translated, and the two catalogues have the
// same keys, so a language never falls back to English by accident.
func TestEverySchemaLabelIsTranslated(t *testing.T) {
	t.Parallel()
	if len(catalogs["en"]) == 0 || len(catalogs["fr"]) != len(catalogs["en"]) {
		t.Fatalf("catalogues: en has %d keys, fr %d", len(catalogs["en"]), len(catalogs["fr"]))
	}
	for k := range catalogs["en"] {
		if _, ok := catalogs["fr"][k]; !ok {
			t.Errorf("fr has no %q", k)
		}
	}
	for _, g := range runnerSchema()["groups"].([]any) {
		group := g.(map[string]any)
		if group["label"] == "" {
			t.Errorf("group %v has no label", group["id"])
		}
		for _, f := range group["fields"].([]any) {
			field := f.(map[string]any)
			if field["label"] == "" || field["label"] == nil {
				t.Errorf("field %v has no label in English", field["id"])
			}
			options, _ := field["options"].([]any)
			for _, o := range options {
				opt := o.(map[string]any)
				if opt["label"] == "" {
					t.Errorf("option %v of %v has no label", opt["id"], field["id"])
				}
			}
		}
	}
}
