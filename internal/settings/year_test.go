package settings

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Release year (issue #35): validated, persisted, and 0 clears it. Files
// written before the field existed still load, with an unknown year.
func TestCustomGameYearValidation(t *testing.T) {
	now := time.Now().Year()
	for _, year := range []int{0, 1970, 1998, now, now + 1} {
		if err := ValidateReleaseYear(year); err != nil {
			t.Fatalf("ValidateReleaseYear(%d) = %v, want nil", year, err)
		}
	}
	for _, year := range []int{-1, 1, 1969, now + 2, 9999} {
		if err := ValidateReleaseYear(year); err == nil {
			t.Fatalf("ValidateReleaseYear(%d) = nil, want an error", year)
		}
	}
}

func TestSetCustomGameYearPersistsAndClears(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "game.sh")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	g, err := st.AddCustomGame("My Game", exe)
	if err != nil {
		t.Fatal(err)
	}
	if g.ReleaseYear != 0 {
		t.Fatalf("new game year = %d, want 0 (unknown)", g.ReleaseYear)
	}
	if err := st.SetCustomGameYear(g.ID, 2011); err != nil {
		t.Fatal(err)
	}
	got := st.CustomGames()
	if len(got) != 1 || got[0].ReleaseYear != 2011 {
		t.Fatalf("year after set = %v, want [2011]", got)
	}
	// Reopen: the year survives a restart.
	st2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := st2.CustomGames(); len(got) != 1 || got[0].ReleaseYear != 2011 {
		t.Fatalf("year after reopen = %v, want [2011]", got)
	}
	// 0 clears it, nonsense is refused and keeps the stored value.
	if err := st2.SetCustomGameYear(g.ID, 0); err != nil {
		t.Fatal(err)
	}
	if got := st2.CustomGames(); got[0].ReleaseYear != 0 {
		t.Fatalf("year after clear = %d, want 0", got[0].ReleaseYear)
	}
	if err := st2.SetCustomGameYear(g.ID, 1800); err == nil {
		t.Fatal("SetCustomGameYear(1800) = nil, want an error")
	}
	if err := st2.SetCustomGameYear("custom-nope", 2000); err == nil {
		t.Fatal("SetCustomGameYear(unknown id) = nil, want an error")
	}
}

func TestOldSettingsWithoutYearStillLoad(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.json")
	raw := `{"version":2,"custom_games":[{"id":"custom-1","title":"Old","executable":"/x","added_at":1}]}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got := st.CustomGames()
	if len(got) != 1 || got[0].ReleaseYear != 0 {
		t.Fatalf("old file games = %+v, want one game with year 0", got)
	}
}
