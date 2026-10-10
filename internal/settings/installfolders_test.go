package settings

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestRecentInstallFoldersAreNewestFirstWithoutRepeatsAndCapped(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/g/a", "/g/b", "/g/c", "/g/a", "/g/d", "/g/e", "/g/f"} {
		if err := s.AddRecentInstallFolder(p); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"/g/f", "/g/e", "/g/d", "/g/a", "/g/c"}
	if got := s.RecentInstallFolders(); !slices.Equal(got, want) {
		t.Fatalf("recent = %v, want %v", got, want)
	}
	if err := s.AddRecentInstallFolder("relative/dir"); err == nil {
		t.Error("a relative path was remembered")
	}
	if err := s.AddRecentInstallFolder("/g/x/../y"); err == nil {
		t.Error("an unclean path was remembered")
	}
	if err := s.ForgetRecentInstallFolder("/g/e"); err != nil {
		t.Fatal(err)
	}
	// It survives a restart.
	again, err := Open(s.path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := again.RecentInstallFolders(), []string{"/g/f", "/g/d", "/g/a", "/g/c"}; !slices.Equal(got, want) {
		t.Fatalf("reopened recent = %v, want %v", got, want)
	}
}

func TestGameFolders(t *testing.T) {
	root := t.TempDir()
	s, err := Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	games := filepath.Join(root, "Games")
	if _, err := s.SetGamesFolder(games); err != nil {
		t.Fatal(err)
	}
	ssd := filepath.Join(root, "ssd", "Games")
	got, err := s.AddGameFolder(ssd + "/") // cleaned
	if err != nil || !slices.Equal(got, []string{ssd}) {
		t.Fatalf("AddGameFolder = %v, %v", got, err)
	}
	for _, again := range []string{ssd, games} { // a repeat and the default are not added twice
		if got, err := s.AddGameFolder(again); err != nil || !slices.Equal(got, []string{ssd}) {
			t.Fatalf("AddGameFolder(%s) = %v, %v", again, got, err)
		}
	}
	if _, err := s.AddGameFolder("relative"); err == nil {
		t.Error("a relative folder was added")
	}
	file := filepath.Join(root, "afile")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddGameFolder(file); err == nil {
		t.Error("a file was added as a folder")
	}
	if _, err := s.RemoveGameFolder(games); err == nil {
		t.Error("the default Games folder was removed")
	}
	again, _ := Open(s.path)
	if !slices.Equal(again.GameFolders(), []string{ssd}) {
		t.Fatalf("reopened = %v", again.GameFolders())
	}
	if got, err := s.RemoveGameFolder(ssd); err != nil || len(got) != 0 {
		t.Fatalf("RemoveGameFolder = %v, %v", got, err)
	}
	if got, err := s.RemoveGameFolder(ssd); err != nil || len(got) != 0 { // already gone: not an error
		t.Fatalf("removing twice = %v, %v", got, err)
	}
}

func TestGameFoldersAreCapped(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "settings.json"))
	for i := 0; i < MaxGameFolders; i++ {
		if _, err := s.AddGameFolder(filepath.Join("/g", string(rune('a'+i%26)), string(rune('a'+i/26)))); err != nil {
			t.Fatalf("folder %d: %v", i, err)
		}
	}
	if _, err := s.AddGameFolder("/g/one/too/many"); err == nil {
		t.Fatal("the cap was not enforced")
	}
}
