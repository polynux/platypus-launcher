package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Migration of the settings the UI used to fold into env_vars and
// gamescope_args (#74). Those values now live under the keys runner.options
// declares. migrateSettings moves them there. It takes over an env var only when it is exactly what the old
// UI wrote for the choice the var reads back as, so a value the user typed by
// hand stays where it is. It runs when settings load and when they are saved,
// and running it again changes nothing.

// legacyProtonFlags are the Proton flags the old UI showed, with their typed keys.
var legacyProtonFlags = []struct{ env, key string }{
	{"PROTON_ENABLE_NVAPI", "proton_enable_nvapi"},
	{"PROTON_HIDE_NVIDIA_GPU", "proton_hide_nvidia_gpu"},
	{"PROTON_ENABLE_WAYLAND", "proton_enable_wayland"},
	{"PROTON_NO_D3D11", "proton_no_d3d11"},
	{"PROTON_NO_D3D12", "proton_no_d3d12"},
	{"PROTON_FORCE_LARGE_ADDRESS_AWARE", "proton_force_large_address_aware"},
	{"PROTON_OLD_GL_STRING", "proton_old_gl_string"},
	{"PROTON_LOG", "proton_log"},
}

// legacyGamescopeSwitches are the gamescope switches the old UI wrote into
// gamescope_args, with the spellings it accepted.
var legacyGamescopeSwitches = []struct {
	key       string
	spellings []string
}{
	{"gamescope_fullscreen", []string{"-f", "--fullscreen"}},
	{"gamescope_borderless", []string{"-b", "--borderless"}},
	{"gamescope_grab_cursor", []string{"--force-grab-cursor"}},
	{"gamescope_hdr", []string{"--hdr-enabled"}},
	{"gamescope_vrr", []string{"--adaptive-sync"}},
	{"gamescope_wayland", []string{"--expose-wayland"}},
}

var legacyGamescopeFilters = []string{"linear", "nearest", "fsr", "nis", "pixel"}

// migrateSettings converts one game's saved settings in place and reports
// whether it changed anything. It is idempotent.
func migrateSettings(m map[string]any) bool {
	if m == nil {
		return false
	}
	changed := false
	typed := func(k string) bool { _, ok := m[k]; return ok }
	setTyped := func(k string, v any) {
		m[k] = v
		changed = true
	}
	env := envStrings(m["env_vars"])
	takeOver := func(want map[string]string) bool {
		took := false
		for k, v := range want {
			if cur, ok := env[k]; ok && cur == v {
				delete(env, k)
				took = true
			}
		}
		return took
	}

	// Proton flags: the old list, and the flags the old UI wrote as env vars.
	list, _ := m["proton_flags"].([]any)
	listed := func(env string) bool {
		for _, v := range list {
			if s, _ := v.(string); s == env {
				return true
			}
		}
		return false
	}
	flagsTaken := map[string]string{}
	for _, f := range legacyProtonFlags {
		if typed(f.key) {
			continue
		}
		if listed(f.env) || env[f.env] == "1" {
			setTyped(f.key, true)
			flagsTaken[f.env] = "1"
		}
	}
	if takeOver(flagsTaken) {
		changed = true
	}
	if _, ok := m["proton_flags"]; ok {
		delete(m, "proton_flags")
		changed = true
	}

	// Sync, graphics layer, upscaler, MangoHud and shader cache: the state
	// the old UI read from env (and use_mangohud), and the env it wrote for it.
	if !typed("sync_mode") {
		sync := "fsync"
		switch {
		case env["PROTON_USE_NTSYNC"] == "1":
			sync = "ntsync"
		case env["PROTON_NO_ESYNC"] == "1":
			sync = "off"
		case env["PROTON_NO_FSYNC"] == "1":
			sync = "esync"
		}
		want := map[string]string{}
		switch sync {
		case "esync":
			want["PROTON_NO_FSYNC"] = "1"
		case "ntsync":
			want["PROTON_USE_NTSYNC"] = "1"
		case "off":
			want["PROTON_NO_ESYNC"], want["PROTON_NO_FSYNC"] = "1", "1"
		}
		if takeOver(want) {
			setTyped("sync_mode", sync)
		}
	}
	if !typed("dx_layer") && takeOver(map[string]string{"PROTON_USE_WINED3D": "1"}) {
		setTyped("dx_layer", "wined3d")
	}
	if !typed("upscaler") && takeOver(map[string]string{"WINE_FULLSCREEN_FSR": "1"}) {
		setTyped("upscaler", "fsr1")
	}
	if !typed("shader_cache") && takeOver(map[string]string{"DXVK_STATE_CACHE": "0"}) {
		setTyped("shader_cache", "off")
	}
	if !typed("mangohud_preset") {
		hud, _ := m["use_mangohud"].(bool)
		preset := "off"
		if hud {
			preset = "full"
			switch env["MANGOHUD_CONFIG"] {
			case "preset=1":
				preset = "fps only"
			case "preset=2":
				preset = "minimal"
			}
		}
		if preset != "off" && takeOver(map[string]string{"MANGOHUD_CONFIG": mangohudEnv[preset]}) {
			setTyped("mangohud_preset", preset)
		}
	}

	// Gamescope: the switches and -F filter in gamescope_args, and the size and
	// rate ints the old UI wrote.
	if args, ok := m["gamescope_args"].(string); ok && strings.TrimSpace(args) != "" {
		var kept []string
		tokens := strings.Fields(args)
		for i := 0; i < len(tokens); i++ {
			tok := tokens[i]
			if key, ok := switchFor(tok); ok && !typed(key) {
				setTyped(key, true)
				continue
			}
			if (tok == "-F" || tok == "--filter") && i+1 < len(tokens) && slices.Contains(legacyGamescopeFilters, tokens[i+1]) && !typed("gamescope_filter") {
				setTyped("gamescope_filter", tokens[i+1])
				i++
				continue
			}
			kept = append(kept, tok)
		}
		if out := strings.Join(kept, " "); out != args {
			m["gamescope_args"] = out
			changed = true
		}
	}
	if !typed("gamescope_resolution") {
		w, h := intOf(m["gamescope_w"]), intOf(m["gamescope_h"])
		if w > 0 && h > 0 {
			setTyped("gamescope_resolution", fmt.Sprintf("%dx%d", w, h))
			m["gamescope_w"], m["gamescope_h"] = float64(0), float64(0)
		}
	}
	if !typed("gamescope_rate") {
		if r := intOf(m["gamescope_r"]); r > 0 {
			setTyped("gamescope_rate", strconv.Itoa(r))
			m["gamescope_r"] = float64(0)
		}
	}

	if changed {
		m["env_vars"] = envAny(env)
	}
	return changed
}

// mangohudEnv is the MANGOHUD_CONFIG the old UI wrote for each preset; off writes none.
var mangohudEnv = map[string]string{"off": "", "fps only": "preset=1", "minimal": "preset=2", "full": "preset=4"}

// switchFor is the typed key of a gamescope switch spelled as tok.
func switchFor(tok string) (string, bool) {
	for _, s := range legacyGamescopeSwitches {
		if slices.Contains(s.spellings, tok) {
			return s.key, true
		}
	}
	return "", false
}

func envStrings(v any) map[string]string {
	out := map[string]string{}
	switch e := v.(type) {
	case map[string]any:
		for k, x := range e {
			if s, ok := x.(string); ok {
				out[k] = s
			}
		}
	case map[string]string:
		for k, x := range e {
			out[k] = x
		}
	}
	return out
}

func envAny(env map[string]string) map[string]any {
	out := make(map[string]any, len(env))
	for k, v := range env {
		out[k] = v
	}
	return out
}

func intOf(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}
