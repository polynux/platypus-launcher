package settings

import (
	"os"
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

// Protection facts have their own switch since issue #150: on by default,
// remembered across restarts.
func TestProtectionOnlineDefaultsOnAndPersists(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ProtectionOnline() {
		t.Fatal("protection facts should fetch until the user switches them off")
	}
	if err := s.SetProtectionOnline(false); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.ProtectionOnline() {
		t.Fatal("the protection switch didn't survive a restart")
	}
	if !reopened.RatingsOnline() {
		t.Fatal("switching protection off also switched ratings off")
	}
}

// An old file has one online switch: both new ones take its value (issue #150).
func TestProtectionOnlineTakesTheOldRatingsValue(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		file string
		want bool
	}{
		{"old switch off", `{"version": 2, "fetch_ratings": false}`, false},
		{"old switch on", `{"version": 2, "fetch_ratings": true}`, true},
		{"no switch at all", `{"version": 2}`, true},
		{"version 1 file, old switch off", `{"version": 1, "fetch_ratings": false}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "settings.json")
			if err := os.WriteFile(path, []byte(tc.file), 0o644); err != nil {
				t.Fatal(err)
			}
			s, err := OpenWithHome(path, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if got := s.ProtectionOnline(); got != tc.want {
				t.Fatalf("protection online = %v, want %v", got, tc.want)
			}
			if got := s.RatingsOnline(); got != tc.want {
				t.Fatalf("ratings online = %v, want %v", got, tc.want)
			}
		})
	}
}
