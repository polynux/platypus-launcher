package app

// Zip install at the App level (issue #147): the method wires the manager
// install into the rescan path, and the plugin waits for approval.

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeAppZip(t *testing.T, manifest string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plugin.zip")
	w, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(w)
	for name, body := range map[string]string{"plugin.json": manifest, "run.sh": "#!/bin/sh\n"} {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInstallPluginZipNeedsApproval(t *testing.T) {
	t.Parallel()
	a, _, _ := uninstallApp(t)
	manifest := `{"id":"zippy","name":"Zippy","version":"1.2.0","manifestVersion":1,` +
		`"protocol":{"versions":["1.0"]},"store":"zippy","exec":"run.sh",` +
		`"capabilities":["library"],"permissions":{"network":[],"read":[],"write":[]}}`
	res, err := a.InstallPluginZip(writeAppZip(t, manifest), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "zippy" || res.NeedsConfirm {
		t.Fatalf("result = %+v, want a fresh install", res)
	}
	p, ok := a.plugins.Plugin("zippy")
	if !ok || p.Status != "unapproved" {
		t.Fatalf("plugin = %+v, want unapproved", p)
	}
	if _, ok := a.settings.PluginState("zippy"); ok {
		t.Fatal("zip install must not auto-approve")
	}
}
