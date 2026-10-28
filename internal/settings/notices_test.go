package settings

import (
	"os"
	"path/filepath"
	"testing"
)

// Store client notices are on by default (issue #149) and remembered across
// restarts.
func TestClientNoticesDefaultsOnAndPersists(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ClientNotices() {
		t.Fatal("client notices should show until the user switches them off")
	}
	if err := s.SetClientNotices(false); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.ClientNotices() {
		t.Fatal("the client notices switch didn't survive a restart")
	}
	if !reopened.RatingsOnline() || !reopened.ProtectionOnline() {
		t.Fatal("switching client notices off touched the online switches")
	}
}

// A file written before the switch existed still loads: unset means on.
func TestClientNoticesOnForAnOldFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"version": 2, "fetch_ratings": false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := OpenWithHome(path, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !s.ClientNotices() {
		t.Fatal("an old settings file turned client notices off")
	}
}
