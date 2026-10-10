package settings

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
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
	t.Setenv("HOME", root) // the default folder is ~/Games: keep it under root
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

func TestMakeDefaultKeepsTheOldDefaultInTheList(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	path := filepath.Join(root, "settings.json")
	s, _ := Open(path)
	old := DefaultGamesFolder()
	ssd := filepath.Join(root, "ssd")
	if _, err := s.AddGameFolder(ssd); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetGamesFolder(ssd); err != nil {
		t.Fatal(err)
	}
	if s.GamesFolder() != ssd {
		t.Fatalf("default = %q, want %q", s.GamesFolder(), ssd)
	}
	if got := s.GameFolders(); !slices.Equal(got, []string{old}) {
		t.Fatalf("libraries = %v, want the old default %v", got, old)
	}
	again, _ := Open(path)
	if again.GamesFolder() != ssd || !slices.Equal(again.GameFolders(), []string{old}) {
		t.Fatalf("reopened: default %q, libraries %v", again.GamesFolder(), again.GameFolders())
	}
	if _, err := s.RemoveGameFolder(ssd); err == nil {
		t.Error("the default folder was removed")
	}
	if got, err := s.RemoveGameFolder(old); err != nil || len(got) != 0 {
		t.Fatalf("removing the old default = %v, %v", got, err)
	}
}

func TestVersion1SettingsMigrateIntoTheList(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	legacy := filepath.Join(root, "Games")
	extra := filepath.Join(root, "ssd")
	body := `{"version":1,"games_folder":"` + legacy + `","game_folders":["` + extra + `","` + legacy + `"]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.GamesFolder() != legacy {
		t.Fatalf("default = %q, want the old games_folder %q", s.GamesFolder(), legacy)
	}
	if got := s.GameFolders(); !slices.Equal(got, []string{extra}) {
		t.Fatalf("libraries = %v, want %v", got, []string{extra})
	}
	// The next save writes the list and drops the old key.
	if _, err := s.AddGameFolder(filepath.Join(root, "hdd")); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), `"games_folder"`) || !strings.Contains(string(data), `"version": 2`) {
		t.Fatalf("saved file still has the old layout:\n%s", data)
	}
	again, _ := Open(path)
	if again.GamesFolder() != legacy || len(again.GameFolders()) != 2 {
		t.Fatalf("reopened: default %q, libraries %v", again.GamesFolder(), again.GameFolders())
	}
}

func TestVersion1WithoutFoldersKeepsTheHomeDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := Open(path)
	if s.GamesFolder() != filepath.Join(home, "Games") || len(s.GameFolders()) != 0 {
		t.Fatalf("default %q, libraries %v", s.GamesFolder(), s.GameFolders())
	}
}
