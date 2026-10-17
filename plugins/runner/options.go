package main

import (
	"embed"
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"platypus/internal/runner"
	"platypus/internal/types"
)

// The runner's per-game choices (runner.options, protocol.md). The core stores
// them as typed game settings; effectiveSettings turns them into the env vars
// and gamescope arguments the launch uses, so the UI never builds them.

//go:embed locales/*.json
var localeFiles embed.FS

// locale is the user's language, from initialize and locale.changed (protocol
// 1.14). Labels are looked up in it, falling back to English key by key.
var locale = "en"

var catalogs = func() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, lang := range []string{"en", "fr"} {
		data, err := localeFiles.ReadFile("locales/" + lang + ".json")
		if err != nil {
			continue
		}
		m := map[string]string{}
		_ = json.Unmarshal(data, &m)
		out[lang] = m
	}
	return out
}()

// setLocale takes a BCP 47 tag ("fr-FR") and keeps its language part.
func setLocale(tag string) {
	lang, _, _ := strings.Cut(strings.ToLower(tag), "-")
	if _, ok := catalogs[lang]; ok {
		locale = lang
		return
	}
	locale = "en"
}

// text is the label for key in the current locale, else in English.
func text(key string) string {
	if s, ok := catalogs[locale][key]; ok {
		return s
	}
	return catalogs["en"][key]
}

// protonFlag is one Proton switch: the env var it sets to 1 and the setting that
// turns it on.
var protonFlags = []struct {
	env string
	on  func(types.GameSettings) bool
}{
	{"PROTON_ENABLE_NVAPI", func(s types.GameSettings) bool { return s.ProtonEnableNvapi }},
	{"PROTON_HIDE_NVIDIA_GPU", func(s types.GameSettings) bool { return s.ProtonHideNvidiaGpu }},
	{"PROTON_ENABLE_WAYLAND", func(s types.GameSettings) bool { return s.ProtonEnableWayland }},
	{"PROTON_NO_D3D11", func(s types.GameSettings) bool { return s.ProtonNoD3d11 }},
	{"PROTON_NO_D3D12", func(s types.GameSettings) bool { return s.ProtonNoD3d12 }},
	{"PROTON_FORCE_LARGE_ADDRESS_AWARE", func(s types.GameSettings) bool { return s.ProtonForceLargeAddressAware }},
	{"PROTON_OLD_GL_STRING", func(s types.GameSettings) bool { return s.ProtonOldGlString }},
	{"PROTON_LOG", func(s types.GameSettings) bool { return s.ProtonLog }},
}

// mangohudConfig is the MANGOHUD_CONFIG each preset writes.
var mangohudConfig = map[string]string{"fps only": "preset=1", "minimal": "preset=2", "full": "preset=4"}

// Defaults: an empty choice is the first option of its field.
func syncMode(s types.GameSettings) string {
	if s.SyncMode == "" {
		return "fsync"
	}
	return s.SyncMode
}

func dxLayer(s types.GameSettings) string {
	if s.DxLayer == "" {
		return "dxvk"
	}
	return s.DxLayer
}

func upscaler(s types.GameSettings) string {
	if s.Upscaler == "" {
		return "native"
	}
	return s.Upscaler
}

func shaderCache(s types.GameSettings) string {
	if s.ShaderCache == "" {
		return "on"
	}
	return s.ShaderCache
}

// mangohudPreset is the overlay the game runs with. A game with no preset
// saved takes use_mangohud's old meaning: on is full.
func mangohudPreset(s types.GameSettings) string {
	switch {
	case s.MangoHudPreset != "":
		return s.MangoHudPreset
	case s.UseMangoHud:
		return "full"
	}
	return "off"
}

// derivedEnv is the env the runner's choices set: Proton's switches, sync, the
// DX layer, the upscaler, MangoHud and the shader cache.
func derivedEnv(s types.GameSettings) map[string]string {
	env := map[string]string{}
	for _, f := range protonFlags {
		if f.on(s) {
			env[f.env] = "1"
		}
	}
	switch syncMode(s) {
	case "esync":
		env["PROTON_NO_FSYNC"] = "1"
	case "ntsync":
		env["PROTON_USE_NTSYNC"] = "1"
	case "off":
		env["PROTON_NO_ESYNC"], env["PROTON_NO_FSYNC"] = "1", "1"
	}
	if dxLayer(s) == "wined3d" {
		env["PROTON_USE_WINED3D"] = "1"
	}
	if upscaler(s) == "fsr1" {
		env["WINE_FULLSCREEN_FSR"] = "1"
	}
	if p := mangohudPreset(s); p != "off" {
		env["MANGOHUD_CONFIG"] = mangohudConfig[p]
	}
	if shaderCache(s) == "off" {
		env["DXVK_STATE_CACHE"] = "0"
	}
	return env
}

// gamescopeArgs is the gamescope arguments the switches and filter give, then
// the user's own gamescope_args: the order the old UI wrote them in.
func gamescopeArgs(s types.GameSettings) string {
	var parts []string
	for _, sw := range []struct {
		on   bool
		flag string
	}{
		{s.GamescopeFullscreen, "-f"},
		{s.GamescopeBorderless, "-b"},
		{s.GamescopeGrabCursor, "--force-grab-cursor"},
		{s.GamescopeHDR, "--hdr-enabled"},
		{s.GamescopeVRR, "--adaptive-sync"},
		{s.GamescopeWayland, "--expose-wayland"},
	} {
		if sw.on {
			parts = append(parts, sw.flag)
		}
	}
	if s.GamescopeFilter != "" {
		parts = append(parts, "-F", s.GamescopeFilter)
	}
	return strings.Join(append(parts, strings.Fields(s.GamescopeArgs)...), " ")
}

// effectiveSettings is the settings a launch uses: the runner's choices turned
// into env vars and gamescope arguments. The user's own env_vars win over them,
// as they always have.
func effectiveSettings(s types.GameSettings) types.GameSettings {
	env := derivedEnv(s)
	for k, v := range s.EnvVars {
		env[k] = v
	}
	s.EnvVars = env
	s.UseMangoHud = mangohudPreset(s) != "off"
	if w, h, ok := strings.Cut(s.GamescopeResolution, "x"); ok {
		if wi, err1 := strconv.Atoi(w); err1 == nil {
			if hi, err2 := strconv.Atoi(h); err2 == nil {
				s.GamescopeW, s.GamescopeH = wi, hi
			}
		}
	}
	if r, err := strconv.Atoi(s.GamescopeRate); err == nil {
		s.GamescopeR = r
	}
	s.GamescopeArgs = gamescopeArgs(s)
	return s
}

// field is one schema field; its label and description come from the catalogue.
func field(id, kind string, def any, extra map[string]any) map[string]any {
	f := map[string]any{
		"id": id, "kind": kind, "scope": "game", "default": def,
		"label": text(id + ".label"), "description": text(id + ".description"),
	}
	for k, v := range extra {
		f[k] = v
	}
	return f
}

// choices is a select's options with catalogue labels: field.<id> keys, the
// empty id labelled field.default.
func choices(id string, ids ...string) []any {
	out := make([]any, 0, len(ids))
	for _, opt := range ids {
		label := text(id + "." + opt)
		if opt == "" {
			label = text(id + ".default")
		}
		if label == "" {
			label = opt // a value the same in every language, like -F linear
		}
		out = append(out, map[string]any{"id": opt, "label": label})
	}
	return out
}

// runnerSchema is the per-game runner choices. Every field is a GameSettings key
// (no migration needed); the wine prefix is a core section, not offered here.
func runnerSchema() map[string]any {
	group := func(id string, fields ...any) map[string]any {
		return map[string]any{"id": id, "label": text("group." + id), "fields": fields}
	}
	var protonFields []any
	for _, f := range protonFlags {
		key := strings.ToLower(f.env)
		protonFields = append(protonFields, field(key, "bool", false, nil))
	}
	gamescopeRates := []string{"", "60", "75", "90", "120", "144", "165", "240"}
	rateOpts := make([]any, 0, len(gamescopeRates))
	for _, r := range gamescopeRates {
		label := r + " Hz"
		if r == "" {
			label = text("gamescope_rate.default")
		}
		rateOpts = append(rateOpts, map[string]any{"id": r, "label": label})
	}
	resOpts := []any{map[string]any{"id": "", "label": text("gamescope_resolution.default")}}
	for _, r := range []string{"1280x720", "1280x800", "1920x1080", "1920x1200", "2560x1440", "3840x2160"} {
		resOpts = append(resOpts, map[string]any{"id": r, "label": r})
	}
	return map[string]any{"groups": []any{
		group("runner",
			field("runner_id", "select", "auto", map[string]any{"options": runnerOptions()}),
		),
		group("wrappers",
			field("use_gamemode", "bool", true, nil),
			field("use_gamescope", "bool", false, nil),
		),
		group("graphics",
			field("dx_layer", "select", "dxvk", map[string]any{"options": choices("dx_layer", "dxvk", "wined3d")}),
			field("upscaler", "select", "native", map[string]any{"options": choices("upscaler", "native", "fsr1")}),
			field("shader_cache", "select", "on", map[string]any{"options": choices("shader_cache", "on", "off")}),
			field("mangohud_preset", "select", "off", map[string]any{"options": choices("mangohud_preset", "off", "fps only", "minimal", "full")}),
		),
		group("sync",
			field("sync_mode", "select", "fsync", map[string]any{"options": choices("sync_mode", "fsync", "esync", "ntsync", "off")}),
		),
		withDescription(group("proton", protonFields...), text("proton.description")),
		group("gamescope",
			field("gamescope_resolution", "select", "", map[string]any{"options": resOpts}),
			field("gamescope_rate", "select", "", map[string]any{"options": rateOpts}),
			field("gamescope_filter", "select", "", map[string]any{"options": choices("gamescope_filter", "", "linear", "nearest", "fsr", "nis", "pixel")}),
			field("gamescope_fullscreen", "bool", false, nil),
			field("gamescope_borderless", "bool", false, nil),
			field("gamescope_grab_cursor", "bool", false, nil),
			field("gamescope_hdr", "bool", false, nil),
			field("gamescope_vrr", "bool", false, nil),
			field("gamescope_wayland", "bool", false, nil),
			field("gamescope_args", "text", "", nil),
		),
		group("anticheat",
			field("eac_runtime", "bool", false, nil),
			field("battleye_runtime", "bool", false, nil),
		),
	}}
}

func withDescription(g map[string]any, desc string) map[string]any {
	g["description"] = desc
	return g
}

// runnerOptions are the runner choices: the automatic pick, then every runner
// installed here.
func runnerOptions() []any {
	out := []any{map[string]any{"id": "auto", "label": text("runner_id.auto")}}
	for _, r := range runner.ScanProtonVersionsFor(os.Getenv("HOME")) {
		if strings.EqualFold(r.ID, "auto") {
			continue
		}
		label := r.Name
		if runner.IsUmu(r) {
			label = text("runner_id.umu")
		}
		out = append(out, map[string]any{"id": r.ID, "label": label})
	}
	return out
}
