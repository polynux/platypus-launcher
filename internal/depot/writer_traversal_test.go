package depot

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWriterSymlinkEscape proves a manifest symlink + file-through-symlink
// cannot write outside the install dir (STEAM-3). Files must land before
// symlinks, and file writes through symlink components must be refused.
func TestWriterSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "pwned")
	m := &Manifest{Files: []File{
		{Name: "link", Flags: FlagSymlink, LinkTarget: outside},
		{Name: "link/pwned", Size: 3, Chunks: nil},
	}}
	// Give the traversal file a real chunk so both writer paths hit it.
	key := testKey()
	mm, enc := buildFixtureDepot(t, key, 1, 1)
	_ = enc
	c := mm.Files[0].Chunks[0]
	plain := []byte("abc")
	// Override with a tiny known chunk.
	m.Files[1].Chunks = []Chunk{{ID: c.ID, Checksum: Adler0(plain), Offset: 0, UncompressedLength: uint32(len(plain))}}
	data := map[string][]byte{string(c.ID): plain}
	if err := WriteChunks(m, dir, data); err == nil {
		t.Fatal("WriteChunks should refuse file through symlink")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("outside file created: %v", target)
	}
	// Streaming writer must refuse too.
	dir2 := t.TempDir()
	w, err := Preallocate(m, dir2)
	if err == nil {
		if err := w.Write(string(c.ID), plain); err == nil {
			_ = w.Close()
			t.Fatal("InstallWriter should refuse file through symlink")
		}
		w.Close()
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("outside file created via stream: %v", target)
	}
}
