package stores

import (
	"strings"
	"testing"

	"platypus/internal/plugins"
)

// Copies are the store's words, cut down to what the picker can show: an
// entry with no owner is dropped, names fall back to ids, duplicates go, and
// a preferred copy that isn't listed is cleared.
func TestCopiesAreNormalized(t *testing.T) {
	long := strings.Repeat("x", 500)
	in := []plugins.LibraryCopy{
		{OwnerID: "", OwnerName: "nobody", Available: true},
		{OwnerID: "ana", OwnerName: "  Ana  ", Available: true, DLC: []plugins.LibraryDLC{{ID: "", Name: "no id"}, {ID: "sound", Name: ""}, {ID: "sound", Name: "again"}}},
		{OwnerID: "ana", OwnerName: "Ana twice", Available: false},
		{OwnerID: "ben", OwnerName: long, Note: long, Available: true, InUse: true},
	}
	copies, pref := normalizeCopies(in, "ben")
	if len(copies) != 2 {
		t.Fatalf("copies = %+v, want Ana and Ben once each", copies)
	}
	ana, ben := copies[0], copies[1]
	if ana.OwnerName != "Ana" || !ana.Available || len(ana.DLC) != 1 || ana.DLC[0].Name != "sound" {
		t.Errorf("ana = %+v: the first entry wins, names are trimmed and fall back to the id", ana)
	}
	if got := len([]rune(ben.OwnerName)); got != maxCopyText {
		t.Errorf("ben's name is %d runes, want it cut to %d", got, maxCopyText)
	}
	if !ben.InUse || len([]rune(ben.Note)) != maxCopyText {
		t.Errorf("ben = %+v", ben)
	}
	if pref != "ben" {
		t.Fatalf("preferred = %q, want ben", pref)
	}
	if _, pref := normalizeCopies(in, "carl"); pref != "" {
		t.Fatalf("a preferred copy nobody lends was kept: %q", pref)
	}
	if copies, _ := normalizeCopies(nil, "ana"); len(copies) != 0 {
		t.Fatalf("no copies in: %+v", copies)
	}
}

func TestCopyLimitsHold(t *testing.T) {
	var in []plugins.LibraryCopy
	for i := 0; i < maxCopies+5; i++ {
		in = append(in, plugins.LibraryCopy{OwnerID: string(rune('a' + i)), Available: true})
	}
	if copies, _ := normalizeCopies(in, ""); len(copies) != maxCopies {
		t.Fatalf("%d copies kept, want %d", len(copies), maxCopies)
	}
	var dlc []plugins.LibraryDLC
	for i := 0; i < maxCopyDLC+5; i++ {
		dlc = append(dlc, plugins.LibraryDLC{ID: strings.Repeat("d", i+1)})
	}
	copies, _ := normalizeCopies([]plugins.LibraryCopy{{OwnerID: "ana", DLC: dlc}}, "")
	if len(copies[0].DLC) != maxCopyDLC || copies[0].DLC[0].ID != "d" {
		t.Fatalf("%d DLC kept, want %d", len(copies[0].DLC), maxCopyDLC)
	}
}
