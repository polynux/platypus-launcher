package settings

import (
	"path/filepath"
	"testing"
)

func TestLanguageDefaultSystemAndPersist(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenWithHome(filepath.Join(dir, "settings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Language() != "" {
		t.Fatalf("Language = %q, want System (empty)", s.Language())
	}
	if err := s.SetLanguage("fr-CA"); err != nil {
		t.Fatal(err)
	}
	if s.Language() != "fr-CA" {
		t.Fatalf("Language = %q, want fr-CA", s.Language())
	}
	reopened, err := OpenWithHome(filepath.Join(dir, "settings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Language() != "fr-CA" {
		t.Fatalf("reopened Language = %q, want fr-CA", reopened.Language())
	}
	if err := reopened.SetLanguage(""); err != nil {
		t.Fatal(err)
	}
	if reopened.Language() != "" {
		t.Fatalf("Language = %q, want System (empty)", reopened.Language())
	}
}

func TestLanguageRejectsGarbage(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenWithHome(filepath.Join(dir, "settings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"!", "f r", "toolongtag-", "en--US", "1234"} {
		if err := s.SetLanguage(bad); err == nil {
			t.Fatalf("SetLanguage(%q) succeeded, want an error", bad)
		}
	}
	if s.Language() != "" {
		t.Fatalf("Language = %q, want unchanged System (empty)", s.Language())
	}
}
