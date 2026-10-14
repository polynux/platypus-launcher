package settings

import (
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/types"
)

func TestGameStatusPersistsAndClears(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, _ := Open(path)
	if s.GameStatus("g") != "" {
		t.Fatal("a game starts with no status")
	}
	for _, st := range []string{types.GameStatusBacklog, types.GameStatusPlaying, types.GameStatusFinished, types.GameStatusAbandoned} {
		if !types.ValidGameStatus(st) {
			t.Fatalf("%q is not a valid status", st)
		}
	}
	if err := s.SetGameStatus("g", types.GameStatusPlaying); err != nil {
		t.Fatal(err)
	}
	again, _ := Open(path)
	if again.GameStatus("g") != types.GameStatusPlaying {
		t.Fatalf("status after reload = %q", again.GameStatus("g"))
	}
	statuses, _ := again.GameStatusNotes()
	if statuses["g"] != types.GameStatusPlaying {
		t.Fatalf("snapshot statuses = %v", statuses)
	}
	if err := again.SetGameStatus("g", ""); err != nil {
		t.Fatal(err)
	}
	third, _ := Open(path)
	if third.GameStatus("g") != "" {
		t.Fatal("status survived the clear")
	}
}

func TestSetGameStatusRejectsNonsense(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "settings.json"))
	if err := s.SetGameStatus("", types.GameStatusBacklog); err == nil {
		t.Fatal("accepted an empty game id")
	}
	if err := s.SetGameStatus("g", "wishlist"); err == nil {
		t.Fatal("accepted an unknown status")
	}
	if got := s.GameStatus("g"); got != "" {
		t.Fatalf("a rejected status was stored: %q", got)
	}
}

func TestGameNotesPersistAndClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, _ := Open(path)
	if s.GameNotes("g") != "" {
		t.Fatal("a game starts with no notes")
	}
	if err := s.SetGameNotes("g", "  remember the cave\nsecond line\t"); err != nil {
		t.Fatal(err)
	}
	if got := s.GameNotes("g"); got != "remember the cave\nsecond line" {
		t.Fatalf("notes = %q", got)
	}
	again, _ := Open(path)
	if again.GameNotes("g") != "remember the cave\nsecond line" {
		t.Fatalf("notes after reload = %q", again.GameNotes("g"))
	}
	_, notes := again.GameStatusNotes()
	if notes["g"] != "remember the cave\nsecond line" {
		t.Fatalf("snapshot notes = %v", notes)
	}
	if err := again.SetGameNotes("g", "   "); err != nil {
		t.Fatal(err)
	}
	third, _ := Open(path)
	if third.GameNotes("g") != "" {
		t.Fatal("notes survived the clear")
	}
}

func TestSetGameNotesRejectsNonsense(t *testing.T) {
	s, _ := Open(filepath.Join(t.TempDir(), "settings.json"))
	if err := s.SetGameNotes("", "hi"); err == nil {
		t.Fatal("accepted an empty game id")
	}
	if err := s.SetGameNotes("g", strings.Repeat("x", MaxGameNotes+1)); err == nil {
		t.Fatal("accepted notes over the cap")
	}
	if err := s.SetGameNotes("g", "a\x00b"); err == nil {
		t.Fatal("accepted a control character")
	}
	if err := s.SetGameNotes("g", "a\rb"); err == nil {
		t.Fatal("accepted a carriage return")
	}
	if got := s.GameNotes("g"); got != "" {
		t.Fatalf("rejected notes were stored: %q", got)
	}
	// Newline and tab are the allowed controls.
	if err := s.SetGameNotes("g", "a\nb\tc"); err != nil {
		t.Fatalf("newline and tab were refused: %v", err)
	}
	if got := s.GameNotes("g"); got != "a\nb\tc" {
		t.Fatalf("notes = %q", got)
	}
}
