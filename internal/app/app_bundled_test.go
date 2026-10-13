package app

// Bundled steam plugin (issue #54): installed when missing, updated when the
// bundled copy is newer, otherwise left alone. Every run uses temp dirs;
// the real plugins folder is never touched.

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/plugins"
	"platypus/internal/settings"
)

func bundledApp(t *testing.T, pluginsDir string) *App {
	t.Helper()
	root := t.TempDir()
	st, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	mgr := plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"),
		DataDir: filepath.Join(root, "data"), Settings: st,
	})
	t.Cleanup(mgr.Shutdown)
	return &App{host: stderrHost{}, settings: st, plugins: mgr, optHome: t.TempDir()}
}

// copyFixture puts the fixture's steam folder where the bundled dir builder
// expects it: bundledFixture returns a temp parent per call, so rebuild the
// layout here.
func writeBundled(t *testing.T, bundledDir, version string) {
	t.Helper()
	dir := filepath.Join(bundledDir, "steam")
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"steam","name":"Steam","version":"` + version + `","manifestVersion":1,` +
		`"protocol":{"versions":["1.15"]},"store":"steam","gameIdPrefix":"steam_","exec":"bin/run.sh",` +
		`"capabilities":["library","launch"],"permissions":{"network":[],"read":[],"write":[]}}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestCompareVersions(t *testing.T) {
	t.Parallel()
	cases := []struct{ a, b string; want int }{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"0.4.0", "1.0.0", -1},
		{"1.10.0", "1.9.0", 1},
		{"1.0", "1.0.0", 0},
		{"2.0", "10.0", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestBundledInstallsWhenMissing(t *testing.T) {
	// No t.Parallel: the test pins PLATYPUS_BUNDLED_PLUGINS via Setenv.
	bundledDir := t.TempDir()
	writeBundled(t, bundledDir, "1.0.0")
	t.Setenv("PLATYPUS_BUNDLED_PLUGINS", bundledDir)
	pluginsDir := filepath.Join(t.TempDir(), "plugins")
	a := bundledApp(t, pluginsDir)

	found, err := a.plugins.Discover()
	if err != nil {
		t.Fatal(err)
	}
	updated := a.ensureBundledSteam(found)
	var steam *plugins.Plugin
	for i, p := range updated {
		if p.Manifest.ID == "steam" {
			steam = &updated[i]
		}
	}
	if steam == nil {
		t.Fatal("bundled steam plugin was not installed")
	}
	if steam.Status != plugins.StatusReady {
		t.Fatalf("steam status = %q (%s), want ready", steam.Status, steam.Problem)
	}
	if st, ok := a.settings.PluginState("steam"); !ok || !st.Enabled || st.Hash != steam.Hash {
		t.Fatalf("trust = %+v, want approved+enabled with the installed hash", st)
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "steam", "bin", "run.sh")); err != nil {
		t.Fatalf("installed exec missing: %v", err)
	}
	// No temp leftovers.
	entries, _ := os.ReadDir(pluginsDir)
	for _, e := range entries {
		if len(e.Name()) > 0 && e.Name()[0] == '.' {
			t.Fatalf("leftover %s in the plugins folder", e.Name())
		}
	}
	// A second run changes nothing.
	again := a.ensureBundledSteam(updated)
	if len(again) != len(updated) {
		t.Fatal("second ensure changed the plugin list")
	}
}

func TestBundledUpdatesWhenNewer(t *testing.T) {
	// No t.Parallel: the test pins PLATYPUS_BUNDLED_PLUGINS via Setenv.
	bundledDir := t.TempDir()
	writeBundled(t, bundledDir, "1.0.0")
	t.Setenv("PLATYPUS_BUNDLED_PLUGINS", bundledDir)
	pluginsDir := filepath.Join(t.TempDir(), "plugins")
	a := bundledApp(t, pluginsDir)

	found, _ := a.plugins.Discover()
	updated := a.ensureBundledSteam(found)
	// The user turned the plugin off: an update keeps it off.
	if err := a.plugins.SetEnabled("steam", false); err != nil {
		t.Fatal(err)
	}
	writeBundled(t, bundledDir, "1.0.1")
	updated2 := a.ensureBundledSteam(updated)
	var steam *plugins.Plugin
	for i, p := range updated2 {
		if p.Manifest.ID == "steam" {
			steam = &updated2[i]
		}
	}
	if steam == nil || steam.Manifest.Version != "1.0.1" {
		t.Fatalf("steam not updated: %+v", steam)
	}
	st, _ := a.settings.PluginState("steam")
	if st.Hash != steam.Hash {
		t.Fatal("trust still pins the old hash")
	}
	if st.Enabled {
		t.Fatal("update re-enabled a plugin the user turned off")
	}
	if steam.Status != plugins.StatusDisabled {
		t.Fatalf("status = %q, want disabled (user choice kept)", steam.Status)
	}
}

func TestBundledLeavesUserInstallAlone(t *testing.T) {
	// No t.Parallel: the test pins PLATYPUS_BUNDLED_PLUGINS via Setenv.
	bundledDir := t.TempDir()
	writeBundled(t, bundledDir, "1.0.0")
	t.Setenv("PLATYPUS_BUNDLED_PLUGINS", bundledDir)
	pluginsDir := filepath.Join(t.TempDir(), "plugins")
	a := bundledApp(t, pluginsDir)

	found, _ := a.plugins.Discover()
	_ = a.ensureBundledSteam(found)
	// The user reinstalls from install.sh with local changes (same version,
	// different hash): the bundled copy must not overwrite it.
	userExec := filepath.Join(pluginsDir, "steam", "bin", "run.sh")
	if err := os.WriteFile(userExec, []byte("#!/bin/sh\necho user\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	after, _ := a.plugins.Discover()
	var userHash string
	for _, p := range after {
		if p.Manifest.ID == "steam" {
			userHash = p.Hash
		}
	}
	// Re-approve the user's copy so the manager knows it.
	if err := a.plugins.Approve("steam"); err != nil {
		t.Fatal(err)
	}
	kept := a.ensureBundledSteam(after)
	for _, p := range kept {
		if p.Manifest.ID == "steam" && p.Hash != userHash {
			t.Fatal("same-version user install was overwritten by the bundled copy")
		}
	}
	body, _ := os.ReadFile(userExec)
	if string(body) != "#!/bin/sh\necho user\n" {
		t.Fatal("user exec was replaced")
	}
	// A newer user install is left alone too.
	writeBundled(t, bundledDir, "0.9.0")
	kept = a.ensureBundledSteam(kept)
	for _, p := range kept {
		if p.Manifest.ID == "steam" && p.Manifest.Version != "1.0.0" {
			t.Fatalf("newer user install downgraded to %s", p.Manifest.Version)
		}
	}
}

func TestBundledNeverTouchesADifferentFolder(t *testing.T) {
	// No t.Parallel: the test pins PLATYPUS_BUNDLED_PLUGINS via Setenv.
	bundledDir := t.TempDir()
	writeBundled(t, bundledDir, "1.0.0")
	t.Setenv("PLATYPUS_BUNDLED_PLUGINS", bundledDir)
	pluginsDir := filepath.Join(t.TempDir(), "plugins")
	// Something else sits at plugins/steam with another id.
	other := filepath.Join(pluginsDir, "steam")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "plugin.json"), []byte(
		`{"id":"other","name":"Other","version":"9.9.9","manifestVersion":1,`+
			`"protocol":{"versions":["1.15"]},"store":"other","exec":"bin/run.sh",`+
			`"capabilities":["library"],"permissions":{"network":[],"read":[],"write":[]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(other, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("user data"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := bundledApp(t, pluginsDir)
	found, _ := a.plugins.Discover()
	kept := a.ensureBundledSteam(found)
	for _, p := range kept {
		if p.Manifest.ID == "steam" && p.Status == plugins.StatusReady {
			t.Fatal("bundled copy overwrote a different plugin folder")
		}
	}
	if _, ok := a.settings.PluginState("steam"); ok {
		t.Fatal("trust was seeded for a folder that isn't the steam plugin")
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatal("different folder was touched")
	}
}
