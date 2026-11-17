//go:build !windows

package platform

import (
	"path/filepath"
	"testing"
)

// Issue #83: the Unix roots and protected folders are what they always were.
func TestIsRootUnix(t *testing.T) {
	t.Parallel()
	if !IsRoot("/") {
		t.Fatal("the filesystem root is not a root")
	}
	for _, path := range []string{"/mnt", "/usr", "", "relative"} {
		if IsRoot(path) {
			t.Fatalf("%q is not a filesystem root", path)
		}
	}
}

func TestIsProtectedUnix(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]bool{
		"/usr":             true,
		"/usr/share/games": true,
		"/etc":             true,
		"/usrx":            false,
		"/usrlocal/games":  false,
		"/home/me":         false,
		"/mnt/games":       false,
		"/":                false, // IsRoot's job, refused with its own message
	} {
		if got := IsProtected(path); got != want {
			t.Errorf("IsProtected(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSpaceStatsTempDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	total, free, err := SpaceStats(dir)
	if err != nil {
		t.Fatal(err)
	}
	if total <= 0 || free <= 0 || free > total {
		t.Fatalf("total = %d, free = %d", total, free)
	}
	// A folder that doesn't exist yet resolves through its parent.
	if _, _, err := SpaceStats(filepath.Join(dir, "missing", "child")); err != nil {
		t.Fatal(err)
	}
	if n, err := FreeSpace(dir); err != nil || n <= 0 {
		t.Fatalf("FreeSpace = %d, %v; want the room there", n, err)
	}
}
