package paths

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/datadir"
)

func TestPrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PLATYPUS_DATA_DIR", "")
	t.Setenv("PLATYPUS_CONFIG_DIR", "")
	t.Setenv("PLATYPUS_CACHE_DIR", "")
	t.Setenv("PLATYPUS_PLUGINS", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	if got := DataDirFor(home); got != filepath.Join(home, ".local", "share", "platypus") {
		t.Fatalf("unset data = %q", got)
	}
	if got := ConfigDirFor(home); got != filepath.Join(home, ".config", "platypus") {
		t.Fatalf("unset config = %q", got)
	}
	if got := CacheDirFor(home); got != filepath.Join(home, ".cache", "platypus") {
		t.Fatalf("unset cache = %q", got)
	}
	if got := PluginsDirFor(filepath.Join(home, ".local", "share", "platypus")); got != filepath.Join(home, ".local", "share", "platypus", "plugins") {
		t.Fatalf("unset plugins = %q", got)
	}

	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg-data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "xdg-cache"))
	if got := DataDirFor(home); got != filepath.Join(home, "xdg-data", "platypus") {
		t.Fatalf("xdg data = %q", got)
	}
	if got := ConfigDirFor(home); got != filepath.Join(home, "xdg-config", "platypus") {
		t.Fatalf("xdg config = %q", got)
	}
	if got := CacheDirFor(home); got != filepath.Join(home, "xdg-cache", "platypus") {
		t.Fatalf("xdg cache = %q", got)
	}

	t.Setenv("PLATYPUS_DATA_DIR", filepath.Join(home, "p-data"))
	t.Setenv("PLATYPUS_CONFIG_DIR", filepath.Join(home, "p-config"))
	t.Setenv("PLATYPUS_CACHE_DIR", filepath.Join(home, "p-cache"))
	t.Setenv("PLATYPUS_PLUGINS", filepath.Join(home, "p-plugins"))
	if got := DataDirFor(home); got != filepath.Join(home, "p-data") {
		t.Fatalf("platypus data = %q", got)
	}
	if got := ConfigDirFor(home); got != filepath.Join(home, "p-config") {
		t.Fatalf("platypus config = %q", got)
	}
	if got := CacheDirFor(home); got != filepath.Join(home, "p-cache") {
		t.Fatalf("platypus cache = %q", got)
	}
	if got := PluginsDirFor(filepath.Join(home, "p-data")); got != filepath.Join(home, "p-plugins") {
		t.Fatalf("platypus plugins = %q", got)
	}
}

func TestRelativeValuesIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PLATYPUS_DATA_DIR", "relative/dir")
	t.Setenv("PLATYPUS_CONFIG_DIR", "relative/cfg")
	t.Setenv("PLATYPUS_CACHE_DIR", "relative/cache")
	t.Setenv("PLATYPUS_PLUGINS", "relative/plugins")
	t.Setenv("XDG_DATA_HOME", "relative/xdg-data")
	t.Setenv("XDG_CONFIG_HOME", "relative/xdg-config")
	t.Setenv("XDG_CACHE_HOME", "relative/xdg-cache")

	if got := DataDirFor(home); got != filepath.Join(home, ".local", "share", "platypus") {
		t.Fatalf("relative data = %q, want legacy default", got)
	}
	if got := ConfigDirFor(home); got != filepath.Join(home, ".config", "platypus") {
		t.Fatalf("relative config = %q", got)
	}
	if got := CacheDirFor(home); got != filepath.Join(home, ".cache", "platypus") {
		t.Fatalf("relative cache = %q", got)
	}
	if got := PluginsDirFor(DataDirFor(home)); got != filepath.Join(home, ".local", "share", "platypus", "plugins") {
		t.Fatalf("relative plugins = %q", got)
	}
}

func TestLegacyFallback(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, ".local", "share", "platypus")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(home, "elsewhere")
	t.Setenv("XDG_DATA_HOME", elsewhere)
	t.Setenv("PLATYPUS_DATA_DIR", "")
	t.Setenv("PLATYPUS_PLUGINS", "")
	got := ResolveFor(home)
	if got.DataDir != legacy {
		t.Fatalf("legacy fallback = %q, want %q", got.DataDir, legacy)
	}
	if got.PluginsDir != filepath.Join(legacy, "plugins") {
		t.Fatalf("legacy plugins = %q", got.PluginsDir)
	}
	// When the XDG folder exists too, it wins (no fallback).
	if err := os.MkdirAll(filepath.Join(elsewhere, "platypus"), 0o755); err != nil {
		t.Fatal(err)
	}
	got = ResolveFor(home)
	if want := filepath.Join(elsewhere, "platypus"); got.DataDir != want {
		t.Fatalf("xdg exists = %q, want %q", got.DataDir, want)
	}
}

func TestDatadirOverrideWins(t *testing.T) {
	home := t.TempDir()
	ov := filepath.Join(home, "override")
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg"))
	t.Setenv("PLATYPUS_DATA_DIR", "")
	datadir.Use(ov)
	t.Cleanup(func() { datadir.Use("") })
	if got := ResolveFor(home); got.DataDir != ov {
		t.Fatalf("override = %q, want %q", got.DataDir, ov)
	}
}

func TestSteamRootsKeepsLegacy(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("XDG_DATA_HOME", xdg)
	roots := SteamRoots(home)
	if len(roots) == 0 || roots[0] != filepath.Join(home, ".local", "share", "Steam") {
		t.Fatalf("first root = %v, want legacy first", roots)
	}
	found := false
	for _, r := range roots {
		if r == filepath.Join(xdg, "Steam") {
			found = true
		}
	}
	if !found {
		t.Fatalf("roots = %v, want XDG candidate included", roots)
	}
}
