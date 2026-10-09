package types

import "testing"

// A version bump is a new term: the old acceptance doesn't count for it.
func TestAVersionBumpMakesATermPendingAgain(t *testing.T) {
	terms := []GameTerm{{ID: "eula", Version: "2"}, {ID: "privacy", Version: "3"}}
	accepted := []TermAcceptance{{ID: "eula", Version: "1", At: 10}, {ID: "privacy", Version: "3", At: 20}}
	marked := MarkAccepted(terms, accepted)
	pending := Pending(marked)
	if len(pending) != 1 || pending[0].ID != "eula" {
		t.Fatalf("pending = %+v, want only the licence at its new version", pending)
	}
	if marked[1].AcceptedAt != 20 {
		t.Fatalf("privacy accepted at %d, want 20", marked[1].AcceptedAt)
	}
	if terms[1].AcceptedAt != 0 {
		t.Fatal("MarkAccepted changed its input")
	}
}

func TestNothingPendingWhenEveryVersionIsAccepted(t *testing.T) {
	marked := MarkAccepted([]GameTerm{{ID: "a", Version: "1"}}, []TermAcceptance{{ID: "a", Version: "1", At: 5}})
	if p := Pending(marked); len(p) != 0 {
		t.Fatalf("pending = %+v", p)
	}
	if p := Pending(nil); len(p) != 0 {
		t.Fatalf("no terms, pending = %+v", p)
	}
}
