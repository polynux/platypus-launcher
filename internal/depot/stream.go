// Streaming install writer: preallocated files + WriteAt per landed chunk.
// Bounds memory to in-flight chunks (conns x chunk) instead of the whole
// game. Files are written before symlinks so a symlink entry can never
// redirect a file write outside the install dir (see STEAM-3).
package depot

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type chunkLoc struct {
	fileIdx int
	offset  int64
}

type openFile struct {
	path   string
	fh     *os.File
	isExec bool
}

// InstallWriter materialises one depot manifest into dir as chunks land.
// Call Preallocate once, then Write for each fetched chunk, then Close to
// apply exec bits and symlinks (symlinks last, files first).
type InstallWriter struct {
	dir   string
	man   *Manifest
	files []openFile
	index map[string][]chunkLoc
	syms  []File
}

// Preallocate creates dirs + empty files truncated to size and builds the
// chunk -> file-offset index. It opens one handle per regular file.
func Preallocate(m *Manifest, dir string) (*InstallWriter, error) {
	files := append([]File{}, m.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	// Refuse files under a symlink entry upfront: symlinks land last, so
	// link/pwned would otherwise be written to a real dir that the later
	// symlink cannot replace.
	syms := map[string]bool{}
	for _, f := range files {
		if f.IsSymlink() && f.LinkTarget != "" {
			syms[filepath.FromSlash(f.Name)] = true
		}
	}
	for _, f := range files {
		if f.IsDir() || (f.IsSymlink() && f.LinkTarget != "") {
			continue
		}
		rel := filepath.FromSlash(f.Name)
		for s := range syms {
			if rel == s || len(rel) > len(s) && rel[len(s)] == filepath.Separator && rel[:len(s)] == s {
				return nil, fmt.Errorf("refusing write through symlink %q: %q", s, f.Name)
			}
		}
	}
	w := &InstallWriter{dir: dir, man: m, index: map[string][]chunkLoc{}}
	for _, f := range files {
		rel := filepath.FromSlash(f.Name)
		if rel == "" || rel == "." || strings.Contains(rel, "..") {
			return nil, fmt.Errorf("unsafe path %q", f.Name)
		}
		// Reject absolute paths and clean-escape (defence in depth;
		// symlink-component check happens in Write/Close).
		if filepath.IsAbs(rel) {
			return nil, fmt.Errorf("unsafe path %q", f.Name)
		}
		if filepath.Clean(rel) != rel {
			return nil, fmt.Errorf("unsafe path %q", f.Name)
		}
		path := filepath.Join(dir, rel)
		if f.IsDir() {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return nil, err
			}
			continue
		}
		if f.IsSymlink() && f.LinkTarget != "" {
			// Deferred to Close, after all file writes.
			w.syms = append(w.syms, f)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		fh, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
		if err != nil {
			return nil, err
		}
		if err := fh.Truncate(int64(f.Size)); err != nil {
			fh.Close()
			return nil, err
		}
		idx := len(w.files)
		w.files = append(w.files, openFile{path: path, fh: fh, isExec: f.IsExec()})
		ch := append([]Chunk{}, f.Chunks...)
		sort.Slice(ch, func(i, j int) bool { return ch[i].Offset < ch[j].Offset })
		for _, c := range ch {
			w.index[string(c.ID)] = append(w.index[string(c.ID)], chunkLoc{fileIdx: idx, offset: int64(c.Offset)})
		}
	}
	return w, nil
}

// Write puts one chunk's bytes into every file offset that needs it.
func (w *InstallWriter) Write(id string, data []byte) error {
	locs, ok := w.index[id]
	if !ok {
		return fmt.Errorf("chunk %s not in manifest", shortID(id))
	}
	for _, l := range locs {
		// Guard against a symlink component swapped in mid-install:
		// resolve the parent dir and refuse escapes.
		if err := checkNoSymlinkEscape(w.dir, w.files[l.fileIdx].path); err != nil {
			return err
		}
		if _, err := w.files[l.fileIdx].fh.WriteAt(data, l.offset); err != nil {
			return err
		}
	}
	return nil
}

// Close flushes file data, applies exec bits, then creates symlinks last.
// Symlinks are validated to stay inside the install dir.
func (w *InstallWriter) Close() error {
	var first error
	for _, f := range w.files {
		if err := f.fh.Sync(); err != nil && first == nil {
			first = err
		}
		f.fh.Close()
		if f.isExec {
			if fi, err := os.Stat(f.path); err == nil {
				os.Chmod(f.path, fi.Mode()|0o100)
			}
		}
	}
	// Symlinks last so file writes can never traverse them.
	for _, s := range w.syms {
		rel := filepath.FromSlash(s.Name)
		path := filepath.Join(w.dir, rel)
		if err := checkNoSymlinkEscape(w.dir, path); err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		os.Remove(path)
		if err := os.Symlink(s.LinkTarget, path); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// checkNoSymlinkEscape resolves existing parent components and refuses any
// file whose directory escapes dir via symlinks.
func checkNoSymlinkEscape(dir, path string) error {
	parent := filepath.Dir(path)
	// Resolve as much as exists; missing tail is fine.
	resolved := parent
	if fi, err := os.Lstat(parent); err == nil {
		_ = fi
		if ev, err := filepath.EvalSymlinks(parent); err == nil {
			resolved = ev
		} else {
			// Walk up to the nearest existing ancestor.
			cur := parent
			for {
				cur = filepath.Dir(cur)
				if cur == "." || cur == "/" || cur == dir {
					break
				}
				if _, err := os.Lstat(cur); err == nil {
					if ev, err := filepath.EvalSymlinks(cur); err == nil {
						rel, err := filepath.Rel(cur, parent)
						if err == nil {
							resolved = filepath.Join(ev, rel)
						}
					}
					break
				}
				if len(cur) >= len(parent) {
					break
				}
			}
		}
	} else {
		// Parent doesn't exist yet: resolve nearest existing ancestor.
		cur := parent
		for {
			next := filepath.Dir(cur)
			if next == cur {
				break
			}
			cur = next
			if _, err := os.Lstat(cur); err == nil {
				if ev, err := filepath.EvalSymlinks(cur); err == nil {
					rel, err := filepath.Rel(cur, parent)
					if err == nil {
						resolved = filepath.Join(ev, rel)
					}
				}
				break
			}
		}
	}
	cleanDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		cleanDir = dir
	}
	rel, err := filepath.Rel(cleanDir, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refusing write outside install dir: %q", path)
	}
	return nil
}

func shortID(id string) string {
	if len(id) > 8 {
		return fmt.Sprintf("%x...", []byte(id)[:4])
	}
	return fmt.Sprintf("%x", []byte(id))
}
