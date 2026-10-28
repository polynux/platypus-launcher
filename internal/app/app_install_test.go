package app

// Zip install at the App level (issue #147): the method wires the manager
// install into the rescan path, and the plugin waits for approval.

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/dataplug"
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

// A zipped data plugin's tweaks stay inert until approval, and the picked
// theme list ignores its theme until then (issue #147: never auto-approved).
func TestInstallDataZipWaitsForApproval(t *testing.T) {
	t.Parallel()
	a, _, _ := uninstallApp(t)
	manifest := `{"id":"datatweak","name":"Data","version":"1.0.0","manifestVersion":1,"kind":"data","description":"test"}`
	tweaks := `{"tweaks":[{"id":"x","kind":"recommended","title":"T","reason":"R","env":{"LD_PRELOAD":"evil"}}]}`
	path := filepath.Join(t.TempDir(), "plugin.zip")
	w, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(w)
	for name, body := range map[string]string{"plugin.json": manifest, "tweaks.json": tweaks} {
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
	if _, err := a.InstallPluginZip(path, false); err != nil {
		t.Fatal(err)
	}
	if p, ok := a.plugins.Plugin("datatweak"); !ok || p.Status != "unapproved" {
		t.Fatalf("plugin = %+v, want unapproved", p)
	}
	hasDataTweak := func() bool {
		for _, tw := range a.data.Tweaks(dataplug.DetectSystem()) {
			if tw.ID == "x" {
				return true
			}
		}
		return false
	}
	if hasDataTweak() {
		t.Fatal("tweaks apply before approval")
	}
	if err := a.ApprovePlugin("datatweak"); err != nil {
		t.Fatalf("approving a data plugin: %v", err)
	}
	if !hasDataTweak() {
		t.Fatal("data tweak missing after approval")
	}
}

// A zipped theme becomes pickable only after approval: before that it is
// listed nowhere, after that findTheme resolves it.
func TestInstallThemeZipBecomesPickableAfterApproval(t *testing.T) {
	t.Parallel()
	a, _, _ := uninstallApp(t)
	path := filepath.Join(t.TempDir(), "plugin.zip")
	w, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(w)
	manifest := `{"id":"fixture-theme","name":"Fixture theme","version":"1.0.0","manifestVersion":1,"kind":"data","description":"test","theme":"theme.toml"}`
	for name, body := range map[string]string{
		"plugin.json":         manifest,
		"theme.toml":          pluginThemeTOML,
		"assets/bg.png":       string(pluginThemePNG),
		"fonts/fixture.woff2": string(pluginThemeWOFF),
	} {
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
	if _, err := a.InstallPluginZip(path, false); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.findTheme("fixture"); ok {
		t.Fatal("theme applies before approval")
	}
	if err := a.ApprovePlugin("fixture-theme"); err != nil {
		t.Fatalf("approving a theme plugin: %v", err)
	}
	if _, ok := a.findTheme("fixture"); !ok {
		t.Fatal("approved theme not found")
	}
	if got := a.GetTheme("fixture"); got.ID != "fixture" {
		t.Fatalf("GetTheme = %q, want fixture", got.ID)
	}
}
