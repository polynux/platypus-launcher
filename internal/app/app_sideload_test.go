package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/settings"
	"platypus/internal/stores"
)

type fakePickerHost struct {
	stderrHost
	file     FileDialog
	fileRet  string
	fileErr  error
	dir      DirectoryDialog
	dirRet   string
	dirErr   error
}

func (f *fakePickerHost) OpenFile(d FileDialog) (string, error) {
	f.file = d
	return f.fileRet, f.fileErr
}

func (f *fakePickerHost) OpenDirectory(d DirectoryDialog) (string, error) {
	f.dir = d
	return f.dirRet, f.dirErr
}

func TestPickGameExecutableUsesPicker(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	exe := filepath.Join(home, "Games", "game", "game.exe")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	h := &fakePickerHost{fileRet: exe}
	a := &App{host: h, settings: nil}
	got, err := a.PickGameExecutable(exe)
	if err != nil || got != exe {
		t.Fatalf("PickGameExecutable = %q, %v; want %q", got, err, exe)
	}
	if h.file.Title == "" {
		t.Error("the picker needs a title")
	}
	if len(h.file.Filters) == 0 {
		t.Error("the picker needs an All-files filter")
	}
	// A file start opens its folder, so the dialog lands next to the game.
	if h.file.Default != filepath.Dir(exe) {
		t.Fatalf("default = %q, want %q", h.file.Default, filepath.Dir(exe))
	}
}

func TestPickGameExecutableCancelAndNoPicker(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	a := &App{host: &fakePickerHost{fileRet: ""}}
	if got, err := a.PickGameExecutable(""); err != nil || got != "" {
		t.Fatalf("cancel = %q, %v; want empty", got, err)
	}
	a = &App{host: &fakePickerHost{fileErr: errors.New("no window to show a file picker")}}
	if _, err := a.PickGameExecutable(""); err == nil {
		t.Fatal("no picker: want the host error so the UI can degrade to the text field")
	}
}

func TestPickFolderUsesPicker(t *testing.T) {
	h := &fakePickerHost{dirRet: "/games/work"}
	a := &App{host: h}
	got, err := a.PickFolder("Choose the working directory", "/games")
	if err != nil || got != "/games/work" {
		t.Fatalf("PickFolder = %q, %v", got, err)
	}
	if h.dir.Title != "Choose the working directory" {
		t.Fatalf("title = %q", h.dir.Title)
	}
	h2 := &fakePickerHost{dirRet: ""}
	a2 := &App{host: h2}
	if got, err := a2.PickFolder("t", "/x"); err != nil || got != "" {
		t.Fatalf("cancel = %q, %v; want empty", got, err)
	}
}

func TestSaveGameSettingsValidatesWorkingDir(t *testing.T) {
	a := sideloadApp(t)
	id := "g1"
	work := t.TempDir()
	file := filepath.Join(t.TempDir(), "f.exe")
	if err := os.WriteFile(file, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		dir  string
		ok   bool
	}{
		{"empty keeps the old behaviour", "", true},
		{"an existing directory", work, true},
		{"relative is refused", "games/work", false},
		{"missing is refused", filepath.Join(t.TempDir(), "nope"), false},
		{"a file is refused", file, false},
	} {
		if err := a.SaveGameSettings(id, map[string]any{"working_dir": tc.dir}); (err == nil) != tc.ok {
			t.Errorf("%s: err = %v, want ok=%v", tc.name, err, tc.ok)
		}
	}
}

func sideloadApp(t *testing.T) *App {
	t.Helper()
	root := t.TempDir()
	st, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	return &App{settings: st}
}

// The custom-game launch path honours the working directory and the Wine
// prefix stored in the game's GameSettings (issues #33, #34): the runner
// runs in the working dir (empty = the executable's folder) with the
// per-game prefix in the environment.
func TestCustomGameLaunchHonoursWorkingDirAndPrefix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// A runner the scan finds, so prepareLaunch can resolve "auto".
	fakeProton := filepath.Join(home, ".local/share/Steam/compatibilitytools.d", "FakeTest")
	if err := os.MkdirAll(fakeProton, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fakeProton, "proton"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	st, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	gameDir := filepath.Join(root, "game")
	if err := os.MkdirAll(gameDir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(gameDir, "game.exe")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(root, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	prefix := filepath.Join(root, "prefixes", "mine")
	cg, err := st.AddCustomGame("Mine", exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetGameSettings(cg.ID, map[string]any{
		"working_dir": work, "wine_prefix": prefix, "runner_id": "auto",
	}); err != nil {
		t.Fatal(err)
	}

	a := &App{settings: st, stores: stores.NewRegistry(nil, stores.NewSteam())}
	games := a.customGames()
	if len(games) != 1 {
		t.Fatalf("customGames = %d games, want 1", len(games))
	}
	g := games[0]
	if g.Settings.WorkingDir != work {
		t.Fatalf("working_dir = %q, want %q", g.Settings.WorkingDir, work)
	}
	if g.Settings.WinePrefix != prefix {
		t.Fatalf("wine_prefix = %q, want %q", g.Settings.WinePrefix, prefix)
	}
	p, err := a.prepareLaunch(g, true)
	if err != nil {
		t.Fatalf("prepareLaunch: %v", err)
	}
	if p.dir != work {
		t.Fatalf("launch dir = %q, want the working dir %q", p.dir, work)
	}
	if p.env["WINEPREFIX"] != prefix || p.env["STEAM_COMPAT_DATA_PATH"] != prefix {
		t.Fatalf("prefix env = %q / %q, want %q", p.env["WINEPREFIX"], p.env["STEAM_COMPAT_DATA_PATH"], prefix)
	}

	// Empty working dir falls back to the executable's folder.
	if err := st.SetGameSettings(cg.ID, map[string]any{"runner_id": "auto"}); err != nil {
		t.Fatal(err)
	}
	g = a.customGames()[0]
	p, err = a.prepareLaunch(g, true)
	if err != nil {
		t.Fatalf("prepareLaunch without working_dir: %v", err)
	}
	if p.dir != gameDir {
		t.Fatalf("launch dir = %q, want the executable's folder %q", p.dir, gameDir)
	}
}
