package savebackup

import (
	"path/filepath"
	"testing"

	"platypus/internal/types"
)

// A umu game's prefix is the one the runner plugin reports (the effective umu
// prefix, issue #48). The context uses it as reported, with the steamuser
// account, so saves agree with the launch.
func TestContextForUmuGame(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	game := types.Game{ID: "p.umu.two", Source: "echo", StoreAppID: "two", Title: "Umu Two"}
	want := filepath.Join(home, "prefixes", "p.umu.two", "pfx")
	ctx := NewContextFor(game, home, Prefix{Dir: want, User: "steamuser"})
	if !ctx.Windows || !ctx.Proton || ctx.WinUser != "steamuser" || ctx.Roots["prefix"] != want {
		t.Fatalf("umu ctx = %+v, want prefix %q with steamuser", ctx, want)
	}
}
