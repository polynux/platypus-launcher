package app

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"platypus/internal/apperr"
	"platypus/internal/runner"
	"platypus/internal/steam"
	"platypus/internal/types"
)

// Wine prefix presets (issue #52): named templates applied to a game's
// prefix on demand. Applying merges env and DLL overrides into the game's
// settings (the game's own values always win) and records the rest as
// ordinary per-game settings. Winetricks verbs and the Windows version are
// stored and shown, never executed here.

// builtinPrefixPresets are the hardcoded read-only presets. They live in
// the backend, not in settings.json, and can't be saved over or deleted.
var builtinPrefixPresets = []types.PrefixPreset{
	{
		ID:           "builtin:old-dx9",
		Name:         "Old DirectX 9 game",
		Builtin:      true,
		DLLOverrides: map[string]string{"xaudio2_7": "n,b", "ddraw": "n,b"},
		Winetricks:   []string{"d3dx9", "d3dcompiler_43", "d3dcompiler_47", "xact", "corefonts"},
	},
	{
		ID:             "builtin:dotnet48",
		Name:           "Needs .NET 4.8",
		Builtin:        true,
		Winetricks:     []string{"dotnet48", "corefonts"},
		WindowsVersion: "win10",
	},
	{
		ID:         "builtin:vc-fonts",
		Name:       "Visual C++ + fonts baseline",
		Builtin:    true,
		Winetricks: []string{"vcrun2019", "corefonts"},
	},
}

// ListPrefixPresets returns the built-ins first, then the user's presets by id.
func (a *App) ListPrefixPresets() []types.PrefixPreset {
	out := append([]types.PrefixPreset{}, builtinPrefixPresets...)
	users := a.settings.PrefixPresets()
	ids := make([]string, 0, len(users))
	for id := range users {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		out = append(out, users[id])
	}
	return out
}

// findPrefixPreset returns the built-in or user preset with this id.
func (a *App) findPrefixPreset(id string) (types.PrefixPreset, error) {
	for _, p := range builtinPrefixPresets {
		if p.ID == id {
			return p, nil
		}
	}
	if p, ok := a.settings.PrefixPresets()[id]; ok {
		return p, nil
	}
	return types.PrefixPreset{}, apperr.New("settings.preset.notFound",
		fmt.Sprintf("unknown preset %q", id),
		map[string]string{"id": id})
}

// SavePrefixPreset creates or replaces a user preset, assigning it an id
// when it has none. Built-in ids are read-only.
func (a *App) SavePrefixPreset(p types.PrefixPreset) (types.PrefixPreset, error) {
	return a.settings.SavePrefixPreset(p)
}

// DeletePrefixPreset forgets a user preset. Built-ins can't be deleted.
func (a *App) DeletePrefixPreset(id string) error {
	return a.settings.DeletePrefixPreset(id)
}

// presetGame returns the game as launches see it: sideloaded games carry
// their saved settings, and store games from the current library get their
// prefix pin overlaid. Anything else is unknown.
func (a *App) presetGame(id string) (types.Game, error) {
	for _, g := range a.customGames() {
		if g.ID == id {
			return g, nil
		}
	}
	var saved map[string]map[string]any
	if a.settings != nil {
		saved = a.settings.GameSettings()
	}
	a.library.mu.Lock()
	defer a.library.mu.Unlock()
	for _, e := range a.library.entries {
		for _, g := range e.games {
			if g.ID != id {
				continue
			}
			if m, ok := saved[id]; ok {
				if s, _ := m["wine_prefix"].(string); strings.TrimSpace(s) != "" {
					g.Settings.WinePrefix = s
				}
			}
			return g, nil
		}
	}
	return types.Game{}, apperr.New("game.notFound",
		fmt.Sprintf("unknown game %q", id),
		map[string]string{"id": id})
}

// presetEnv reads the game's env_vars as strings: the settings map holds
// whatever JSON decoded (usually map[string]any).
func presetEnv(m map[string]any) map[string]string {
	out := map[string]string{}
	switch v := m["env_vars"].(type) {
	case map[string]string:
		for k, s := range v {
			out[k] = s
		}
	case map[string]any:
		for k, s := range v {
			if s, ok := s.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

// presetVerbs reads the game's recorded winetricks verbs: a string list,
// however JSON decoded it.
func presetVerbs(m map[string]any) []string {
	var out []string
	switch v := m["winetricks"].(type) {
	case []string:
		out = append(out, v...)
	case []any:
		for _, w := range v {
			if s, ok := w.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// composeOverrides adds the preset's DLLs to a WINEDLLOVERRIDES value,
// keeping the game's own entries: a DLL already named there (in any case)
// stays exactly as the user wrote it.
func composeOverrides(existing string, dlls map[string]string) (string, bool) {
	nameOf := func(part string) string {
		name, _, _ := strings.Cut(part, "=")
		return strings.ToLower(strings.TrimSpace(name))
	}
	var parts []string
	seen := map[string]bool{}
	for _, part := range strings.Split(existing, ";") {
		if strings.TrimSpace(part) == "" {
			continue
		}
		parts = append(parts, strings.TrimSpace(part))
		seen[nameOf(part)] = true
	}
	keys := make([]string, 0, len(dlls))
	for k := range dlls {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	added := false
	for _, k := range keys {
		if seen[strings.ToLower(k)] {
			continue
		}
		parts = append(parts, k+"="+dlls[k])
		seen[strings.ToLower(k)] = true
		added = true
	}
	return strings.Join(parts, ";"), added
}

// ApplyPrefixPreset merges a preset into a game's settings: the game is
// looked up first (unknown ids are refused), and so are games without a
// Platypus prefix: native builds and store-client launches. The prefix folder
// resolves exactly like a launch and is created when missing, env vars fill
// the gaps the game's own values leave (reported as skipped), DLL overrides
// join WINEDLLOVERRIDES without touching the game's entries, launch tokens
// append when absent, and the preset id, its verbs and the Windows version
// are recorded as ordinary per-game settings (the version only when the game
// has none yet: the user's own always wins). Nothing is executed.
// Re-applying the same preset changes nothing.
func (a *App) ApplyPrefixPreset(gameID, presetID string) (types.PrefixApplyResult, error) {
	res := types.PrefixApplyResult{Env: map[string]string{}, Skipped: []string{}, Verbs: []string{}}
	p, err := a.findPrefixPreset(presetID)
	if err != nil {
		return res, err
	}
	game, err := a.presetGame(gameID)
	if err != nil {
		return res, err
	}
	if game.Native {
		return res, apperr.New("settings.preset.native",
			fmt.Sprintf("presets don't apply to %s: it runs natively, without a Wine prefix", game.Title),
			map[string]string{"title": game.Title})
	}
	if steam.IsClientLaunch(game) {
		return res, apperr.New("settings.preset.clientLaunch",
			fmt.Sprintf("presets don't apply to %s: its store client launches it and owns its prefix", game.Title),
			map[string]string{"title": game.Title})
	}
	cur := a.settings.GameSettings()[gameID]
	next := make(map[string]any, len(cur)+8)
	for k, v := range cur {
		next[k] = v
	}
	changed := false

	// The prefix folder exists before anything records it: launches assume it.
	if err := os.MkdirAll(runner.PrefixPath(game, a.homeDir(), a.effectivePrefixesFolder()), 0o755); err != nil {
		return res, err
	}

	// The game's own env values always win over the preset's.
	env := presetEnv(next)
	keys := make([]string, 0, len(p.Env))
	for k := range p.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	envChanged := false
	for _, k := range keys {
		if _, ok := env[k]; ok {
			res.Skipped = append(res.Skipped, k)
			continue
		}
		env[k] = p.Env[k]
		envChanged = true
	}
	if len(p.DLLOverrides) > 0 {
		merged, added := composeOverrides(env["WINEDLLOVERRIDES"], p.DLLOverrides)
		if added || merged != env["WINEDLLOVERRIDES"] {
			env["WINEDLLOVERRIDES"] = merged
			envChanged = true
		}
	}
	if envChanged {
		next["env_vars"] = env
		changed = true
	}
	res.Env = env
	res.Overrides = env["WINEDLLOVERRIDES"]

	if fields := strings.Fields(p.LaunchArgs); len(fields) > 0 {
		args, _ := next["launch_arguments"].(string)
		have := map[string]bool{}
		for _, t := range strings.Fields(args) {
			have[t] = true
		}
		var missing []string
		for _, t := range fields {
			if !have[t] {
				missing = append(missing, t)
				have[t] = true
			}
		}
		if len(missing) > 0 {
			if strings.TrimSpace(args) == "" {
				next["launch_arguments"] = strings.Join(missing, " ")
			} else {
				next["launch_arguments"] = strings.TrimSpace(args) + " " + strings.Join(missing, " ")
			}
			changed = true
		}
	}

	verbs := presetVerbs(next)
	seen := map[string]bool{}
	for _, w := range verbs {
		seen[w] = true
	}
	verbsChanged := false
	for _, w := range p.Winetricks {
		if !seen[w] {
			verbs = append(verbs, w)
			seen[w] = true
			verbsChanged = true
		}
	}
	if verbsChanged {
		next["winetricks"] = verbs
		changed = true
	}
	if res.Verbs = verbs; res.Verbs == nil {
		res.Verbs = []string{}
	}

	// An empty version means win10, like everywhere the version is read. The
	// game's own version always wins: a preset only fills a missing one.
	version := p.WindowsVersion
	if version == "" {
		version = "win10"
	}
	if cur, _ := next["windows_version"].(string); cur == "" {
		next["windows_version"] = version
		changed = true
	}
	res.WindowsVersion, _ = next["windows_version"].(string)

	if cur, _ := next["prefix_preset"].(string); cur != p.ID {
		next["prefix_preset"] = p.ID
		changed = true
	}

	if changed {
		if err := a.settings.SetGameSettings(gameID, next); err != nil {
			return res, err
		}
	}
	return res, nil
}
