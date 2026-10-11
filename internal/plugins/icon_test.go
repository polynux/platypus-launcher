package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><circle cx="8" cy="8" r="7"/></svg>`

var testPNG = append([]byte("\x89PNG\r\n\x1a\n"), []byte("fake-png-body")...)

// writeLogoPlugin creates pluginsDir/id with an executable stub, plugin.json
// (extra overrides fields) and the given extra files.
func writeLogoPlugin(t *testing.T, pluginsDir, id string, extra map[string]any, files map[string][]byte) string {
	t.Helper()
	dir := filepath.Join(pluginsDir, id)
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin/run"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m := map[string]any{
		"id":       id,
		"name":     "Logo Test",
		"version":  "1.0.0",
		"protocol": map[string]any{"versions": []string{"1.0"}, "transport": "jsonrpc-stdio"},
		"store":    "logotest",
		"exec":     "bin/run",
	}
	for k, v := range extra {
		m[k] = v
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestIconManifestValidation(t *testing.T) {
	for name, icon := range map[string]string{
		"escape":   "../evil.svg",
		"absolute": "/etc/icon.svg",
		"gif":      "icon.gif",
		"jpg":      "icon.jpg",
		"no ext":   "icon",
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			writeLogoPlugin(t, e.plugins, "logotest", map[string]any{"icon": icon}, nil)
			p := e.discover(t)["logotest"]
			if p.Status != StatusInvalid || !strings.Contains(p.Problem, "icon") {
				t.Fatalf("want invalid icon, got %+v", p)
			}
		})
	}
}

func TestIconPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "good.svg"), []byte(testSVG), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "good.png"), testPNG, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "declared.xml"), []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"></svg>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "text.svg"), []byte("<html><script>alert(1)</script></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "text.txt"), []byte("just text"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "empty.svg"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	big := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, MaxIconBytes)...)
	if err := os.WriteFile(filepath.Join(dir, "big.png"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.svg")
	if err := os.WriteFile(outside, []byte(testSVG), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.svg")); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name  string
		icon  string
		ctype string
	}{
		{"svg", "good.svg", IconSVG},
		{"png", "good.png", IconPNG},
		{"xml declaration", "declared.xml", IconSVG},
		// The extension is only a manifest hint; the content decides.
		{"svg content under txt name", "declared.xml", IconSVG},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, ctype, err := (Manifest{Icon: c.icon}).IconPath(dir)
			if err != nil || ctype != c.ctype {
				t.Fatalf("IconPath(%q) = %q, %v", c.icon, ctype, err)
			}
		})
	}

	for _, c := range []struct{ name, icon string }{
		{"missing", "nope.svg"},
		{"escape", "../outside.svg"},
		{"absolute", "/etc/hostname"},
		{"symlink escape", "link.svg"},
		{"oversize", "big.png"},
		{"html as svg", "text.svg"},
		{"plain text", "text.txt"},
		{"empty", "empty.svg"},
		{"directory", "sub"},
		{"none", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if _, _, err := (Manifest{Icon: c.icon}).IconPath(dir); err == nil {
				t.Fatalf("IconPath(%q) succeeded", c.icon)
			}
		})
	}
}

func TestIconURL(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		e := newEnv(t)
		writeLogoPlugin(t, e.plugins, "logotest", nil, nil)
		e.discover(t)
		if got := e.m.IconURL("logotest"); got != "" {
			t.Fatalf("IconURL = %q", got)
		}
	})

	t.Run("missing file means no URL", func(t *testing.T) {
		e := newEnv(t)
		writeLogoPlugin(t, e.plugins, "logotest", map[string]any{"icon": "icon.svg"}, nil)
		e.discover(t)
		if got := e.m.IconURL("logotest"); got != "" {
			t.Fatalf("IconURL = %q", got)
		}
		if _, _, ok := e.m.IconFile("logotest"); ok {
			t.Fatal("IconFile found a missing logo")
		}
	})

	t.Run("valid logo is served", func(t *testing.T) {
		e := newEnv(t)
		writeLogoPlugin(t, e.plugins, "logotest", map[string]any{"icon": "icon.svg"}, map[string][]byte{
			"icon.svg": []byte(testSVG),
		})
		p := e.discover(t)["logotest"]
		if p.Status != StatusUnapproved {
			t.Fatalf("plugin: %+v", p)
		}
		u := e.m.IconURL("logotest")
		if !strings.HasPrefix(u, IconURLPrefix+"logotest") {
			t.Fatalf("IconURL = %q", u)
		}
		path, ctype, ok := e.m.IconFile("logotest")
		if !ok || ctype != IconSVG {
			t.Fatalf("IconFile = %q, %q, %v", path, ctype, ok)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("logo path: %v", err)
		}
		// The hash versions the URL, so a replaced logo is a new address.
		if !strings.Contains(u, "?v=") {
			t.Fatalf("unversioned IconURL = %q", u)
		}
	})

	t.Run("wrong type means no URL", func(t *testing.T) {
		e := newEnv(t)
		writeLogoPlugin(t, e.plugins, "logotest", map[string]any{"icon": "icon.svg"}, map[string][]byte{
			"icon.svg": []byte("not an image"),
		})
		e.discover(t)
		if got := e.m.IconURL("logotest"); got != "" {
			t.Fatalf("IconURL = %q", got)
		}
	})

	t.Run("oversize means no URL", func(t *testing.T) {
		e := newEnv(t)
		big := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, MaxIconBytes)...)
		writeLogoPlugin(t, e.plugins, "logotest", map[string]any{"icon": "icon.png"}, map[string][]byte{
			"icon.png": big,
		})
		e.discover(t)
		if got := e.m.IconURL("logotest"); got != "" {
			t.Fatalf("IconURL = %q", got)
		}
	})

	t.Run("unknown plugin", func(t *testing.T) {
		e := newEnv(t)
		e.discover(t)
		if got := e.m.IconURL("nope"); got != "" {
			t.Fatalf("IconURL = %q", got)
		}
	})
}
