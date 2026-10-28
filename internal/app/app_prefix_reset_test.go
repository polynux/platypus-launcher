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
// A custom prefix through a symlinked subfolder must not escape the root:
// <root>/link/Documents with link pointing outside is refused, and the
// outside folder is left alone.
func TestResetPrefixRefusesSymlinkedParent(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "Documents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "Documents", "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, _ := resetTestApp(t, []types.Game{resetWineGame("linked", nil)})
	// The saved setting wins over the default, like a launch.
	root := a.effectivePrefixesFolder()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetGameSettings("linked", map[string]any{"wine_prefix": filepath.Join(root, "link", "Documents")}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ResetPrefix("linked"); resetCode(t, err) != "prefix.reset.outsideRoot" {
		t.Fatalf("reset through a symlinked parent = %v, want a refusal", err)
	}
	if data, err := os.ReadFile(filepath.Join(outside, "Documents", "keep.txt")); err != nil || string(data) != "keep" {
		t.Fatalf("the outside folder was touched: %q, %v", data, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "link", "Documents", runner.FreshMarker)); !os.IsNotExist(err) {
		t.Fatalf("a fresh prefix was created through the link: %v", err)
	}
}

// A prefix that is itself a symlink is refused, never followed.
func TestResetPrefixRefusesSymlinkedPrefix(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, home := resetTestApp(t, []types.Game{resetWineGame("linkedself", nil)})
	prefix := runner.PrefixPath(types.Game{ID: "linkedself"}, home, a.effectivePrefixesFolder())
	if err := os.MkdirAll(filepath.Dir(prefix), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, prefix); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ResetPrefix("linkedself"); resetCode(t, err) != "prefix.reset.symlink" {
		t.Fatalf("reset of a symlinked prefix = %v, want a refusal", err)
	}
	if data, err := os.ReadFile(filepath.Join(outside, "keep.txt")); err != nil || string(data) != "keep" {
		t.Fatalf("the link target was touched: %q, %v", data, err)
	}
}

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

// A custom prefix whose parent doesn't exist yet under a symlinked folder is
// refused too (issue #163): the missing part must not hide the link, or the
// reset would create empty folders and its marker outside the root.
func TestResetPrefixRefusesMissingParentUnderASymlink(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	a, _ := resetTestApp(t, []types.Game{resetWineGame("missing", nil)})
	root := a.effectivePrefixesFolder()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	// The saved setting wins over the default, like a launch.
	prefix := filepath.Join(root, "link", "new", "Documents")
	if err := a.settings.SetGameSettings("missing", map[string]any{"wine_prefix": prefix}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ResetPrefix("missing"); resetCode(t, err) != "prefix.reset.outsideRoot" {
		t.Fatalf("reset under a link with a missing parent = %v, want a refusal", err)
	}
	if _, err := os.Lstat(filepath.Join(outside, "new")); !os.IsNotExist(err) {
		t.Fatalf("folders were created outside the prefixes folder: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "link", "new", "Documents", runner.FreshMarker)); !os.IsNotExist(err) {
		t.Fatalf("a fresh prefix was created through the link: %v", err)
	}
}

// Prefix backups pile up silently: Storage lists them with their size, and
// deleting one frees it (issue #163).
func TestPrefixBackupsListAndDelete(t *testing.T) {
	t.Parallel()
	a, _ := resetTestApp(t, []types.Game{resetWineGame("kept", nil)})
	root := a.effectivePrefixesFolder()
	old := filepath.Join(root, "kept.bak-1700000000")
	if err := os.MkdirAll(filepath.Join(old, "drive_c"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(old, "drive_c", "x.dll"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	newer := filepath.Join(root, "kept.bak-1800000000-1")
	if err := os.MkdirAll(newer, 0o755); err != nil {
		t.Fatal(err)
	}
	// A game's own prefix and a link are not backups.
	if err := os.MkdirAll(filepath.Join(root, "kept"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(old, filepath.Join(root, "linked.bak-1")); err != nil {
		t.Fatal(err)
	}

	got := a.PrefixBackups()
	if len(got) != 2 {
		t.Fatalf("backups = %+v, want the two .bak-* folders only", got)
	}
	// Stalest first, with the space each takes.
	if got[0].Name != "kept.bak-1700000000" || got[0].Bytes != 5 || got[0].At != 1700000000 {
		t.Fatalf("first backup = %+v", got[0])
	}
	if got[1].Name != "kept.bak-1800000000-1" || got[1].Bytes != 0 {
		t.Fatalf("second backup = %+v", got[1])
	}

	freed, err := a.DeletePrefixBackup(old)
	if err != nil {
		t.Fatal(err)
	}
	if freed != 5 {
		t.Fatalf("freed = %d, want the folder's bytes", freed)
	}
	if _, err := os.Lstat(old); !os.IsNotExist(err) {
		t.Fatalf("the backup survived: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(root, "kept")); err != nil {
		t.Fatalf("deleting a backup touched the game's prefix: %v", err)
	}
	if got := a.PrefixBackups(); len(got) != 1 || got[0].Path != newer {
		t.Fatalf("after the delete: %+v", got)
	}
}

// Only a backup folder in the prefixes folder can be deleted: the path comes
// from the UI, so core checks it again.
func TestDeletePrefixBackupRefusesAnythingElse(t *testing.T) {
	t.Parallel()
	a, _ := resetTestApp(t, []types.Game{resetWineGame("safe", nil)})
	root := a.effectivePrefixesFolder()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	prefix := filepath.Join(root, "safe")
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "elsewhere.bak-1")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{prefix, outside, filepath.Join(root, "safe.txt")} {
		if _, err := a.DeletePrefixBackup(path); err == nil {
			t.Fatalf("deleted %q, want a refusal", path)
		}
	}
	if _, err := os.Lstat(prefix); err != nil {
		t.Fatalf("the game's prefix was deleted: %v", err)
	}
	if _, err := os.Lstat(outside); err != nil {
		t.Fatalf("a folder outside the prefixes folder was deleted: %v", err)
	}
}

// A backup a game points back at is its live prefix (m2): it is never listed
// as reclaimable, deleting it is refused, and a running game keeps it too.
func TestPrefixBackupsLeaveALivePrefixAlone(t *testing.T) {
	t.Parallel()
	a, _ := resetTestApp(t, []types.Game{resetWineGame("restored", nil), resetWineGame("busy", nil)})
	root := a.effectivePrefixesFolder()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	// The user "restored" an old prefix by pointing the game at it.
	restored := filepath.Join(root, "restored.bak-1700000000")
	busy := filepath.Join(root, "busy.bak-1700000001")
	spare := filepath.Join(root, "gone.bak-1700000002")
	for _, dir := range []string{restored, busy, spare} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.settings.SetGameSettings("restored", map[string]any{"wine_prefix": restored}); err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetGameSettings("busy", map[string]any{"wine_prefix": busy}); err != nil {
		t.Fatal(err)
	}
	a.sessionRunning = func(id string) bool { return id == "busy" }

	got := a.PrefixBackups()
	if len(got) != 1 || got[0].Path != spare {
		t.Fatalf("backups = %+v, want only the one no game uses", got)
	}
	if _, err := a.DeletePrefixBackup(restored); resetCode(t, err) != "prefix.backup.inUse" {
		t.Fatalf("deleting a live prefix = %v, want prefix.backup.inUse", err)
	}
	if _, err := a.DeletePrefixBackup(busy); resetCode(t, err) != "game.running" {
		t.Fatalf("deleting the prefix of a running game = %v, want game.running", err)
	}
	if _, err := os.Lstat(restored); err != nil {
		t.Fatalf("the live prefix was deleted: %v", err)
	}
}

// A parent that can't be resolved at all (a dangling link, a loop, no access)
// is refused rather than trusted lexically (m5): the reset must not write
// through it, even outside the root.
func TestResetPrefixRefusesAParentItCannotResolve(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	a, _ := resetTestApp(t, []types.Game{resetWineGame("dangling", nil)})
	root := a.effectivePrefixesFolder()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(root, "dang")); err != nil {
		t.Fatal(err)
	}
	prefix := filepath.Join(root, "dang", "Documents")
	if err := a.settings.SetGameSettings("dangling", map[string]any{"wine_prefix": prefix}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ResetPrefix("dangling"); resetCode(t, err) != "prefix.reset.outsideRoot" {
		t.Fatalf("reset through a dangling link = %v, want a refusal", err)
	}
	if _, err := os.Lstat(filepath.Join(outside, "missing")); !os.IsNotExist(err) {
		t.Fatalf("the reset created the link's target: %v", err)
	}
}
