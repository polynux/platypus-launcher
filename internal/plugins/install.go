package plugins

// Zip plugin install (issue #147): unpack into a temp folder, validate the
// manifest and refuse anything unsafe, then move it into the plugins folder
// atomically and rescan. A zip install is never auto-approved: the new plugin
// goes through the usual approval sheet.

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"platypus/internal/settings"
)

// installMaxBytes bounds the unpacked total of a plugin zip (zip bombs), and
// installMaxFiles the entry count. Vars (not consts) so tests can shrink
// them without writing hundreds of megabytes.
var installMaxBytes = int64(256 << 20)
var installMaxFiles = 10000

// InstallResult is what InstallFromZip answers: the installed plugin, or a
// replace question when the id is already installed.
type InstallResult struct {
	ID             string
	Version        string
	Replaced       bool
	NeedsConfirm   bool
	CurrentVersion string
	NewVersion     string
}

// InstallFromZip validates zipPath and installs the one plugin it holds, at
// the archive root or in a single top-level folder. An id that is already
// installed is never merged: with confirmReplace false it answers NeedsConfirm
// ("would replace vX with vY") and writes nothing; with true it replaces the
// whole folder atomically. Bundled plugins can't be replaced this way.
func (m *Manager) InstallFromZip(zipPath string, confirmReplace bool) (InstallResult, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return InstallResult{}, fmt.Errorf("not a plugin zip: %w", err)
	}
	defer r.Close()
	staged, man, err := m.stageZip(&r.Reader)
	if err != nil {
		return InstallResult{}, err
	}
	defer os.RemoveAll(filepath.Dir(staged))
	if BundledPluginIDs[man.ID] {
		return InstallResult{}, fmt.Errorf("the bundled %s plugin can't be replaced from a zip", man.ID)
	}
	// "Already installed" is the folder, not a readable manifest: a broken,
	// half-edited or incompatible plugin still asks before it is replaced,
	// and is never deleted without an explicit confirm.
	if _, err := os.Lstat(filepath.Join(m.dir, man.ID)); err == nil {
		current, newVersion := "", man.Version
		if cur, err := LoadManifest(filepath.Join(m.dir, man.ID)); err == nil && cur.ID == man.ID {
			current = cur.Version
		}
		if !confirmReplace {
			return InstallResult{
				ID: man.ID, Version: current,
				NeedsConfirm: true, CurrentVersion: current, NewVersion: newVersion,
			}, nil
		}
		if err := installDirFromSrc(m.dir, man.ID, staged); err != nil {
			return InstallResult{}, err
		}
		if man.IsData() {
			if err := m.markDataPending(man.ID); err != nil {
				return InstallResult{}, err
			}
		}
		found, err := m.Discover()
		if err != nil {
			return InstallResult{}, err
		}
		_ = found
		version := man.Version
		if p, ok := m.Plugin(man.ID); ok {
			version = p.Manifest.Version
		}
		return InstallResult{ID: man.ID, Version: version, Replaced: true}, nil
	}
	if err := installDirFromSrc(m.dir, man.ID, staged); err != nil {
		return InstallResult{}, err
	}
	if man.IsData() {
		if err := m.markDataPending(man.ID); err != nil {
			return InstallResult{}, err
		}
	}
	found, err := m.Discover()
	if err != nil {
		return InstallResult{}, err
	}
	_ = found
	version := man.Version
	if p, ok := m.Plugin(man.ID); ok {
		version = p.Manifest.Version
	}
	return InstallResult{ID: man.ID, Version: version}, nil
}

// stageZip unpacks the zip into a temp folder and validates it, returning the
// staged plugin folder and its manifest. Refused: absolute paths, "..",
// symlinks escaping the folder, device files, oversized archives, a missing
// or invalid manifest. Nothing is written outside the temp folder.
func (m *Manager) stageZip(r *zip.Reader) (string, Manifest, error) {
	files := skipZipCruft(r.File)
	if len(files) > installMaxFiles {
		return "", Manifest{}, fmt.Errorf("refusing plugin zip: %d entries (max %d)", len(files), installMaxFiles)
	}
	var total uint64
	budget := uint64(installMaxBytes)
	for _, f := range files {
		// Refuse a single entry larger than the whole budget before
		// summing: a zip64 size near 2^64-1 would wrap an int64 total
		// negative and sail through the sum check below.
		if f.UncompressedSize64 > budget {
			return "", Manifest{}, fmt.Errorf("refusing plugin zip: entry %q declares %d bytes (max %d MB)", f.Name, f.UncompressedSize64, installMaxBytes>>20)
		}
		total += f.UncompressedSize64
		if total > budget {
			return "", Manifest{}, fmt.Errorf("refusing plugin zip: unpacked data over %d MB (zip bomb?)", installMaxBytes>>20)
		}
	}
	root := zipRoot(files)
	parent, err := os.MkdirTemp("", "plugin-zip-*")
	if err != nil {
		return "", Manifest{}, err
	}
	staged := filepath.Join(parent, "staged")
	if err := os.MkdirAll(staged, 0o755); err != nil {
		os.RemoveAll(parent)
		return "", Manifest{}, err
	}
	fail := func(format string, args ...any) (string, Manifest, error) {
		os.RemoveAll(parent)
		return "", Manifest{}, fmt.Errorf(format, args...)
	}
	// One byte budget shared by every entry's real bytes (not the declared
	// sizes): an entry that runs past it is refused and the temp dir goes.
	remaining := installMaxBytes
	for _, f := range files {
		rel, ok := zipTarget(root, f.Name)
		if !ok {
			return fail("refusing plugin zip: entry %q escapes the plugin folder", f.Name)
		}
		if err := extractZipEntry(f, filepath.Join(staged, rel), &remaining); err != nil {
			os.RemoveAll(parent)
			if ue, ok := err.(*zipUnsafeError); ok {
				return "", Manifest{}, fmt.Errorf("refusing plugin zip: %s", ue.msg)
			}
			return "", Manifest{}, err
		}
	}
	if err := auditStagedSymlinks(staged); err != nil {
		os.RemoveAll(parent)
		return "", Manifest{}, fmt.Errorf("refusing plugin zip: %s", err.Error())
	}
	data, err := os.ReadFile(filepath.Join(staged, ManifestFile))
	if err != nil {
		os.RemoveAll(parent)
		return "", Manifest{}, fmt.Errorf("refusing plugin zip: no %s (%v)", ManifestFile, err)
	}
	var man Manifest
	if err := json.Unmarshal(data, &man); err != nil {
		os.RemoveAll(parent)
		return "", Manifest{}, fmt.Errorf("refusing plugin zip: %s is invalid: %w", ManifestFile, err)
	}
	if err := man.validate(); err != nil {
		os.RemoveAll(parent)
		return "", Manifest{}, fmt.Errorf("refusing plugin zip: invalid manifest: %w", err)
	}
	// The manifest's exec gets its exec bit: zips often lose it.
	if man.Exec != "" && !man.IsData() {
		if p := filepath.Join(staged, filepath.FromSlash(man.Exec)); isLocalRel(staged, p) {
			if st, err := os.Stat(p); err == nil && !st.Mode().IsDir() {
				_ = os.Chmod(p, 0o755)
			}
		}
		if _, err := man.ExecPath(staged); err != nil {
			os.RemoveAll(parent)
			return "", Manifest{}, fmt.Errorf("refusing plugin zip: %v", err)
		}
	}
	return staged, man, nil
}

// skipZipCruft drops the metadata macOS leaves in zips.
func skipZipCruft(files []*zip.File) []*zip.File {
	out := files[:0]
	for _, f := range files {
		name := strings.TrimPrefix(f.Name, "./")
		if name == "" || strings.HasPrefix(name, "__MACOSX/") || name == "__MACOSX" {
			continue
		}
		if base := name[strings.LastIndex(name, "/")+1:]; base == ".DS_Store" {
			continue
		}
		out = append(out, f)
	}
	return out
}

// zipRoot is the single top-level folder when every entry lives under one,
// else "": one plugin per zip, at the archive root or in one folder.
func zipRoot(files []*zip.File) string {
	tops := map[string]bool{}
	for _, f := range files {
		name := strings.Trim(strings.TrimPrefix(f.Name, "./"), "/")
		if name == "" {
			continue
		}
		seg := name
		if i := strings.IndexByte(name, '/'); i >= 0 {
			seg = name[:i]
		}
		tops[seg] = true
	}
	if len(tops) != 1 {
		return ""
	}
	var top string
	for t := range tops {
		top = t
	}
	for _, f := range files {
		name := strings.Trim(strings.TrimPrefix(f.Name, "./"), "/")
		if name == top && !strings.HasSuffix(f.Name, "/") {
			return "" // a bare file at the root: the plugin is the root
		}
	}
	return top
}

// zipTarget maps an entry name to its path inside the staged folder.
func zipTarget(root, name string) (string, bool) {
	name = strings.TrimPrefix(name, "./")
	if root != "" {
		if name != root && !strings.HasPrefix(name, root+"/") {
			return "", false
		}
		name = strings.TrimPrefix(strings.TrimPrefix(name, root), "/")
		if name == "" {
			return "", true // the top folder itself
		}
	}
	name = filepath.FromSlash(name)
	if name == "" || name == "." {
		return "", true
	}
	if !filepath.IsLocal(name) {
		return "", false
	}
	return name, true
}

type zipUnsafeError struct{ msg string }

func (e *zipUnsafeError) Error() string { return e.msg }

// extractZipEntry writes one entry under dst (already joined), counting its
// real bytes against the shared budget. Symlinks are stored as symlinks only
// when their target stays inside the plugin folder; device files and anything
// that isn't a dir, file or symlink is refused.
func extractZipEntry(f *zip.File, dst string, remaining *int64) error {
	mode := f.FileInfo().Mode()
	switch {
	case f.FileInfo().IsDir():
		return os.MkdirAll(dst, 0o755)
	case mode&os.ModeSymlink != 0:
		rc, err := f.Open()
		if err != nil {
			return err
		}
		targetBytes, err := io.ReadAll(io.LimitReader(rc, 1<<20))
		rc.Close()
		if err != nil {
			return err
		}
		target := string(targetBytes)
		if !filepath.IsLocal(filepath.FromSlash(target)) {
			return &zipUnsafeError{fmt.Sprintf("symlink %q points outside the plugin folder", f.Name)}
		}
		// Chains of inner links can still escape where each hop looks
		// local; auditStagedSymlinks resolves them after extraction.
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		_ = os.Remove(dst)
		return os.Symlink(target, dst)
	case mode.IsRegular():
		if mode&(os.ModeDevice|os.ModeCharDevice|os.ModeNamedPipe|os.ModeSocket|os.ModeIrregular) != 0 {
			return &zipUnsafeError{fmt.Sprintf("entry %q is not a regular file", f.Name)}
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		perm := os.FileMode(0o644)
		if mode.Perm()&0o111 != 0 {
			perm = 0o755
		}
		out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
		if err != nil {
			return err
		}
		// CopyN one byte past the rest of the budget: copying exactly that
		// many means the entry doesn't fit. A short entry ends in EOF (the
		// reader verifies the size and CRC there); anything else is a
		// corrupt or hostile archive.
		n, err := io.CopyN(out, rc, *remaining+1)
		if err == nil {
			_ = out.Close()
			return &zipUnsafeError{fmt.Sprintf("entry %q doesn't fit the remaining %d bytes", f.Name, *remaining)}
		}
		if err != io.EOF {
			_ = out.Close()
			return err
		}
		*remaining -= n
		// The entry must be over: a further read has to say EOF, which also
		// surfaces a checksum mismatch the copy above would have hidden.
		var one [1]byte
		if _, err := rc.Read(one[:]); err != io.EOF {
			_ = out.Close()
			if err == nil {
				return &zipUnsafeError{fmt.Sprintf("entry %q is longer than declared", f.Name)}
			}
			return err
		}
		if err := out.Close(); err != nil {
			return err
		}
		return nil
	default:
		return &zipUnsafeError{fmt.Sprintf("entry %q is not a file, folder or symlink", f.Name)}
	}
}

// auditStagedSymlinks resolves every staged symlink and refuses the ones
// that escape the folder (a chain of inner links can escape where each hop
// looks local). Broken links are refused too: nothing in a plugin needs one.
func auditStagedSymlinks(staged string) error {
	// Resolve the staging dir first: with a symlinked TMPDIR every
	// legitimate inner link would otherwise read as escaping it.
	if resolved, err := filepath.EvalSymlinks(staged); err == nil {
		staged = resolved
	}
	var bad string
	err := filepath.Walk(staged, func(path string, info os.FileInfo, err error) error {
		if err != nil || bad != "" {
			return err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return nil
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			bad = fmt.Sprintf("symlink %q is broken", relOf(staged, path))
			return nil
		}
		if !sameOrWithin(staged, resolved) {
			bad = fmt.Sprintf("symlink %q escapes the plugin folder", relOf(staged, path))
		}
		return nil
	})
	if err != nil {
		return err
	}
	if bad != "" {
		return fmt.Errorf("%s", bad)
	}
	return nil
}

func relOf(base, path string) string {
	if rel, err := filepath.Rel(base, path); err == nil {
		return rel
	}
	return path
}

func sameOrWithin(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return filepath.IsLocal(rel)
}

func isLocalRel(base, path string) bool { return sameOrWithin(base, path) }

// markDataPending files an empty approval record for a freshly installed
// data plugin, so its fixes, tweaks and theme wait for approval instead of
// applying at once (issue #147). An existing record (a confirmed replace of
// an approved plugin) is kept, so the new bits show up as changed.
func (m *Manager) markDataPending(id string) error {
	if _, ok := m.trust.PluginState(id); ok {
		return nil
	}
	return m.trust.SetPluginState(id, settings.PluginState{})
}

// copyPluginDir replicates the staged folder: dirs, regular files with
// their mode, and symlinks as symlinks (so the installed hash matches what
// was validated).
func copyPluginDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		out := filepath.Join(dst, rel)
		fi, err := os.Lstat(path)
		if err != nil {
			return err
		}
		switch {
		case fi.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(target, out)
		case fi.IsDir():
			return os.MkdirAll(out, fi.Mode().Perm())
		case fi.Mode().IsRegular():
			if err := streamFile(path, out, fi.Mode().Perm()); err != nil {
				return err
			}
			return nil
		default:
			return fmt.Errorf("install: refusing special file %s", rel)
		}
	})
}

// streamFile copies one regular file without holding it all in memory: a
// staged zip entry can be as large as the whole install budget.
func streamFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	// A short write still reports through Close only when Sync fails; the
	// copy error is the one that matters.
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// installDirFromSrc moves the staged folder src into pluginsDir/id
// atomically: a temp dir next to the target, then renames. An existing target
// is only replaced after the caller verified it is the same plugin; it is
// moved aside first so a failed rename can roll back, and never merged.
func installDirFromSrc(pluginsDir, id, src string) error {
	if pluginsDir == "" || id == "" || src == "" {
		return fmt.Errorf("plugin %s: bad paths", id)
	}
	if !filepath.IsLocal(id) || id == "." || id == ".." {
		return fmt.Errorf("plugin %s: bad plugin id", id)
	}
	dst := filepath.Join(pluginsDir, id)
	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return fmt.Errorf("plugin %s: %w", id, err)
	}
	tmp, err := os.MkdirTemp(pluginsDir, ".install-"+id+".*")
	if err != nil {
		return fmt.Errorf("plugin %s: %w", id, err)
	}
	// From here on every failure removes the temp dir; the target is only
	// touched by renames.
	if err := copyPluginDir(src, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("plugin %s: %w", id, err)
	}
	if _, err := os.Lstat(dst); err == nil {
		bak, err := os.MkdirTemp(pluginsDir, ".install-"+id+".bak.*")
		if err != nil {
			_ = os.RemoveAll(tmp)
			return fmt.Errorf("plugin %s: %w", id, err)
		}
		_ = os.Remove(bak)
		if err := os.Rename(dst, bak); err != nil {
			_ = os.RemoveAll(tmp)
			_ = os.Remove(bak)
			return fmt.Errorf("plugin %s: %w", id, err)
		}
		if err := os.Rename(tmp, dst); err != nil {
			_ = os.Rename(bak, dst)
			_ = os.RemoveAll(tmp)
			return fmt.Errorf("plugin %s: %w", id, err)
		}
		_ = os.RemoveAll(bak)
		return nil
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("plugin %s: %w", id, err)
	}
	return nil
}
