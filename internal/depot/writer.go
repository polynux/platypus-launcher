// Install writer: preallocated files, WriteAt chunks, flags and symlinks.
// Files sort by path so a partial install is mostly whole files.
package depot

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WriteChunks materialises data (chunk id -> uncompressed bytes) into dir
// according to m. It preallocates each file, writes chunks with WriteAt
// (no per-file lock; disjoint offsets), and applies flags.
func WriteChunks(m *Manifest, dir string, data map[string][]byte) error {
	files := append([]File{}, m.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	for _, f := range files {
		if err := writeOne(dir, f, data); err != nil {
			return err
		}
	}
	return nil
}

func writeOne(dir string, f File, data map[string][]byte) error {
	rel := filepath.FromSlash(f.Name)
	if rel == "" || strings.Contains(rel, "..") {
		return fmt.Errorf("unsafe path %q", f.Name)
	}
	path := filepath.Join(dir, rel)
	if f.IsDir() {
		return os.MkdirAll(path, 0o755)
	}
	if f.IsSymlink() && f.LinkTarget != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		os.Remove(path)
		return os.Symlink(f.LinkTarget, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	fh, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer fh.Close()
	if err := fh.Truncate(int64(f.Size)); err != nil {
		return err
	}
	ch := append([]Chunk{}, f.Chunks...)
	sort.Slice(ch, func(i, j int) bool { return ch[i].Offset < ch[j].Offset })
	for _, c := range ch {
		plain, ok := data[string(c.ID)]
		if !ok {
			return fmt.Errorf("missing chunk %x for %s", c.ID, f.Name)
		}
		if uint32(len(plain)) != c.UncompressedLength {
			return fmt.Errorf("chunk %x has %d bytes, want %d", c.ID, len(plain), c.UncompressedLength)
		}
		if _, err := fh.WriteAt(plain, int64(c.Offset)); err != nil {
			return err
		}
	}
	if f.IsExec() {
		if fi, err := fh.Stat(); err == nil {
			os.Chmod(path, fi.Mode()|0o100)
		}
	}
	return nil
}

// Verify checks every file's SHA-1 against FileHash and returns the names
// that mismatch (missing files count as mismatched).
func Verify(m *Manifest, dir string) ([]string, error) {
	var bad []string
	for _, f := range m.Files {
		if f.IsDir() || f.IsSymlink() {
			continue
		}
		if len(f.FileHash) == 0 {
			continue
		}
		path := filepath.Join(dir, filepath.FromSlash(f.Name))
		data, err := os.ReadFile(path)
		if err != nil {
			bad = append(bad, f.Name)
			continue
		}
		sum := sha1.Sum(data)
		if string(sum[:]) != string(f.FileHash) {
			bad = append(bad, f.Name)
		}
	}
	return bad, nil
}

// Diff compares old and new manifests for updates: which chunks can be
// reused by copying from the old files, which files are gone, and the new
// manifest itself. Reuse is by chunk id, like DepotDownloader.
func Diff(oldM, newM *Manifest) (reuse map[string][]string, gone []string) {
	oldChunks := map[string][]string{} // chunk id -> file names holding it
	if oldM != nil {
		for _, f := range oldM.Files {
			for _, c := range f.Chunks {
				oldChunks[string(c.ID)] = append(oldChunks[string(c.ID)], f.Name)
			}
		}
	}
	oldFiles := map[string]bool{}
	if oldM != nil {
		for _, f := range oldM.Files {
			oldFiles[f.Name] = true
		}
	}
	reuse = map[string][]string{}
	for _, f := range newM.Files {
		for _, c := range f.Chunks {
			if holders, ok := oldChunks[string(c.ID)]; ok {
				reuse[string(c.ID)] = holders
			}
		}
	}
	newFiles := map[string]bool{}
	for _, f := range newM.Files {
		newFiles[f.Name] = true
	}
	for name := range oldFiles {
		if !newFiles[name] {
			gone = append(gone, name)
		}
	}
	sort.Strings(gone)
	return reuse, gone
}
