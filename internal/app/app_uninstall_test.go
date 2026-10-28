package app

// Plugin uninstall (issue #161): the folder goes, the core state goes with
// it, bundled plugins are refused, and game files are never touched.

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/dataplug"
	"platypus/internal/jobs"
	"platypus/internal/plugins"
	"platypus/internal/settings"
	"platypus/internal/stores"
	"platypus/internal/types"
)

func uninstallApp(t *testing.T) (*App, string, string) {
	t.Helper()
	root := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), root)
	if err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	installEchoApp(t, pluginsDir)
	mgr := plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"),
		DataDir: filepath.Join(root, "plugin-data"), Settings: st, Home: root,
	})
	t.Cleanup(mgr.Shutdown)
	if _, err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	a := &App{
		host: stderrHost{}, settings: st, plugins: mgr,
		optHome: root, dataDir: root,
		data:   dataplug.NewRegistry(),
		stores: stores.NewRegistry(nil),
	}
	a.library.snapDir = filepath.Join(root, "library-snapshots")
	var err2 error
	a.jobs, err2 = jobs.Open(filepath.Join(root, "jobs.json"), jobBackend{a}, nil)
	if err2 != nil {
		t.Fatal(err2)
	}
	return a, root, pluginsDir
}

func TestUninstallRemovesFolderAndState(t *testing.T) {
	t.Parallel()
	a, root, pluginsDir := uninstallApp(t)

	if err := a.settings.SetPluginValues("echo", map[string]any{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetPluginGameSettings("p.echo.tiny", settings.PluginGameValues{Source: "echo", StoreID: "tiny", Values: map[string]any{"x": 1}}); err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetChatAlerts("echo", false); err != nil {
		t.Fatal(err)
	}
	// The plugin's own files (login tokens live here) and a snapshot row.
	if err := os.MkdirAll(filepath.Join(root, "plugin-data", "echo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin-data", "echo", "refresh.token"), []byte("SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(a.library.snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.library.snapDir, "echo.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Game files live outside the plugins folder: uninstall must keep them.
	keep := filepath.Join(root, "Games", "echo", "tiny", "game.bin")
	if err := os.MkdirAll(filepath.Dir(keep), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keep, []byte("game"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.UninstallPlugin("echo", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(pluginsDir, "echo")); !os.IsNotExist(err) {
		t.Fatalf("plugin folder still there: %v", err)
	}
	if _, ok := a.settings.PluginState("echo"); ok {
		t.Fatal("approval record still there")
	}
	if got := a.settings.PluginValues("echo"); len(got) != 0 {
		t.Fatalf("plugin values still there: %v", got)
	}
	if _, err := os.Stat(filepath.Join(root, "plugin-data", "echo")); !os.IsNotExist(err) {
		t.Fatal("plugin data folder still there")
	}
	// The snapshot stays by default: its installed rows show as
	// "store missing" instead of dropping out of the library.
	if _, err := os.Stat(filepath.Join(a.library.snapDir, "echo.json")); err != nil {
		t.Fatalf("snapshot should stay by default: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("game files were touched: %v", err)
	}
	// Per-game settings stay by default, so a reinstall restores them.
	if _, ok := a.settings.PluginGameSettings("p.echo.tiny"); !ok {
		t.Fatal("per-game settings should stay by default")
	}
	if got := a.settings.ChatAlerts("echo"); !got {
		t.Fatal("chat switch should reset to the default (on)")
	}

	// Reinstalling starts clean: a fresh folder needs a new approval.
	installEchoApp(t, pluginsDir)
	found, err := a.plugins.Discover()
	if err != nil {
		t.Fatal(err)
	}
	var status plugins.Status
	for _, p := range found {
		if p.Manifest.ID == "echo" {
			status = p.Status
		}
	}
	if status != plugins.StatusUnapproved {
		t.Fatalf("reinstalled status = %q, want unapproved", status)
	}
}

func TestUninstallRemoveGamesForgetsPerGameSettings(t *testing.T) {
	t.Parallel()
	a, _, _ := uninstallApp(t)
	if err := a.settings.SetPluginGameSettings("p.echo.tiny", settings.PluginGameValues{Source: "echo", StoreID: "tiny", Values: map[string]any{"x": 1}}); err != nil {
		t.Fatal(err)
	}
	if err := a.UninstallPlugin("echo", true); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.settings.PluginGameSettings("p.echo.tiny"); ok {
		t.Fatal("per-game settings should go with removeGames")
	}
}

func TestUninstallRemoveGamesDropsSnapshot(t *testing.T) {
	t.Parallel()
	a, root, _ := uninstallApp(t)
	if err := os.MkdirAll(a.library.snapDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.library.snapDir, "echo.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.UninstallPlugin("echo", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.library.snapDir, "echo.json")); !os.IsNotExist(err) {
		t.Fatal("snapshot should go with removeGames")
	}
	_ = root
}

func TestUninstallBundledRefused(t *testing.T) {
	t.Parallel()
	a, _, pluginsDir := uninstallApp(t)
	// A bundled plugin sits in the folder like any other: the refusal is by
	// id, not by what is on disk.
	dir := filepath.Join(pluginsDir, "steam")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(`{"id":"steam","name":"Steam","version":"1.0.0","api":1,"store":"steam","exec":"run.sh"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.UninstallPlugin("steam", false); err == nil {
		t.Fatal("bundled uninstall should fail")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("bundled folder was touched: %v", err)
	}
}

func TestUninstallUnknownRefused(t *testing.T) {
	t.Parallel()
	a, _, _ := uninstallApp(t)
	if err := a.UninstallPlugin("nope", false); err == nil {
		t.Fatal("unknown plugin uninstall should fail")
	}
}

func TestUninstallRefusesTraversal(t *testing.T) {
	t.Parallel()
	a, root, _ := uninstallApp(t)
	marker := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../keep.txt", "a/b", "..", ".", "", "echo/../../keep.txt"} {
		if err := a.plugins.Uninstall(id); err == nil {
			t.Fatalf("Uninstall(%q) should fail", id)
		}
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("file outside the plugins folder was touched: %v", err)
	}
}

// A store-missing game launches from its executable on disk (issue #161:
// installed games stay launchable where possible); without the flag the same
// game refuses with "isn't approved and enabled". The store really is gone
// here: the plugin was uninstalled first.
func TestStoreMissingGameLaunchesFromDisk(t *testing.T) {
	t.Parallel()
	a, _, _ := uninstallApp(t)
	if err := a.UninstallPlugin("echo", false); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(t.TempDir(), "tiny")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	missing := types.Game{ID: "p.echo.tiny", Source: "echo", Title: "Tiny", IsInstalled: true, Executable: exe, Native: true, StoreMissing: true}
	p, err := a.prepareLaunch(missing, true)
	if err != nil {
		t.Fatalf("store-missing launch: %v", err)
	}
	if len(p.args) == 0 || p.args[0] != exe {
		t.Fatalf("args = %v, want the executable on disk", p.args)
	}
	plain := missing
	plain.StoreMissing = false
	if _, err := a.prepareLaunch(plain, true); err == nil {
		t.Fatal("a game whose store is gone must not launch without the flag")
	}
}

// A flagged game still refuses when a plugin with that id is installed
// (disabled, unapproved or changed): only a missing store launches direct.
func TestStoreMissingLaunchRefusedWhenPluginInstalled(t *testing.T) {
	t.Parallel()
	a, _, _ := uninstallApp(t)
	exe := filepath.Join(t.TempDir(), "tiny")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	missing := types.Game{ID: "p.echo.tiny", Source: "echo", Title: "Tiny", IsInstalled: true, Executable: exe, Native: true, StoreMissing: true}
	if _, err := a.prepareLaunch(missing, true); err == nil {
		t.Fatal("an installed store must keep the not-approved refusal")
	}
}

func TestUninstallPickedThemeFallsBack(t *testing.T) {
	t.Parallel()
	a, mgr := themeApp(t, "fixture-theme", "fixture", pluginThemeTOML, map[string][]byte{
		"assets/bg.png": pluginThemePNG,
	})
	a.dataDir = t.TempDir()
	a.host = stderrHost{}
	a.data = dataplug.NewRegistry()
	a.library.snapDir = filepath.Join(a.dataDir, "library-snapshots")
	var err error
	a.jobs, err = jobs.Open(filepath.Join(a.dataDir, "jobs.json"), jobBackend{a}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.settings.SetTheme("fixture"); err != nil {
		t.Fatal(err)
	}
	_ = mgr
	if err := a.UninstallPlugin("fixture-theme", false); err != nil {
		t.Fatal(err)
	}
	if got := a.settings.Theme(); got != "" {
		t.Fatalf("theme = %q, want the built-in default (empty)", got)
	}
}
