package app

// The local-art capability (protocol 1.15, issue #44): the core serves a
// plugin's file:// art at /platypus-art/<plugin-id>/<base64url(path)> with
// Cache-Control: max-age=86400 and nosniff, only inside the plugin's
// declared permissions.read folders.

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/plugins"
	"platypus/internal/settings"
)

func artTestManager(t *testing.T, artdir string) *plugins.Manager {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "plugins", "echo")
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,` +
		`"protocol":{"versions":["1.15"]},"store":"echo","exec":"bin/run.sh",` +
		`"capabilities":["library","launch","local-art"],` +
		`"permissions":{"network":[],"read":[` + quote(t, artdir) + `],"write":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	mgr := plugins.NewManager(plugins.Options{
		Dir: filepath.Join(root, "plugins"), LogDir: filepath.Join(root, "logs"),
		DataDir: filepath.Join(root, "data"), Settings: st,
	})
	t.Cleanup(mgr.Shutdown)
	if _, err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}
	if p, ok := mgr.Plugin("echo"); !ok {
		t.Fatal("echo not discovered")
	} else if p.Status != plugins.StatusUnapproved && p.Status != plugins.StatusReady {
		t.Fatalf("echo status = %q", p.Status)
	}
	return mgr
}

func TestPluginGameArtServesInsideReadFolders(t *testing.T) {
	t.Parallel()
	artdir := t.TempDir()
	art := filepath.Join(artdir, "cover.png")
	if err := os.WriteFile(art, []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{plugins: artTestManager(t, artdir)}
	encoded := base64.RawURLEncoding.EncodeToString([]byte(art))
	req := httptest.NewRequest("GET", "/platypus-art/echo/"+encoded, nil)
	rec := httptest.NewRecorder()
	PluginGameArt(a, http.NotFoundHandler()).ServeHTTP(rec, req)
	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "max-age=86400" {
		t.Fatalf("Cache-Control = %q", cc)
	}
	if ns := res.Header.Get("X-Content-Type-Options"); ns != "nosniff" {
		t.Fatalf("nosniff = %q", ns)
	}
}

func TestPluginGameArtServesLegacyRelativeSnapshotPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rel := filepath.Join("appcache", "librarycache", "440", "cover.jpg")
	art := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(art), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(art, []byte("legacy-art"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{plugins: artTestManager(t, root)}
	req := httptest.NewRequest("GET", "/platypus-art/echo/"+filepath.ToSlash(rel), nil)
	rec := httptest.NewRecorder()
	PluginGameArt(a, http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "legacy-art" {
		t.Fatalf("legacy art response = %d %q", rec.Code, rec.Body.String())
	}
	for _, bad := range []string{"../secret.png", "/etc/passwd"} {
		req := httptest.NewRequest("GET", "/platypus-art/echo/"+bad, nil)
		rec := httptest.NewRecorder()
		PluginGameArt(a, http.NotFoundHandler()).ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("unsafe legacy path %q status = %d, want 404", bad, rec.Code)
		}
	}
}

func TestPluginGameArtRefusesOutsideReadFolders(t *testing.T) {
	t.Parallel()
	a := &App{plugins: artTestManager(t, t.TempDir())}
	for _, path := range []string{"/etc/passwd", "/tmp/evil.bmp", "/tmp/evil.txt"} {
		encoded := base64.RawURLEncoding.EncodeToString([]byte(path))
		req := httptest.NewRequest("GET", "/platypus-art/echo/"+encoded, nil)
		rec := httptest.NewRecorder()
		PluginGameArt(a, http.NotFoundHandler()).ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", path, rec.Code)
		}
	}
	// Unknown plugins get a 404 (the path decodes but no plugin owns it);
	// the logo prefix falls through to its handler. The bundled steam
	// plugin's art rides this handler too (issue #54), so a bad steam
	// address 404s here instead of falling through.
	unknown := "/platypus-art/nope/" + base64.RawURLEncoding.EncodeToString([]byte("/tmp/x.png"))
	req := httptest.NewRequest("GET", unknown, nil)
	rec := httptest.NewRecorder()
	PluginGameArt(a, http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("%s: status = %d, want 404", unknown, rec.Code)
	}
	for _, target := range []string{"/platypus-art/steam/x", "/platypus-art/steam/" + base64.RawURLEncoding.EncodeToString([]byte("/tmp/x.png"))} {
		req := httptest.NewRequest("GET", target, nil)
		rec := httptest.NewRecorder()
		PluginGameArt(a, http.NotFoundHandler()).ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", target, rec.Code)
		}
	}
	for _, target := range []string{"/platypus-art/plugin/echo", "/platypus-art/noslash"} {
		req := httptest.NewRequest("GET", target, nil)
		rec := httptest.NewRecorder()
		PluginGameArt(a, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		})).ServeHTTP(rec, req)
		if rec.Code != http.StatusTeapot {
			t.Fatalf("%s: status = %d, want fallthrough", target, rec.Code)
		}
	}
}

// A plugin's own data folder is always servable, without a read permission.
func TestPluginGameArtServesFromTheDataFolder(t *testing.T) {
	t.Parallel()
	mgr := artTestManager(t, t.TempDir())
	dataDir := mgr.DataDirOf("echo")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	art := filepath.Join(dataDir, "cover.png")
	if err := os.WriteFile(art, []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{plugins: mgr}
	req := httptest.NewRequest("GET", "/platypus-art/echo/"+base64.RawURLEncoding.EncodeToString([]byte(art)), nil)
	rec := httptest.NewRecorder()
	PluginGameArt(a, http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// The art handlers run as one chain in the core (cmd/platypus-core): custom
// art, plugin local art, plugin logos and theme assets each reach their own
// handler. Custom art paths (/platypus-art/<hash>/cover.png) have the same
// shape as local-art ones and were swallowed by PluginGameArt once (issue #58).
func TestArtHandlerChainServesEveryKind(t *testing.T) {
	t.Parallel()
	a := customApp(t)
	artdir := t.TempDir()
	local := filepath.Join(artdir, "cover.png")
	if err := os.WriteFile(local, []byte("\x89PNG\r\n\x1a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.plugins = artTestManager(t, artdir)
	c, err := a.SetCustomArt("g1", "cover", writePNG(t))
	if err != nil {
		t.Fatal(err)
	}
	chain := PluginArt(a, ThemeArt(a, PluginGameArt(a, CustomArt(a, http.NotFoundHandler()))))
	for name, target := range map[string]string{
		"custom art":       strings.SplitN(c.Art["cover"], "?", 2)[0],
		"plugin local art": "/platypus-art/echo/" + base64.RawURLEncoding.EncodeToString([]byte(local)),
	} {
		rec := httptest.NewRecorder()
		chain.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s (%s): status = %d, want 200", name, target, rec.Code)
		}
	}
}
