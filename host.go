package main

import (
	"errors"
	"fmt"
	"os"
)

// Host is the shell the App runs in: the Wails window, or the Electron shell
// that talks to the core process (core_main.go). The App only ever reaches the
// UI through it, so the same methods serve both.
type Host interface {
	// Emit sends an event to the UI. data is JSON-encoded.
	Emit(event string, data ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
	// OpenFile and OpenDirectory show the native picker and return the chosen
	// path, or "" when the user cancels.
	OpenFile(FileDialog) (string, error)
	OpenDirectory(DirectoryDialog) (string, error)
}

// FileDialog describes a file picker.
type FileDialog struct {
	Title   string       `json:"title"`
	Filters []FileFilter `json:"filters,omitempty"`
}

// FileFilter is one group of file extensions, without the dot.
type FileFilter struct {
	Name       string   `json:"name"`
	Extensions []string `json:"extensions"`
}

// DirectoryDialog describes a folder picker.
type DirectoryDialog struct {
	Title     string `json:"title"`
	Default   string `json:"default,omitempty"`
	CanCreate bool   `json:"canCreate,omitempty"`
}

// stderrHost is the host before a shell exists (tests, early startup): logs go
// to stderr, events go nowhere and there are no dialogs.
type stderrHost struct{}

func (stderrHost) Emit(string, ...any) {}

func (stderrHost) Infof(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...) }

func (stderrHost) Warnf(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...) }

func (stderrHost) Errorf(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...) }

func (stderrHost) OpenFile(FileDialog) (string, error) {
	return "", errors.New("no window to show a file picker")
}

func (stderrHost) OpenDirectory(DirectoryDialog) (string, error) {
	return "", errors.New("no window to show a folder picker")
}
