package migrate

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// hashLimit is the size up to which verification compares file contents; a
// bigger file (a Wine prefix's disk image) is compared by size, mode and mtime.
const hashLimit = 32 << 20

// copyVerified copies the folder src to dst without touching src: into
// dst+".migrating", checked against the original, then renamed to dst. On any
// failure the partial copy is removed and dst does not exist.
func copyVerified(src, dst string) (err error) {
	tmp := dst + ".migrating"
	// A leftover is a copy of ours from an interrupted run (it was never
	// renamed to dst, so nothing uses it).
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(tmp)
		}
	}()
	real, err := filepath.EvalSymlinks(src) // the folder itself may be a symlink
	if err != nil {
		return err
	}
	if err := copyTree(real, tmp); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	if err := verifyTree(real, tmp); err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	return os.Rename(tmp, dst)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
			// Directory permissions are set once everything is verified.
			return nil
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case info.Mode().IsRegular():
			return copyFile(path, target, info)
		default:
			// Sockets, pipes and devices have no content to carry over.
			return nil
		}
	})
}

func copyFile(src, dst string, info fs.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Chmod(dst, info.Mode().Perm()); err != nil { // umask
		return err
	}
	return os.Chtimes(dst, info.ModTime(), info.ModTime())
}

// verifyTree checks that dst holds exactly what src holds: the same paths,
// kinds, sizes, permissions, symlink targets and (up to hashLimit) contents.
// It also gives the copied directories their permissions.
func verifyTree(src, dst string) error {
	seen := map[string]bool{}
	var dirs []string
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		seen[rel] = true
		si, err := d.Info()
		if err != nil {
			return err
		}
		if !si.IsDir() && !si.Mode().IsRegular() && si.Mode()&fs.ModeSymlink == 0 {
			delete(seen, rel) // skipped on copy
			return nil
		}
		target := filepath.Join(dst, rel)
		di, err := os.Lstat(target)
		if err != nil {
			return fmt.Errorf("%s is missing from the copy", rel)
		}
		if si.Mode().Type() != di.Mode().Type() {
			return fmt.Errorf("%s: kind differs", rel)
		}
		switch {
		case si.IsDir():
			dirs = append(dirs, rel)
		case si.Mode()&fs.ModeSymlink != 0:
			a, _ := os.Readlink(path)
			b, _ := os.Readlink(target)
			if a != b {
				return fmt.Errorf("%s: link target differs", rel)
			}
		default:
			if si.Size() != di.Size() || si.Mode().Perm() != di.Mode().Perm() {
				return fmt.Errorf("%s: size or permissions differ", rel)
			}
			if si.Size() <= hashLimit {
				ok, err := sameContent(path, target)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("%s: content differs", rel)
				}
			} else if !si.ModTime().Equal(di.ModTime()) {
				return fmt.Errorf("%s: modification time differs", rel)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Nothing extra in the copy.
	err = filepath.WalkDir(dst, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dst, path)
		if !seen[rel] {
			return fmt.Errorf("%s is in the copy but not in the original", rel)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Deepest first, so a read-only parent doesn't block its children.
	for i := len(dirs) - 1; i >= 0; i-- {
		si, err := os.Stat(filepath.Join(src, dirs[i]))
		if err != nil {
			return err
		}
		if err := os.Chmod(filepath.Join(dst, dirs[i]), si.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func sameContent(a, b string) (bool, error) {
	ha, err := fileHash(a)
	if err != nil {
		return false, err
	}
	hb, err := fileHash(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(ha, hb), nil
}

func fileHash(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}
