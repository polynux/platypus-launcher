package main

import (
	"os"
	"path/filepath"
	"testing"
)

// STEAM-17: gone-file removal must delete directories and surface errors.
// os.Remove on a directory entry silently survives; RemoveAll is required.
func TestRemoveGoneDeletesDirs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "old.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "olddir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "f"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removeGone(dir, []string{"old.txt", "olddir"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "old.txt")); !os.IsNotExist(err) {
		t.Fatal("old.txt survives")
	}
	if _, err := os.Stat(sub); !os.IsNotExist(err) {
		t.Fatalf("gone dir survives: %v", sub)
	}
}
