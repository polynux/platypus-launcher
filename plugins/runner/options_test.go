package main

import (
	"reflect"
	"testing"

	"platypus/internal/types"
)

// The env and gamescope arguments the old UI wrote for the same choices (its
// derivedEnv and composeGamescopeArgs). The runner writes them now, so these
// expectations pin the behaviour users had.
func TestDerivedEnvMatchesWhatTheOldUIWrote(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		saved map[string]any
		env   map[string]string
	}{
		{"defaults write nothing", map[string]any{}, map[string]string{}},
		{"proton flag", map[string]any{"proton_enable_nvapi": true}, map[string]string{"PROTON_ENABLE_NVAPI": "1"}},
		{"esync", map[string]any{"sync_mode": "esync"}, map[string]string{"PROTON_NO_FSYNC": "1"}},
		{"ntsync", map[string]any{"sync_mode": "ntsync"}, map[string]string{"PROTON_USE_NTSYNC": "1"}},
		{"sync off writes both", map[string]any{"sync_mode": "off"}, map[string]string{"PROTON_NO_ESYNC": "1", "PROTON_NO_FSYNC": "1"}},
		{"wined3d", map[string]any{"dx_layer": "wined3d"}, map[string]string{"PROTON_USE_WINED3D": "1"}},
		{"fsr", map[string]any{"upscaler": "fsr1"}, map[string]string{"WINE_FULLSCREEN_FSR": "1"}},
		{"fps only", map[string]any{"mangohud_preset": "fps only"}, map[string]string{"MANGOHUD_CONFIG": "preset=1"}},
		{"minimal", map[string]any{"mangohud_preset": "minimal"}, map[string]string{"MANGOHUD_CONFIG": "preset=2"}},
		{"full", map[string]any{"mangohud_preset": "full"}, map[string]string{"MANGOHUD_CONFIG": "preset=4"}},
		{"shader cache off", map[string]any{"shader_cache": "off"}, map[string]string{"DXVK_STATE_CACHE": "0"}},
		{"a saved use_mangohud with no preset is full", map[string]any{"use_mangohud": true}, map[string]string{"MANGOHUD_CONFIG": "preset=4"}},
	}
	for _, c := range cases {
		if got := derivedEnv(choiceSet(c.saved)); !reflect.DeepEqual(got, c.env) {
			t.Errorf("%s: env = %v, want %v", c.name, got, c.env)
		}
	}
}

// The gamescope arguments are the switches in the old UI's order, then its -F
// filter, then the user's own arguments.
func TestGamescopeArgsMatchTheOldUIOrder(t *testing.T) {
	t.Parallel()
	c := choiceSet{"gamescope_vrr": true, "gamescope_fullscreen": true, "gamescope_wayland": true, "gamescope_filter": "fsr"}
	if got, want := gamescopeArgs(c, "--foo  --bar=1"), "-f --adaptive-sync --expose-wayland -F fsr --foo --bar=1"; got != want {
		t.Fatalf("gamescope args = %q, want %q", got, want)
	}
}

// The launch uses the choices: size and rate from the selects, the switches as
// arguments, and the user's env_vars win over a derived value.
func TestEffectiveSettingsAppliesTheChoices(t *testing.T) {
	t.Parallel()
	s := effectiveSettings(types.GameSettings{UseGamescope: true}, map[string]any{
		"gamescope_resolution": "1920x1080", "gamescope_rate": "144",
		"mangohud_preset": "full", "dx_layer": "wined3d",
		"env_vars": map[string]any{"MANGOHUD_CONFIG": "cpu_temp", "FOO": "bar"},
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

// A game saved by the old UI launches with the env the old UI gave it: the
// legacy keys are migrated in memory, and the migrated values are what counts.
func TestLegacySavedKeysLaunchAsBefore(t *testing.T) {
	t.Parallel()
	s := effectiveSettings(types.GameSettings{UseGamescope: true}, map[string]any{
		"use_mangohud":   true,
		"proton_flags":   []any{"PROTON_ENABLE_NVAPI"},
		"gamescope_args": "-f --extra",
		"gamescope_w":    float64(1280), "gamescope_h": float64(720),
		"env_vars": map[string]any{"PROTON_USE_NTSYNC": "1", "MANGOHUD_CONFIG": "preset=2", "FOO": "bar"},
	})
	want := map[string]string{"PROTON_ENABLE_NVAPI": "1", "PROTON_USE_NTSYNC": "1", "MANGOHUD_CONFIG": "preset=2", "FOO": "bar"}
	if !reflect.DeepEqual(s.EnvVars, want) {
		t.Errorf("env = %v, want %v", s.EnvVars, want)
	}
	if s.GamescopeArgs != "-f --extra" {
		t.Errorf("gamescope args = %q, want the switch and the extra argument", s.GamescopeArgs)
	}
	if s.GamescopeW != 1280 || s.GamescopeH != 720 {
		t.Errorf("gamescope size = %dx%d, want the legacy 1280x720", s.GamescopeW, s.GamescopeH)
	}
}

// runner.env gives the env and wrappers the Steam launch options spell out.
func TestRunnerEnvReportsEnvAndWrappers(t *testing.T) {
	t.Parallel()
	got := handle(request("runner.env", map[string]any{
		"game":    map[string]any{"id": "g", "settings": map[string]any{"use_gamemode": true}},
		"options": map[string]any{"proton_log": true, "mangohud_preset": "minimal"},
	}))
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

// The automatic runner names the default runner it picks, or says none is installed.
func TestAutomaticRunnerNamesItsPick(t *testing.T) {
	defer setLocale("en")
	setLocale("en")
	if got := autoLabel([]types.ProtonVersion{{ID: "a", Name: "GE-Proton 9", IsDefault: true}}); got != "Automatic (uses GE-Proton 9)" {
		t.Errorf("with a default = %q", got)
	}
	if got := autoLabel(nil); got != "Automatic (no runner installed)" {
		t.Errorf("with none = %q", got)
	}
	setLocale("fr")
	if got := autoLabel(nil); got != "Automatique (aucun runner installé)" {
		t.Errorf("fr with none = %q", got)
	}
}
