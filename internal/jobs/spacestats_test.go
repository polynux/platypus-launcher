package jobs

import (
	"path/filepath"
	"testing"
)

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
}
