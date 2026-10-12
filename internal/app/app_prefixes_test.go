package app

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/runner"
	"platypus/internal/settings"
	"platypus/internal/types"
)

// gameWithID is a game carrying the store's pinned prefix, so runner.PrefixPath
// resolves it the way a launch would.
func gameWithID(id string, st *settings.Store) types.Game {
	g := types.Game{ID: id}
	if m, ok := st.GameSettings()[id]; ok {
		if s, _ := m["wine_prefix"].(string); s != "" {
			g.Settings.WinePrefix = s
		}
	}
	return g
}

// Changing the prefixes folder must not strand existing prefixes: a game
// whose prefix folder exists in the old folder keeps it (pinned in its
// settings), while a game with no prefix gets the new folder.
func TestSetPrefixesFolderPinsExistingPrefixes(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	// The old effective folder is the default: <home>/.local/share/platypus/prefixes.
	old := filepath.Join(home, ".local/share/platypus", "prefixes")

	// g1 launched before: its prefix folder holds saves, and it has no
	// wine_prefix of its own.
	g1dir := filepath.Join(old, "g1")
	if err := os.MkdirAll(g1dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(g1dir, "save.dat"), []byte("saves"), 0o644); err != nil {
		t.Fatal(err)
	}
	// g2 has a prefix of its own elsewhere.
	own := filepath.Join(home, "elsewhere", "g2prefix")
	if err := os.MkdirAll(own, 0o755); err != nil {
		t.Fatal(err)
	}

	st, err := settings.OpenWithHome(filepath.Join(t.TempDir(), "settings.json"), home)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetGameSettings("g2", map[string]any{"wine_prefix": own}); err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, optHome: home}

	next := filepath.Join(home, "new-prefixes")
	ls, err := a.SetPrefixesFolder(next)
	if err != nil {
		t.Fatalf("SetPrefixesFolder: %v", err)
	}
	if ls.PrefixesFolder != next {
		t.Fatalf("PrefixesFolder = %q, want %q", ls.PrefixesFolder, next)
	}

	// g1 keeps its old prefix, pinned in its settings.
	pinned, ok := st.GameSettings()["g1"]["wine_prefix"]
	if !ok || pinned != g1dir {
		t.Fatalf("g1 wine_prefix = %v, want %q", pinned, g1dir)
	}
	if got := runner.PrefixPath(gameWithID("g1", st), home, next); got != g1dir {
		t.Fatalf("g1 resolves to %q, want its old prefix %q", got, g1dir)
	}
	// g2's own prefix is untouched.
	if got := st.GameSettings()["g2"]["wine_prefix"]; got != own {
		t.Fatalf("g2 wine_prefix = %v, want %q", got, own)
	}
	// g3 never had a prefix: it gets the new folder.
	if got := runner.PrefixPath(gameWithID("g3", st), home, next); got != filepath.Join(next, "g3") {
		t.Fatalf("g3 resolves to %q, want the new folder", got)
	}
	// Nothing was moved or deleted.
	if _, err := os.Stat(filepath.Join(g1dir, "save.dat")); err != nil {
		t.Fatalf("g1's saves are gone: %v", err)
	}
}

// Resetting to the default pins the same way: the old custom folder's
// prefixes stay where they are.
func TestSetPrefixesFolderResetPinsExistingPrefixes(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	custom := filepath.Join(home, "custom-prefixes")
	g2dir := filepath.Join(custom, "g2")
	if err := os.MkdirAll(g2dir, 0o755); err != nil {
		t.Fatal(err)
	}

	st, err := settings.OpenWithHome(filepath.Join(t.TempDir(), "settings.json"), home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetPrefixesFolder(custom); err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, optHome: home}

	ls, err := a.SetPrefixesFolder("")
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if ls.PrefixesFolder != "" {
		t.Fatalf("after reset PrefixesFolder = %q, want empty", ls.PrefixesFolder)
	}
	if pinned := st.GameSettings()["g2"]["wine_prefix"]; pinned != g2dir {
		t.Fatalf("g2 wine_prefix = %v, want %q", pinned, g2dir)
	}
}

// Chaining A→B→C keeps the prefixes created under each folder: after two
// switches, g1 still resolves into A and g2 into B.
func TestSetPrefixesFolderChainingKeepsEarlierPrefixes(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	folderA := filepath.Join(home, ".local/share/platypus", "prefixes")
	if err := os.MkdirAll(filepath.Join(folderA, "g1"), 0o755); err != nil {
		t.Fatal(err)
	}

	st, err := settings.OpenWithHome(filepath.Join(t.TempDir(), "settings.json"), home)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, optHome: home}

	folderB := filepath.Join(home, "prefixes-B")
	if _, err := a.SetPrefixesFolder(folderB); err != nil {
		t.Fatal(err)
	}
	// A launch under B creates g2's prefix there.
	if err := os.MkdirAll(filepath.Join(folderB, "g2"), 0o755); err != nil {
		t.Fatal(err)
	}

	folderC := filepath.Join(home, "prefixes-C")
	ls, err := a.SetPrefixesFolder(folderC)
	if err != nil {
		t.Fatal(err)
	}
	if ls.PrefixesFolder != folderC {
		t.Fatalf("PrefixesFolder = %q, want %q", ls.PrefixesFolder, folderC)
	}
	if pinned := st.GameSettings()["g1"]["wine_prefix"]; pinned != filepath.Join(folderA, "g1") {
		t.Fatalf("g1 wine_prefix = %v, want the A path", pinned)
	}
	if pinned := st.GameSettings()["g2"]["wine_prefix"]; pinned != filepath.Join(folderB, "g2") {
		t.Fatalf("g2 wine_prefix = %v, want the B path", pinned)
	}
	if got := runner.PrefixPath(gameWithID("g1", st), home, folderC); got != filepath.Join(folderA, "g1") {
		t.Fatalf("g1 resolves to %q, want A", got)
	}
	if got := runner.PrefixPath(gameWithID("g2", st), home, folderC); got != filepath.Join(folderB, "g2") {
		t.Fatalf("g2 resolves to %q, want B", got)
	}
}

// A missing old folder (fresh user) switches without error and pins nothing.
func TestSetPrefixesFolderWithoutOldFolder(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(t.TempDir(), "settings.json"), home)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, optHome: home}
	next := filepath.Join(home, "prefixes")
	if _, err := a.SetPrefixesFolder(next); err != nil {
		t.Fatalf("SetPrefixesFolder: %v", err)
	}
	if len(st.GameSettings()) != 0 {
		t.Fatalf("nothing to keep, but settings gained %v", st.GameSettings())
	}
}
