package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"platypus/internal/plugins"
	"platypus/internal/settings"
)

// CORE-10 UI wiring: the Plugins view offers a Retry action only for plugins
// that gave up after repeated crashes, so give-up must show up in the plugin
// payload the view lists (ListStorePlugins → StorePlugin.GaveUp).

// installEchoApp installs a minimal echo-like plugin: an executable plus a
// manifest, mirroring internal/plugins' test helper but at app level.
func installEchoApp(t *testing.T, pluginsDir string) {
	t.Helper()
	dir := filepath.Join(pluginsDir, "echo")
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	echoBin := filepath.Join(t.TempDir(), "platypus-echo-app")
	out, err := exec.Command("go", "build", "-o", echoBin, "platypus/plugins/echo").CombinedOutput()
	if err != nil {
		t.Fatalf("building echo plugin: %v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin/platypus-echo-app"), mustRead(t, echoBin), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"echo","name":"Echo","version":"1.0.0","api":1,"store":"echo","exec":"bin/platypus-echo-app"}`
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestStorePluginGaveUp(t *testing.T) {
	root := t.TempDir()
	trust, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	m := plugins.NewManager(plugins.Options{
		Dir:      pluginsDir,
		LogDir:   filepath.Join(root, "logs"),
		DataDir:  filepath.Join(root, "data"),
		Settings: trust,
		Timings:  &plugins.Timings{BackoffBase: 20 * time.Millisecond, BackoffMax: 100 * time.Millisecond, MaxCrashes: 3},
	})
	t.Cleanup(m.Shutdown)

	installEchoApp(t, pluginsDir)
	if _, err := m.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := m.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	a := &App{plugins: m, settings: trust}

	// A healthy plugin is not in give-up.
	if got := a.ListStorePlugins(); len(got) != 1 || got[0].GaveUp {
		t.Fatalf("healthy plugin: gave_up = %+v", got)
	}

	// Crash until give-up. Each call waits for its process to be reaped:
	// a call can land on a process still dying from the crash before it,
	// which is the same crash, so the count would fall short.
	for range 3 {
		_ = m.Call(context.Background(), "echo", "crash", nil, nil)
		deadline := time.Now().Add(5 * time.Second)
		for m.Running("echo") != 0 {
			if time.Now().After(deadline) {
				t.Fatal("crashed plugin was never reaped")
			}
			time.Sleep(time.Millisecond)
		}
	}
	if !m.GaveUp("echo") {
		t.Fatal("plugin did not give up after 3 crashes")
	}

	got := a.ListStorePlugins()
	if len(got) != 1 {
		t.Fatalf("plugins = %d", len(got))
	}
	if !got[0].GaveUp {
		t.Fatalf("gave-up plugin not reported: %+v", got[0])
	}

	// After RetryPlugin the give-up clears, so the Retry action disappears.
	if err := m.RetryPlugin("echo"); err != nil {
		t.Fatal(err)
	}
	if got := a.ListStorePlugins(); len(got) != 1 || got[0].GaveUp {
		t.Fatalf("give_up still set after RetryPlugin: %+v", got)
	}
}
