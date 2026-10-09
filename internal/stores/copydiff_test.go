package stores

import (
	"testing"

	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

func dlc(kind string, ids ...string) []types.GameDLC {
	var out []types.GameDLC
	for _, id := range ids {
		out = append(out, types.GameDLC{ID: id, Name: id, Kind: kind})
	}
	return out
}

func copyOf(owner string, dl ...types.GameDLC) types.GameCopy {
	return types.GameCopy{OwnerID: owner, OwnerName: owner, Available: true, DLC: dl}
}

// The preferred copy is the baseline; the counts say what each side has that
// the other lacks, soundtracks apart.
func TestCopiesAreComparedWithThePreferredCopy(t *testing.T) {
	ana := copyOf("ana", append(dlc("dlc", "expansion"), dlc("soundtrack", "ost")...)...)
	ben := copyOf("ben", append(dlc("dlc", "track"), append(dlc("soundtrack", "ost"), dlc("other", "book")...)...)...)
	cmp := compareCopies([]types.GameCopy{ana, ben}, "ana")
	if cmp == nil || cmp.Baseline != "ana" || len(cmp.Diffs) != 1 {
		t.Fatalf("comparison = %+v, want Ana as the baseline and one diff", cmp)
	}
	d := cmp.Diffs[0]
	if d.OwnerID != "ben" || d.OnlyHere != 2 || d.OnlyBaseline != 1 || d.Shared != 0 {
		t.Errorf("ben vs ana = %+v: track and book are only Ben's, the expansion only Ana's", d)
	}
	if d.SoundtracksShared != 1 || d.SoundtracksOnlyHere != 0 || d.SoundtracksOnlyBaseline != 0 {
		t.Errorf("soundtracks = %+v, want the one soundtrack shared", d)
	}
}

func TestWithoutAPreferenceTheCopyWithMostAddOnsIsTheBaseline(t *testing.T) {
	a := copyOf("ana", dlc("dlc", "x")...)
	b := copyOf("ben", dlc("dlc", "x", "y")...)
	cmp := compareCopies([]types.GameCopy{a, b}, "")
	if cmp.Baseline != "ben" || cmp.Diffs[0].OwnerID != "ana" || cmp.Diffs[0].OnlyBaseline != 1 {
		t.Fatalf("comparison = %+v, want Ben (the most add-ons) as the baseline", cmp)
	}
}

func TestATieGoesToTheLowerOwnerID(t *testing.T) {
	cmp := compareCopies([]types.GameCopy{copyOf("ben", dlc("dlc", "x")...), copyOf("ana", dlc("dlc", "y")...)}, "")
	if cmp.Baseline != "ana" {
		t.Fatalf("baseline = %q, want the lower owner id on a tie", cmp.Baseline)
	}
}

func TestIdenticalCopiesHaveNoDifference(t *testing.T) {
	cmp := compareCopies([]types.GameCopy{copyOf("ana", dlc("dlc", "x")...), copyOf("ben", dlc("dlc", "x")...)}, "ana")
	if d := cmp.Diffs[0]; d.OnlyHere+d.OnlyBaseline+d.SoundtracksOnlyHere+d.SoundtracksOnlyBaseline != 0 || d.Shared != 1 {
		t.Fatalf("identical copies: %+v", d)
	}
}

func TestASoundtrackOnlyDifferenceIsCountedApart(t *testing.T) {
	ana := copyOf("ana", dlc("dlc", "x")...)
	ben := copyOf("ben", append(dlc("dlc", "x"), dlc("soundtrack", "ost")...)...)
	d := compareCopies([]types.GameCopy{ana, ben}, "ana").Diffs[0]
	if d.SoundtracksOnlyHere != 1 || d.OnlyHere != 0 || d.OnlyBaseline != 0 {
		t.Fatalf("ben's extra soundtrack counted as game DLC, or not as a soundtrack: %+v", d)
	}
	ben2 := copyOf("ben", dlc("dlc", "x")...)
	ana2 := copyOf("ana", append(dlc("dlc", "x"), dlc("soundtrack", "ost")...)...)
	d = compareCopies([]types.GameCopy{ben2, ana2}, "ben").Diffs[0]
	if d.SoundtracksOnlyHere != 1 || d.OnlyHere != 0 || d.OnlyBaseline != 0 {
		t.Fatalf("ana's soundtrack vs ben's copy = %+v, want one soundtrack only Ana has", d)
	}
}

// Nothing to compare: a single copy, or no copies at all (an own game).
func TestOneCopyOrNoneHasNothingToCompare(t *testing.T) {
	if cmp := compareCopies([]types.GameCopy{copyOf("ana")}, "ana"); cmp != nil {
		t.Fatalf("one copy compared: %+v", cmp)
	}
	if cmp := compareCopies(nil, ""); cmp != nil {
		t.Fatalf("no copies compared: %+v", cmp)
	}
}

// DLC kinds: the ones the UI knows are kept; anything else is game DLC.
func TestDLCKindsAreNormalized(t *testing.T) {
	copies, _ := normalizeCopies([]plugins.LibraryCopy{{
		OwnerID: "ana", Exclude: "  no expansion  ",
		DLC: []plugins.LibraryDLC{
			{ID: "a", Name: "A"}, {ID: "b", Name: "B", Kind: "soundtrack"},
			{ID: "c", Name: "C", Kind: "other"}, {ID: "d", Name: "D", Kind: "from-the-future"},
		},
	}}, "")
	got := copies[0]
	if got.Exclude != "no expansion" {
		t.Errorf("exclude = %q", got.Exclude)
	}
	var kinds []string
	for _, d := range got.DLC {
		kinds = append(kinds, d.Kind)
	}
	if want := []string{"dlc", "soundtrack", "other", "dlc"}; len(kinds) != 4 || kinds[0] != want[0] || kinds[1] != want[1] || kinds[2] != want[2] || kinds[3] != want[3] {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
}
