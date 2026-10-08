package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"hydra-launcher/internal/dataplug"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/settings"
	"hydra-launcher/internal/types"
)

// CORE-5: installing then removing a data plugin followed by rescan must
// activate then drop its fixes/tweaks without a restart.
func TestRescanSyncsDataPlugins(t *testing.T) {
	root := t.TempDir()
	st, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	mgr := plugins.NewManager(plugins.Options{
		Dir:      pluginsDir,
		LogDir:   filepath.Join(root, "logs"),
		DataDir:  filepath.Join(root, "data"),
		Settings: st,
	})
	t.Cleanup(mgr.Shutdown)

	writeData := func() {
		dir := filepath.Join(pluginsDir, "community-fixes")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		manifest := `{"id":"community-fixes","name":"Community fixes","version":"1.0.0","api":1,"kind":"data","data":["fixes.json","tweaks.json"]}`
		if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		fixes := `{"fixes":[{"title":"Rescan Game","source":"community","tier":"gold","description":"Needs this fix."}]}`
		if err := os.WriteFile(filepath.Join(dir, "fixes.json"), []byte(fixes), 0o644); err != nil {
			t.Fatal(err)
		}
		tweaks := `{"tweaks":[{"id":"rescan-tweak","kind":"recommended","title":"Rescan Tweak","reason":"Test.","env":{"HYRDA_RESCAN_TEST":"1"}}]}`
		if err := os.WriteFile(filepath.Join(dir, "tweaks.json"), []byte(tweaks), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeData()

	a := &App{plugins: mgr, data: dataplug.NewRegistry()}

	found, err := mgr.Discover()
	if err != nil {
		t.Fatal(err)
	}
	a.syncDataPlugins(found)

	if f := a.data.Fix("Rescan Game", ""); f == nil {
		t.Fatal("data plugin fix not active after sync")
	}
	if tw := a.GetTweaks(); !slices.ContainsFunc(tw, func(x types.Tweak) bool { return x.ID == "rescan-tweak" }) {
		t.Fatalf("data plugin tweak not active after sync: %v", tw)
	}

	// Remove the plugin and rescan: its fixes/tweaks must go, built-in stays.
	if err := os.RemoveAll(filepath.Join(pluginsDir, "community-fixes")); err != nil {
		t.Fatal(err)
	}
	found, err = mgr.Discover()
	if err != nil {
		t.Fatal(err)
	}
	a.syncDataPlugins(found)

	if f := a.data.Fix("Rescan Game", ""); f != nil {
		t.Fatalf("removed plugin fix still served: %+v", f)
	}
	if tw := a.GetTweaks(); slices.ContainsFunc(tw, func(x types.Tweak) bool { return x.ID == "rescan-tweak" }) {
		t.Fatal("removed plugin tweak still served after rescan")
	}
	if f := a.data.Fix("Fallout: London", ""); f == nil {
		t.Fatal("built-in fix missing after resync")
	}
}
