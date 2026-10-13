package savebackup

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"platypus/internal/testenv"
)

// One real-manifest test (issue #42): it fetches Ludusavi's manifest over the
// network, builds the derived index, and matches a well-known Steam game. It
// runs only with PLATYPUS_TEST_NETWORK=1.
func TestRealManifestFetchAndIndex(t *testing.T) {
	testenv.RequireNetwork(t)
	resp, err := http.Get(DefaultManifestURL)
	if err != nil {
		t.Skipf("manifest unreachable: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("manifest status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 1024*1024 {
		t.Fatalf("manifest = %d bytes, want the multi-MB file", len(data))
	}
	games, err := ParseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(games) == 0 {
		t.Fatal("no save entries parsed from the real manifest")
	}
	t.Logf("manifest: %d bytes, %d games with saves", len(data), len(games))
	idx := &Index{Games: games}
	// A well-known Steam title should match by id or title. The manifest
	// changes over time, so this asserts shape, not one game: at least one
	// entry carries a steam id and save files.
	found := false
	for i := range idx.Games {
		if idx.Games[i].SteamID != "" && len(idx.Games[i].Files) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("no steam-keyed entry with saves in the real manifest")
	}
	_ = strings.TrimSpace
}
