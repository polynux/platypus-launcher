package runner

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/types"
)

// CORE-8: previewing a launch must not create on-disk state. A real launch
// still creates the prefix lazily.
func TestPreviewDoesNotCreatePrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	game := types.Game{ID: "preview-game", Title: "Preview Game"}
	r := types.ProtonVersion{}

	if _, _, err := BuildCommand(game, r, Target{Exe: "/tmp/game.exe"}, true, ""); err != nil {
		t.Fatalf("preview BuildCommand: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local/share/platypus/prefixes")); !os.IsNotExist(err) {
		t.Fatalf("preview created prefixes dir: %v", err)
	}

	if _, _, err := BuildCommand(game, r, Target{Exe: "/tmp/game.exe"}, false, ""); err != nil {
		t.Fatalf("launch BuildCommand: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local/share/platypus/prefixes", "preview-game")); err != nil {
		t.Fatalf("launch did not create prefix: %v", err)
	}
}

// Issue #37: the per-game prefix wins, then the global prefixes folder,
// then the default under Platypus's data.
func TestPrefixPathPrecedence(t *testing.T) {
	home := t.TempDir()
	global := filepath.Join(t.TempDir(), "prefixes")
	game := types.Game{ID: "g1", Settings: types.GameSettings{WinePrefix: "/per/game"}}
	if got := PrefixPath(game, home, global); got != "/per/game" {
		t.Fatalf("per-game prefix = %q", got)
	}
	game.Settings.WinePrefix = ""
	if got, want := PrefixPath(game, home, global), filepath.Join(global, "g1"); got != want {
		t.Fatalf("global folder = %q, want %q", got, want)
	}
	if got, want := PrefixPath(game, home, ""), filepath.Join(home, ".local/share/platypus/prefixes", "g1"); got != want {
		t.Fatalf("default = %q, want %q", got, want)
	}
}

// Issue #37: a launch with a global prefixes folder creates the game's
// folder inside it on demand, and a preview creates nothing.
func TestGlobalPrefixesFolderCreatedOnDemand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	global := filepath.Join(home, "my-prefixes")
	game := types.Game{ID: "global-game", Title: "Global Game"}
	r := types.ProtonVersion{}

	if _, _, err := BuildCommand(game, r, Target{Exe: "/tmp/game.exe"}, true, global); err != nil {
		t.Fatalf("preview BuildCommand: %v", err)
	}
	if _, err := os.Stat(global); !os.IsNotExist(err) {
		t.Fatalf("preview created the global folder: %v", err)
	}

	args, env, err := BuildCommand(game, r, Target{Exe: "/tmp/game.exe"}, false, global)
	if err != nil {
		t.Fatalf("launch BuildCommand: %v", err)
	}
	_ = args
	want := filepath.Join(global, "global-game")
	if env["WINEPREFIX"] != want || env["STEAM_COMPAT_DATA_PATH"] != want {
		t.Fatalf("prefix env = %q / %q, want %q", env["WINEPREFIX"], env["STEAM_COMPAT_DATA_PATH"], want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("launch did not create the prefix: %v", err)
	}
}
