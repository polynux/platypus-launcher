package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

// CORE-6: the install-folder override is always a valid launch location, so
// it needs the same system-folder guard as AllowFolder.
func TestSetInstallFolderRejectsSystemFolders(t *testing.T) {
	e := newEnv(t)
	e.ready(t)
	home, _ := os.UserHomeDir()
	for _, bad := range []string{"/", "/usr/bin", "/etc", home} {
		if bad == "" {
			continue
		}
		if err := e.m.SetInstallFolder("echo", bad); err == nil {
			t.Errorf("SetInstallFolder(%q) succeeded, want error", bad)
		}
	}
	ok := filepath.Join(t.TempDir(), "Games", "epic")
	if err := e.m.SetInstallFolder("echo", ok); err != nil {
		t.Fatalf("SetInstallFolder(%q): %v", ok, err)
	}
	// Empty restores the default and must keep working.
	if err := e.m.SetInstallFolder("echo", ""); err != nil {
		t.Fatalf("SetInstallFolder reset: %v", err)
	}
}
