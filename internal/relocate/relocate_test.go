package relocate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestPlacedFailureUndoesTheMove(t *testing.T) {
	for _, same := range []bool{true, false} {
		root := t.TempDir()
		from := game(t, root)
		to := filepath.Join(root, "dst", "Game")
		must(t, os.MkdirAll(filepath.Dir(to), 0o755))
		boom := errors.New("can't record the move")
		_, err := Move(context.Background(), from, to, Options{
			SameFS: sameFS(same),
			Placed: func(a, b string) error {
				if a != from || b != to {
					t.Errorf("Placed got %q, %q", a, b)
				}
				return boom
			},
		})
		if !errors.Is(err, boom) {
			t.Fatalf("same=%v: err = %v", same, err)
		}
		if _, err := os.Stat(filepath.Join(from, "game.sh")); err != nil {
			t.Fatalf("same=%v: game not back at its folder: %v", same, err)
		}
		if _, err := os.Stat(to); !os.IsNotExist(err) {
			t.Fatalf("same=%v: destination left: %v", same, err)
		}
	}
}

func TestPlacedSeesTheGameAtTheDestinationBeforeTheSourceGoes(t *testing.T) {
	root := t.TempDir()
	from := game(t, root)
	to := filepath.Join(root, "dst", "Game")
	must(t, os.MkdirAll(filepath.Dir(to), 0o755))
	_, err := Move(context.Background(), from, to, Options{
		SameFS: sameFS(false),
		Placed: func(a, b string) error {
			if _, err := os.Stat(filepath.Join(b, "game.sh")); err != nil {
				return err
			}
			if _, err := os.Stat(a); err != nil {
				return err
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(from); !os.IsNotExist(err) {
		t.Fatal("source kept after a successful move")
	}
}

func TestAnInterruptedStagingFolderIsReplaced(t *testing.T) {
	root := t.TempDir()
	from := game(t, root)
	to := filepath.Join(root, "dst", "Game")
	stage := to + StageSuffix
	must(t, os.MkdirAll(stage, 0o755))
	must(t, writeMarker(stage, marker{From: from, To: to}))
	must(t, os.WriteFile(filepath.Join(stage, "partial"), []byte("x"), 0o644))
	if _, err := Move(context.Background(), from, to, Options{SameFS: sameFS(false)}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(to, "partial")); !os.IsNotExist(err) {
		t.Fatal("an earlier partial copy ended up in the game")
	}
}

// A staging name that a move didn't make is someone's folder: it is refused,
// not deleted.
func TestAnUnmarkedStagingFolderIsNotDeleted(t *testing.T) {
	root := t.TempDir()
	from := game(t, root)
	to := filepath.Join(root, "dst", "Game")
	stage := to + StageSuffix
	must(t, os.MkdirAll(stage, 0o755))
	keep := filepath.Join(stage, "keep.txt")
	must(t, os.WriteFile(keep, []byte("precious"), 0o644))
	_, err := Move(context.Background(), from, to, Options{SameFS: sameFS(false)})
	if err == nil {
		t.Fatal("a move deleted a folder it did not make")
	}
	if b, err := os.ReadFile(keep); err != nil || string(b) != "precious" {
		t.Fatalf("the folder was touched: %q %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(from, "game.sh")); err != nil {
		t.Fatalf("the game was touched: %v", err)
	}
}

// Every copied file is flushed, and so is every folder that holds one, before
// the move is placed and the source may go.
func TestCopiesAreFlushedBeforeThePlacedHook(t *testing.T) {
	var mu sync.Mutex
	var files, dirs []string
	oldFile, oldDir := syncFile, syncDir
	t.Cleanup(func() { syncFile, syncDir = oldFile, oldDir })
	syncFile = func(f *os.File) error {
		mu.Lock()
		defer mu.Unlock()
		files = append(files, f.Name())
		return f.Sync()
	}
	syncDir = func(dir string) error {
		mu.Lock()
		defer mu.Unlock()
		dirs = append(dirs, dir)
		return oldDir(dir)
	}
	root := t.TempDir()
	from := game(t, root)
	to := filepath.Join(root, "dst", "Game")
	must(t, os.MkdirAll(filepath.Dir(to), 0o755))
	_, err := Move(context.Background(), from, to, Options{
		SameFS: sameFS(false),
		Placed: func(a, b string) error {
			mu.Lock()
			defer mu.Unlock()
			// game.sh, a.pak, b.bin and the marker, each flushed in the copy.
			if len(files) < 4 {
				t.Errorf("%d files flushed before Placed, want at least 4", len(files))
			}
			if !slicesContainsPrefix(dirs, to+StageSuffix) {
				t.Errorf("the staging folder was not flushed before Placed: %v", dirs)
			}
			if _, err := os.Stat(a); err != nil {
				t.Errorf("the source was gone at Placed: %v", err)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !slicesContainsPrefix(dirs, filepath.Dir(to)) {
		t.Errorf("the destination's parent was not flushed after the rename: %v", dirs)
	}
}

func slicesContainsPrefix(paths []string, prefix string) bool {
	for _, p := range paths {
		if p == prefix || strings.HasPrefix(p, prefix+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// A crash after the game was placed at the destination: the next move finishes
// it (records, removes the source) instead of failing on the folder it made.
func TestAnInterruptedMoveIsFinishedNotCopiedAgain(t *testing.T) {
	for _, same := range []bool{true, false} {
		root := t.TempDir()
		from := game(t, root)
		to := filepath.Join(root, "dst", "Game")
		must(t, os.MkdirAll(filepath.Dir(to), 0o755))
		func() {
			defer func() { recover() }() // the crash
			Move(context.Background(), from, to, Options{
				SameFS: sameFS(same),
				Placed: func(a, b string) error { panic("crash") },
			})
		}()
		recorded := 0
		res, err := Move(context.Background(), from, to, Options{
			SameFS: sameFS(same),
			Placed: func(a, b string) error {
				recorded++
				return nil
			},
		})
		if err != nil || res.Leftover != "" {
			t.Fatalf("same=%v: resume: %v (leftover %q)", same, err, res.Leftover)
		}
		if recorded != 1 {
			t.Fatalf("same=%v: recorded %d times, want 1", same, recorded)
		}
		if _, err := os.Stat(from); !os.IsNotExist(err) {
			t.Fatalf("same=%v: source kept after resume: %v", same, err)
		}
		if b, err := os.ReadFile(filepath.Join(to, "data", "deep", "b.bin")); err != nil || string(b) != "bee" {
			t.Fatalf("same=%v: game damaged by resume: %q %v", same, b, err)
		}
		if _, err := os.Stat(filepath.Join(to, markerName)); !os.IsNotExist(err) {
			t.Fatalf("same=%v: marker left in the game: %v", same, err)
		}
	}
}

// A symlinked source is refused: the move would act on what the link points at.
func TestALinkedSourceIsRefused(t *testing.T) {
	root := t.TempDir()
	from := game(t, root)
	alias := filepath.Join(root, "alias")
	must(t, os.Symlink(from, alias))
	to := filepath.Join(root, "dst", "Game")
	must(t, os.MkdirAll(filepath.Dir(to), 0o755))
	if _, err := Move(context.Background(), alias, to, Options{SameFS: sameFS(true)}); err == nil {
		t.Fatal("a linked source was moved")
	}
	if _, err := os.Stat(filepath.Join(from, "game.sh")); err != nil {
		t.Fatalf("the game was touched: %v", err)
	}
}

// A destination whose parent is a link into the game is inside the game, even
// though its own path doesn't say so.
func TestADestinationThroughALinkIntoTheGameIsRefused(t *testing.T) {
	root := t.TempDir()
	from := game(t, root)
	must(t, os.MkdirAll(filepath.Join(from, "data"), 0o755))
	alias := filepath.Join(root, "alias")
	must(t, os.Symlink(filepath.Join(from, "data"), alias))
	to := filepath.Join(alias, "Copy")
	if _, err := Move(context.Background(), from, to, Options{SameFS: sameFS(true)}); err == nil {
		t.Fatal("a move into the game through a link was accepted")
	}
	if _, err := os.Stat(filepath.Join(from, "game.sh")); err != nil {
		t.Fatalf("the game was touched: %v", err)
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
