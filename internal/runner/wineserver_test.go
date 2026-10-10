package runner

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"platypus/internal/types"
)

func touchExec(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func hasEnv(env []string, kv string) bool { return slices.Contains(env, kv) }

// Proton keeps its Wine prefix in <compatdata>/pfx; wineserver must be asked
// about that directory, using the wineserver bundled with the same Proton.
func TestWineserverStopCommandForProton(t *testing.T) {
	dir := t.TempDir()
	proton := filepath.Join(dir, "GE-Proton", "proton")
	touchExec(t, proton)
	ws := filepath.Join(dir, "GE-Proton", "files", "bin", "wineserver")
	touchExec(t, ws)

	cmd := WineserverStopCommand(types.ProtonVersion{Path: proton}, map[string]string{"WINEPREFIX": "/pfx/game"}, false)
	if cmd == nil {
		t.Fatal("no command for a proton runner")
	}
	if cmd.Path != ws || !slices.Equal(cmd.Args[1:], []string{"-k"}) {
		t.Fatalf("got %v", cmd.Args)
	}
	if !hasEnv(cmd.Env, "WINEPREFIX=/pfx/game/pfx") {
		t.Fatalf("WINEPREFIX should point at the pfx dir, env=%v", cmd.Env)
	}
	force := WineserverStopCommand(types.ProtonVersion{Path: proton}, map[string]string{"WINEPREFIX": "/pfx/game"}, true)
	if !slices.Equal(force.Args[1:], []string{"-k9"}) {
		t.Fatalf("force args: %v", force.Args)
	}
}

func TestWineserverStopCommandForPlainWine(t *testing.T) {
	dir := t.TempDir()
	wine := filepath.Join(dir, "bin", "wine")
	touchExec(t, wine)
	ws := filepath.Join(dir, "bin", "wineserver")
	touchExec(t, ws)

	cmd := WineserverStopCommand(types.ProtonVersion{Path: wine}, map[string]string{"WINEPREFIX": "/pfx/game"}, false)
	if cmd == nil || cmd.Path != ws {
		t.Fatalf("got %+v", cmd)
	}
	if !hasEnv(cmd.Env, "WINEPREFIX=/pfx/game") {
		t.Fatalf("env=%v", cmd.Env)
	}
}

// Native games and launches without a prefix have no wineserver to stop.
func TestWineserverStopCommandNone(t *testing.T) {
	if WineserverStopCommand(types.ProtonVersion{}, map[string]string{"WINEPREFIX": "/x"}, false) != nil {
		t.Fatal("no runner, no command")
	}
	dir := t.TempDir()
	wine := filepath.Join(dir, "wine")
	touchExec(t, wine)
	touchExec(t, filepath.Join(dir, "wineserver"))
	if WineserverStopCommand(types.ProtonVersion{Path: wine}, map[string]string{}, false) != nil {
		t.Fatal("no WINEPREFIX, no command")
	}
}
