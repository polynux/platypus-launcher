package savebackup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/types"
)

func TestContextForSteamProtonGame(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	steamRoot := filepath.Join(home, ".local", "share", "Steam")
	compat := filepath.Join(steamRoot, "steamapps", "compatdata", "765", "pfx", "drive_c", "users", "steamuser", "AppData", "Roaming")
	if err := os.MkdirAll(compat, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(steamRoot, "userdata", "999"), 0o755); err != nil {
		t.Fatal(err)
	}
	game := types.Game{ID: "steam_765", Source: types.SourceSteam, Store: types.StoreSteam, StoreAppID: "765", Title: "Proton Game"}
	ctx := NewContextFor(game, home, Prefix{Store: &StoreContext{
		Roots: map[string]string{"steam": steamRoot}, StoreUserID: "999",
		Prefix:  filepath.Join(steamRoot, "steamapps", "compatdata", "765", "pfx"),
		Windows: true, Proton: true, WinUser: "steamuser",
	}})
	if !ctx.Windows || !ctx.Proton || ctx.WinUser != "steamuser" {
		t.Fatalf("ctx = %+v, want Proton steamuser", ctx)
	}
	wantPrefix := filepath.Join(steamRoot, "steamapps", "compatdata", "765", "pfx")
	if ctx.Roots["prefix"] != wantPrefix {
		t.Fatalf("prefix = %q, want %q", ctx.Roots["prefix"], wantPrefix)
	}
	if ctx.StoreUserID != "999" {
		t.Fatalf("storeUserID = %q, want 999", ctx.StoreUserID)
	}
	// <storeUserId> expands for the single-user Steam game.
	expanded, un := expandPattern(`<home>/x/<storeUserId>/save`, ctx, game)
	if un != 0 || len(expanded) != 1 || !strings.Contains(expanded[0], "999") {
		t.Fatalf("expanded = %v, un=%d, want single path with 999", expanded, un)
	}
}

func TestContextForSteamNativeGame(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	game := types.Game{ID: "steam_321", Source: types.SourceSteam, Store: types.StoreSteam, StoreAppID: "321", Title: "Native Game"}
	ctx := NewContextFor(game, home, Prefix{})
	if ctx.Windows || ctx.Proton {
		t.Fatalf("native ctx = %+v, want Windows=false", ctx)
	}
	if _, ok := ctx.Roots["prefix"]; ok {
		t.Fatalf("native ctx has prefix %q, want none", ctx.Roots["prefix"])
	}
}

func TestNonSteamContextNeedsNoSteamInstallation(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	install := filepath.Join(home, "Games", "gog")
	game := types.Game{ID: "gog-1", Source: "gog", Store: types.StoreGOG, InstallDir: install}
	ctx := NewContextFor(game, home, Prefix{})
	if ctx.Roots["home"] != home || ctx.Roots["install"] != install {
		t.Fatalf("generic roots = %v", ctx.Roots)
	}
	if _, ok := ctx.Roots["steam"]; ok {
		t.Fatalf("generic game acquired a Steam root: %v", ctx.Roots)
	}
}

func TestContextForPluginGameUsesPlatypusPrefix(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	prefixes := filepath.Join(home, "prefixes")
	game := types.Game{ID: "p.echo.tiny", Source: "echo", StoreAppID: "tiny", Title: "Echo Tiny"}
	proton := Prefix{Dir: filepath.Join(prefixes, "p.echo.tiny", "pfx"), User: "steamuser"}
	ctx := NewContextFor(game, home, proton)
	want := filepath.Join(prefixes, "p.echo.tiny", "pfx")
	if !ctx.Windows || !ctx.Proton || ctx.WinUser != "steamuser" || ctx.Roots["prefix"] != want {
		t.Fatalf("ctx = %+v, want prefix %q", ctx, want)
	}
	// A pinned prefix wins, like the runner.
	game.Settings.WinePrefix = filepath.Join(home, "mine")
	ctx = NewContextFor(game, home, Prefix{Dir: filepath.Join(home, "mine")})
	if ctx.Roots["prefix"] != filepath.Join(home, "mine") || ctx.Proton {
		t.Fatalf("pinned ctx = %+v", ctx)
	}
}

func TestContextForNativeGameHasNoPrefix(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	game := types.Game{ID: "p.echo.nat", Source: "echo", StoreAppID: "nat", Title: "Nat", Native: true}
	ctx := NewContextFor(game, home, Prefix{Dir: filepath.Join(home, "prefixes", "p.echo.nat", "pfx"), User: "steamuser"})
	if ctx.Windows {
		t.Fatalf("native ctx = %+v, want Windows=false", ctx)
	}
	if _, ok := ctx.Roots["prefix"]; ok {
		t.Fatalf("native prefix = %q, want none", ctx.Roots["prefix"])
	}
}

func TestStoreUserIDUnresolvedWhenPluginCannotResolveIt(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	game := types.Game{ID: "steam_765", Source: types.SourceSteam, Store: types.StoreSteam, StoreAppID: "765", Title: "G"}
	ctx := NewContextFor(game, home, Prefix{Store: &StoreContext{}})
	if ctx.StoreUserID != "" {
		t.Fatalf("storeUserID = %q, want empty with two users", ctx.StoreUserID)
	}
	_, un := expandPattern(`C:\users\<storeUserId>\save`, ctx, game)
	if un != 1 {
		t.Fatalf("unresolved = %d, want 1", un)
	}
}
