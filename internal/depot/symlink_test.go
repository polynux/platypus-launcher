package depot

// A stale or hostile folder must not redirect a manifest file's open (and
// its truncation) outside the install folder through a symlink.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreallocateRefusesASymlinkedManifestFile(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "game.sh")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	man := &Manifest{Files: []File{{Name: "game.sh", Size: 4}}}
	w, err := Preallocate(man, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Write("chunk", []byte("data")); err != nil {
		// The chunk isn't in the manifest; the writer refuses it, which is
		// fine — what matters is that the open didn't follow the symlink.
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "keep me" {
		t.Fatalf("the symlink target was written: %q, %v", data, err)
	}
	if fi, err := os.Lstat(filepath.Join(dir, "game.sh")); err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("game.sh wasn't replaced with a regular file: %v %v", fi, err)
	}
}

func TestPreallocateRefusesASymlinkedParent(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "sub")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	man := &Manifest{Files: []File{{Name: "sub/game.bin", Size: 4}}}
	if _, err := Preallocate(man, dir); err == nil {
		t.Fatal("a manifest file under a symlinked directory was accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "game.bin")); !os.IsNotExist(err) {
		t.Fatal("the write landed outside the install folder")
	}
}
