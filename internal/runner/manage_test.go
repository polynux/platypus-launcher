package runner

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/compat"
	"platypus/internal/types"
)

// Issue #142: only user-installed builds in the runner folder are removable.
func TestRemovableOnlyUserBuilds(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	build := filepath.Join(UserRunnerDir(home), "GE-Proton10-4")
	if err := os.MkdirAll(build, 0o755); err != nil {
		t.Fatal(err)
	}
	proton := filepath.Join(build, "proton")
	if err := os.WriteFile(proton, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	user := types.ProtonVersion{ID: "compat:GE-Proton10-4", Path: proton}
	if !Removable(user, home) {
		t.Fatal("compat build in the runner folder must be removable")
	}
	if _, err := BuildDirForRemovable(user, home); err != nil {
		t.Fatalf("removable build dir: %v", err)
	}
	for _, r := range []types.ProtonVersion{
		{ID: "system:wine", Path: "/usr/bin/wine", IsSystem: true},
		{ID: "steam:Proton 9.0", Path: filepath.Join(home, ".local/share/Steam/steamapps/common/Proton 9.0/proton")},
		{ID: "lutris:foo", Path: filepath.Join(home, ".local/share/lutris/runners/wine/foo/bin/wine"), IsCustom: true},
		{ID: "compat:GE-Proton10-4", Path: "/elsewhere/proton"},
	} {
		if Removable(r, home) {
			t.Fatalf("runner %q must never be removable", r.ID)
		}
		if _, err := BuildDirForRemovable(r, home); err == nil {
			t.Fatalf("BuildDirForRemovable(%q) must refuse", r.ID)
		}
	}
}

// Issue #142: the Runners page shows each runner's size.
func TestDirSizeSumsFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a"), bytes.Repeat([]byte("x"), 100), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b"), bytes.Repeat([]byte("y"), 50), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DirSize(dir); got != 150 {
		t.Fatalf("DirSize = %d, want 150", got)
	}
	if got := DirSize(filepath.Join(dir, "missing")); got != 0 {
		t.Fatalf("missing DirSize = %d, want 0", got)
	}
}

// Issue #142: unpacking is as safe as plugin zips (no traversal, no symlink
// escape, no size bombs).
func TestUnpackTarGzRefusesUnsafe(t *testing.T) {
	t.Parallel()
	writeTar := func(t *testing.T, entries map[string]string, symlinks map[string]string) []byte {
		t.Helper()
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gz)
		for name, body := range entries {
			if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
				t.Fatal(err)
			}
			if _, err := tw.Write([]byte(body)); err != nil {
				t.Fatal(err)
			}
		}
		for name, target := range symlinks {
			if err := tw.WriteHeader(&tar.Header{Name: name, Linkname: target, Typeflag: tar.TypeSymlink}); err != nil {
				t.Fatal(err)
			}
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	good := writeTar(t, map[string]string{"GE-Proton10-4/proton": "#!/bin/sh\n"}, nil)
	dest := t.TempDir()
	top, err := UnpackTarGz(bytes.NewReader(good), dest, 0)
	if err != nil || top != "GE-Proton10-4" {
		t.Fatalf("good unpack = %q, %v", top, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "GE-Proton10-4", "proton")); err != nil {
		t.Fatalf("unpacked proton missing: %v", err)
	}
	for name, entries := range map[string]map[string]string{
		"traversal": {"../evil": "x"},
		"absolute":  {"/evil": "x"},
	} {
		raw := writeTar(t, entries, nil)
		if _, err := UnpackTarGz(bytes.NewReader(raw), t.TempDir(), 0); err == nil {
			t.Fatalf("%s tar must be refused", name)
		}
	}
	escape := writeTar(t, map[string]string{"GE-Proton10-4/proton": "x"}, map[string]string{"GE-Proton10-4/link": "/etc/passwd"})
	if _, err := UnpackTarGz(bytes.NewReader(escape), t.TempDir(), 0); err == nil {
		t.Fatal("symlink escape must be refused")
	}
	big := bytes.Repeat([]byte("x"), 100)
	var bigBuf bytes.Buffer
	gz := gzip.NewWriter(&bigBuf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "GE-Proton10-4/big", Mode: 0o644, Size: int64(len(big)), Typeflag: tar.TypeReg})
	_, _ = tw.Write(big)
	_ = tw.Close()
	_ = gz.Close()
	if _, err := UnpackTarGz(bytes.NewReader(bigBuf.Bytes()), t.TempDir(), 10); err == nil {
		t.Fatal("size bomb must be refused")
	}
}

// Issue #142: checksums from the release are verified before unpacking.
func TestVerifyChecksumFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha512.Sum512([]byte("hello"))
	hexSum := hex.EncodeToString(sum[:])
	if err := VerifyChecksumFile(path, hexSum+"  f\n"); err != nil {
		t.Fatalf("good checksum: %v", err)
	}
	if err := VerifyChecksumFile(path, hex.EncodeToString(bytes.Repeat([]byte{1}, 64))); err == nil {
		t.Fatal("bad checksum must fail")
	}
}

// Issue #142: installable versions come through the existing downloader
// (compat Fetcher); tests use httptest, never the network.
func TestFetchAvailableThroughFetcher(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"tag_name":"GE-Proton10-4","name":"GE-Proton10-4","assets":[{"name":"GE-Proton10-4.tar.gz","browser_download_url":"http://example.com/g.tar.gz","size":123},{"name":"GE-Proton10-4.sha512sum","browser_download_url":"http://example.com/g.sha512sum"}]},{"tag_name":"empty","name":"empty","assets":[]}]`)
	}))
	defer srv.Close()
	fetcher := compat.NewFetcher("test")
	list, err := FetchAvailable(context.Background(), fetcher, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Tag != "GE-Proton10-4" || list[0].URL != "http://example.com/g.tar.gz" || list[0].ChecksumURL != "http://example.com/g.sha512sum" {
		t.Fatalf("available = %+v", list)
	}
}
