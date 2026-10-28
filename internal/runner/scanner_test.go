package runner

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/types"
)

// markedDefault is the id of the one runner the scan marked as the runner
// "auto" resolves to.
func markedDefault(t *testing.T, versions []types.ProtonVersion) string {
	t.Helper()
	found := ""
	for _, r := range versions {
		if !r.IsDefault {
			continue
		}
		if found != "" {
			t.Fatalf("two runners marked default: %q and %q", found, r.ID)
		}
		found = r.ID
	}
	return found
}

// The scan marks the picked default, the umu entry included (issue #146): it is
// what an automatic game runs when umu is installed. Switching the pick moves
// the mark, and the automatic pick has it when nothing is set.
func TestScanMarksThePickedDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PATH", t.TempDir()) // only the temp home's umu-run counts
	for _, p := range []string{
		filepath.Join(home, ".local/share/Steam/steamapps/common/Proton 9.0/proton"),
		filepath.Join(home, ".local/bin/umu-run"),
	} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if got := markedDefault(t, ScanProtonVersionsFor(home, "")); got != "steam:Proton 9.0" {
		t.Fatalf("automatic marks %q, want the Proton build", got)
	}
	if got := markedDefault(t, ScanProtonVersionsFor(home, UmuRunnerID)); got != UmuRunnerID {
		t.Fatalf("a default of umu marks %q, want the umu entry", got)
	}
	if got := markedDefault(t, ScanProtonVersionsFor(home, "steam:Proton 4.11")); got != "steam:Proton 9.0" {
		t.Fatalf("an uninstalled default marks %q, want the automatic pick", got)
	}
}
