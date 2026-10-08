package settings

import (
	"path/filepath"
	"testing"
)

func TestCancelCleanupDays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, _ := Open(path)
	if got := s.CancelCleanupDays(); got != DefaultCancelCleanupDays {
		t.Fatalf("default = %d", got)
	}
	if err := s.SetCancelCleanupDays(0); err != nil {
		t.Fatal(err)
	}
	// 0 is a real choice (delete at once), not "unset".
	s, _ = Open(path)
	if got := s.CancelCleanupDays(); got != 0 {
		t.Fatalf("0 days must persist as 0, got %d", got)
	}
	if err := s.SetCancelCleanupDays(30); err != nil {
		t.Fatal(err)
	}
	s, _ = Open(path)
	if got := s.CancelCleanupDays(); got != 30 {
		t.Fatalf("got %d", got)
	}
	for _, bad := range []int{-1, MaxCancelCleanupDays + 1} {
		if err := s.SetCancelCleanupDays(bad); err == nil {
			t.Fatalf("%d days accepted", bad)
		}
	}
	if got := s.CancelCleanupDays(); got != 30 {
		t.Fatalf("a refused value changed the setting: %d", got)
	}
}
