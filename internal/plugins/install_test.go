package plugins

// Zip install (issue #147): a valid zip lands waiting for approval, and every
// unsafe archive is refused without writing outside a temp folder.

import (
	"archive/zip"
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
