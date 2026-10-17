package main

import (
	"embed"
	"encoding/json"
	"maps"
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

// choiceSet is the game's saved settings as the runner reads them: the keys
// runner.options declares, plus the modeled keys the launch reads. An absent key
// takes the default runner.options declares.
type choiceSet map[string]any

func (c choiceSet) flag(key string) bool {
	b, _ := c[key].(bool)
	return b
}

func (c choiceSet) text(key string) string {
	s, _ := c[key].(string)
	return s
}

// or is the text of key, or def when the game has none.
func (c choiceSet) or(key, def string) string {
	if s := c.text(key); s != "" {
		return s
	}
	return def
}

// protonFlags are the Proton switches: the setting that turns each on, and the
// env var it sets to 1.
var protonFlags = []struct{ key, env string }{
	{"proton_enable_nvapi", "PROTON_ENABLE_NVAPI"},
	{"proton_hide_nvidia_gpu", "PROTON_HIDE_NVIDIA_GPU"},
	{"proton_enable_wayland", "PROTON_ENABLE_WAYLAND"},
	{"proton_no_d3d11", "PROTON_NO_D3D11"},
	{"proton_no_d3d12", "PROTON_NO_D3D12"},
	{"proton_force_large_address_aware", "PROTON_FORCE_LARGE_ADDRESS_AWARE"},
	{"proton_old_gl_string", "PROTON_OLD_GL_STRING"},
	{"proton_log", "PROTON_LOG"},
}

// mangohudConfig is the MANGOHUD_CONFIG each preset writes.
var mangohudConfig = map[string]string{"fps only": "preset=1", "minimal": "preset=2", "full": "preset=4"}

// The defaults runner.options declares: an empty or absent choice is the first
// option of its field.
func syncMode(c choiceSet) string    { return c.or("sync_mode", "fsync") }
func dxLayer(c choiceSet) string     { return c.or("dx_layer", "dxvk") }
func upscaler(c choiceSet) string    { return c.or("upscaler", "native") }
func shaderCache(c choiceSet) string { return c.or("shader_cache", "on") }

// mangohudPreset is the overlay the game runs with. A game with no preset saved
// takes use_mangohud's old meaning: on is full.
func mangohudPreset(c choiceSet) string {
	if p := c.text("mangohud_preset"); p != "" {
		return p
	}
	if c.flag("use_mangohud") {
		return "full"
	}
	return "off"
}

// derivedEnv is the env the runner's choices set: Proton's switches, sync, the
// DX layer, the upscaler, MangoHud and the shader cache.
func derivedEnv(c choiceSet) map[string]string {
	env := map[string]string{}
	for _, f := range protonFlags {
		if c.flag(f.key) {
			env[f.env] = "1"
		}
	}
	switch syncMode(c) {
	case "esync":
		env["PROTON_NO_FSYNC"] = "1"
	case "ntsync":
		env["PROTON_USE_NTSYNC"] = "1"
	case "off":
		env["PROTON_NO_ESYNC"], env["PROTON_NO_FSYNC"] = "1", "1"
	}
	if dxLayer(c) == "wined3d" {
		env["PROTON_USE_WINED3D"] = "1"
	}
	if upscaler(c) == "fsr1" {
		env["WINE_FULLSCREEN_FSR"] = "1"
	}
	if p := mangohudPreset(c); p != "off" {
		env["MANGOHUD_CONFIG"] = mangohudConfig[p]
	}
	if shaderCache(c) == "off" {
		env["DXVK_STATE_CACHE"] = "0"
	}
	return env
}

// gamescopeArgs is the gamescope arguments the switches and filter give, then
// the user's own gamescope_args: the order the old UI wrote them in.
func gamescopeArgs(c choiceSet, extra string) string {
	var parts []string
	for _, sw := range []struct {
		key, flag string
	}{
		{"gamescope_fullscreen", "-f"},
		{"gamescope_borderless", "-b"},
		{"gamescope_grab_cursor", "--force-grab-cursor"},
		{"gamescope_hdr", "--hdr-enabled"},
		{"gamescope_vrr", "--adaptive-sync"},
		{"gamescope_wayland", "--expose-wayland"},
	} {
		if c.flag(sw.key) {
			parts = append(parts, sw.flag)
		}
	}
	if f := c.text("gamescope_filter"); f != "" {
		parts = append(parts, "-F", f)
	}
	return strings.Join(append(parts, strings.Fields(extra)...), " ")
}

// settingsMap is a game's typed settings as a saved-settings map.
func settingsMap(s types.GameSettings) map[string]any {
	out := map[string]any{}
	if raw, err := json.Marshal(s); err == nil {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

// effectiveSettings is the settings a launch uses: the runner's choices turned
// into env vars and gamescope arguments. The game's saved keys are read from opts
// (the core sends them all); the typed settings fill in what opts lacks. Legacy
// keys are migrated in memory first, so a game saved by the old UI launches as it
// did before the choices moved. The user's own env_vars win over a derived value.
func effectiveSettings(s types.GameSettings, opts map[string]any) types.GameSettings {
	saved := settingsMap(s)
	maps.Copy(saved, opts)
	migrateSettings(saved)
	c := choiceSet(saved)

	env := envStrings(c["env_vars"])
	derived := derivedEnv(c)
	maps.Copy(derived, env)
	s.EnvVars = derived
	s.UseMangoHud = mangohudPreset(c) != "off"
	if w, h, ok := strings.Cut(c.text("gamescope_resolution"), "x"); ok {
		if wi, err1 := strconv.Atoi(w); err1 == nil {
			if hi, err2 := strconv.Atoi(h); err2 == nil {
				s.GamescopeW, s.GamescopeH = wi, hi
			}
		}
	}
	if r, err := strconv.Atoi(c.text("gamescope_rate")); err == nil {
		s.GamescopeR = r
	}
	s.GamescopeArgs = gamescopeArgs(c, c.text("gamescope_args"))
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
		protonFields = append(protonFields, field(f.key, "bool", false, nil))
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
	installed := runner.ScanProtonVersionsFor(os.Getenv("HOME"))
	out := []any{map[string]any{"id": "auto", "label": autoLabel(installed)}}
	for _, r := range installed {
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

// autoLabel names what the automatic pick runs with: the default runner, when
// one is installed.
func autoLabel(installed []types.ProtonVersion) string {
	for _, r := range installed {
		if r.IsDefault {
			return strings.ReplaceAll(text("runner_id.autoNamed"), "{{name}}", r.Name)
		}
	}
	return text("runner_id.autoNone")
}
