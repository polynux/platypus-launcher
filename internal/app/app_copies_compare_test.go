package app

import (
	"testing"
)

// The copy comparison (protocol 1.11 kinds, computed by core) on the echo
// "shared" game: Ana's and Ben's copies differ. Ben has the most add-ons, so
// he is the baseline until the user prefers a copy.
func TestSharedGameCopiesAreComparedWithTheBaseline(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	shared := e.libraryGame(t, "shared")
	cmp := shared.CopyCompare
	if cmp == nil || cmp.Baseline != "ben" || len(cmp.Diffs) != 1 {
		t.Fatalf("comparison = %+v, want Ben as the baseline and one diff", cmp)
	}
	ana := cmp.Diffs[0]
	if ana.OwnerID != "ana" || ana.OnlyHere != 1 || ana.OnlyBaseline != 2 || ana.SoundtracksShared != 1 {
		t.Fatalf("ana vs ben = %+v: the expansion is only Ana's, the track pack and art book only Ben's", ana)
	}
}

func TestPreferringACopyMakesItTheBaseline(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	if err := e.a.SetPreferredCopy(e.libraryGame(t, "shared"), "ana"); err != nil {
		t.Fatal(err)
	}
	cmp := e.libraryGame(t, "shared").CopyCompare
	if cmp == nil || cmp.Baseline != "ana" || cmp.Diffs[0].OwnerID != "ben" {
		t.Fatalf("comparison = %+v, want Ana (preferred) as the baseline", cmp)
	}
	if d := cmp.Diffs[0]; d.OnlyHere != 2 || d.OnlyBaseline != 1 || d.SoundtracksShared != 1 {
		t.Fatalf("ben vs ana = %+v", d)
	}
}

// A game the user owns has no copies, so nothing is compared.
func TestOwnGamesHaveNoCopyComparison(t *testing.T) {
	e := familyApp(t, familyProtocols, familyCaps)
	if own := e.libraryGame(t, "tiny"); own.CopyCompare != nil || own.Copies != nil {
		t.Fatalf("an own game compared: %+v", own.CopyCompare)
	}
}
