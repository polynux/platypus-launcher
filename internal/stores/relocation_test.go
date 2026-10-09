package stores

import (
	"path/filepath"
	"slices"
	"testing"

	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

func TestRebasePathOnlyTouchesPathsInsideTheFolder(t *testing.T) {
	cases := map[string]string{
		"/games/old":           "/games/new",
		"/games/old/game.sh":   "/games/new/game.sh",
		"/games/old2/game.sh":  "/games/old2/game.sh",
		"/games/other/game.sh": "/games/other/game.sh",
		"":                     "",
	}
	for in, want := range cases {
		if got := rebasePath(in, "/games/old", "/games/new"); got != want {
			t.Errorf("rebasePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRebaseSpecLeavesArgumentsAlone(t *testing.T) {
	spec := plugins.LaunchSpec{
		Exe: "/games/old/game.sh", Cwd: "/games/old", Args: []string{"/games/old/x"},
		Command: []string{"/games/old/run", "--flag"},
	}
	got := rebaseSpec(spec, "/games/old", "/games/new")
	if got.Exe != "/games/new/game.sh" || got.Cwd != "/games/new" {
		t.Fatalf("exe/cwd = %q %q", got.Exe, got.Cwd)
	}
	if !slices.Equal(got.Command, []string{"/games/new/run", "--flag"}) {
		t.Fatalf("command = %v", got.Command)
	}
	if !slices.Equal(got.Args, []string{"/games/old/x"}) {
		t.Fatalf("args changed: %v", got.Args)
	}
	if !slices.Equal(spec.Command, []string{"/games/old/run", "--flag"}) {
		t.Fatal("the input spec was modified")
	}
}

func TestLibraryShowsTheFolderCoreMovedTo(t *testing.T) {
	moved := types.Relocation{From: "/games/old/Tiny", To: "/mnt/disk/Tiny"}
	s := (&Plugin{m: plugins.NewManager(plugins.Options{})}).WithRelocations(func(id string) (types.Relocation, bool) {
		return moved, id == GameID("", "tiny")
	})
	lg := plugins.LibraryGame{ID: "tiny", Installed: true, InstallPath: "/games/old/Tiny"}
	g := s.toGame(lg, []string{"/games/old"})
	if g.InstallDir != "/mnt/disk/Tiny" {
		t.Fatalf("install dir = %q", g.InstallDir)
	}
	if g.OutsideFolder {
		t.Fatal("the moved game is shown outside the folders Hydra launches from")
	}

	// A plugin that has moved the game itself reports the new folder, and
	// that one is kept.
	lg.InstallPath = "/mnt/disk/Tiny"
	if g := s.toGame(lg, []string{"/games/old"}); g.InstallDir != "/mnt/disk/Tiny" {
		t.Fatalf("plugin-reported folder replaced: %q", g.InstallDir)
	}
	lg.InstallPath = "/games/elsewhere/Tiny"
	if g := s.toGame(lg, nil); g.InstallDir != filepath.Clean("/games/elsewhere/Tiny") {
		t.Fatalf("unrelated folder replaced: %q", g.InstallDir)
	}
}
