package stores

import (
	"strings"
	"testing"

	"platypus/internal/settings"
	"platypus/internal/types"
)

// Protocol 1.16 (issue #51): a store's per-game status and notes hints,
// carried as store suggestions, never as the user's own values.

func TestStoreStatusNormalize(t *testing.T) {
	for _, raw := range []string{"backlog", "playing", "finished", "abandoned"} {
		if got := storeStatus(raw, true); got != raw {
			t.Errorf("storeStatus(%q) = %q, want it kept", raw, got)
		}
	}
	for _, raw := range []string{"", "wishlist", "PLAYING", "playing ", "\x00"} {
		if got := storeStatus(raw, true); got != "" {
			t.Errorf("storeStatus(%q) = %q, want no suggestion", raw, got)
		}
	}
	if got := storeStatus("playing", false); got != "" {
		t.Errorf("pre-1.16 status = %q, want no suggestion", got)
	}
}

func TestStoreNotesNormalize(t *testing.T) {
	if got := storeNotes("a suggestion", true); got != "a suggestion" {
		t.Errorf("storeNotes = %q, want it kept", got)
	}
	if got := storeNotes(strings.Repeat("x", settings.MaxGameNotes+1), true); got != "" {
		t.Error("overlong notes were kept")
	}
	if got := storeNotes("", true); got != "" {
		t.Errorf("empty notes = %q, want no suggestion", got)
	}
	if got := storeNotes("a suggestion", false); got != "" {
		t.Errorf("pre-1.16 notes = %q, want no suggestion", got)
	}
}

func TestEchoReportsHintsOnSuggestionsOnly(t *testing.T) {
	e := newEchoEnv(t)
	e.login(t)
	g := e.game(t, "tiny")
	if g.StoreStatus != types.GameStatusPlaying || g.StoreNotes == "" {
		t.Fatalf("tiny hints = %q/%q, want playing and a note", g.StoreStatus, g.StoreNotes)
	}
	if g.Status != "" || g.UserNotes != "" {
		t.Errorf("hints leaked onto the user's fields: %q/%q", g.Status, g.UserNotes)
	}
	others := 0
	lib, err := e.store.Library(e.ctx(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range lib {
		if g.StoreAppID == "tiny" {
			continue
		}
		others++
		if g.StoreStatus != "" || g.StoreNotes != "" {
			t.Errorf("%s carries hints: %q/%q", g.StoreAppID, g.StoreStatus, g.StoreNotes)
		}
	}
	if others == 0 {
		t.Fatal("echo's library only has tiny")
	}
}
