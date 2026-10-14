package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"platypus/internal/apperr"
	"platypus/internal/settings"
	"platypus/internal/types"
)

// Wine prefix presets (issue #52): the built-ins are listed read-only, user
// presets validate on save and persist in settings.json, and applying one
// merges into the game's settings without ever running wine or winetricks.
// Temp dirs only.

func presetApp(t *testing.T) (*App, string) {
	t.Helper()
	home := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(t.TempDir(), "settings.json"), home)
	if err != nil {
		t.Fatal(err)
	}
	return &App{settings: st, optHome: home}, home
}

// presetExeGame adds a sideloaded Windows game and returns its id.
func presetExeGame(t *testing.T, a *App, home, title string) string {
	t.Helper()
	exe := filepath.Join(home, title, title+".exe")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("MZ\x90\x00"), 0o755); err != nil {
		t.Fatal(err)
	}
	g, err := a.settings.AddCustomGame(title, exe)
	if err != nil {
		t.Fatal(err)
	}
	return g.ID
}

// The three built-ins list with their exact content and can't be deleted or
// saved over.
func TestPrefixPresetsBuiltins(t *testing.T) {
	t.Parallel()
	a, _ := presetApp(t)

	got := a.ListPrefixPresets()
	if len(got) != 3 {
		t.Fatalf("ListPrefixPresets = %+v, want the 3 built-ins", got)
	}
	dx9 := got[0]
	if dx9.ID != "builtin:old-dx9" || !dx9.Builtin || dx9.Name != "Old DirectX 9 game" {
		t.Fatalf("first built-in = %+v", dx9)
	}
	if dx9.DLLOverrides["xaudio2_7"] != "n,b" || dx9.DLLOverrides["ddraw"] != "n,b" {
		t.Fatalf("old-dx9 overrides = %v", dx9.DLLOverrides)
	}
	if !reflect.DeepEqual(dx9.Winetricks, []string{"d3dx9", "d3dcompiler_43", "d3dcompiler_47", "xact", "corefonts"}) {
		t.Fatalf("old-dx9 verbs = %v", dx9.Winetricks)
	}
	if dotnet := got[1]; dotnet.ID != "builtin:dotnet48" || !dotnet.Builtin ||
		!reflect.DeepEqual(dotnet.Winetricks, []string{"dotnet48", "corefonts"}) || dotnet.WindowsVersion != "win10" {
		t.Fatalf("second built-in = %+v", dotnet)
	}
	if vc := got[2]; vc.ID != "builtin:vc-fonts" || !vc.Builtin ||
		!reflect.DeepEqual(vc.Winetricks, []string{"vcrun2019", "corefonts"}) {
		t.Fatalf("third built-in = %+v", vc)
	}

	if err := a.DeletePrefixPreset("builtin:old-dx9"); apperr.CodeOf(err) != "settings.preset.builtin" {
		t.Fatalf("delete built-in err = %v, want settings.preset.builtin", err)
	}
	over := dx9
	over.Name = "Changed"
	if _, err := a.SavePrefixPreset(over); apperr.CodeOf(err) != "settings.preset.builtin" {
		t.Fatalf("save over built-in err = %v, want settings.preset.builtin", err)
	}
	if err := a.DeletePrefixPreset("user-nope"); apperr.CodeOf(err) != "settings.preset.notFound" {
		t.Fatalf("delete unknown err = %v, want settings.preset.notFound", err)
	}
	if _, err := a.ApplyPrefixPreset("g1", "preset-nope"); apperr.CodeOf(err) != "settings.preset.notFound" {
		t.Fatalf("apply unknown err = %v, want settings.preset.notFound", err)
	}
}

// User presets validate on save: a name, verb-shaped verbs, variable-name
// env keys and a known Windows version (or none, meaning win10).
func TestPrefixPresetsValidation(t *testing.T) {
	t.Parallel()
	a, _ := presetApp(t)

	for _, tc := range []struct {
		name   string
		preset types.PrefixPreset
		code   string
	}{
		{"empty name", types.PrefixPreset{Name: "  "}, "settings.preset.nameMissing"},
		{"blank verb", types.PrefixPreset{Name: "P", Winetricks: []string{"  "}}, "settings.preset.badVerb"},
		{"spaced verb", types.PrefixPreset{Name: "P", Winetricks: []string{"not a verb"}}, "settings.preset.badVerb"},
		{"bad env key", types.PrefixPreset{Name: "P", Env: map[string]string{"1BAD": "x"}}, "settings.preset.badEnvKey"},
		{"dashed env key", types.PrefixPreset{Name: "P", Env: map[string]string{"HAS-DASH": "x"}}, "settings.preset.badEnvKey"},
		{"empty dll value", types.PrefixPreset{Name: "P", DLLOverrides: map[string]string{"dxgi": " "}}, "settings.preset.badOverride"},
		{"unknown windows version", types.PrefixPreset{Name: "P", WindowsVersion: "winxp"}, "settings.preset.badWindowsVersion"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := a.SavePrefixPreset(tc.preset); apperr.CodeOf(err) != tc.code {
				t.Fatalf("err = %v, want %s", err, tc.code)
			}
		})
	}
	if got := a.ListPrefixPresets(); len(got) != 3 {
		t.Fatalf("rejected saves listed %d presets, want only the 3 built-ins", len(got))
	}
}

// A user preset saves, lists, deletes and survives a reload.
func TestPrefixPresetsUserRoundTrip(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	st, err := settings.OpenWithHome(settingsPath, home)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, optHome: home}

	saved, err := a.SavePrefixPreset(types.PrefixPreset{
		Name:           "Mine",
		Winetricks:     []string{"vcrun2019"},
		Env:            map[string]string{"FOO": "1"},
		WindowsVersion: "win11",
		LaunchArgs:     "-novid",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" || saved.Builtin {
		t.Fatalf("saved = %+v, want an assigned user id", saved)
	}
	if got := a.ListPrefixPresets(); len(got) != 4 || got[3].ID != saved.ID {
		t.Fatalf("list after save = %+v, want built-ins plus the user preset last", got)
	}

	// Renaming the same id replaces it instead of adding another.
	renamed := saved
	renamed.Name = "Renamed"
	if _, err := a.SavePrefixPreset(renamed); err != nil {
		t.Fatal(err)
	}
	if got := a.ListPrefixPresets(); len(got) != 4 || got[3].Name != "Renamed" {
		t.Fatalf("list after re-save = %+v, want one renamed preset", got)
	}

	if err := a.DeletePrefixPreset(saved.ID); err != nil {
		t.Fatal(err)
	}
	if got := a.ListPrefixPresets(); len(got) != 3 {
		t.Fatalf("list after delete = %+v, want only the built-ins", got)
	}

	// Persistence: a save survives reopening the settings file.
	again, err := a.SavePrefixPreset(types.PrefixPreset{Name: "Kept", Env: map[string]string{"A": "b"}})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := settings.OpenWithHome(settingsPath, home)
	if err != nil {
		t.Fatal(err)
	}
	b := &App{settings: reopened, optHome: home}
	found := false
	for _, p := range b.ListPrefixPresets() {
		if p.ID == again.ID && p.Name == "Kept" && p.Env["A"] == "b" {
			found = true
		}
	}
	if !found {
		t.Fatalf("list after reload = %+v, want the kept preset", b.ListPrefixPresets())
	}
}

// Applying merges: the game's own env values win (reported as skipped), DLL
// overrides join WINEDLLOVERRIDES without touching the game's entries,
// launch tokens append once, and the id, verbs and version are recorded.
func TestApplyPrefixPresetMerges(t *testing.T) {
	t.Parallel()
	a, home := presetApp(t)
	id := presetExeGame(t, a, home, "Gem")

	if err := a.settings.SetGameSettings(id, map[string]any{
		"env_vars": map[string]any{
			"FOO":              "user",
			"WINEDLLOVERRIDES": "dxgi=n,b",
		},
		"launch_arguments": "-novid",
		"winetricks":       []any{"corefonts"},
	}); err != nil {
		t.Fatal(err)
	}
	saved, err := a.SavePrefixPreset(types.PrefixPreset{
		Name:           "Mine",
		Env:            map[string]string{"FOO": "preset", "BAR": "preset"},
		DLLOverrides:   map[string]string{"DXGI": "b,n", "ddraw": "n,b"},
		Winetricks:     []string{"corefonts", "d3dx9"},
		WindowsVersion: "win8",
		LaunchArgs:     "-novid -fullscreen",
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := a.ApplyPrefixPreset(id, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	// The user's FOO wins and is reported; BAR is added.
	if res.Env["FOO"] != "user" || res.Env["BAR"] != "preset" {
		t.Fatalf("merged env = %v", res.Env)
	}
	if !reflect.DeepEqual(res.Skipped, []string{"FOO"}) {
		t.Fatalf("skipped = %v, want [FOO]", res.Skipped)
	}
	// DXGI stays exactly as the user wrote it (any case); ddraw is added.
	if res.Overrides != "dxgi=n,b;ddraw=n,b" {
		t.Fatalf("overrides = %q, want the user's entry kept plus ddraw", res.Overrides)
	}
	if res.Env["WINEDLLOVERRIDES"] != res.Overrides {
		t.Fatalf("env overrides = %q, want %q", res.Env["WINEDLLOVERRIDES"], res.Overrides)
	}
	if !reflect.DeepEqual(res.Verbs, []string{"corefonts", "d3dx9"}) {
		t.Fatalf("verbs = %v, want the union", res.Verbs)
	}
	if res.WindowsVersion != "win8" {
		t.Fatalf("windows version = %q, want win8", res.WindowsVersion)
	}

	m := a.settings.GameSettings()[id]
	if m["prefix_preset"] != saved.ID {
		t.Fatalf("recorded preset = %v, want %s", m["prefix_preset"], saved.ID)
	}
	if m["windows_version"] != "win8" {
		t.Fatalf("recorded version = %v, want win8", m["windows_version"])
	}
	if m["launch_arguments"] != "-novid -fullscreen" {
		t.Fatalf("launch args = %v, present tokens not duplicated", m["launch_arguments"])
	}
	// The prefix folder was created, although nothing ran.
	wantPrefix := filepath.Join(home, ".local/share/platypus", "prefixes", id)
	if st, err := os.Stat(wantPrefix); err != nil || !st.IsDir() {
		t.Fatalf("prefix folder %s: %v", wantPrefix, err)
	}
}

// Re-applying the same preset changes nothing; an empty version records win10.
func TestApplyPrefixPresetIdempotent(t *testing.T) {
	t.Parallel()
	a, home := presetApp(t)
	id := presetExeGame(t, a, home, "Gem")

	saved, err := a.SavePrefixPreset(types.PrefixPreset{Name: "Mine"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := a.ApplyPrefixPreset(id, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.WindowsVersion != "win10" {
		t.Fatalf("version = %q, want the win10 default", first.WindowsVersion)
	}
	snap, err := json.Marshal(a.settings.GameSettings()[id])
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.ApplyPrefixPreset(id, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	again, err := json.Marshal(a.settings.GameSettings()[id])
	if err != nil {
		t.Fatal(err)
	}
	if string(snap) != string(again) {
		t.Fatalf("re-apply changed settings:\n%s\n%s", snap, again)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("re-apply result differs:\n%+v\n%+v", first, second)
	}
}

// Unknown games are refused before anything is created: a gameID like
// "../x" never reaches the filesystem (issue #52).
func TestApplyPrefixPresetUnknownGame(t *testing.T) {
	t.Parallel()
	a, _ := presetApp(t)

	for _, id := range []string{"g-nope", "../x"} {
		if _, err := a.ApplyPrefixPreset(id, "builtin:vc-fonts"); apperr.CodeOf(err) != "game.notFound" {
			t.Fatalf("apply to %q err = %v, want game.notFound", id, err)
		}
	}
	if got := a.settings.GameSettings(); len(got) != 0 {
		t.Fatalf("refused applies left settings: %v", got)
	}
}

// A pinned prefix resolves like a launch: the pin wins over the default folder.
func TestApplyPrefixPresetUsesPinnedPrefix(t *testing.T) {
	t.Parallel()
	a, home := presetApp(t)
	id := presetExeGame(t, a, home, "Pinned")

	pin := filepath.Join(home, "elsewhere", "pfx")
	if err := a.settings.SetGameSettings(id, map[string]any{"wine_prefix": pin}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ApplyPrefixPreset(id, "builtin:vc-fonts"); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(pin); err != nil || !st.IsDir() {
		t.Fatalf("pinned prefix %s: %v", pin, err)
	}
}

// Games without a Platypus prefix refuse presets, like PrefixUsage skips
// them: native builds and games their store client launches (issue #52).
func TestApplyPrefixPresetRefusesPrefixlessGames(t *testing.T) {
	t.Parallel()
	a, home := presetApp(t)

	nativeExe := filepath.Join(home, "Native", "native")
	if err := os.MkdirAll(filepath.Dir(nativeExe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nativeExe, []byte{0x7f, 'E', 'L', 'F', 0x02, 0x01, 0x01, 0x00}, 0o755); err != nil {
		t.Fatal(err)
	}
	native, err := a.settings.AddCustomGame("Native", nativeExe)
	if err != nil {
		t.Fatal(err)
	}
	a.library.set("echo", []types.Game{
		{ID: "p.echo.native", Source: "echo", Store: types.StoreGOG, Title: "Echo Native", IsInstalled: true, Native: true},
		{ID: "steam_440", Source: types.SourceSteam, Store: types.StoreSteam, StoreAppID: "440", Title: "Client Game", IsInstalled: true},
	}, nil)

	if _, err := a.ApplyPrefixPreset(native.ID, "builtin:vc-fonts"); apperr.CodeOf(err) != "settings.preset.native" {
		t.Fatalf("native sideload err = %v, want settings.preset.native", err)
	}
	if _, err := a.ApplyPrefixPreset("p.echo.native", "builtin:vc-fonts"); apperr.CodeOf(err) != "settings.preset.native" {
		t.Fatalf("native store game err = %v, want settings.preset.native", err)
	}
	if _, err := a.ApplyPrefixPreset("steam_440", "builtin:vc-fonts"); apperr.CodeOf(err) != "settings.preset.clientLaunch" {
		t.Fatalf("client launch err = %v, want settings.preset.clientLaunch", err)
	}
	if got := a.settings.GameSettings(); len(got) != 0 {
		t.Fatalf("refused applies left settings: %v", got)
	}
}

// The game's own Windows version always wins: a preset only fills a missing
// one, and the result reports what is recorded.
func TestApplyPrefixPresetKeepsUserWindowsVersion(t *testing.T) {
	t.Parallel()
	a, home := presetApp(t)
	id := presetExeGame(t, a, home, "Seven")

	if err := a.settings.SetGameSettings(id, map[string]any{"windows_version": "win7"}); err != nil {
		t.Fatal(err)
	}
	res, err := a.ApplyPrefixPreset(id, "builtin:dotnet48")
	if err != nil {
		t.Fatal(err)
	}
	if res.WindowsVersion != "win7" {
		t.Fatalf("result version = %q, want the user's win7", res.WindowsVersion)
	}
	if got := a.settings.GameSettings()[id]["windows_version"]; got != "win7" {
		t.Fatalf("recorded version = %v, want win7", got)
	}
	// The preset still applied otherwise: its verbs are recorded.
	if !reflect.DeepEqual(res.Verbs, []string{"dotnet48", "corefonts"}) {
		t.Fatalf("verbs = %v, want the preset's verbs", res.Verbs)
	}
}
