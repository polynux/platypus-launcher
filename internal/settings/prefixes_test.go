package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// Issue #37: the global prefixes folder is validated on save and "" resets
// it to the default; it need not exist yet (created on demand at launch).
func TestPrefixesFolderValidation(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := st.PrefixesFolder(); got != "" {
		t.Fatalf("PrefixesFolder = %q, want empty (the default)", got)
	}

	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		path string
		ok   bool
	}{
		{"absolute is kept", "/home/you/prefixes", true},
		{"a folder that doesn't exist yet is kept", filepath.Join(dir, "new-prefixes"), true},
		{"relative is refused", "prefixes/mine", false},
		{"a file is refused", file, false},
	} {
		if _, err := st.SetPrefixesFolder(tc.path); (err == nil) != tc.ok {
			t.Errorf("%s: err = %v, want ok=%v", tc.name, err, tc.ok)
		}
	}

	if _, err := st.SetPrefixesFolder("/home/you/prefixes"); err != nil {
		t.Fatal(err)
	}
	if got := st.PrefixesFolder(); got != "/home/you/prefixes" {
		t.Fatalf("PrefixesFolder = %q", got)
	}
	// Reset to the default.
	if got, err := st.SetPrefixesFolder(""); err != nil || got != "" {
		t.Fatalf("reset = %q, %v; want empty", got, err)
	}
	if got := st.PrefixesFolder(); got != "" {
		t.Fatalf("after reset PrefixesFolder = %q, want empty", got)
	}
	// Survives a reload.
	if _, err := st.SetPrefixesFolder("/srv/prefixes"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.PrefixesFolder(); got != "/srv/prefixes" {
		t.Fatalf("after reload PrefixesFolder = %q", got)
	}
}
