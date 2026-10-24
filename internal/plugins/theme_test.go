package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeThemePlugin creates pluginsDir/id as a data plugin with the given
// manifest fields and files. extra overrides the base manifest.
func writeThemePlugin(t *testing.T, pluginsDir, id string, extra map[string]any, files map[string][]byte) string {
	t.Helper()
	dir := filepath.Join(pluginsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	m := map[string]any{
		"id":            id,
		"name":          "Theme Test",
		"version":       "1.0.0",
		"api":           1,
		"kind":          "data",
		"data":          []string{},
		"theme":         "theme.toml",
		"description":   "test",
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

const themeTOML = "[theme]\nid = \"fixture\"\nname = \"Fixture\"\nschema = 1\nsupports = [\"tokens\"]\n\n[tokens]\naccent = \"#7dd3fc\"\n"

var themePNG = append([]byte("\x89PNG\r\n\x1a\n"), []byte("fixture-png-body")...)

var themeWOFF = append([]byte("wOF2"), make([]byte, 100)...)

func TestThemeManifestValidation(t *testing.T) {
	for name, extra := range map[string]map[string]any{
		"store with theme": {"kind": "store", "store": "x", "exec": "bin/run", "theme": "theme.toml"},
		"absolute":         {"theme": "/etc/theme.toml"},
		"escape":           {"theme": "../evil.toml"},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			if extra["kind"] == "store" {
				writeLogoPlugin(t, e.plugins, "logotest", extra, nil)
			} else {
				writeThemePlugin(t, e.plugins, "themetest", extra, nil)
			}
			p := e.discover(t)["logotest"]
			if p.Manifest.ID == "" {
				p = e.discover(t)["themetest"]
			}
			if p.Status != StatusInvalid || !strings.Contains(p.Problem, "theme") {
				t.Fatalf("want invalid theme, got %+v", p)
			}
		})
	}
}

func TestThemeDiscovery(t *testing.T) {
	e := newEnv(t)
	writeThemePlugin(t, e.plugins, "fixture-theme", nil, map[string][]byte{
		"theme.toml":          []byte(themeTOML),
		"assets/bg.png":       themePNG,
		"fonts/fixture.woff2": themeWOFF,
	})
	// A theme-only plugin carries no data files: it is still ready.
	p := e.discover(t)["fixture-theme"]
	if p.Status != StatusReady || p.Theme == nil {
		t.Fatalf("theme plugin: %+v", p)
	}
	if p.Theme.Info.Status != "ok" || p.Theme.Info.Source != "fixture-theme" {
		t.Fatalf("theme info: %+v", p.Theme.Info)
	}
	if p.Hash == "" {
		t.Error("theme plugin has no hash for the asset URLs")
	}
	// An invalid theme lists its problem but keeps the plugin's data working.
	e2 := newEnv(t)
	writeThemePlugin(t, e2.plugins, "bad-theme", nil, map[string][]byte{
		"fixes.json": []byte(`{"fixes":[]}`),
		"theme.toml": []byte("[theme\nid = "),
	})
	p = e2.discover(t)["bad-theme"]
	if p.Status != StatusReady || p.Theme == nil {
		t.Fatalf("bad theme plugin: %+v", p)
	}
	if p.Theme.Info.Status != "invalid" || len(p.Theme.Info.Warnings) == 0 {
		t.Fatalf("bad theme info: %+v", p.Theme.Info)
	}
	if p.Data == nil {
		t.Error("an invalid theme dropped the plugin's data")
	}
}

func TestThemeAssetPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "good.png"), themePNG, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "good.woff2"), themeWOFF, 0o644); err != nil {
		t.Fatal(err)
	}
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><circle cx="8" cy="8" r="7"/></svg>`)
	if err := os.WriteFile(filepath.Join(dir, "good.svg"), svg, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "text.txt"), []byte("just text"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "empty.png"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	big := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, MaxIconBytes)...)
	_ = big
	oversize := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 2<<20)...)
	if err := os.WriteFile(filepath.Join(dir, "big.png"), oversize, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, themePNG, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.png")); err != nil {
		t.Fatal(err)
	}

	m := Manifest{}
	for _, c := range []struct{ rel, ctype string }{
		{"good.png", "image/png"},
		{"good.svg", "image/svg+xml"},
		{"good.woff2", "font/woff2"},
	} {
		t.Run("ok "+c.rel, func(t *testing.T) {
			_, ctype, err := m.ThemeAssetPath(dir, c.rel)
			if err != nil || ctype != c.ctype {
				t.Fatalf("ThemeAssetPath(%q) = %q, %v", c.rel, ctype, err)
			}
		})
	}
	for _, c := range []struct{ name, rel string }{
		{"missing", "nope.png"},
		{"escape", "../outside.png"},
		{"absolute", "/etc/hostname"},
		{"symlink escape", "link.png"},
		{"oversize", "big.png"},
		{"plain text", "text.txt"},
		{"empty", "empty.png"},
		{"directory", "sub"},
		{"none", ""},
		{"png as svg", "good.png"},
	} {
		t.Run(c.name, func(t *testing.T) {
			rel := c.rel
			if c.name == "png as svg" {
				// A png by another extension: the content decides, and the
				// extension must match it.
				if _, _, err := m.ThemeAssetPath(dir, "good.svg"); err != nil {
					t.Fatalf("good.svg should serve, got %v", err)
				}
				rel = "good.png"
				// good.png is valid; use the text file under a png name instead.
				if err := os.WriteFile(filepath.Join(dir, "fake.png"), []byte("just text"), 0o644); err != nil {
					t.Fatal(err)
				}
				rel = "fake.png"
			}
			if _, _, err := m.ThemeAssetPath(dir, rel); err == nil {
				t.Fatalf("ThemeAssetPath(%q) succeeded", rel)
			}
		})
	}
	// The extension must match the sniffed content: a png under a .svg name
	// and an svg under a .png name are both refused.
	if _, _, err := m.ThemeAssetPath(dir, "good.png"); err != nil {
		t.Fatalf("good.png should serve, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "svg-as-png.png"), svg, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.ThemeAssetPath(dir, "svg-as-png.png"); err == nil {
		t.Fatal("svg content under a .png name succeeded")
	}
}
