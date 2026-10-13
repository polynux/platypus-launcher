package app

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/runner"
	"platypus/internal/settings"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// umuApp builds an App on a temp home with one fake Proton build. The fake
// umu-run goes on a temp PATH (issue #48: no real binaries in tests).
func umuApp(t *testing.T, withUmu bool) *App {
	t.Helper()
	home := t.TempDir()
	fakeProton := filepath.Join(home, ".local/share/Steam/compatibilitytools.d", "FakeProton")
	if err := os.MkdirAll(fakeProton, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeProton, "proton"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if withUmu {
		if err := os.WriteFile(filepath.Join(bin, "umu-run"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	root := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), home)
	if err != nil {
		t.Fatal(err)
	}
	return &App{settings: st, stores: stores.NewRegistry(nil, stores.NewSteam()), optHome: home}
}

// With umu installed, a game with no explicit runner prefers umu; an
// explicit Proton choice still runs directly.
func TestSelectRunnerPrefersUmuForAuto(t *testing.T) {
	a := umuApp(t, true)
	auto := types.Game{ID: "g", Title: "G", Settings: types.GameSettings{RunnerID: "auto"}}
	r, err := a.selectRunner(auto)
	if err != nil {
		t.Fatalf("selectRunner(auto): %v", err)
	}
	if !runner.IsUmu(r) {
		t.Fatalf("auto with umu installed picked %q, want umu", r.ID)
	}
	explicit := auto
	explicit.Settings.RunnerID = "compat:FakeProton"
	r, err = a.selectRunner(explicit)
	if err != nil {
		t.Fatalf("selectRunner(explicit): %v", err)
	}
	if runner.IsUmu(r) || r.ID != "compat:FakeProton" {
		t.Fatalf("explicit Proton picked %q, want the direct build", r.ID)
	}
}

// Explicit "umu" without the binary is a missing-runner error, not a silent
// direct launch.
func TestSelectRunnerUmuMissingErrors(t *testing.T) {
	a := umuApp(t, false)
	g := types.Game{ID: "g", Title: "G", Settings: types.GameSettings{RunnerID: "umu"}}
	if _, err := a.selectRunner(g); err == nil {
		t.Fatal("explicit umu without umu-run must fail")
	}
}

// The auto launch through umu carries GAMEID/PROTONPATH and the umu binary.
func TestPrepareLaunchAutoUsesUmu(t *testing.T) {
	a := umuApp(t, true)
	exe := filepath.Join(t.TempDir(), "g.exe")
	if err := os.WriteFile(exe, []byte("MZ\x90\x00"), 0o755); err != nil {
		t.Fatal(err)
	}
	g := types.Game{ID: "g", Title: "G", Source: types.SourceCustom, Store: types.StoreCustom, Executable: exe}
	p, err := a.prepareLaunch(g, true)
	if err != nil {
		t.Fatalf("prepareLaunch: %v", err)
	}
	if !runner.IsUmu(p.runner) {
		t.Fatalf("auto launch runner = %q, want umu", p.runner.ID)
	}
	if p.env["GAMEID"] != "umu-0" {
		t.Fatalf("GAMEID = %q, want umu-0", p.env["GAMEID"])
	}
	if p.env["PROTONPATH"] == "" || p.env["WINEPREFIX"] == "" {
		t.Fatalf("umu launch misses PROTONPATH/WINEPREFIX: %v", p.env)
	}
	found := false
	for i, arg := range p.args {
		if runner.IsUmuPath(arg) && i+1 < len(p.args) && p.args[i+1] == exe {
			found = true
		}
	}
	if !found {
		t.Fatalf("launch command %q doesn't run the exe through umu-run", p.args)
	}
}
