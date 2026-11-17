package steam_test

// Library update detection (issue #209): library.list says an installed game
// has an update from its recorded build and its channel's tip, cached so a
// listing doesn't re-read the store for every game. Client-managed installs
// are the Steam client's to update and stay out.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeInstalled(t *testing.T, e *env, games map[string]map[string]any) {
	t.Helper()
	data, err := json.Marshal(games)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.pluginData(), "installed.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSteamLibraryReportsUpdateAvailable(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", versionsAppinfo(e.fix.GID))
	writeInstalled(t, e, map[string]map[string]any{
		"4321": {"path": filepath.Join(e.folder, "Standalone_Game"), "title": "Standalone Game", "mode": "none", "build": "50"},
	})
	if game := e.find(e.library(), "4321"); !game.UpdateAvailable {
		t.Fatalf("build 50 under public build 100: updateAvailable = false")
	}
	// The second listing answers from the cached tip.
	if game := e.find(e.library(), "4321"); !game.UpdateAvailable {
		t.Fatalf("cached tip: updateAvailable = false")
	}
	// At the channel's tip there is nothing to install.
	writeInstalled(t, e, map[string]map[string]any{
		"4321": {"path": filepath.Join(e.folder, "Standalone_Game"), "title": "Standalone Game", "mode": "none", "build": "100"},
	})
	if game := e.find(e.library(), "4321"); game.UpdateAvailable {
		t.Fatalf("build 100 at the public tip: updateAvailable = true")
	}
}

func TestSteamClientManagedGameHasNoLibraryUpdate(t *testing.T) {
	e := setup(t)
	e.login()
	e.setCMState("appinfo", versionsAppinfo(e.fix.GID))
	writeInstalled(t, e, map[string]map[string]any{
		"4321": {"path": filepath.Join(e.folder, "Standalone_Game"), "title": "Standalone Game", "mode": "client", "build": "50"},
	})
	if game := e.find(e.library(), "4321"); game.UpdateAvailable {
		t.Fatal("a client-managed install must not offer a Platypus update")
	}
}
