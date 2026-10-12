package app

import (
	"path/filepath"
	"testing"

	"platypus/internal/jobs"
	"platypus/internal/types"
)

// Discarding deletes files, so Platypus checks the folder it hands the plugin
// before anything leaves the process.
func TestCheckDiscardDirRefusesDangerousFolders(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	games := filepath.Join(home, "Games")
	ok := filepath.Join(games, "epic")
	if err := checkDiscardDirFor(ok, games, home); err != nil {
		t.Fatalf("a plugin folder under the games root was refused: %v", err)
	}
	custom := filepath.Join(t.TempDir(), "mnt", "fast")
	if err := checkDiscardDirFor(custom, games, home); err != nil {
		t.Fatalf("a custom plugin folder elsewhere was refused: %v", err)
	}
	for name, dir := range map[string]string{
		"empty":      "",
		"relative":   "Games/epic",
		"unclean":    ok + "/../epic",
		"root":       "/",
		"home":       home,
		"parent":     filepath.Dir(home),
		"games root": games,
		"usr":        "/usr/share/games",
		"etc":        "/etc",
	} {
		if err := checkDiscardDirFor(dir, games, home); err == nil {
			t.Errorf("%s (%q) accepted", name, dir)
		}
	}
}

func TestDiscardParams(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	games := filepath.Join(home, "Games")
	j := types.Job{ID: "j1", Kind: jobs.KindUpdate, Source: "steam-standalone", StoreGameID: "620", Title: "Portal 2", InstallDir: filepath.Join(games, "steam")}
	p, err := discardParamsFor(j, games, home)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"jobId": "j1", "id": "620", "kind": "update", "installDir": j.InstallDir}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("%s = %v, want %v (all: %v)", k, p[k], v, p)
		}
	}
	if _, ok := p["title"]; ok {
		t.Errorf("the title is cosmetic and must not be sent: %v", p)
	}
	j.InstallDir = home
	if _, err := discardParamsFor(j, games, home); err == nil {
		t.Error("a job whose folder is the home directory produced discard params")
	}
	// Uninstalls have nothing to discard.
	if _, err := discardParamsFor(types.Job{Kind: jobs.KindUninstall}, games, home); err == nil {
		t.Error("uninstall produced discard params")
	}
}
