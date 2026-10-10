package app

import (
	"testing"

	"platypus/internal/jobs"
	"platypus/internal/types"
)

// Install params: plugins now accept `title` in game.install/game.update (GOG
// records it into its installed index, steam-standalone keys its dl request
// on it), and steam-standalone accepts branch/branchPassword. Platypus sends the
// title the job already has, and sends no branch at all — an empty one must
// be omitted, and building branch/password UI is explicitly out of scope.

func TestStartParamsCarryTitle(t *testing.T) {
	t.Run("install carries title and no branch", func(t *testing.T) {
		j := types.Job{ID: "j1", Kind: jobs.KindInstall, Source: "echo", GameID: "g1", StoreGameID: "tiny", Title: "Tiny", InstallDir: "/g/tiny"}
		params := installParams(j, nil)
		if params["title"] != "Tiny" {
			t.Fatalf("game.install params missing title: %v", params)
		}
		if _, ok := params["branch"]; ok {
			t.Fatalf("game.install params must not carry a branch: %v", params)
		}
		if _, ok := params["branchPassword"]; ok {
			t.Fatalf("game.install params must not carry a branch password: %v", params)
		}
		if params["id"] != "tiny" || params["jobId"] != "j1" || params["installDir"] != "/g/tiny" {
			t.Fatalf("base params broken: %v", params)
		}
	})

	t.Run("update carries title too", func(t *testing.T) {
		j := types.Job{ID: "j2", Kind: jobs.KindUpdate, Source: "echo", GameID: "g1", StoreGameID: "tiny", Title: "Tiny", InstallDir: "/g/tiny"}
		params := installParams(j, nil)
		if params["title"] != "Tiny" {
			t.Fatalf("game.update params missing title: %v", params)
		}
	})

	t.Run("uninstall carries no title and no installDir", func(t *testing.T) {
		j := types.Job{ID: "j3", Kind: jobs.KindUninstall, Source: "echo", GameID: "g1", StoreGameID: "tiny", Title: "Tiny"}
		params := installParams(j, nil)
		if _, ok := params["title"]; ok {
			t.Fatalf("game.uninstall params must not carry a title: %v", params)
		}
		if _, ok := params["installDir"]; ok {
			t.Fatalf("game.uninstall params must not carry installDir: %v", params)
		}
	})

	t.Run("empty title is omitted", func(t *testing.T) {
		j := types.Job{ID: "j4", Kind: jobs.KindInstall, Source: "echo", GameID: "g1", StoreGameID: "tiny", InstallDir: "/g/tiny"}
		params := installParams(j, nil)
		if _, ok := params["title"]; ok {
			t.Fatalf("empty title must be omitted: %v", params)
		}
	})
}
