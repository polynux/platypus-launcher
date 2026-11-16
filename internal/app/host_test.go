package app

import (
	"errors"
	"testing"

	"platypus/internal/apperr"
	"platypus/internal/plugins"
	"platypus/internal/types"
)

// openHost records what the App asks the shell to open.
type openHost struct {
	stderrHost
	path string
	err  error
}

func (h *openHost) OpenPath(path string) error { h.path = path; return h.err }

// The folders the core offers to open go through the host: the shell picks the
// OS handler instead of the core shelling out to xdg-open (issue #84).
func TestOpenFoldersUseTheHost(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	h := &openHost{}
	a := &App{host: h, plugins: plugins.NewManager(plugins.Options{Dir: dir})}
	if err := a.OpenPluginsFolder(); err != nil || h.path != dir {
		t.Fatalf("OpenPluginsFolder = %v, opened %q; want %q", err, h.path, dir)
	}
	h.path = ""
	game := types.Game{ID: "g", Title: "G", InstallDir: "/games/g"}
	if err := a.OpenGameFolder(game); err != nil || h.path != game.InstallDir {
		t.Fatalf("OpenGameFolder = %v, opened %q; want %q", err, h.path, game.InstallDir)
	}
}

// A host that can't open (no window, or the shell's handler refused) surfaces
// as the error the UI translates.
func TestOpenFolderHostErrorFailsCoded(t *testing.T) {
	t.Parallel()
	h := &openHost{err: errors.New("no window to open a file or folder")}
	a := &App{host: h, plugins: plugins.NewManager(plugins.Options{Dir: t.TempDir()})}
	if err := a.OpenPluginsFolder(); !errors.Is(err, h.err) {
		t.Fatalf("OpenPluginsFolder = %v, want the host error", err)
	}
	game := types.Game{ID: "g", Title: "G", InstallDir: "/games/g"}
	if err := a.OpenGameFolder(game); apperr.CodeOf(err) != "storage.openFailed" {
		t.Fatalf("OpenGameFolder = %v, want storage.openFailed", err)
	}
}

// Without a shell there is nothing to open with.
func TestStderrHostCannotOpen(t *testing.T) {
	t.Parallel()
	var h Host = stderrHost{}
	if err := h.OpenPath("/games/g"); err == nil {
		t.Fatal("OpenPath: want an error without a window")
	}
	if err := h.OpenURL("https://example.com"); err == nil {
		t.Fatal("OpenURL: want an error without a window")
	}
}
