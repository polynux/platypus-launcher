package main

// A stale item folder is replaced, never written through: an uploader's
// symlink must not let a Workshop download touch a file outside the content
// root (B1).

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceWorkshopDirDoesNotFollowASymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(outside, "victim.txt")
	if err := os.WriteFile(victim, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "9200")
	if err := os.Symlink(outside, dir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	partial := dir + ".partial"
	if err := os.MkdirAll(partial, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(partial, "addon.gma"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := replaceWorkshopDir(dir, partial); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(victim); err != nil || string(data) != "keep me" {
		t.Fatalf("the symlink target was touched: %q, %v", data, err)
	}
	if fi, err := os.Lstat(dir); err != nil || !fi.IsDir() {
		t.Fatalf("item folder wasn't replaced with a real directory: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "addon.gma")); err != nil || string(data) != "new" {
		t.Fatalf("new content = %q, %v", data, err)
	}
	if _, err := os.Lstat(partial); !os.IsNotExist(err) {
		t.Fatalf("staging folder stayed: %v", err)
	}
}

func TestWorkshopItemDirsRemovesPlantedSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"9200", "9200.partial"} {
		if err := os.Symlink(outside, filepath.Join(root, name)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	dir, partial, err := workshopItemDirs(root, "9200")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dir, partial} {
		if fi, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("%s is still a symlink: %v %v", path, fi, err)
		}
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("the symlink target was removed: %v", err)
	}
}
