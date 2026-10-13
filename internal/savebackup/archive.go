package savebackup

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"platypus/internal/types"
)

// backupFileName is <utc>-<trigger>-<shortid>.zip.
func backupFileName(trigger string, now time.Time) string {
	stamp := now.UTC().Format("20060102T150405")
	return fmt.Sprintf("%s-%s-%s.zip", stamp, trigger, shortID())
}

// gameBackupDir is <backupsRoot>/<gameID>.
func gameBackupDir(backupsRoot, gameID string) string {
	return filepath.Join(backupsRoot, gameID)
}

// Enumerate expands resolved patterns into files without following symlinks:
// symlinked files are counted as skipped, and paths escaping every context
// root are skipped and counted. It returns the files, skipped and unresolved
// counts, and the total bytes.
func Enumerate(patterns []resolvedPattern, ctx Context) (files []Enumerated, skipped, unresolved, count int, bytes int64) {
	seen := map[string]bool{}
	for _, rp := range patterns {
		matches, err := expandGlob(rp.Pattern)
		if err != nil || len(matches) == 0 {
			// A glob matching nothing is not an error: the game simply has
			// no saves there. A literal missing path is the same.
			continue
		}
		for _, m := range matches {
			if err := checkInside(m, ctx); err != nil {
				skipped++
				continue
			}
			st, err := os.Lstat(m)
			if err != nil {
				continue
			}
			if st.Mode()&fs.ModeSymlink != 0 {
				skipped++ // never follow symlinks
				continue
			}
			if !st.Mode().IsRegular() {
				if st.IsDir() {
					// A bare user path may name a folder: walk it.
					n, s, b := walkDirInto(m, rp, ctx, seen, &files)
					_ = n
					skipped += s
					bytes += b
					continue
				}
				skipped++
				continue
			}
			key := rp.Root + "\x00" + relTo(m, ctx, rp)
			if seen[key] {
				continue
			}
			seen[key] = true
			files = append(files, Enumerated{Abs: m, Root: rp.Root, Rel: relTo(m, ctx, rp), Size: st.Size(), Mtime: st.ModTime().Unix()})
			bytes += st.Size()
		}
	}
	return files, skipped, unresolved, len(files), bytes
}

// Enumerated is one file to store, with its root label and relative path.
type Enumerated struct {
	Abs   string
	Root  string
	Rel   string
	Size  int64
	Mtime int64
}

func walkDirInto(dir string, rp resolvedPattern, ctx Context, seen map[string]bool, files *[]Enumerated) (n, skipped int, bytes int64) {
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			skipped++
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			skipped++
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if err := checkInside(p, ctx); err != nil {
			skipped++
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			skipped++
			return nil
		}
		rel := relTo(p, ctx, rp)
		key := rp.Root + "\x00" + rel
		if seen[key] {
			return nil
		}
		seen[key] = true
		*files = append(*files, Enumerated{Abs: p, Root: rp.Root, Rel: rel, Size: info.Size(), Mtime: info.ModTime().Unix()})
		bytes += info.Size()
		n++
		return nil
	})
	return n, skipped, bytes
}

func relTo(abs string, ctx Context, rp resolvedPattern) string {
	if rp.Root != "" {
		if root, ok := ctx.Roots[rp.Root]; ok && root != "" {
			if rel, err := filepath.Rel(root, abs); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return rel
			}
		}
	}
	// Fall back to the longest root, then the base name. Labels sort so
	// ties (home vs install, both the same folder) always pick one root:
	// dedupe compares root+path across runs.
	best, bestLabel := "", ""
	for _, label := range sortedKeys(ctx.Roots) {
		root := ctx.Roots[label]
		if root != "" && (abs == root || strings.HasPrefix(abs, root+string(filepath.Separator))) && len(root) > len(best) {
			best, bestLabel = root, label
		}
	}
	_ = bestLabel
	if best != "" {
		if rel, err := filepath.Rel(best, abs); err == nil {
			return rel
		}
	}
	return filepath.Base(abs)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// checkInside refuses paths escaping every context root: after expansion the
// path must resolve inside a context root (prefix, install dir, home, XDG
// dirs, Steam root). Symlinks are resolved with EvalSymlinks for the
// containment check only; the backup itself never follows them.
func checkInside(path string, ctx Context) error {
	clean := filepath.Clean(path)
	for _, root := range ctx.Roots {
		if root == "" {
			continue
		}
		r := filepath.Clean(root)
		if clean == r || strings.HasPrefix(clean, r+string(filepath.Separator)) {
			return nil
		}
	}
	// A symlinked parent may still land inside: resolve and re-check.
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		for _, root := range ctx.Roots {
			if root == "" {
				continue
			}
			rr, err := filepath.EvalSymlinks(filepath.Clean(root))
			if err != nil {
				rr = filepath.Clean(root)
			}
			if resolved == rr || strings.HasPrefix(resolved, rr+string(filepath.Separator)) {
				return nil
			}
		}
	}
	return fmt.Errorf("outside any save root")
}

// expandGlob expands a pattern with * as in the manifest spec. It uses the
// standard glob (which never follows symlinks at match time: Lstat later
// enforces it per file).
func expandGlob(pattern string) ([]string, error) {
	if !strings.ContainsAny(pattern, "*?[") {
		if _, err := os.Lstat(pattern); err != nil {
			return nil, nil
		}
		return []string{pattern}, nil
	}
	return filepath.Glob(pattern)
}

// WriteBackup writes one backup zip: <backupsRoot>/<gameID>/<utc>-<trigger>-<shortid>.zip,
// with platypus-backup.json last. It writes a .tmp file, then renames, so a
// write failure leaves no version behind. Above RefuseBytes it refuses;
// above WarnBytes the caller warns.
func WriteBackup(backupsRoot string, game types.Game, trigger string, files []Enumerated, roots map[string]string, manifestTag string, unresolved, skipped int, now time.Time) (*types.SaveBackup, int64, error) {
	var total int64
	for _, f := range files {
		total += f.Size
	}
	if total > RefuseBytes {
		return nil, total, fmt.Errorf("saves are %s, above the 10 GiB limit: narrow the save paths", humanBytes(total))
	}
	dir := gameBackupDir(backupsRoot, game.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, total, err
	}
	name := backupFileName(trigger, now)
	tmp := filepath.Join(dir, name+".tmp")
	final := filepath.Join(dir, name)
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, total, err
	}
	zw := zip.NewWriter(f)
	metas := make([]types.SaveFile, 0, len(files))
	for _, ef := range files {
		data, err := os.ReadFile(ef.Abs)
		if err != nil {
			zw.Close()
			f.Close()
			os.Remove(tmp)
			return nil, total, err
		}
		// Re-check the symlink at write time: a file swapped for a link
		// between enumerate and write is skipped, never followed.
		if st, err := os.Lstat(ef.Abs); err != nil || st.Mode()&fs.ModeSymlink != 0 || !st.Mode().IsRegular() {
			skipped++
			continue
		}
		sum := sha256.Sum256(data)
		w, err := zw.Create(filepath.Join(ef.Root, ef.Rel))
		if err != nil {
			zw.Close()
			f.Close()
			os.Remove(tmp)
			return nil, total, err
		}
		if _, err := w.Write(data); err != nil {
			zw.Close()
			f.Close()
			os.Remove(tmp)
			return nil, total, err
		}
		metas = append(metas, types.SaveFile{Root: ef.Root, Path: ef.Rel, Size: int64(len(data)), Sha256: hex.EncodeToString(sum[:]), Mtime: ef.Mtime})
	}
	meta := types.BackupMeta{
		Version: VersionForMeta(), GameID: game.ID, Title: game.Title, Source: game.Source,
		CreatedAt: now.Unix(), Trigger: trigger, Manifest: manifestTag, Roots: roots,
		Files: metas, Unresolved: unresolved, Skipped: skipped, Bytes: total,
	}
	mb, _ := json.MarshalIndent(meta, "", "  ")
	w, err := zw.Create("platypus-backup.json")
	if err != nil {
		zw.Close()
		f.Close()
		os.Remove(tmp)
		return nil, total, err
	}
	if _, err := w.Write(mb); err != nil {
		zw.Close()
		f.Close()
		os.Remove(tmp)
		return nil, total, err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		os.Remove(tmp)
		return nil, total, err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return nil, total, err
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return nil, total, err
	}
	st, err := os.Stat(final)
	if err != nil {
		return nil, total, err
	}
	id := strings.TrimSuffix(name, ".zip")
	return &types.SaveBackup{ID: id, GameID: game.ID, CreatedAt: now.Unix(), Trigger: trigger, Files: len(metas), Bytes: total, Size: st.Size()}, total, nil
}

// VersionForMeta is the backup metadata version.
func VersionForMeta() int { return BackupMetaVersion }

// CleanStaleTmp removes stale .tmp files at startup.
func CleanStaleTmp(backupsRoot string) int {
	n := 0
	entries, err := os.ReadDir(backupsRoot)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub, err := os.ReadDir(filepath.Join(backupsRoot, e.Name()))
		if err != nil {
			continue
		}
		for _, f := range sub {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".tmp") {
				if os.Remove(filepath.Join(backupsRoot, e.Name(), f.Name())) == nil {
					n++
				}
			}
		}
	}
	return n
}

// ListBackups lists a game's backups, newest first. Only Platypus-written
// zips (holding platypus-backup.json) count.
func ListBackups(backupsRoot, gameID string) []types.SaveBackup {
	dir := gameBackupDir(backupsRoot, gameID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []types.SaveBackup
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".zip") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		meta, size, ok := readMeta(p)
		if !ok {
			continue
		}
		out = append(out, types.SaveBackup{
			ID: strings.TrimSuffix(e.Name(), ".zip"), GameID: gameID,
			CreatedAt: meta.CreatedAt, Trigger: meta.Trigger,
			Files: len(meta.Files), Bytes: meta.Bytes, Size: size,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt > out[j].CreatedAt
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func readMeta(zipPath string) (types.BackupMeta, int64, bool) {
	var meta types.BackupMeta
	st, err := os.Stat(zipPath)
	if err != nil {
		return meta, 0, false
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return meta, 0, false
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == "platypus-backup.json" {
			rc, err := f.Open()
			if err != nil {
				return meta, 0, false
			}
			err = json.NewDecoder(rc).Decode(&meta)
			rc.Close()
			if err != nil {
				return meta, 0, false
			}
			return meta, st.Size(), true
		}
	}
	return meta, 0, false
}

// ReadBackupMeta reads a backup's embedded metadata.
func ReadBackupMeta(backupsRoot, gameID, backupID string) (types.BackupMeta, bool) {
	meta, _, ok := readMeta(filepath.Join(gameBackupDir(backupsRoot, gameID), backupID+".zip"))
	return meta, ok
}

// DedupeEqual says the enumerated files equal the newest backup's
// path+size+mtime, so no new version is needed.
func DedupeEqual(files []Enumerated, meta types.BackupMeta) bool {
	if len(files) != len(meta.Files) {
		return false
	}
	// Compare as sets of root+path+size+mtime.
	want := map[string]types.SaveFile{}
	for _, f := range meta.Files {
		want[f.Root+"\x00"+f.Path] = f
	}
	for _, ef := range files {
		got, ok := want[ef.Root+"\x00"+ef.Rel]
		if !ok || got.Size != ef.Size || got.Mtime != ef.Mtime {
			return false
		}
	}
	return true
}

// PruneOldest prunes oldest-first after a successful backup down to keep
// versions, only Platypus-written zips, never the newest one.
func PruneOldest(backupsRoot, gameID string, keep int) int {
	if keep < 1 {
		keep = 1
	}
	list := ListBackups(backupsRoot, gameID)
	if len(list) <= keep {
		return 0
	}
	// Never the newest one: even a keep of 0 keeps it.
	pruned := 0
	for _, b := range list[keep:] {
		// list is newest-first; list[0] is newest and keep>=1, so the
		// newest is never in this slice.
		if os.Remove(filepath.Join(gameBackupDir(backupsRoot, gameID), b.ID+".zip")) == nil {
			pruned++
		}
	}
	return pruned
}

// PreviewRestore compares a backup against disk: overwritten (exists with
// different bytes), added (missing) and kept (identical) counts. Only files
// in the backup are considered; extra files on disk are never deleted.
func PreviewRestore(backupsRoot, gameID, backupID string, ctx Context) (types.RestorePreview, types.BackupMeta, error) {
	var pv types.RestorePreview
	meta, ok := ReadBackupMeta(backupsRoot, gameID, backupID)
	if !ok {
		return pv, meta, fmt.Errorf("unknown backup %q", backupID)
	}
	for _, f := range meta.Files {
		root, ok := meta.Roots[f.Root]
		if !ok {
			if r, ok := ctx.Roots[f.Root]; ok {
				root = r
			} else {
				continue
			}
		}
		// Zip-slip check on the stored relative path.
		if !safeRel(f.Path) {
			continue
		}
		abs := filepath.Join(root, f.Path)
		data, err := os.ReadFile(abs)
		if err != nil {
			pv.Added++
		} else {
			sum := sha256.Sum256(data)
			if hex.EncodeToString(sum[:]) == f.Sha256 {
				pv.Kept++
			} else {
				pv.Overwritten++
			}
		}
		pv.Files++
		pv.Bytes += f.Size
	}
	return pv, meta, nil
}

// Restore writes a backup's files to disk. Only files in the backup are
// written; extra files are never deleted. Zip-slip paths are refused. It
// returns overwritten/added counts.
func Restore(backupsRoot, gameID, backupID string, ctx Context) (overwritten, added int, err error) {
	zipPath := filepath.Join(gameBackupDir(backupsRoot, gameID), backupID+".zip")
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, 0, err
	}
	defer zr.Close()
	var meta types.BackupMeta
	contents := map[string][]byte{}
	for _, f := range zr.File {
		if f.Name == "platypus-backup.json" {
			rc, err := f.Open()
			if err != nil {
				return 0, 0, err
			}
			derr := json.NewDecoder(rc).Decode(&meta)
			rc.Close()
			if derr != nil {
				return 0, 0, derr
			}
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return 0, 0, err
		}
		data, err := io.ReadAll(io.LimitReader(rc, RefuseBytes+1))
		rc.Close()
		if err != nil {
			return 0, 0, err
		}
		contents[f.Name] = data
	}
	for _, mf := range meta.Files {
		if !safeRel(mf.Path) {
			return overwritten, added, fmt.Errorf("backup holds an unsafe path; restore refused")
		}
		root, ok := meta.Roots[mf.Root]
		if !ok {
			if r, ok := ctx.Roots[mf.Root]; ok {
				root = r
			} else {
				continue
			}
		}
		abs := filepath.Join(root, mf.Path)
		if err := checkInside(abs, ctx); err != nil {
			continue
		}
		// Never follow a symlinked destination: refuse the file.
		if st, err := os.Lstat(abs); err == nil && st.Mode()&fs.ModeSymlink != 0 {
			continue
		}
		data, ok := contents[filepath.Join(mf.Root, mf.Path)]
		if !ok {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return overwritten, added, err
		}
		_, existed := os.Lstat(abs)
		if err := os.WriteFile(abs, data, 0o644); err != nil {
			return overwritten, added, err
		}
		if existed == nil {
			if prev, err := os.ReadFile(abs); err == nil && len(prev) == len(data) {
				// written; count by prior existence is enough
			}
		}
		if existed != nil {
			added++
		} else {
			overwritten++
		}
		_ = os.Chtimes(abs, time.Now(), time.Unix(mf.Mtime, 0))
	}
	return overwritten, added, nil
}

func safeRel(p string) bool {
	if p == "" || filepath.IsAbs(p) {
		return false
	}
	clean := filepath.Clean(p)
	return clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator)) && clean == p
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 4 {
		div *= unit
		exp++
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	return fmt.Sprintf("%.1f %s", float64(n)/float64(div), units[exp])
}

var _ = sha256.Sum256
