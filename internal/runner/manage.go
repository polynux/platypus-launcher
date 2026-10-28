package runner

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"platypus/internal/compat"
	"platypus/internal/types"
)

// UserRunnerDir is where user-installed builds live (issue #142): the first
// compatibilitytools.d folder. Steam scans it, so a build unpacked here is
// picked for games without leaving the launcher.
func UserRunnerDir(home string) string {
	return filepath.Join(home, ".local/share/Steam/compatibilitytools.d")
}

// compatDirsFor mirrors the scanner's compatibilitytools.d folders.
func compatDirsFor(home string) []string {
	return []string{
		filepath.Join(home, ".local/share/Steam/compatibilitytools.d"),
		filepath.Join(home, ".steam/root/compatibilitytools.d"),
		filepath.Join(home, ".steam/compatibilitytools.d"),
	}
}

// Removable reports whether runner may be deleted (issue #142): only a
// user-installed build in the runner folder (compat:*). System Wine and
// Steam-managed Proton are never removed.
func Removable(r types.ProtonVersion, home string) bool {
	if r.IsSystem || r.ID == "" {
		return false
	}
	if !strings.HasPrefix(r.ID, "compat:") {
		return false
	}
	if r.Path == "" {
		return false
	}
	// The build folder is the proton binary's parent
	// (<compat>/<build>/proton); it must live inside one of the compat dirs.
	buildDir := filepath.Dir(r.Path)
	resolved, err := filepath.EvalSymlinks(buildDir)
	if err != nil {
		resolved = buildDir
	}
	for _, d := range compatDirsFor(home) {
		rd, err := filepath.EvalSymlinks(d)
		if err != nil {
			rd = d
		}
		if sameOrWithin(rd, resolved) && resolved != rd {
			return true
		}
	}
	return false
}

// BuildDirForRemovable returns the folder RemoveRunnerBuild deletes. It
// refuses anything Removable refuses.
func BuildDirForRemovable(r types.ProtonVersion, home string) (string, error) {
	if !Removable(r, home) {
		return "", fmt.Errorf("refusing to remove runner %q: only user-installed builds in the runner folder can be removed", r.ID)
	}
	buildDir := filepath.Clean(filepath.Dir(r.Path))
	if buildDir == "" || buildDir == "/" {
		return "", fmt.Errorf("refusing to remove runner %q", r.ID)
	}
	return buildDir, nil
}

// DirSize walks path and sums regular file sizes (issue #142). Symlinks are
// not followed; missing folders size 0. Errors mid-walk are ignored: the
// Runners page shows what could be read.
func DirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if fi, err := d.Info(); err == nil {
				total += fi.Size()
			}
		}
		return nil
	})
	return total
}

// sameOrWithin reports whether path is base or inside it.
func sameOrWithin(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel == "." || filepath.IsLocal(rel)
}

// DefaultReleasesURL lists GE-Proton releases (issue #142).
const DefaultReleasesURL = "https://api.github.com/repos/GloriousEggroll/proton-ge-custom/releases"

// maxRunnerBytes bounds an unpacked runner build (size bomb guard, same
// spirit as plugin zips): GE-Proton is ~400-600 MB unpacked.
var maxRunnerBytes = int64(2 << 30)

// FetchAvailable lists installable GE-Proton releases through the existing
// downloader (compat Fetcher). baseURL overrides the GitHub endpoint in
// tests (httptest). Network never goes elsewhere.
func FetchAvailable(ctx context.Context, fetcher *compat.Fetcher, baseURL string) ([]types.AvailableRunner, error) {
	if baseURL == "" {
		baseURL = DefaultReleasesURL
	}
	if fetcher == nil {
		fetcher = compat.NewFetcher("platypus-runner")
	}
	raw, err := fetcher.Get(ctx, baseURL)
	if err != nil {
		return nil, err
	}
	var releases []struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(raw, &releases); err != nil {
		return nil, fmt.Errorf("cannot parse releases: %w", err)
	}
	out := make([]types.AvailableRunner, 0, len(releases))
	for _, rel := range releases {
		if rel.TagName == "" {
			continue
		}
		var tarURL, sumURL string
		var size int64
		for _, a := range rel.Assets {
			lower := strings.ToLower(a.Name)
			if strings.HasSuffix(lower, ".tar.gz") && tarURL == "" {
				tarURL = a.URL
				size = a.Size
			}
			if strings.HasSuffix(lower, ".sha512sum") && sumURL == "" {
				sumURL = a.URL
			}
		}
		if tarURL == "" {
			continue
		}
		name := rel.Name
		if name == "" {
			name = rel.TagName
		}
		out = append(out, types.AvailableRunner{Tag: rel.TagName, Name: name, URL: tarURL, ChecksumURL: sumURL, SizeBytes: size})
	}
	return out, nil
}

// VerifyChecksumFile checks file against the expected hex SHA-512.
func VerifyChecksumFile(path, expectedHex string) error {
	fields := strings.Fields(expectedHex)
	if len(fields) == 0 {
		return fmt.Errorf("empty checksum")
	}
	expectedHex = fields[0]
	expected, err := hex.DecodeString(strings.TrimSpace(expectedHex))
	if err != nil || len(expected) != sha512.Size {
		return fmt.Errorf("bad checksum: %q", expectedHex)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha512.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if sum := h.Sum(nil); !equalBytes(sum, expected) {
		return fmt.Errorf("checksum mismatch for %s", filepath.Base(path))
	}
	return nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// Download fetches url through the existing downloader's client, writing to
// w with progress. It is the small streaming half Fetcher.Get doesn't cover:
// release JSON goes through Fetcher.Get, the tarball streams here through
// the same client. At most maxBytes+1 response bytes are read: anything past
// the cap fails instead of filling the disk.
func Download(ctx context.Context, client *http.Client, url string, w io.Writer, progress func(done, total int64), maxBytes int64) error {
	if client == nil {
		client = http.DefaultClient
	}
	if maxBytes <= 0 {
		maxBytes = maxRunnerBytes
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "platypus-runner")
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return &compat.StatusError{Status: resp.StatusCode}
	}
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 1<<20)
	for {
		n, rerr := io.LimitReader(resp.Body, maxBytes+1-done).Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			if done > maxBytes {
				return fmt.Errorf("runner download over %d MB", maxBytes>>20)
			}
			if progress != nil {
				progress(done, total)
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				return nil
			}
			return rerr
		}
	}
}

// DownloadLimit bounds one runner tarball: the release asset size plus slack
// for checksums and compression variance, or the unpack budget when the
// release names no size.
func DownloadLimit(assetSize int64) int64 {
	const slack = 64 << 20
	if assetSize > 0 {
		return assetSize + slack
	}
	return maxRunnerBytes
}

// maxRunnerEntries bounds a runner archive's entry count (inode-exhaustion
// guard, same spirit as the size budget).
var maxRunnerEntries = 200000

// UnpackTarGz unpacks a GE-Proton tar.gz into destDir with the same safety
// rules as plugin zips: no absolute paths, no "..", no symlink escape, no
// device files, and a shared unpack budget (size bombs). Proton builds carry
// relative symlinks with ".." hops that stay inside the build (their
// default_pfx DLL links), so a link target is validated relative to the
// link's own folder and must stay under the archive's single top-level
// folder. Every write goes through an os.Root on the staging folder, so no
// write can follow a link out; chains that escape are refused by the final
// sweep. It returns the top-level build folder name.
func UnpackTarGz(ctx context.Context, r io.Reader, destDir string, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		maxBytes = maxRunnerBytes
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(destDir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", fmt.Errorf("not a gzip archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	remaining := maxBytes
	entries := 0
	var top string
	var links []linkCheck
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		// Pax global headers carry no files.
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		name = filepath.FromSlash(name)
		if name == "" || name == "." {
			continue
		}
		if !filepath.IsLocal(name) {
			return "", fmt.Errorf("refusing tar entry %q: escapes the runner folder", hdr.Name)
		}
		entries++
		if entries > maxRunnerEntries {
			return "", fmt.Errorf("refusing tar archive: over %d entries", maxRunnerEntries)
		}
		if top == "" {
			top = strings.Split(name, string(filepath.Separator))[0]
		}
		if name != top && !strings.HasPrefix(name, top+string(filepath.Separator)) {
			return "", fmt.Errorf("refusing tar entry %q: outside the build folder %q", hdr.Name, top)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0o755); err != nil {
				return "", err
			}
		case tar.TypeReg, tar.TypeRegA:
			if hdr.Size < 0 || hdr.Size > remaining {
				return "", fmt.Errorf("refusing tar entry %q: unpacked data over %d MB (size bomb?)", hdr.Name, maxBytes>>20)
			}
			if dir := filepath.Dir(name); dir != "." {
				if err := root.MkdirAll(dir, 0o755); err != nil {
					return "", err
				}
			}
			perm := os.FileMode(0o644)
			if hdr.FileInfo().Mode().Perm()&0o111 != 0 {
				perm = 0o755
			}
			out, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
			if err != nil {
				return "", err
			}
			n, err := io.CopyN(out, tr, remaining+1)
			if err == nil {
				_ = out.Close()
				return "", fmt.Errorf("refusing tar entry %q: doesn't fit the remaining %d bytes", hdr.Name, remaining)
			}
			if err != io.EOF {
				_ = out.Close()
				return "", err
			}
			remaining -= n
			if err := out.Close(); err != nil {
				return "", err
			}
		case tar.TypeSymlink:
			// The target is read relative to the link's own folder, like the
			// filesystem reads it, and must stay under the build folder.
			t := filepath.Join(filepath.Dir(name), filepath.FromSlash(hdr.Linkname))
			if !filepath.IsLocal(t) || (t != top && !strings.HasPrefix(t, top+string(filepath.Separator))) {
				return "", fmt.Errorf("refusing tar entry %q: symlink points outside the runner folder", hdr.Name)
			}
			if dir := filepath.Dir(name); dir != "." {
				if err := root.MkdirAll(dir, 0o755); err != nil {
					return "", err
				}
			}
			_ = root.Remove(name)
			if err := root.Symlink(hdr.Linkname, name); err != nil {
				return "", err
			}
			links = append(links, linkCheck{path: name})
		default:
			return "", fmt.Errorf("refusing tar entry %q: not a file, folder or symlink", hdr.Name)
		}
	}
	if top == "" {
		return "", fmt.Errorf("empty runner archive")
	}
	resolvedDest, err := filepath.EvalSymlinks(destDir)
	if err != nil {
		return "", err
	}
	for _, l := range links {
		resolved, err := filepath.EvalSymlinks(filepath.Join(resolvedDest, l.path))
		if err != nil {
			return "", fmt.Errorf("refusing tar symlink %q: broken", relOf(resolvedDest, filepath.Join(resolvedDest, l.path)))
		}
		if !sameOrWithin(resolvedDest, resolved) {
			return "", fmt.Errorf("refusing tar symlink %q: escapes the runner folder", relOf(resolvedDest, filepath.Join(resolvedDest, l.path)))
		}
	}
	return top, nil
}

type linkCheck struct {
	path string
}

// stagePrefixes are the temp stage folders a runner install leaves behind
// when the core dies mid-install: the current one inside the runner folder,
// and the legacy one next to it from before the stage moved inside.
var stagePrefixes = []string{".platypus-runner-", ".runner-install-"}

// CleanStaleRunnerStages deletes leftover install stages for home's runner
// folder: a killed core never resumes them, and the scanner ignores them
// (no proton file), so they only take space.
func CleanStaleRunnerStages(home string) error {
	destDir := UserRunnerDir(home)
	resolved, err := filepath.EvalSymlinks(destDir)
	if err != nil {
		resolved = destDir
	}
	dirs := map[string]bool{resolved: true, filepath.Dir(filepath.Clean(destDir)): true}
	var first error
	for dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			stale := false
			for _, p := range stagePrefixes {
				if strings.HasPrefix(name, p) {
					stale = true
					break
				}
			}
			if !stale {
				continue
			}
			if err := os.RemoveAll(filepath.Join(dir, name)); err != nil && first == nil {
				first = err
			}
		}
	}
	return first
}

func relOf(base, path string) string {
	if rel, err := filepath.Rel(base, path); err == nil {
		return rel
	}
	return path
}
