// Package relocate moves a game's install folder to another place when the
// game's store plugin has no game.move (docs/plugins/protocol.md, "Moving a
// game"). On one filesystem the folder is renamed. Across filesystems it is
// copied into a staging folder next to the destination, checked file by file
// against the source, flushed to disk, renamed into place, and only then is the
// source removed. Until that last step the source is untouched: a failure or a
// cancel removes the staging folder and leaves the game where it was.
//
// Every folder a move writes carries a marker file (markerName) naming the
// move. An interrupted move is resumed from the marker: a staging folder
// without one is never deleted, and a destination with one that was already
// placed is finished rather than copied again.
package relocate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// StageSuffix names the temporary copy: "<destination>.platypus-move".
const StageSuffix = ".platypus-move"

// markerName is the file a move writes into its staging folder before it
// copies anything, and into the destination once the copy is renamed there. It
// is removed when the move completes and is never counted as part of the game.
const markerName = ".platypus-move-marker.json"

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

// marker names the move a folder belongs to and how many bytes of game it
// holds (the size a finished move must still have).
type marker struct {
	From string `json:"from"`
	To   string `json:"to"`
	Size int64  `json:"size"`
}

// Flushes, replaced by tests to check that nothing is removed before the
// copy is on disk.
var (
	syncFile = func(f *os.File) error { return f.Sync() }
	syncDir  = func(dir string) error {
		d, err := os.Open(dir)
		if err != nil {
			return err
		}
		return errors.Join(d.Sync(), d.Close())
	}
)

// Move relocates the folder from to the path to. to's parent folder must
// exist and to itself must not. A from folder that contains to, or is
// contained in it, is refused, and so is a from that is a link. An interrupted
// move to the same destination is finished or cleaned up first.
func Move(ctx context.Context, from, to string, opts Options) (Result, error) {
	from, to = filepath.Clean(from), filepath.Clean(to)
	if !filepath.IsAbs(from) || !filepath.IsAbs(to) {
		return Result{}, errors.New("a move needs absolute paths")
	}
	if m, ok := readMarker(to); ok && m.From == from && m.To == to {
		return finish(from, to, m, opts)
	}
	realFrom, realTo, err := Check(from, to)
	if err != nil {
		return Result{}, err
	}
	if err := clearStage(realTo + StageSuffix); err != nil {
		return Result{}, err
	}
	size, err := treeSize(realFrom)
	if err != nil {
		return Result{}, err
	}
	m := marker{From: from, To: to, Size: size}
	sameFS := opts.SameFS
	if sameFS == nil {
		sameFS = sameDevice
	}
	same, err := sameFS(realFrom, filepath.Dir(realTo))
	if err != nil {
		return Result{}, err
	}
	if same {
		return renameMove(ctx, from, to, realFrom, realTo, m, opts)
	}
	return copyMove(ctx, from, to, realFrom, realTo, m, opts)
}

// Check says whether a game in from may move to to. It returns both paths with
// their symlinks resolved: from must be a real folder, not a link; to must be
// free, with an existing parent; and neither may contain the other.
func Check(from, to string) (string, string, error) {
	from, to = filepath.Clean(from), filepath.Clean(to)
	if err := checkPaths(from, to); err != nil {
		return "", "", err
	}
	if info, err := os.Lstat(from); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return "", "", fmt.Errorf("%s is a link, not the game's folder", from)
	}
	if info, err := os.Stat(from); err != nil || !info.IsDir() {
		return "", "", fmt.Errorf("%s is not a folder to move", from)
	}
	if _, err := os.Lstat(to); err == nil {
		return "", "", fmt.Errorf("%s: %w", to, ErrExists)
	}
	parent := filepath.Dir(to)
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return "", "", fmt.Errorf("the folder %s does not exist", parent)
	}
	realFrom, err := realPath(from)
	if err != nil {
		return "", "", err
	}
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", "", err
	}
	realTo := filepath.Join(realParent, filepath.Base(to))
	if err := checkPaths(realFrom, realTo); err != nil {
		return "", "", err
	}
	return realFrom, realTo, nil
}

// realPath resolves the links in the parent of p. p itself may be missing; its
// own name is kept as is.
func realPath(p string) (string, error) {
	parent, err := filepath.EvalSymlinks(filepath.Dir(p))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(p)), nil
}

// renameMove moves the folder by renaming it, on one filesystem.
func renameMove(ctx context.Context, from, to, realFrom, realTo string, m marker, opts Options) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := writeMarker(realFrom, m); err != nil {
		return Result{}, err
	}
	if err := os.Rename(realFrom, realTo); err != nil {
		return Result{}, errors.Join(err, removeMarker(realFrom))
	}
	back := func(err error) (Result, error) {
		return Result{}, errors.Join(err, os.Rename(realTo, realFrom), removeMarker(realFrom))
	}
	if err := syncDir(filepath.Dir(realFrom)); err != nil {
		return back(err)
	}
	if err := syncDir(filepath.Dir(realTo)); err != nil {
		return back(err)
	}
	if err := placed(opts, from, to); err != nil {
		return back(err)
	}
	// The move is complete: a failure to drop the marker only leaves a file
	// behind.
	_ = removeMarker(realTo)
	return Result{}, nil
}

// copyMove copies the folder into a staging folder next to the destination,
// verifies it, flushes it and renames it into place. The source goes last.
func copyMove(ctx context.Context, from, to, realFrom, realTo string, m marker, opts Options) (Result, error) {
	stage := realTo + StageSuffix
	if err := os.Mkdir(stage, 0o755); err != nil {
		return Result{}, err
	}
	fail := func(err error) (Result, error) {
		return Result{}, errors.Join(err, os.RemoveAll(stage))
	}
	if err := writeMarker(stage, m); err != nil {
		return fail(err)
	}
	c := &copier{ctx: ctx, progress: opts.Progress, total: m.Size}
	if err := c.copyTree(realFrom, stage); err != nil {
		return fail(err)
	}
	if err := sameTree(realFrom, stage); err != nil {
		return fail(fmt.Errorf("the copy doesn't match the game: %w", err))
	}
	if err := syncTree(stage); err != nil {
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if err := os.Rename(stage, realTo); err != nil {
		return fail(err)
	}
	if err := syncDir(filepath.Dir(realTo)); err != nil {
		return Result{}, errors.Join(err, os.RemoveAll(realTo))
	}
	if err := placed(opts, from, to); err != nil {
		return Result{}, errors.Join(err, os.RemoveAll(realTo))
	}
	_ = removeMarker(realTo)
	// From here on the move is complete at to; the source is only cleaned up.
	if err := os.RemoveAll(realFrom); err != nil {
		return Result{Leftover: from}, nil
	}
	return Result{}, nil
}

// finish completes a move that was placed at to before Platypus stopped: the
// destination must still hold the size the move copied, then the caller
// records the move, the marker goes and the source is removed.
func finish(from, to string, m marker, opts Options) (Result, error) {
	size, err := treeSize(to)
	if err != nil {
		return Result{}, err
	}
	if size != m.Size {
		return Result{}, fmt.Errorf("%s doesn't match the move that was interrupted; both folders are left as they are", to)
	}
	if err := placed(opts, from, to); err != nil {
		return Result{}, err
	}
	_ = removeMarker(to)
	if err := os.RemoveAll(from); err != nil {
		return Result{Leftover: from}, nil
	}
	return Result{}, nil
}

// clearStage removes a staging folder an earlier move left. Only a folder
// with the marker is Platypus's to delete; anything else is refused.
func clearStage(stage string) error {
	if _, err := os.Lstat(stage); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if _, ok := readMarker(stage); !ok {
		return fmt.Errorf("%s is in the way and was not made by a move; remove it and try again", stage)
	}
	if err := os.RemoveAll(stage); err != nil {
		return fmt.Errorf("clearing an earlier copy at %s: %w", stage, err)
	}
	return nil
}

// placed calls the Placed hook, when there is one.
func placed(opts Options, from, to string) error {
	if opts.Placed == nil {
		return nil
	}
	return opts.Placed(from, to)
}

// writeMarker writes the marker into dir and flushes it.
func writeMarker(dir string, m marker) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, markerName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = syncFile(f)
	}
	return errors.Join(err, f.Close())
}

// readMarker returns the marker in dir, if there is a valid one.
func readMarker(dir string) (marker, bool) {
	data, err := os.ReadFile(filepath.Join(dir, markerName))
	if err != nil {
		return marker{}, false
	}
	var m marker
	if err := json.Unmarshal(data, &m); err != nil {
		return marker{}, false
	}
	return m, true
}

// removeMarker deletes the marker from dir. A missing one is not an error.
func removeMarker(dir string) error {
	err := os.Remove(filepath.Join(dir, markerName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
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

// copyTree copies src into dst, which exists. The marker of a move is not
// part of the game and is left out.
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
		if rel == markerName {
			return nil
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

// copyFile copies one file and flushes it to disk before it returns.
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
	if err == nil {
		err = syncFile(out)
	}
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

// syncTree flushes every folder under dir, so the names of the copied files
// are on disk as well as their contents.
func syncTree(dir string) error {
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return syncDir(path)
		}
		return nil
	})
}

// treeSize adds up the regular files under dir, in bytes. The marker is not
// counted.
func treeSize(dir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() && path != filepath.Join(dir, markerName) {
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
// target, keyed by relative path. The marker is left out.
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
		if rel == markerName {
			return nil
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
