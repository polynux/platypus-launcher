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
	"platypus/internal/themes"
)

var pluginThemeTOML = "[theme]\nid = \"fixture\"\nname = \"Fixture\"\nschema = 1\ncatalog = \"1.0\"\nsupports = [\"tokens\", \"variants\", \"layouts\"]\n\n" +
	"[tokens]\nground = \"#0b1118\"\naccent = \"#7dd3fc\"\nfont-sans = '\"Fixture Sans\", system-ui, sans-serif'\n\n" +
	"[fonts]\n\"Fixture Sans\" = \"fonts/fixture.woff2\"\n\n" +
	"[[wash]]\nat = \"10% 10%\"\nsize = \"100px 100px\"\ncolor = \"#000000\"\nfade = \"50%\"\nimage = \"assets/bg.png\"\n\n" +
	"[variants]\n\"settings.presentation\" = \"sheet\"\n"

var pluginThemePNG = append([]byte("\x89PNG\r\n\x1a\n"), []byte("fixture-png-body")...)

var pluginThemeWOFF = append([]byte("wOF2"), make([]byte, 100)...)

// themeApp installs a data plugin carrying a theme and returns the app serving
// it. files holds the plugin folder's extra files beyond plugin.json.
func themeApp(t *testing.T, id, themeID, toml string, files map[string][]byte) (*App, *plugins.Manager) {
	t.Helper()
	root := t.TempDir()
	trust, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), root)
	if err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	dir := filepath.Join(pluginsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":` + quoteJSON(id) + `,"name":"Fixture theme","version":"1.0.0","api":1,"kind":"data","description":"test","theme":"theme.toml"}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if toml != "" {
		if err := os.WriteFile(filepath.Join(dir, "theme.toml"), []byte(toml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_ = themeID
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"),
		DataDir: filepath.Join(root, "data"), Settings: trust, Home: root,
	})
	t.Cleanup(m.Shutdown)
	if _, err := m.Discover(); err != nil {
		t.Fatal(err)
	}
	return &App{plugins: m, settings: trust, optHome: root}, m
}

func quoteJSON(s string) string {
	return `"` + s + `"`
}

// A plugin theme appears in the picker with its source, resolves with art
// URLs, and picks like a built-in.
func TestPluginThemeListsResolvesAndPicks(t *testing.T) {
	a, _ := themeApp(t, "fixture-theme", "fixture", pluginThemeTOML, map[string][]byte{
		"assets/bg.png":       pluginThemePNG,
		"fonts/fixture.woff2": pluginThemeWOFF,
	})
	list := a.ListThemes()
	var found *struct {
		id, source string
		status     string
	}
	for _, th := range list {
		if th.ID == "fixture" {
			th := th
			found = &struct {
				id, source string
				status     string
			}{th.ID, th.Source, th.Status}
			if th.Source != "fixture-theme" {
				t.Fatalf("theme source = %q, want the plugin id", th.Source)
			}
			if th.Warnings == nil || th.Supports == nil {
				t.Fatalf("nil lists would reach the UI as null: %+v", th)
			}
		}
	}
	if found == nil || found.status != themes.StatusOK {
		t.Fatalf("plugin theme missing from ListThemes: %+v", list)
	}
	got := a.GetTheme("fixture")
	if got.ID != "fixture" {
		t.Fatalf("GetTheme(fixture) = %q", got.ID)
	}
	fontURL, ok := got.Fonts["Fixture Sans"]
	if !ok || !strings.HasPrefix(fontURL, themes.ThemeURLPrefix+"fixture-theme/fonts/fixture.woff2") {
		t.Fatalf("fonts = %v, want the art URL", got.Fonts)
	}
	if len(got.Wash) != 1 || !strings.HasPrefix(got.Wash[0].Image, themes.ThemeURLPrefix+"fixture-theme/assets/bg.png") {
		t.Fatalf("wash = %+v, want the art URL", got.Wash)
	}
	if _, err := a.SetTheme("fixture"); err != nil {
		t.Fatalf("SetTheme(fixture) = %v", err)
	}
	if got := a.GetTheme(""); got.ID != "fixture" {
		t.Fatalf("GetTheme(\"\") after pick = %q", got.ID)
	}
	if layout := a.GetThemeLayout("dashboard"); layout == nil || len(layout.Regions) != 4 || layout.Regions[0].Name != "top" {
		// The fixture ships no dashboard layout: the built-in arrangement stays.
		t.Fatalf("unexpected layout: %+v", layout)
	}
	if _, err := a.SetTheme(""); err != nil {
		t.Fatalf("SetTheme(\"\") = %v", err)
	}
}

// Resolving the same plugin theme twice keeps its asset URLs both times.
// withThemeURLs must not write into the manager's cached theme: the resolved
// wash shares its backing array, and mapping an already absolute URL fails
// the inside-the-folder check, silently dropping the image (D9).
func TestPluginThemeRepeatedResolvesKeepAssets(t *testing.T) {
	a, _ := themeApp(t, "fixture-theme", "fixture", pluginThemeTOML, map[string][]byte{
		"assets/bg.png":       pluginThemePNG,
		"fonts/fixture.woff2": pluginThemeWOFF,
	})
	for i := 0; i < 3; i++ {
		got := a.GetTheme("fixture")
		if len(got.Wash) != 1 || !strings.HasPrefix(got.Wash[0].Image, themes.ThemeURLPrefix+"fixture-theme/assets/bg.png") {
			t.Fatalf("resolve %d: wash = %+v, want the art URL", i, got.Wash)
		}
		if _, ok := got.Fonts["Fixture Sans"]; !ok {
			t.Fatalf("resolve %d: fonts = %v, want the shipped family", i, got.Fonts)
		}
	}
	if _, err := a.SetTheme("fixture"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if got := a.GetTheme(""); got.Wash[0].Image == "" {
			t.Fatalf("picked resolve %d lost the wash image: %+v", i, got.Wash)
		}
	}
}

// An invalid plugin theme is listed with its problem and can't be picked,
// exactly like a broken built-in file.
func TestPluginThemeInvalidListsAndRefuses(t *testing.T) {
	a, _ := themeApp(t, "bad-theme", "bad", "[theme\nid = ", nil)
	var info *struct {
		status string
		n      int
	}
	for _, th := range a.ListThemes() {
		if th.Source == "bad-theme" {
			th := th
			info = &struct {
				status string
				n      int
			}{th.Status, len(th.Warnings)}
		}
	}
	if info == nil || info.status != themes.StatusInvalid || info.n == 0 {
		t.Fatalf("invalid plugin theme missing: %+v", a.ListThemes())
	}
	if _, err := a.SetTheme("bad"); err == nil {
		t.Fatal("SetTheme(bad) succeeded, want an error")
	}
}

// A plugin theme reusing a built-in id is listed as invalid, so two entries
// never share a pickable id. The built-in keeps resolving.
func TestPluginThemeCollidingIdListsInvalid(t *testing.T) {
	toml := "[theme]\nid = \"cool\"\nname = \"Copy\"\nschema = 1\nsupports = [\"tokens\"]\n"
	a, _ := themeApp(t, "copycat", "cool", toml, nil)
	var dup int
	for _, th := range a.ListThemes() {
		if th.ID == "cool" && th.Source == "copycat" {
			dup++
			if th.Status != themes.StatusInvalid {
				t.Fatalf("colliding theme status = %q, want invalid", th.Status)
			}
		}
	}
	if dup != 1 {
		t.Fatalf("colliding theme missing: %+v", a.ListThemes())
	}
	if got := a.GetTheme("cool"); got.ID != "cool" || len(got.Fonts) != 0 {
		t.Fatalf("colliding GetTheme = %+v", got)
	}
}

// Theme assets ride the token-protected art route with locked headers.
func TestThemeArtServesAndRefuses(t *testing.T) {
	a, _ := themeApp(t, "fixture-theme", "fixture", pluginThemeTOML, map[string][]byte{
		"assets/bg.png":       pluginThemePNG,
		"fonts/fixture.woff2": pluginThemeWOFF,
	})
	got := a.GetTheme("fixture")
	fontURL := strings.SplitN(got.Fonts["Fixture Sans"], "?", 2)[0]
	washURL := strings.SplitN(got.Wash[0].Image, "?", 2)[0]

	h := ThemeArt(a, http.NotFoundHandler())
	for name, target := range map[string]string{"font": fontURL, "wash": washURL} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s: %d, want 200", name, target, rec.Code)
		}
		if ns := rec.Header().Get("X-Content-Type-Options"); ns != "nosniff" {
			t.Errorf("%s nosniff = %q", name, ns)
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
			t.Errorf("%s Cache-Control = %q", name, cc)
		}
		if csp := rec.Header().Get("Content-Security-Policy"); csp != "default-src 'none'" {
			t.Errorf("%s CSP = %q", name, csp)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fontURL, nil))
	if rec.Body.String() != string(pluginThemeWOFF) {
		t.Error("font body differs")
	}

	// Anything outside the theme route passes through; unknown plugins, deeper
	// traversals and bad ids are 404, never a file.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("X-Next", "1") })
	h = ThemeArt(a, next)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/assets/index.js", nil))
	if rec2.Header().Get("X-Next") != "1" {
		t.Error("a request that is not theme art did not reach the next handler")
	}
	for _, target := range []string{
		themes.ThemeURLPrefix + "nope/assets/bg.png",
		themes.ThemeURLPrefix + "fixture-theme/../customization.json",
		themes.ThemeURLPrefix + "fixture-theme/%2e%2e/customization.json",
		themes.ThemeURLPrefix + "fixture-theme/",
		themes.ThemeURLPrefix + "FIXTURE/assets/bg.png",
		themes.ThemeURLPrefix + "fixture-theme/assets/missing.png",
		themes.ThemeURLPrefix + "plugin/echo",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", target, rec.Code)
		}
	}
	// A symlink inside the folder pointing outside never serves, even when the
	// target holds a valid image: the file must live in the folder.
	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, pluginThemePNG, 0o644); err != nil {
		t.Fatal(err)
	}
	// Find the plugin dir through the manager: the first (only) plugin.
	var plugDir string
	if a.plugins != nil {
		if p, ok := a.plugins.Plugin("fixture-theme"); ok {
			plugDir = p.Dir
		}
	}
	if plugDir == "" {
		t.Fatal("fixture-theme plugin missing")
	}
	if err := os.Symlink(outside, filepath.Join(plugDir, "assets", "link.png")); err != nil {
		t.Fatal(err)
	}
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, httptest.NewRequest(http.MethodGet, themes.ThemeURLPrefix+"fixture-theme/assets/link.png", nil))
	if rec3.Code != http.StatusNotFound {
		t.Errorf("symlink escape: %d, want 404", rec3.Code)
	}
}
