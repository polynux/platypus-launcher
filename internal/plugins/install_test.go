package plugins

// Zip install (issue #147): a valid zip lands waiting for approval, and every
// unsafe archive is refused without writing outside a temp folder.

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/settings"
)

func zipManager(t *testing.T) (*Manager, *settings.Store, string) {
	t.Helper()
	root := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), root)
	if err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	m := NewManager(Options{Dir: pluginsDir, LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: st, Home: root})
	t.Cleanup(m.Shutdown)
	if _, err := m.Discover(); err != nil {
		t.Fatal(err)
	}
	return m, st, pluginsDir
}

const zipManifest = `{"id":"zippy","name":"Zippy","version":"1.2.0","manifestVersion":1,` +
	`"protocol":{"versions":["1.0"]},"store":"zippy","exec":"run.sh",` +
	`"capabilities":["library"],"permissions":{"network":[],"read":[],"write":[]}}`

type zipEntry struct {
	name string
	mode os.FileMode
	body string
}

func writeTestZip(t *testing.T, entries []zipEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plugin.zip")
	w, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(w)
	for _, e := range entries {
		fh := &zip.FileHeader{Name: e.name, Method: zip.Store}
		fh.SetMode(e.mode)
		fw, err := zw.CreateHeader(fh)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func validZipEntries() []zipEntry {
	return []zipEntry{
		{name: "plugin.json", mode: 0o644, body: zipManifest},
		{name: "run.sh", mode: 0o644, body: "#!/bin/sh\necho hi\n"},
	}
}

func TestInstallValidZipNeedsApproval(t *testing.T) {
	t.Parallel()
	m, _, pluginsDir := zipManager(t)
	res, err := m.InstallFromZip(writeTestZip(t, validZipEntries()), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "zippy" || res.Version != "1.2.0" || res.NeedsConfirm || res.Replaced {
		t.Fatalf("result = %+v, want fresh install of zippy 1.2.0", res)
	}
	p, ok := m.Plugin("zippy")
	if !ok {
		t.Fatal("installed plugin not listed")
	}
	if p.Status != StatusUnapproved {
		t.Fatalf("status = %q, want unapproved (never auto-approved)", p.Status)
	}
	if st, err := os.Stat(filepath.Join(pluginsDir, "zippy", "run.sh")); err != nil || st.Mode().Perm()&0o111 == 0 {
		t.Fatalf("exec lost its exec bit: %v %v", st, err)
	}
}

func TestInstallZipInSingleFolder(t *testing.T) {
	t.Parallel()
	m, _, _ := zipManager(t)
	entries := []zipEntry{
		{name: "zippy/plugin.json", mode: 0o644, body: zipManifest},
		{name: "zippy/run.sh", mode: 0o755, body: "#!/bin/sh\n"},
	}
	res, err := m.InstallFromZip(writeTestZip(t, entries), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "zippy" {
		t.Fatalf("result = %+v, want zippy", res)
	}
	if _, ok := m.Plugin("zippy"); !ok {
		t.Fatal("installed plugin not listed")
	}
}

func TestInstallZipRefusals(t *testing.T) {
	t.Parallel()
	cases := map[string][]zipEntry{
		"traversal": {
			{name: "plugin.json", mode: 0o644, body: zipManifest},
			{name: "run.sh", mode: 0o644, body: "#!/bin/sh\n"},
			{name: "../evil.txt", mode: 0o644, body: "evil"},
		},
		"nested traversal": {
			{name: "zippy/plugin.json", mode: 0o644, body: zipManifest},
			{name: "zippy/run.sh", mode: 0o644, body: "#!/bin/sh\n"},
			{name: "zippy/sub/../../evil.txt", mode: 0o644, body: "evil"},
		},
		"absolute": {
			{name: "plugin.json", mode: 0o644, body: zipManifest},
			{name: "run.sh", mode: 0o644, body: "#!/bin/sh\n"},
			{name: "/evil.txt", mode: 0o644, body: "evil"},
		},
		"symlink escape": {
			{name: "plugin.json", mode: 0o644, body: zipManifest},
			{name: "run.sh", mode: 0o644, body: "#!/bin/sh\n"},
			{name: "link", mode: os.ModeSymlink | 0o755, body: "/etc/passwd"},
		},
		"symlink dotdot": {
			{name: "plugin.json", mode: 0o644, body: zipManifest},
			{name: "run.sh", mode: 0o644, body: "#!/bin/sh\n"},
			{name: "sub/link", mode: os.ModeSymlink | 0o755, body: "../../evil"},
		},
		"device": {
			{name: "plugin.json", mode: 0o644, body: zipManifest},
			{name: "run.sh", mode: 0o644, body: "#!/bin/sh\n"},
			{name: "devnull", mode: os.ModeDevice | 0o600, body: ""},
		},
		"missing manifest": {
			{name: "run.sh", mode: 0o644, body: "#!/bin/sh\n"},
		},
		"invalid manifest": {
			{name: "plugin.json", mode: 0o644, body: `{"id":"!!!","name":""}`},
			{name: "run.sh", mode: 0o644, body: "#!/bin/sh\n"},
		},
	}
	for name, entries := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m, _, pluginsDir := zipManager(t)
			if _, err := m.InstallFromZip(writeTestZip(t, entries), false); err == nil {
				t.Fatalf("%s: install should fail", name)
			}
			found, err := m.Discover()
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range found {
				if p.Manifest.ID == "zippy" {
					t.Fatalf("%s: refused install still listed", name)
				}
			}
			if _, err := os.Lstat(filepath.Join(pluginsDir, "zippy")); !os.IsNotExist(err) {
				t.Fatalf("%s: refused install wrote the plugins folder", name)
			}
		})
	}
}

func TestInstallZipBombRefused(t *testing.T) {
	// No t.Parallel: shrinks the package limits for the duration.
	oldBytes := installMaxBytes
	installMaxBytes = 100
	t.Cleanup(func() { installMaxBytes = oldBytes })
	m, _, pluginsDir := zipManager(t)
	if _, err := m.InstallFromZip(writeTestZip(t, validZipEntries()), false); err == nil {
		t.Fatal("oversized unpacked data should fail")
	}
	if _, err := os.Lstat(filepath.Join(pluginsDir, "zippy")); !os.IsNotExist(err) {
		t.Fatal("refused install wrote the plugins folder")
	}
}

func TestInstallReplaceAsksFirst(t *testing.T) {
	t.Parallel()
	m, _, pluginsDir := zipManager(t)
	if _, err := m.InstallFromZip(writeTestZip(t, validZipEntries()), false); err != nil {
		t.Fatal(err)
	}
	// A leftover file from the old version: replace must not merge.
	if err := os.WriteFile(filepath.Join(pluginsDir, "zippy", "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	v2 := `{"id":"zippy","name":"Zippy","version":"2.0.0","manifestVersion":1,` +
		`"protocol":{"versions":["1.0"]},"store":"zippy","exec":"run.sh",` +
		`"capabilities":["library"],"permissions":{"network":[],"read":[],"write":[]}}`
	res, err := m.InstallFromZip(writeTestZip(t, []zipEntry{
		{name: "plugin.json", mode: 0o644, body: v2},
		{name: "run.sh", mode: 0o644, body: "#!/bin/sh\n"},
	}), false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.NeedsConfirm || res.CurrentVersion != "1.2.0" || res.NewVersion != "2.0.0" {
		t.Fatalf("result = %+v, want a replace question for 1.2.0 -> 2.0.0", res)
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "zippy", "old.txt")); err != nil {
		t.Fatalf("asking must not touch the old folder: %v", err)
	}
	res, err = m.InstallFromZip(writeTestZip(t, []zipEntry{
		{name: "plugin.json", mode: 0o644, body: v2},
		{name: "run.sh", mode: 0o644, body: "#!/bin/sh\n"},
	}), true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Replaced || res.Version != "2.0.0" {
		t.Fatalf("result = %+v, want a replace with 2.0.0", res)
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "zippy", "old.txt")); !os.IsNotExist(err) {
		t.Fatal("replace merged into the old folder instead of replacing it")
	}
	p, ok := m.Plugin("zippy")
	if !ok || p.Manifest.Version != "2.0.0" {
		t.Fatalf("plugin = %+v, want version 2.0.0", p)
	}
}

// An existing folder whose manifest doesn't load still asks before it is
// replaced: it is some installed (maybe incompatible) plugin, or at least
// not ours to delete sight unseen.
func TestInstallExistingUnreadableFolderAsksFirst(t *testing.T) {
	t.Parallel()
	m, _, pluginsDir := zipManager(t)
	dir := filepath.Join(pluginsDir, "zippy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte("half-edited{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "precious.txt"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	v2 := `{"id":"zippy","name":"Zippy","version":"2.0.0","manifestVersion":1,` +
		`"protocol":{"versions":["1.0"]},"store":"zippy","exec":"run.sh",` +
		`"capabilities":["library"],"permissions":{"network":[],"read":[],"write":[]}}`
	zipv2 := func() string {
		return writeTestZip(t, []zipEntry{
			{name: "plugin.json", mode: 0o644, body: v2},
			{name: "run.sh", mode: 0o644, body: "#!/bin/sh\n"},
		})
	}
	res, err := m.InstallFromZip(zipv2(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.NeedsConfirm {
		t.Fatalf("result = %+v, want a replace question for the unreadable folder", res)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "precious.txt")); err != nil || string(got) != "mine" {
		t.Fatal("asking must not touch the existing folder")
	}
	res, err = m.InstallFromZip(zipv2(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Replaced || res.Version != "2.0.0" {
		t.Fatalf("result = %+v, want a replace with 2.0.0", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "precious.txt")); !os.IsNotExist(err) {
		t.Fatal("confirmed replace must not merge into the old folder")
	}
}

// A data zip waits for approval instead of applying at once, and approval
// works for a data plugin without a theme.
func TestInstallDataZipNeedsApproval(t *testing.T) {
	t.Parallel()
	m, _, _ := zipManager(t)
	entries := []zipEntry{
		{name: "plugin.json", mode: 0o644, body: `{"id":"datatweak","name":"Data","version":"1.0.0","manifestVersion":1,"kind":"data","description":"test"}`},
		{name: "tweaks.json", mode: 0o644, body: `{"tweaks":[{"id":"x","kind":"recommended","title":"T","reason":"R","env":{"LD_PRELOAD":"evil"}}]}`},
	}
	res, err := m.InstallFromZip(writeTestZip(t, entries), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "datatweak" {
		t.Fatalf("result = %+v, want datatweak", res)
	}
	p, ok := m.Plugin("datatweak")
	if !ok || p.Status != StatusUnapproved {
		t.Fatalf("plugin = %+v, want unapproved (never auto-approved)", p)
	}
	if err := m.Approve("datatweak"); err != nil {
		t.Fatalf("approving a themeless data plugin: %v", err)
	}
	if p, _ := m.Plugin("datatweak"); p.Status != StatusReady {
		t.Fatalf("plugin = %+v, want ready after approval", p)
	}
}

func TestInstallBundledRefused(t *testing.T) {
	t.Parallel()
	m, _, pluginsDir := zipManager(t)
	manifest := `{"id":"steam","name":"Steam","version":"9.9.0","manifestVersion":1,` +
		`"protocol":{"versions":["1.0"]},"store":"steam","exec":"run.sh",` +
		`"capabilities":["library"],"permissions":{"network":[],"read":[],"write":[]}}`
	if _, err := m.InstallFromZip(writeTestZip(t, []zipEntry{
		{name: "plugin.json", mode: 0o644, body: manifest},
		{name: "run.sh", mode: 0o644, body: "#!/bin/sh\n"},
	}), true); err == nil {
		t.Fatal("bundled replace should fail")
	}
	if _, err := os.Lstat(filepath.Join(pluginsDir, "steam")); !os.IsNotExist(err) {
		t.Fatal("refused install wrote the plugins folder")
	}
}

// deflateBytes compresses body with raw deflate, for crafted archives whose
// declared sizes don't match their real bytes.
func deflateBytes(t *testing.T, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	fw, err := flate.NewWriter(&buf, flate.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// writeRawZip builds a zip with caller-supplied headers and raw (already
// compressed) bodies, so a test can lie about an entry's declared size the
// way a zip bomb does.
func writeRawZip(t *testing.T, honest []zipEntry, raws []zip.FileHeader, bodies [][]byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plugin.zip")
	w, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(w)
	for _, e := range honest {
		fh := &zip.FileHeader{Name: e.name, Method: zip.Store}
		fh.SetMode(e.mode)
		fw, err := zw.CreateHeader(fh)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	for i, fh := range raws {
		fw, err := zw.CreateRaw(&fh)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(bodies[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// More entries than the limit are refused without writing anything.
func TestInstallTooManyEntriesRefused(t *testing.T) {
	// No t.Parallel: shrinks the package limit for the duration.
	oldMax := installMaxFiles
	installMaxFiles = 2
	t.Cleanup(func() { installMaxFiles = oldMax })
	m, _, pluginsDir := zipManager(t)
	entries := []zipEntry{
		{name: "plugin.json", mode: 0o644, body: zipManifest},
		{name: "run.sh", mode: 0o644, body: "#!/bin/sh\n"},
		{name: "extra.txt", mode: 0o644, body: "x"},
	}
	if _, err := m.InstallFromZip(writeTestZip(t, entries), false); err == nil {
		t.Fatal("too many entries should fail")
	}
	if _, err := os.Lstat(filepath.Join(pluginsDir, "zippy")); !os.IsNotExist(err) {
		t.Fatal("refused install wrote the plugins folder")
	}
}

// A zip64 entry declaring 2^64-1 with a real 2 MB deflate stream must be
// refused: the old int64 total wrapped negative and each entry was cut
// silently, installing truncated garbage.
func TestInstallZipBombDeclaredSizeOverflowRefused(t *testing.T) {
	// No t.Parallel: shrinks the package limit for the duration.
	oldMax := installMaxBytes
	installMaxBytes = 1 << 20
	t.Cleanup(func() { installMaxBytes = oldMax })
	m, _, pluginsDir := zipManager(t)
	comp := deflateBytes(t, make([]byte, 2<<20))
	raw := zip.FileHeader{
		Name:               "bomb.bin",
		Method:             zip.Deflate,
		CompressedSize64:   uint64(len(comp)),
		UncompressedSize64: ^uint64(0),
	}
	path := writeRawZip(t, validZipEntries(), []zip.FileHeader{raw}, [][]byte{comp})
	if _, err := m.InstallFromZip(path, false); err == nil {
		t.Fatal("declared 2^64-1 size should fail")
	}
	if _, err := os.Lstat(filepath.Join(pluginsDir, "zippy")); !os.IsNotExist(err) {
		t.Fatal("refused install wrote the plugins folder")
	}
}

// An entry lying about a small declared size with real bytes past the shared
// budget must be refused too, not cut silently.
func TestInstallZipBombLiedSizeOverBudgetRefused(t *testing.T) {
	// No t.Parallel: shrinks the package limit for the duration.
	oldMax := installMaxBytes
	installMaxBytes = 1 << 10
	t.Cleanup(func() { installMaxBytes = oldMax })
	m, _, pluginsDir := zipManager(t)
	comp := deflateBytes(t, make([]byte, 4<<10))
	raw := zip.FileHeader{
		Name:               "bomb.bin",
		Method:             zip.Deflate,
		CompressedSize64:   uint64(len(comp)),
		UncompressedSize64: 100, // lies: the stream holds 4 KB
	}
	path := writeRawZip(t, validZipEntries(), []zip.FileHeader{raw}, [][]byte{comp})
	if _, err := m.InstallFromZip(path, false); err == nil {
		t.Fatal("real bytes over the shared budget should fail")
	}
	if _, err := os.Lstat(filepath.Join(pluginsDir, "zippy")); !os.IsNotExist(err) {
		t.Fatal("refused install wrote the plugins folder")
	}
}
