package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/apperr"
	"platypus/internal/runner"
	"platypus/internal/types"
)

// resetTestApp is an App whose library holds the given installed wine games
// and whose prefixes live under a temp dir.
func resetTestApp(t *testing.T, games []types.Game) (*App, string) {
	t.Helper()
	src := &fakeSource{id: "echo", games: games}
	a := libraryApp(t, src)
	home := t.TempDir()
	a.optHome = home
	a.sessionRunning = func(string) bool { return false }
	if _, err := a.GetLibrary(); err != nil {
		t.Fatal(err)
	}
	return a, home
}

func resetWineGame(id string, mutate func(*types.Game)) types.Game {
	g := types.Game{ID: id, Source: "echo", Store: "echo", StoreAppID: id, Title: id, IsInstalled: true}
	if mutate != nil {
		mutate(&g)
	}
	return g
}

func resetCode(t *testing.T, err error) string {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want an apperr", err)
	}
	return ae.Code
}

// Issue #144 review: resetting moves the broken prefix aside and recreates
// it fresh, so the next scan no longer flags it.
func TestResetPrefixMovesBrokenPrefixAsideAndRecreates(t *testing.T) {
	t.Parallel()
	a, home := resetTestApp(t, []types.Game{resetWineGame("broke", nil)})
	prefix := runner.PrefixPath(types.Game{ID: "broke"}, home, a.effectivePrefixesFolder())
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prefix, "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := a.ResetPrefix("broke")
	if err != nil {
		t.Fatal(err)
	}
	if res.Prefix != prefix {
		t.Fatalf("prefix = %q, want %q", res.Prefix, prefix)
	}
	if res.Backup == "" {
		t.Fatal("no backup: the old prefix must be kept, not deleted")
	}
	if data, err := os.ReadFile(filepath.Join(res.Backup, "old.txt")); err != nil || string(data) != "old" {
		t.Fatalf("backup kept %q, %v: the old files must survive", data, err)
	}
	if st, err := os.Stat(filepath.Join(prefix, "drive_c")); err == nil && st.IsDir() {
		t.Fatal("the fresh prefix must not invent a Windows side")
	}
	if _, err := os.Stat(filepath.Join(prefix, runner.FreshMarker)); err != nil {
		t.Fatalf("no fresh marker: %v", err)
	}
	// The next scan reads the fresh prefix as fresh, not damaged.
	lib, err := a.GetLibrary()
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range lib.Games {
		if g.ID == "broke" && g.PrefixDamaged {
			t.Fatalf("a just-reset prefix still reads damaged: %+v", g)
		}
	}
}

// Resetting a game with no prefix just creates it: nothing to keep.
func TestResetPrefixMissingPrefixCreatesFresh(t *testing.T) {
	t.Parallel()
	a, home := resetTestApp(t, []types.Game{resetWineGame("fresh", nil)})
	prefix := runner.PrefixPath(types.Game{ID: "fresh"}, home, a.effectivePrefixesFolder())
	res, err := a.ResetPrefix("fresh")
	if err != nil {
		t.Fatal(err)
	}
	if res.Backup != "" {
		t.Fatalf("backup = %q, want none: there was no prefix", res.Backup)
	}
	if _, err := os.Stat(filepath.Join(res.Prefix, runner.FreshMarker)); err != nil {
		t.Fatalf("no fresh prefix: %v", err)
	}
	if res.Prefix != prefix {
		t.Fatalf("prefix = %q, want %q", res.Prefix, prefix)
	}
}

// Only the game's own prefix path is ever touched: a user-chosen folder
// outside the prefixes root is refused.
func TestResetPrefixRefusesCustomPrefixOutsideRoot(t *testing.T) {
	t.Parallel()
	outside := filepath.Join(t.TempDir(), "mine")
	a, _ := resetTestApp(t, []types.Game{resetWineGame("custom", func(g *types.Game) {
		g.Settings.WinePrefix = outside
	})})
	// The saved setting wins over the default, like a launch.
	if err := a.settings.SetGameSettings("custom", map[string]any{"wine_prefix": outside}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ResetPrefix("custom"); resetCode(t, err) != "prefix.reset.outsideRoot" {
		t.Fatalf("resetting a custom folder must be refused, got %v", err)
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatal("the custom folder was touched")
	}
}

func TestResetPrefixRefusesGamesWithoutACorePrefix(t *testing.T) {
	t.Parallel()
	a, _ := resetTestApp(t, []types.Game{
		resetWineGame("native", func(g *types.Game) { g.Native = true }),
		resetWineGame("client", func(g *types.Game) { g.ClientLaunch = true }),
	})
	if _, err := a.ResetPrefix("native"); resetCode(t, err) != "prefix.reset.native" {
		t.Fatalf("native reset = %v, want a refusal", err)
	}
	if _, err := a.ResetPrefix("client"); resetCode(t, err) != "prefix.reset.clientLaunch" {
		t.Fatalf("client reset = %v, want a refusal", err)
	}
	if _, err := a.ResetPrefix("no-such-game"); resetCode(t, err) != "game.notFound" {
		t.Fatalf("unknown reset = %v, want game.notFound", err)
	}
}

func TestResetPrefixRefusesRunningGame(t *testing.T) {
	t.Parallel()
	a, _ := resetTestApp(t, []types.Game{resetWineGame("busy", nil)})
	a.sessionRunning = func(id string) bool { return id == "busy" }
	if _, err := a.ResetPrefix("busy"); resetCode(t, err) != "game.running" {
		t.Fatalf("running reset = %v, want game.running", err)
	}
}

// The assembled library flags store-only history but honours a fresh reset.
func TestResetPrefixClearsTheFlagForAPlayedGame(t *testing.T) {
	t.Parallel()
	a, home := resetTestApp(t, []types.Game{resetWineGame("played", func(g *types.Game) {
		g.LastPlayed = 1700000000
	})})
	prefix := runner.PrefixPath(types.Game{ID: "played"}, home, a.effectivePrefixesFolder())
	// A broken prefix on a played game flags; resetting clears it.
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		t.Fatal(err)
	}
	lib, err := a.GetLibrary()
	if err != nil {
		t.Fatal(err)
	}
	flagged := false
	for _, g := range lib.Games {
		if g.ID == "played" {
			flagged = g.PrefixDamaged
		}
	}
	if !flagged {
		t.Fatal("a broken prefix on a played game should flag before the reset")
	}
	if _, err := a.ResetPrefix("played"); err != nil {
		t.Fatal(err)
	}
	lib, err = a.GetLibrary()
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range lib.Games {
		if g.ID == "played" && g.PrefixDamaged {
			t.Fatal("the flag stayed after the reset")
		}
	}
}
