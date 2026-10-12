package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// Artwork switch (issue #36): on by default like RatingsOnline, persisted,
// and old files without the field still load as on.
func TestArtworkOnlineDefaultsOnAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ArtworkOnline() {
		t.Fatal("artwork lookup should be on until the user switches it off")
	}
	if err := s.SetArtworkOnline(false); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.ArtworkOnline() {
		t.Fatal("the switch didn't survive a restart")
	}
}

func TestOldSettingsWithoutArtworkSwitchLoadAsOn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"version":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ArtworkOnline() {
		t.Fatal("old files should load with the lookup on")
	}
}
