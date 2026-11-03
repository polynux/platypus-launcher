package settings

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/apperr"
)

// The colour scheme (issue #134) starts empty (dark), persists, and refuses a
// value the UI never sends.
func TestColorSchemePersistsAndDefaultsEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenWithHome(filepath.Join(dir, "settings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.ColorScheme() != "" {
		t.Fatalf("ColorScheme = %q, want the default (empty)", s.ColorScheme())
	}
	for _, scheme := range []string{ColorSchemeDark, ColorSchemeLight, ColorSchemeBlack} {
		if err := s.SetColorScheme(scheme); err != nil {
			t.Fatal(err)
		}
		if s.ColorScheme() != scheme {
			t.Fatalf("ColorScheme = %q, want %q", s.ColorScheme(), scheme)
		}
	}
	reopened, err := OpenWithHome(filepath.Join(dir, "settings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.ColorScheme() != ColorSchemeBlack {
		t.Fatalf("reopened ColorScheme = %q, want black", reopened.ColorScheme())
	}
	err = s.SetColorScheme("sepia")
	if err == nil {
		t.Fatal("SetColorScheme(sepia) succeeded, want an error")
	}
	if ae, ok := err.(*apperr.Error); !ok || ae.Code != "settings.colorScheme.invalid" {
		t.Fatalf("SetColorScheme(sepia) error = %v, want settings.colorScheme.invalid", err)
	}
	if s.ColorScheme() != ColorSchemeBlack {
		t.Fatalf("a refused value changed the setting: %q", s.ColorScheme())
	}
}

func TestDefaultAccentPersistsAndDefaultsToTheme(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	oldPath := filepath.Join(t.TempDir(), "old-settings.json")
	if err := os.WriteFile(oldPath, []byte(`{"version":2,"game_tone":"whole"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	old, err := Open(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := old.DefaultAccent(); got != "" {
		t.Fatalf("old settings DefaultAccent = %q, want empty", got)
	}
	invalidPath := filepath.Join(t.TempDir(), "invalid-settings.json")
	if err := os.WriteFile(invalidPath, []byte(`{"version":2,"default_accent":"red;--x:1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid, err := Open(invalidPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := invalid.DefaultAccent(); got != "" {
		t.Fatalf("invalid stored DefaultAccent = %q, want empty", got)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.DefaultAccent(); got != "" {
		t.Fatalf("DefaultAccent = %q, want empty", got)
	}
	if err := s.SetDefaultAccent("#2878D0"); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.DefaultAccent(); got != "#2878D0" {
		t.Fatalf("DefaultAccent = %q", got)
	}
	if err := reopened.SetDefaultAccent("blue"); err == nil {
		t.Fatal("invalid accent accepted")
	}
}
