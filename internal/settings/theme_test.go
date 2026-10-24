package settings

import (
	"path/filepath"
	"testing"
)

func TestThemePersistsAndDefaultsEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenWithHome(filepath.Join(dir, "settings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Theme() != "" {
		t.Fatalf("Theme = %q, want the default (empty)", s.Theme())
	}
	if err := s.SetTheme("cool"); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenWithHome(filepath.Join(dir, "settings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Theme() != "cool" {
		t.Fatalf("reopened Theme = %q, want cool", reopened.Theme())
	}
}
