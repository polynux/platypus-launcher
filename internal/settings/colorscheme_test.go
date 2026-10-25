package settings

import (
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
