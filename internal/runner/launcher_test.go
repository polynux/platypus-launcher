package runner

import (
	"os"
	"path/filepath"
	"testing"

	"hydra-launcher/internal/types"
)

// CORE-8: previewing a launch must not create on-disk state. A real launch
// still creates the prefix lazily.
func TestPreviewDoesNotCreatePrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	game := types.Game{ID: "preview-game", Title: "Preview Game"}
	r := types.ProtonVersion{}

	if _, _, err := BuildCommand(game, r, Target{Exe: "/tmp/game.exe"}, true); err != nil {
		t.Fatalf("preview BuildCommand: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local/share/hydra/prefixes")); !os.IsNotExist(err) {
		t.Fatalf("preview created prefixes dir: %v", err)
	}

	if _, _, err := BuildCommand(game, r, Target{Exe: "/tmp/game.exe"}, false); err != nil {
		t.Fatalf("launch BuildCommand: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local/share/hydra/prefixes", "preview-game")); err != nil {
		t.Fatalf("launch did not create prefix: %v", err)
	}
}
