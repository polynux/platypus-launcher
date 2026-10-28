package app

import (
	"path/filepath"
	"slices"
	"testing"

	"platypus/internal/types"
)

// fakeProtonBuilds writes two Proton builds into home so the runner plugin's
// scan finds them: a GE build (what the automatic pick prefers) and a plain
// Proton release.
func fakeProtonBuilds(t *testing.T, home string) (ge, proton string) {
	t.Helper()
	ge = writeExe(t, filepath.Join(home, ".local/share/Steam/compatibilitytools.d/GE-Proton10-34/proton"), []byte("#!/bin/sh\n"), true)
	proton = writeExe(t, filepath.Join(home, ".local/share/Steam/steamapps/common/Proton 9.0/proton"), []byte("#!/bin/sh\n"), true)
	return ge, proton
}

// defaultRunnerID is the runner the list marks as the one "auto" resolves to.
func defaultRunnerID(list []types.ProtonVersion) string {
	for _, r := range list {
		if r.IsDefault {
			return r.ID
		}
	}
	return ""
}

// Picking a default runner (issue #146) is what "auto" resolves to: the
// runners list marks it, and a game without a runner_id of its own launches
// with it. An explicit per-game choice still wins, and clearing the default
// goes back to the automatic pick.
func TestDefaultRunnerIsWhatNewGamesUse(t *testing.T) {
	home := t.TempDir()
	a := nativeTestApp(t, home)
	// No umu-run on PATH: the auto path stays a direct Proton launch, which is
	// what this test reads (umu wrapping the pick is tested in internal/runner).
	t.Setenv("PATH", t.TempDir())
	gePath, protonPath := fakeProtonBuilds(t, home)

	if got := defaultRunnerID(a.GetProtonVersions()); got != "compat:GE-Proton10-34" {
		t.Fatalf("automatic default = %q, want the GE build", got)
	}
	if got := a.GetLauncherSettings().DefaultRunnerID; got != "" {
		t.Fatalf("DefaultRunnerID = %q before any pick, want empty", got)
	}

	saved, err := a.SetDefaultRunner("steam:Proton 9.0")
	if err != nil {
		t.Fatal(err)
	}
	if saved.DefaultRunnerID != "steam:Proton 9.0" {
		t.Fatalf("saved DefaultRunnerID = %q", saved.DefaultRunnerID)
	}
	list := a.GetProtonVersions()
	if got := defaultRunnerID(list); got != "steam:Proton 9.0" {
		t.Fatalf("default after the pick = %q, want the picked runner", got)
	}
	marked := 0
	for _, r := range list {
		if r.IsDefault {
			marked++
		}
	}
	if marked != 1 {
		t.Fatalf("%d runners marked default, want exactly one", marked)
	}

	// A game that never saved a runner_id launches through the picked default.
	exe := writeExe(t, filepath.Join(home, "game.exe"), []byte("MZ\x90\x00"), true)
	cg, err := a.settings.AddCustomGame("Win", exe)
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.prepareLaunch(a.customGames()[0], true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(p.args, protonPath) {
		t.Fatalf("launch args = %q, want the default runner %q", p.args, protonPath)
	}
	if slices.Contains(p.args, gePath) {
		t.Fatalf("launch args = %q, used the automatic pick instead of the default", p.args)
	}

	// An explicit per-game choice is untouched by the launcher default.
	if err := a.settings.SetGameSettings(cg.ID, map[string]any{"runner_id": "compat:GE-Proton10-34"}); err != nil {
		t.Fatal(err)
	}
	p, err = a.prepareLaunch(a.customGames()[0], true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(p.args, gePath) {
		t.Fatalf("explicit runner launch args = %q, want %q", p.args, gePath)
	}

	// "" goes back to the automatic pick.
	if saved, err = a.SetDefaultRunner(""); err != nil {
		t.Fatal(err)
	}
	if saved.DefaultRunnerID != "" {
		t.Fatalf("cleared DefaultRunnerID = %q", saved.DefaultRunnerID)
	}
	if got := defaultRunnerID(a.GetProtonVersions()); got != "compat:GE-Proton10-34" {
		t.Fatalf("default after clearing = %q, want the automatic pick", got)
	}
}
