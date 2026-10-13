package savebackup

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/types"
)

// umu runs with the pfx dir itself (issue #48): a fresh prefix resolves to
// the prefix, a Proton-made one (<compat>/pfx) to its pfx dir, both with the
// steamuser account, so saves agree with the launch.
func TestContextForUmuGame(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	prefixes := filepath.Join(home, "prefixes")
	game := types.Game{ID: "p.umu.one", Source: "echo", StoreAppID: "one", Title: "Umu One"}
	ctx := NewContextFor(game, home, prefixes, filepath.Join(home, "data"), "/usr/bin/umu-run", "")
	want := filepath.Join(prefixes, "p.umu.one")
	if !ctx.Windows || ctx.WinUser != "steamuser" || ctx.Roots["prefix"] != want {
		t.Fatalf("fresh umu ctx = %+v, want prefix %q with steamuser", ctx, want)
	}

	legacy := types.Game{ID: "p.umu.two", Source: "echo", StoreAppID: "two", Title: "Umu Two"}
	if err := os.MkdirAll(filepath.Join(prefixes, "p.umu.two", "pfx", "drive_c"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx = NewContextFor(legacy, home, prefixes, filepath.Join(home, "data"), "/usr/bin/umu-run", "")
	if want := filepath.Join(prefixes, "p.umu.two", "pfx"); ctx.Roots["prefix"] != want || ctx.WinUser != "steamuser" {
		t.Fatalf("legacy umu ctx = %+v, want prefix %q with steamuser", ctx, want)
	}
}
