package settings

import (
	"path/filepath"
	"testing"

	"platypus/internal/apperr"
)

// The game colour (issue #123) starts empty (accent), persists, and refuses a
// value the UI never sends.
func TestGameTonePersistsAndDefaultsEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenWithHome(filepath.Join(dir, "settings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.GameTone() != "" {
		t.Fatalf("GameTone = %q, want the default (empty)", s.GameTone())
	}
	if err := s.SetGameTone(GameToneWhole); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenWithHome(filepath.Join(dir, "settings.json"), dir)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.GameTone() != GameToneWhole {
		t.Fatalf("reopened GameTone = %q, want whole", reopened.GameTone())
	}
	err = s.SetGameTone("purple")
	if err == nil {
		t.Fatal("SetGameTone(purple) succeeded, want an error")
	}
	if ae, ok := err.(*apperr.Error); !ok || ae.Code != "settings.gameTone.invalid" {
		t.Fatalf("SetGameTone(purple) error = %v, want settings.gameTone.invalid", err)
	}
	if s.GameTone() != GameToneWhole {
		t.Fatalf("a refused value changed the setting: %q", s.GameTone())
	}
}
