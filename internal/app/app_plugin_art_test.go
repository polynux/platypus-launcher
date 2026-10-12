package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/plugins"
	"platypus/internal/settings"
)

const testIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><circle cx="8" cy="8" r="7"/></svg>`

// iconApp installs a plugin with a logo and returns the app serving it.
func iconApp(t *testing.T, manifest, iconName, iconBody string) (*App, *plugins.Manager) {
	t.Helper()
	root := t.TempDir()
	trust, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), root)
	if err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	dir := filepath.Join(pluginsDir, "echo")
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin/run"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if iconName != "" {
		if err := os.WriteFile(filepath.Join(dir, iconName), []byte(iconBody), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := plugins.NewManager(plugins.Options{
		Dir:      pluginsDir,
		LogDir:   filepath.Join(root, "logs"),
		DataDir:  filepath.Join(root, "data"),
		Settings: trust,
		Home:     root,
	})
	t.Cleanup(m.Shutdown)
	if _, err := m.Discover(); err != nil {
		t.Fatal(err)
	}
	return &App{plugins: m, settings: trust, optHome: root}, m
}

const iconManifest = `{"id":"echo","name":"Echo","version":"1.0.0","api":1,"store":"echo","exec":"bin/run","icon":"icon.svg"}`

func TestPluginLogoIsServedWithLockedHeaders(t *testing.T) {
	t.Parallel()
	a, _ := iconApp(t, iconManifest, "icon.svg", testIconSVG)
	list := a.ListStorePlugins()
	if len(list) != 1 || list[0].IconURL == "" {
		t.Fatalf("StorePlugin.IconURL = %+v", list)
	}
	path := strings.SplitN(list[0].IconURL, "?", 2)[0]

	h := PluginArt(a, http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("logo request: %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "default-src 'none'" {
		t.Errorf("Content-Security-Policy = %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Errorf("Cache-Control = %q", got)
	}
	if rec.Body.String() != testIconSVG {
		t.Errorf("logo body = %q", rec.Body.String())
	}
}

func TestPluginLogoFallthroughAndRefusals(t *testing.T) {
	t.Parallel()
	a, _ := iconApp(t, iconManifest, "icon.svg", testIconSVG)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("X-Next", "1") })
	h := PluginArt(a, next)

	// Anything outside the logo route passes through.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/index.js", nil))
	if rec.Header().Get("X-Next") != "1" {
		t.Error("a request that is not a logo did not reach the next handler")
	}

	// Unknown plugins, deeper paths and bad ids are 404, never a file.
	for _, path := range []string{
		plugins.IconURLPrefix + "nope",
		plugins.IconURLPrefix + "echo/extra",
		plugins.IconURLPrefix + "../customization.json",
		plugins.IconURLPrefix + "",
		plugins.IconURLPrefix + "ECHO",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, rec.Code)
		}
	}
}

func TestPluginLogoMissingMeansNoURL(t *testing.T) {
	t.Parallel()
	// No icon in the manifest: no URL, and the plugin is otherwise fine.
	a, _ := iconApp(t, `{"id":"echo","name":"Echo","version":"1.0.0","api":1,"store":"echo","exec":"bin/run"}`, "", "")
	list := a.ListStorePlugins()
	if len(list) != 1 || list[0].IconURL != "" || list[0].Status == "invalid" {
		t.Fatalf("no-icon plugin: %+v", list)
	}
	rec := httptest.NewRecorder()
	PluginArt(a, http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, plugins.IconURLPrefix+"echo", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing logo: %d, want 404", rec.Code)
	}

	// A manifest naming a file that isn't there: still a valid plugin, no URL.
	a, _ = iconApp(t, iconManifest, "", "")
	list = a.ListStorePlugins()
	if len(list) != 1 || list[0].IconURL != "" {
		t.Fatalf("missing file: %+v", list)
	}
	if p := list[0]; p.Status == "invalid" {
		t.Fatalf("a missing logo broke the plugin: %+v", p)
	}
}
