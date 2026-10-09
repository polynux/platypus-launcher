package relocate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// game builds a small install folder under root/from: nested files, an
// empty folder and a relative link.
func game(t *testing.T, root string) string {
	t.Helper()
	from := filepath.Join(root, "src", "Game")
	must(t, os.MkdirAll(filepath.Join(from, "data", "deep"), 0o755))
	must(t, os.WriteFile(filepath.Join(from, "game.sh"), []byte("#!/bin/sh\n"), 0o755))
	must(t, os.WriteFile(filepath.Join(from, "data", "a.pak"), make([]byte, 300<<10), 0o644))
	must(t, os.WriteFile(filepath.Join(from, "data", "deep", "b.bin"), []byte("bee"), 0o600))
	must(t, os.MkdirAll(filepath.Join(from, "empty"), 0o755))
	must(t, os.Symlink("game.sh", filepath.Join(from, "run")))
	return from
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// sameFS is a stand-in for two filesystems: false is "across".
func sameFS(same bool) func(a, b string) (bool, error) {
	return func(a, b string) (bool, error) { return same, nil }
}

func TestSameFilesystemRenamesTheFolder(t *testing.T) {
	root := t.TempDir()
	from := game(t, root)
	to := filepath.Join(root, "dst", "Game")
	must(t, os.MkdirAll(filepath.Dir(to), 0o755))
	res, err := Move(context.Background(), from, to, Options{SameFS: sameFS(true)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Leftover != "" {
		t.Fatalf("leftover %q after a rename", res.Leftover)
	}
	if _, err := os.Stat(from); !os.IsNotExist(err) {
		t.Fatalf("source still there: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(to, "data", "deep", "b.bin")); string(b) != "bee" {
		t.Fatalf("moved file = %q", b)
	}
}

func TestCrossFilesystemCopiesVerifiesAndRemovesTheSource(t *testing.T) {
	root := t.TempDir()
	from := game(t, root)
	to := filepath.Join(root, "dst", "Game")
	must(t, os.MkdirAll(filepath.Dir(to), 0o755))
	var last, total int64
	res, err := Move(context.Background(), from, to, Options{
		SameFS:   sameFS(false),
		Progress: func(done, all int64) { last, total = done, all },
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Leftover != "" {
		t.Fatalf("leftover %q", res.Leftover)
	}
	if last != total || total != 300<<10+3+10 {
		t.Fatalf("progress ended at %d of %d", last, total)
	}
	if _, err := os.Stat(from); !os.IsNotExist(err) {
		t.Fatalf("source not removed: %v", err)
	}
	if l, err := os.Readlink(filepath.Join(to, "run")); err != nil || l != "game.sh" {
		t.Fatalf("link = %q, %v", l, err)
	}
	if info, err := os.Stat(filepath.Join(to, "game.sh")); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("script mode: %v %v", info, err)
	}
	if _, err := os.Stat(to + StageSuffix); !os.IsNotExist(err) {
		t.Fatalf("staging folder left behind: %v", err)
	}
}

func TestCancelRollsBackAndKeepsTheSource(t *testing.T) {
	root := t.TempDir()
	from := game(t, root)
	to := filepath.Join(root, "dst", "Game")
	must(t, os.MkdirAll(filepath.Dir(to), 0o755))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := Move(ctx, from, to, Options{
		SameFS: sameFS(false),
		Progress: func(done, total int64) {
			if done > 0 {
				cancel()
			}
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want cancel", err)
	}
	if b, err := os.ReadFile(filepath.Join(from, "data", "deep", "b.bin")); err != nil || string(b) != "bee" {
		t.Fatalf("source damaged: %q %v", b, err)
	}
	if _, err := os.Stat(to); !os.IsNotExist(err) {
		t.Fatalf("destination left: %v", err)
	}
	if _, err := os.Stat(to + StageSuffix); !os.IsNotExist(err) {
		t.Fatalf("staging folder left: %v", err)
	}
}

func TestAFailedCopyRollsBackAndKeepsTheSource(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads unreadable files, so there is no failure to provoke")
	}
	root := t.TempDir()
	from := game(t, root)
	locked := filepath.Join(from, "data", "a.pak")
	must(t, os.Chmod(locked, 0o000))
	to := filepath.Join(root, "dst", "Game")
	must(t, os.MkdirAll(filepath.Dir(to), 0o755))
	if _, err := Move(context.Background(), from, to, Options{SameFS: sameFS(false)}); err == nil {
		t.Fatal("a move that can't read a file succeeded")
	}
	must(t, os.Chmod(locked, 0o644))
	if _, err := os.Stat(from); err != nil {
		t.Fatalf("source gone: %v", err)
	}
	if _, err := os.Stat(to + StageSuffix); !os.IsNotExist(err) {
		t.Fatalf("staging folder left: %v", err)
	}
	if _, err := os.Stat(to); !os.IsNotExist(err) {
		t.Fatalf("destination left: %v", err)
	}
}

func TestAnInterruptedStagingFolderIsReplaced(t *testing.T) {
	root := t.TempDir()
	from := game(t, root)
	to := filepath.Join(root, "dst", "Game")
	must(t, os.MkdirAll(to+StageSuffix, 0o755))
	must(t, os.WriteFile(filepath.Join(to+StageSuffix, "partial"), []byte("x"), 0o644))
	if _, err := Move(context.Background(), from, to, Options{SameFS: sameFS(false)}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(to, "partial")); !os.IsNotExist(err) {
		t.Fatal("an earlier partial copy ended up in the game")
	}
}

func TestRefusals(t *testing.T) {
	root := t.TempDir()
	from := game(t, root)
	must(t, os.MkdirAll(filepath.Join(root, "taken"), 0o755))
	cases := map[string]string{
		"the same folder":         from,
		"inside the game":         filepath.Join(from, "data", "elsewhere"),
		"a folder the game is in": filepath.Dir(from),
		"an existing folder":      filepath.Join(root, "taken"),
		"a missing parent":        filepath.Join(root, "nope", "deeper", "Game"),
		"a relative path":         "relative/Game",
	}
	for name, to := range cases {
		if _, err := Move(context.Background(), from, to, Options{SameFS: sameFS(true)}); err == nil {
			t.Errorf("%s: move to %q accepted", name, to)
		}
	}
	if _, err := os.Stat(filepath.Join(from, "game.sh")); err != nil {
		t.Fatalf("a refused move touched the game: %v", err)
	}
}
