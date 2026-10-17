package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// The old UI's saved shape: its choices folded into env_vars and gamescope_args,
// with the gamescope size and rate ints it wrote.
func legacyGame() map[string]any {
	return map[string]any{
		"use_mangohud":     true,
		"use_gamescope":    true,
		"gamescope_w":      float64(1920),
		"gamescope_h":      float64(1080),
		"gamescope_r":      float64(144),
		"gamescope_args":   "-f -F fsr --foo --bar=1",
		"proton_flags":     []any{"PROTON_ENABLE_NVAPI"},
		"launch_arguments": "-novid",
		"env_vars": map[string]any{
			"PROTON_USE_NTSYNC":   "1",
			"WINE_FULLSCREEN_FSR": "1",
			"MANGOHUD_CONFIG":     "preset=2",
			"DXVK_STATE_CACHE":    "0",
			"PROTON_USE_WINED3D":  "1",
			"FOO":                 "bar",
		},
	}
}

// The old UI's choices become typed keys, its env vars are gone, and the user's
// own env var stays.
func TestMigrateLegacyRecordIntoTypedKeys(t *testing.T) {
	m := legacyGame()
	if !migrateSettings(m) {
		t.Fatal("a legacy record reported no change")
	}
	want := map[string]any{
		"proton_enable_nvapi":  true,
		"sync_mode":            "ntsync",
		"upscaler":             "fsr1",
		"mangohud_preset":      "minimal",
		"shader_cache":         "off",
		"dx_layer":             "wined3d",
		"gamescope_fullscreen": true,
		"gamescope_filter":     "fsr",
		"gamescope_args":       "--foo --bar=1",
		"gamescope_resolution": "1920x1080",
		"gamescope_rate":       "144",
		"gamescope_w":          float64(0),
		"gamescope_h":          float64(0),
		"gamescope_r":          float64(0),
		"env_vars":             map[string]any{"FOO": "bar"},
		"use_mangohud":         true,
		"use_gamescope":        true,
		"launch_arguments":     "-novid",
	}
	if _, ok := m["proton_flags"]; ok {
		t.Fatal("the legacy proton_flags list is still there")
	}
	for k, v := range want {
		if !reflect.DeepEqual(m[k], v) {
			t.Errorf("%s = %#v, want %#v", k, m[k], v)
		}
	}
}

// A value the user typed by hand is never taken over, even where a legacy
// key with the same name exists.
func TestMigrationKeepsHandTypedEnv(t *testing.T) {
	m := map[string]any{
		"env_vars": map[string]any{
			"PROTON_NO_ESYNC":   "1", // with ntsync: not what the old UI wrote, so it stays
			"PROTON_USE_NTSYNC": "1",
			"MANGOHUD_CONFIG":   "cpu_temp", // a custom config, not a preset
			"DXVK_STATE_CACHE":  "1",
			"PROTON_LOG":        "2", // not the flag's 1, so it stays
		},
		"use_mangohud": true,
	}
	migrateSettings(m)
	env := m["env_vars"].(map[string]any)
	for k, v := range map[string]string{"PROTON_NO_ESYNC": "1", "MANGOHUD_CONFIG": "cpu_temp", "DXVK_STATE_CACHE": "1", "PROTON_LOG": "2"} {
		if env[k] != v {
			t.Errorf("%s = %v, want the hand-typed %q kept", k, env[k], v)
		}
	}
	if _, ok := env["PROTON_USE_NTSYNC"]; ok {
		t.Error("the ntsync value the old UI wrote was not taken over")
	}
	// No config preset was taken over, so the preset stays unset: the runner
	// then takes it from use_mangohud, which is what the old UI showed.
	if m["sync_mode"] != "ntsync" {
		t.Errorf("sync_mode = %v, want ntsync", m["sync_mode"])
	}
	if _, ok := m["mangohud_preset"]; ok {
		t.Errorf("mangohud_preset = %v was written for a hand-typed config", m["mangohud_preset"])
	}
}

// A record saved by the new UI is left as it is, and migrating again changes
// nothing.
func TestMigrationIsIdempotentAndLeavesTypedRecordsAlone(t *testing.T) {
	m := legacyGame()
	migrateSettings(m)
	before := map[string]any{}
	for k, v := range m {
		before[k] = v
	}
	if migrateSettings(m) {
		t.Fatal("a second migration changed a migrated record")
	}
	if !reflect.DeepEqual(before, m) {
		t.Fatal("a second migration changed the record")
	}

	typed := map[string]any{"sync_mode": "off", "env_vars": map[string]any{"PROTON_USE_NTSYNC": "1"}}
	migrateSettings(typed)
	if typed["sync_mode"] != "off" {
		t.Errorf("a typed sync_mode was overwritten: %v", typed["sync_mode"])
	}
	if got := typed["env_vars"].(map[string]any)["PROTON_USE_NTSYNC"]; got != "1" {
		t.Errorf("env var of a typed record was taken over: %v", got)
	}
}

// runner.migrateSettings takes every saved game in one request and answers with
// the games that changed only: a second request changes nothing.
func TestMigrateSettingsIsOneBatchOfChangedGames(t *testing.T) {
	legacy := legacyGame()
	current := map[string]any{"sync_mode": "esync", "env_vars": map[string]any{"FOO": "bar"}}
	resp := handle(request("runner.migrateSettings", map[string]any{"games": map[string]any{
		"legacy": legacy, "current": current, "empty": map[string]any{},
	}}))
	if resp.Error != nil {
		t.Fatalf("runner.migrateSettings: %+v", resp.Error)
	}
	raw, _ := json.Marshal(resp.Result)
	var out struct {
		Changed map[string]map[string]any `json:"changed"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Changed) != 1 || out.Changed["legacy"]["sync_mode"] != "ntsync" {
		t.Fatalf("changed = %v, want only the legacy game, migrated", out.Changed)
	}
	again := handle(request("runner.migrateSettings", map[string]any{"games": map[string]any{"legacy": out.Changed["legacy"]}}))
	raw, _ = json.Marshal(again.Result)
	if !strings.Contains(string(raw), `"changed":{}`) {
		t.Fatalf("a second batch changed %s, want nothing", raw)
	}
	if bad := handle(request("runner.migrateSettings", map[string]any{})); bad.Error == nil || bad.Error.Code != -32602 {
		t.Fatalf("no games: %+v, want -32602", bad.Error)
	}
}
