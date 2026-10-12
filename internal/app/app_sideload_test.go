package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
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
