package settings

import (
	"path/filepath"
	"testing"

	"hydra-launcher/internal/types"
)

// The acceptance record survives a restart, keeps one line per (term, version)
// and keeps the first date of a version it already had.
func TestAcceptedTermsAreKeptPerVersionAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first := []types.TermAcceptance{{ID: "eula", Version: "1", At: 100}}
	if err := s.AcceptTerms("g", first); err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptTerms("g", []types.TermAcceptance{{ID: "eula", Version: "1", At: 200}, {ID: "eula", Version: "2", At: 300}}); err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptTerms("other", []types.TermAcceptance{{ID: "eula", Version: "1", At: 400}}); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got := reopened.AcceptedTerms("g")
	if len(got) != 2 || got[0].At != 100 || got[1].Version != "2" {
		t.Fatalf("accepted for g = %+v, want version 1 (first date kept) and version 2", got)
	}
	if other := reopened.AcceptedTerms("other"); len(other) != 1 || other[0].At != 400 {
		t.Fatalf("accepted for other = %+v", other)
	}
	if none := reopened.AcceptedTerms("nobody"); len(none) != 0 {
		t.Fatalf("a game nobody accepted for has %+v", none)
	}
}

func TestAcceptTermsNeedsAGame(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptTerms("", []types.TermAcceptance{{ID: "a", Version: "1"}}); err == nil {
		t.Fatal("an acceptance with no game was recorded")
	}
}
