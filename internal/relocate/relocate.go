// Package relocate moves a game's install folder to another place when the
// game's store plugin has no game.move (docs/plugins/protocol.md, "Moving a
// game"). On one filesystem the folder is renamed. Across filesystems it is
// copied into a staging folder next to the destination, checked file by file
// against the source, renamed into place, and only then is the source removed.
// Until that last step the source is untouched: a failure or a cancel removes
// the staging folder and leaves the game where it was.
package relocate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// StageSuffix names the temporary copy: "<destination>.hydra-move". A folder
// with that name left by an interrupted move is removed before a new one starts.
const StageSuffix = ".hydra-move"

// ErrExists means the destination is already taken. A move never merges into
// or overwrites an existing folder.
var ErrExists = errors.New("the destination already exists")

// Options tune a move. The zero value is right for real use.
type Options struct {
	// SameFS reports whether two existing paths are on one filesystem. Nil
	// compares their device numbers.
	SameFS func(a, b string) (bool, error)
	// Progress receives the bytes copied and the total, across filesystems
	// only. It may be nil.
	Progress func(done, total int64)
	// Placed runs once the game is complete at to and the source is still
	// there, before the source is removed. The caller records the move here:
	// an error undoes the move and leaves the game at from.
	Placed func(from, to string) error
}

// Result says what a successful move left behind.
type Result struct {
	// Leftover is the source folder when it could not be removed after the
	// copy was complete. The game is at the destination either way.
	Leftover string
}

// Move relocates the folder from to the path to. to's parent folder must
// exist and to itself must not. A from folder that contains to, or is
// contained in it, is refused.
func Move(ctx context.Context, from, to string, opts Options) (Result, error) {
	from, to = filepath.Clean(from), filepath.Clean(to)
	if err := checkPaths(from, to); err != nil {
		return Result{}, err
	}
	if info, err := os.Stat(from); err != nil || !info.IsDir() {
		return Result{}, fmt.Errorf("%s is not a folder to move", from)
	}
	if _, err := os.Lstat(to); err == nil {
		return Result{}, fmt.Errorf("%s: %w", to, ErrExists)
	}
	parent := filepath.Dir(to)
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return Result{}, fmt.Errorf("the folder %s does not exist", parent)
	}
	sameFS := opts.SameFS
	if sameFS == nil {
		sameFS = sameDevice
	}
	same, err := sameFS(from, parent)
	if err != nil {
		return Result{}, err
	}
	if same {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		if err := os.Rename(from, to); err != nil {
			return Result{}, err
		}
		if err := placed(opts, from, to); err != nil {
			return Result{}, errors.Join(err, os.Rename(to, from))
		}
		return Result{}, nil
	}

	stage := to + StageSuffix
	if err := os.RemoveAll(stage); err != nil {
		return Result{}, fmt.Errorf("clearing an earlier copy at %s: %w", stage, err)
	}
	c := &copier{ctx: ctx, progress: opts.Progress}
	if c.total, err = treeSize(from); err != nil {
		return Result{}, err
	}
	if err := c.copyTree(from, stage); err != nil {
		os.RemoveAll(stage)
		return Result{}, err
	}
	if err := sameTree(from, stage); err != nil {
		os.RemoveAll(stage)
		return Result{}, fmt.Errorf("the copy doesn't match the game: %w", err)
	}
	if err := ctx.Err(); err != nil {
		os.RemoveAll(stage)
		return Result{}, err
	}
	if err := os.Rename(stage, to); err != nil {
		os.RemoveAll(stage)
		return Result{}, err
	}
	if err := placed(opts, from, to); err != nil {
		os.RemoveAll(to)
		return Result{}, err
	}
	if err := os.RemoveAll(from); err != nil {
		return Result{Leftover: from}, nil
	}
	return Result{}, nil
}

// placed calls the Placed hook, when there is one.
func placed(opts Options, from, to string) error {
	if opts.Placed == nil {
		return nil
	}
	return opts.Placed(from, to)
}

// TreeSize is the bytes of the regular files under dir: what a move has to
// fit on the destination's filesystem.
func TreeSize(dir string) (int64, error) { return treeSize(dir) }

// checkPaths refuses paths that are not absolute, are the same, or contain
// each other.
func checkPaths(from, to string) error {
	if !filepath.IsAbs(from) || !filepath.IsAbs(to) {
		return errors.New("a move needs absolute paths")
	}
	if from == to {
		return errors.New("the game is already in that folder")
	}
	if strings.HasPrefix(to, from+string(filepath.Separator)) {
		return fmt.Errorf("%s is inside the game's folder", to)
	}
	if strings.HasPrefix(from, to+string(filepath.Separator)) {
		return fmt.Errorf("%s contains the game's folder", to)
	}
	return nil
}

// sameDevice compares the filesystems of two existing paths.
func sameDevice(a, b string) (bool, error) {
	var sa, sb syscall.Stat_t
	if err := syscall.Stat(a, &sa); err != nil {
		return false, err
	}
	if err := syscall.Stat(b, &sb); err != nil {
		return false, err
	}
	return sa.Dev == sb.Dev, nil
}

// copier copies a tree, reporting progress and honoring cancellation between
// reads.
type copier struct {
	ctx      context.Context
	progress func(done, total int64)
	total    int64
	done     int64
}

func (c *copier) copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := c.ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch mode := info.Mode(); {
		case mode.IsDir():
			return os.MkdirAll(target, mode.Perm()|0o700)
		case mode&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case mode.IsRegular():
			return c.copyFile(path, target, mode.Perm())
		default:
			return fmt.Errorf("%s is not a regular file, folder or link", path)
		}
	})
}

func (c *copier) copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, reader{c, in})
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

// reader counts bytes for the progress callback and stops when cancelled.
type reader struct {
	c *copier
	r io.Reader
}

func (r reader) Read(p []byte) (int, error) {
	if err := r.c.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	r.c.done += int64(n)
	if r.c.progress != nil {
		r.c.progress(r.c.done, r.c.total)
	}
	return n, err
}

// treeSize adds up the regular files under dir, in bytes.
func treeSize(dir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}

// sameTree checks that dst holds what src holds: the same names, kinds, link
// targets and file sizes. Contents are not hashed; a copy that was cut short
// or lost files can't pass.
func sameTree(src, dst string) error {
	want, err := listTree(src)
	if err != nil {
		return err
	}
	got, err := listTree(dst)
	if err != nil {
		return err
	}
	if len(want) != len(got) {
		return fmt.Errorf("%d entries copied, %d expected", len(got), len(want))
	}
	for rel, w := range want {
		g, ok := got[rel]
		if !ok || g != w {
			return fmt.Errorf("%s differs after the copy", rel)
		}
	}
	return nil
}

// listTree describes every entry under dir: its kind, and its size or link
// target, keyed by relative path.
func listTree(dir string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch mode := info.Mode(); {
		case mode.IsDir():
			out[rel] = "dir"
		case mode&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			out[rel] = "link:" + link
		default:
			out[rel] = fmt.Sprintf("file:%d", info.Size())
		}
		return nil
	})
	return out, err
}
