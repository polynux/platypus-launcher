package themes

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// The example theme plugin (Refs #106) is the documented starting point for
// theme authors, so it must load with zero warnings.
func TestExampleThemePluginLoads(t *testing.T) {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate themes test")
	}
	dir := filepath.Join(filepath.Dir(source), "..", "..", "plugins", "theme-example")
	th := LoadFS(os.DirFS(dir), "theme.toml", "theme-example")
	if th.Info.Status != StatusOK {
		t.Fatalf("example status %q warnings %v, want ok", th.Info.Status, th.Info.Warnings)
	}
	if len(th.Info.Warnings) != 0 {
		t.Fatalf("example warnings %v, want none", th.Info.Warnings)
	}
	for _, level := range []string{"tokens", "variants", "layouts"} {
		if !slices.Contains(th.Info.Supports, level) {
			t.Fatalf("supports = %v, want all three levels", th.Info.Supports)
		}
	}
	if _, ok := th.Layouts["dashboard"]; !ok {
		t.Fatalf("layouts = %v, want dashboard", th.Layouts)
	}
	if len(th.Wash) != 1 || th.Wash[0].Image != "assets/bg.png" {
		t.Fatalf("wash = %+v, want the image reference", th.Wash)
	}
}
