package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Issue #142: available lists releases through the downloader, and remove
// only deletes user-installed builds. Installs run as core jobs, never in
// the plugin (M5): runner.install is gone.
func TestAvailableAndRemove(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := handle(request("runner.available", map[string]any{"releasesUrl": "http://127.0.0.1:1/releases"}))
	if got.Error == nil {
		t.Fatal("available against a dead endpoint must fail")
	}

	// runner.install was removed with the unsafe plugin install path: the
	// core runs installs as jobs instead.
	got = handle(request("runner.install", map[string]any{"tag": "GE-Proton10-99"}))
	if got.Error == nil || got.Error.Code != -32601 {
		t.Fatalf("runner.install must be unimplemented, got %+v", got.Error)
	}

	build := filepath.Join(home, ".local/share/Steam/compatibilitytools.d/GE-Proton10-99")
	proton := filepath.Join(build, "proton")
	if err := os.MkdirAll(build, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proton, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got = handle(request("runner.remove", map[string]any{"id": "system:wine"}))
	if got.Error == nil {
		t.Fatal("removing system wine must fail")
	}
	got = handle(request("runner.remove", map[string]any{"id": "compat:GE-Proton10-99"}))
	if got.Error != nil {
		t.Fatalf("remove: %+v", got.Error)
	}
	if _, err := os.Stat(proton); !os.IsNotExist(err) {
		t.Fatalf("build still exists: %v", err)
	}
}
