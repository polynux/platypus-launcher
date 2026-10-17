package dataplug

import (
	"strings"
	"testing"
	"testing/fstest"
)

func load(t *testing.T, files map[string]string, listed []string) (*Data, error) {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return Load(fsys, listed)
}

func TestLoadFixesAndTweaks(t *testing.T) {
	d, err := load(t, map[string]string{
		"fixes.json":  `{"fixes":[{"title":"A Game","source":"lutris","tier":"gold","description":"Needs a fix.","winetricks":["vcrun2019"]}]}`,
		"tweaks.json": `{"tweaks":[{"id":"fps","kind":"troubleshoot","title":"Show FPS","reason":"See the framerate.","env":{"DXVK_HUD":"fps"}}]}`,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Fixes) != 1 || d.Fixes[0].Title != "A Game" || len(d.Tweaks) != 1 || d.Tweaks[0].ID != "fps" {
		t.Fatalf("data: %+v", d)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		listed []string
		want   string
	}{
		{
			name:  "unknown field",
			files: map[string]string{"fixes.json": `{"fixes":[{"title":"x","source":"s","tier":"gold","description":"d","unknown":1}]}`},
			want:  "unknown field",
		},
		{
			name:  "bad tier",
			files: map[string]string{"fixes.json": `{"fixes":[{"title":"x","source":"s","tier":"great","description":"d"}]}`},
			want:  "tier",
		},
		{
			name:  "malformed winetricks identifier",
			files: map[string]string{"fixes.json": `{"fixes":[{"title":"x","source":"s","tier":"gold","description":"d","winetricks":["bad verb"]}]}`},
			want:  "not a verb",
		},
		{
			name:  "fix without match key",
			files: map[string]string{"fixes.json": `{"fixes":[{"source":"s","tier":"gold","description":"d"}]}`},
			want:  "title or an appId",
		},
		{
			name:  "tweak without effect",
			files: map[string]string{"tweaks.json": `{"tweaks":[{"id":"x","kind":"recommended","title":"T","reason":"R"}]}`},
			want:  "env vars, wrappers or both",
		},
		{
			name:  "tweak bad kind",
			files: map[string]string{"tweaks.json": `{"tweaks":[{"id":"x","kind":"maybe","title":"T","reason":"R","env":{"A":"1"}}]}`},
			want:  "kind",
		},
		{
			name:  "tweak bad gpu",
			files: map[string]string{"tweaks.json": `{"tweaks":[{"id":"x","kind":"recommended","title":"T","reason":"R","when":{"gpu":["matrox"]},"env":{"A":"1"}}]}`},
			want:  "when.gpu",
		},
		{
			name:  "duplicate tweak id",
			files: map[string]string{"tweaks.json": `{"tweaks":[{"id":"x","kind":"recommended","title":"T","reason":"R","env":{"A":"1"}},{"id":"x","kind":"recommended","title":"T2","reason":"R2","env":{"A":"2"}}]}`},
			want:  "duplicate tweak id",
		},
		{
			name:   "listed file missing",
			files:  map[string]string{"fixes.json": `{"fixes":[]}`},
			listed: []string{"fixes.json", "tweaks.json"},
			want:   "tweaks.json",
		},
		{
			name:  "no files at all",
			files: map[string]string{"other.json": `{}`},
			want:  "no data files",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := load(t, c.files, c.listed)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error containing %q, got %v", c.want, err)
			}
		})
	}
}

func TestRegistryFix(t *testing.T) {
	r := NewRegistry()
	r.Add(&Data{Fixes: []Fix{
		{Title: "By Title", Source: "a", Tier: "gold", Description: "one"},
		{AppID: "620", Source: "b", Tier: "silver", Description: "two"},
	}})
	if f := r.Fix("By Title", "999"); f == nil || f.Source != "a" || f.Description != "one" {
		t.Fatalf("by title: %+v", f)
	}
	if f := r.Fix("Other", "620"); f == nil || f.Source != "b" {
		t.Fatalf("by app id: %+v", f)
	}
	if f := r.Fix("Nothing", "1"); f != nil {
		t.Fatalf("unexpected fix: %+v", f)
	}
	// First match wins, so a plugin can't override the built-in recipe.
	r.Add(&Data{Fixes: []Fix{{Title: "By Title", Source: "override", Tier: "borked", Description: "no"}}})
	if f := r.Fix("By Title", ""); f.Source != "a" {
		t.Fatalf("later fix won: %+v", f)
	}
}

// The built-in tweaks must be exactly what the hardcoded Go list produced.
func TestBuiltinTweaksMatchOldBehaviour(t *testing.T) {
	r := NewRegistry()
	r.Add(Builtin())

	system := System{GPUVendors: []string{"nvidia", "amd"}, Wayland: true, NTSync: true, GameMode: true, MangoHud: true}
	got := r.Tweaks(system)
	want := []string{"nvidia-nvapi", "nvidia-shader-cache", "nvidia-hide", "amd-fsr", "ntsync", "gamemode", "wayland", "no-d3d12", "wined3d", "log"}
	if len(got) != len(want) {
		t.Fatalf("got %d tweaks, want %d: %+v", len(got), len(want), got)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("tweak %d = %q, want %q", i, got[i].ID, id)
		}
	}
	// A machine without NVIDIA, ntsync or Wayland gets the universal ones plus GameMode.
	bare := r.Tweaks(System{GameMode: true})
	var bareIDs []string
	for _, tw := range bare {
		bareIDs = append(bareIDs, tw.ID)
	}
	if strings.Join(bareIDs, ",") != "gamemode,no-d3d12,wined3d,log" {
		t.Fatalf("bare machine: %v", bareIDs)
	}
	// Content and order of the first tweak, and the gamemode wrapper.
	first := got[0]
	if first.Kind != "recommended" || first.Title != "Enable NVAPI" || first.Env["PROTON_ENABLE_NVAPI"] != "1" || first.Wrappers != nil {
		t.Fatalf("first tweak: %+v", first)
	}
	if gm := got[5]; gm.Title != "Run with GameMode" || len(gm.Wrappers) != 1 || gm.Wrappers[0] != "gamemode" {
		t.Fatalf("gamemode tweak: %+v", gm)
	}
	// A negative condition keeps a tweak off a machine that lacks it.
	if ntsync := got[4]; ntsync.Env["PROTON_USE_NTSYNC"] != "1" {
		t.Fatalf("ntsync tweak: %+v", ntsync)
	}
}

func TestBuiltinFixes(t *testing.T) {
	r := NewRegistry()
	r.Add(Builtin())
	f := r.Fix("Fallout: London", "")
	if f == nil || f.Source != "lutris" || f.Tier != "gold" || len(f.Winetricks) != 2 || f.DLLOverrides["xaudio2_7"] != "n,b" {
		t.Fatalf("fallout: %+v", f)
	}
	if r.Fix("Some Other Game", "") != nil {
		t.Fatal("matched an unknown title")
	}
}

// Arcanum's fix is a ddraw DLL override, not a winetricks verb: there is
// no "ddraw" verb, and the override is what its wrapper needs.
func TestBuiltinArcanumFix(t *testing.T) {
	r := NewRegistry()
	r.Add(Builtin())
	f := r.Fix("Arcanum: Of Steamworks and Magick Obscura", "")
	if f == nil {
		t.Fatal("no Arcanum fix")
	}
	if len(f.Winetricks) != 0 {
		t.Fatalf("arcanum verbs = %v, want none", f.Winetricks)
	}
	if f.DLLOverrides["ddraw"] != "n,b" {
		t.Fatalf("arcanum overrides = %v, want ddraw n,b", f.DLLOverrides)
	}
}

func TestTweakIDsCannotBeOverridden(t *testing.T) {
	r := NewRegistry()
	r.Add(Builtin())
	r.Add(&Data{Tweaks: []Tweak{{ID: "log", Kind: "recommended", Title: "Mine", Reason: "R", Env: map[string]string{"X": "1"}}}})
	for _, tw := range r.Tweaks(System{}) {
		if tw.ID == "log" && tw.Title != "Write a Proton log" {
			t.Fatalf("built-in tweak replaced: %+v", tw)
		}
	}
}
