package main

// A Workshop item's update deletes the files its new manifest dropped; a
// stale file left beside the new content would keep loading in the game.

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/depot"
)

func TestPruneWorkshopDirKeepsTheNewManifest(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"keep.txt", "dropped.txt", "sub/keep.bin", "sub/gone.bin", "old/gone.bin"} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	man := &depot.Manifest{Files: []depot.File{{Name: "keep.txt"}, {Name: "sub/keep.bin"}}}
	if err := pruneWorkshopDir(dir, man); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"keep.txt", "sub/keep.bin"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			t.Errorf("%s was pruned but the manifest still lists it: %v", name, err)
		}
	}
	for _, name := range []string{"dropped.txt", "sub/gone.bin"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Errorf("%s stayed on disk", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "old")); !os.IsNotExist(err) {
		t.Errorf("the directory a dropped file was alone in stayed")
	}
	if fi, err := os.Stat(filepath.Join(dir, "sub")); err != nil || !fi.IsDir() {
		t.Errorf("a directory the manifest still uses was pruned: %v", err)
	}
}
