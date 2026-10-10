package settings

import (
	"path/filepath"
	"testing"
)

func TestRatingsOnlineDefaultsOnAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !s.RatingsOnline() {
		t.Fatal("ratings should fetch until the user switches them off")
	}
	if err := s.SetRatingsOnline(false); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.RatingsOnline() {
		t.Fatal("the switch didn't survive a restart")
	}
}

func TestCompatAppIDIsDigitsAndClears(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetCompatAppID("g1", "620 "); err != nil {
		t.Fatal(err)
	}
	if got := s.CompatAppID("g1"); got != "620" {
		t.Fatalf("saved %q, want the trimmed 620", got)
	}
	for _, bad := range []string{"portal-2", "62o", "-1"} {
		if err := s.SetCompatAppID("g1", bad); err == nil {
			t.Fatalf("%q accepted as an AppID", bad)
		}
	}
	if got := s.CompatAppID("g1"); got != "620" {
		t.Fatalf("a rejected value replaced the saved one: %q", got)
	}
	if err := s.SetCompatAppID("g1", ""); err != nil {
		t.Fatal(err)
	}
	if got := s.CompatAppID("g1"); got != "" {
		t.Fatalf("cleared AppID still %q", got)
	}
}
