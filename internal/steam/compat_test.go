package steam

import (
	"os"
	"path/filepath"
	"testing"
)

func fakeRoot(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func TestLibraryFoldersDefaultAndVDF(t *testing.T) {
	t.Parallel()
	root := fakeRoot(t)
	if got := LibraryFolders(root); len(got) != 1 || got[0] != root {
		t.Fatalf("folders = %v, want [root]", got)
	}
	extra := filepath.Join(root, "extra-lib")
	if err := os.MkdirAll(extra, 0o755); err != nil {
		t.Fatal(err)
	}
	vdf := "libraryfolders\n{\n\t\"0\"\n\t{\n\t\t\"path\"\t\t\"" + root + "\"\n\t}\n\t\"1\"\n\t{\n\t\t\"path\"\t\t\"" + extra + "\"\n\t}\n}\n"
	if err := os.MkdirAll(filepath.Join(root, "steamapps"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "steamapps/libraryfolders.vdf"), []byte(vdf), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LibraryFolders(root)
	if len(got) != 2 || got[0] != root || got[1] != extra {
		t.Fatalf("folders = %v, want [root extra]", got)
	}
}

func TestCompatDataDirSearchesLibraries(t *testing.T) {
	t.Parallel()
	root := fakeRoot(t)
	extra := filepath.Join(root, "extra")
	compat := filepath.Join(extra, "steamapps", "compatdata", "12345")
	if err := os.MkdirAll(filepath.Join(compat, "pfx", "drive_c"), 0o755); err != nil {
		t.Fatal(err)
	}
	vdf := "x\n{\n\"0\"\n{\n\"path\"\t\t\"" + root + "\"\n}\n\"1\"\n{\n\"path\"\t\t\"" + extra + "\"\n}\n}\n"
	if err := os.MkdirAll(filepath.Join(root, "steamapps"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "steamapps/libraryfolders.vdf"), []byte(vdf), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := CompatDataDir(root, "12345"); got != compat {
		t.Fatalf("compat = %q, want %q", got, compat)
	}
	if got := CompatDataDir(root, "99999"); got != "" {
		t.Fatalf("compat for missing app = %q, want empty", got)
	}
	if prefix, ok := CompatPrefix(root, "12345"); !ok || prefix != filepath.Join(compat, "pfx") {
		t.Fatalf("prefix = %q,%v, want pfx,true", prefix, ok)
	}
	if _, ok := CompatPrefix(root, "99999"); ok {
		t.Fatal("prefix for missing app should be !ok")
	}
}

func TestSingleUserID(t *testing.T) {
	t.Parallel()
	root := fakeRoot(t)
	if got := SingleUserID(root); got != "" {
		t.Fatalf("empty root user = %q, want empty", got)
	}
	if err := os.MkdirAll(filepath.Join(root, "userdata", "111", "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := SingleUserID(root); got != "111" {
		t.Fatalf("single user = %q, want 111", got)
	}
	if err := os.MkdirAll(filepath.Join(root, "userdata", "222"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := SingleUserID(root); got != "" {
		t.Fatalf("two users = %q, want empty (unresolved)", got)
	}
	// Non-numeric entries are ignored.
	root2 := fakeRoot(t)
	if err := os.MkdirAll(filepath.Join(root2, "userdata", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root2, "userdata", "333"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := SingleUserID(root2); got != "333" {
		t.Fatalf("user with stray dir = %q, want 333", got)
	}
}
