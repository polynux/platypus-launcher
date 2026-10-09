package types

import (
	"errors"
	"slices"
	"testing"
)

func testGroups() []OptionGroup {
	return []OptionGroup{
		{ID: "language", Kind: OptionSelect, Default: "en", Values: []OptionValue{
			{ID: "en", Owned: true}, {ID: "fr", Owned: true}}},
		{ID: "dlc", Kind: OptionMulti, DLC: true, Values: []OptionValue{
			{ID: "pass", Label: "Season Pass", SizeBytes: 100, Owned: true},
			{ID: "deluxe", Label: "Deluxe", SizeBytes: 50, Owned: false},
			{ID: "old", Label: "Old pack", SizeBytes: 7, Installed: true}}},
		{ID: "packs", Kind: OptionMulti, Values: []OptionValue{
			{ID: "hd", SizeBytes: 300}, {ID: "docs", SizeBytes: 3}}},
		{ID: "subs", Kind: OptionBool, Default: true},
	}
}

func TestNormalizeIgnoresUnknownKindsAndBrokenGroups(t *testing.T) {
	in := []OptionGroup{
		{ID: "carousel", Kind: "carousel", Values: []OptionValue{{ID: "x"}}},
		{ID: "", Kind: OptionSelect, Values: []OptionValue{{ID: "x"}}},
		{ID: "empty", Kind: OptionMulti},
		{ID: "lang", Kind: OptionSelect, Default: "nope", Values: []OptionValue{{ID: "de"}, {ID: "de"}, {ID: "it"}}},
		{ID: "lang", Kind: OptionMulti, Values: []OptionValue{{ID: "dup"}}},
		{ID: "bad-url", Kind: OptionMulti, Values: []OptionValue{{ID: "a", StoreURL: "javascript:alert(1)", SizeBytes: -4}}},
	}
	got := NormalizeGroups(in)
	if len(got) != 2 || got[0].ID != "lang" || got[1].ID != "bad-url" {
		t.Fatalf("normalized to %+v", got)
	}
	if got[0].Default != "de" || len(got[0].Values) != 2 || got[0].Values[0].Label != "de" {
		t.Fatalf("select default not the first value, or duplicates kept: %+v", got[0])
	}
	if v := got[1].Values[0]; v.StoreURL != "" || v.SizeBytes != 0 {
		t.Fatalf("store url kept or size negative: %+v", v)
	}
}

func TestResolveFillsDefaultsForEveryGroup(t *testing.T) {
	got, err := ResolveOptions(testGroups(), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if got["language"] != "en" || got["subs"] != true {
		t.Fatalf("defaults = %v", got)
	}
	if list, ok := got["dlc"].([]string); !ok || len(list) != 0 {
		t.Fatalf("multi default = %#v, want an empty list", got["dlc"])
	}
}

func TestStrictRefusesUnownedDLCAndUnknownChoices(t *testing.T) {
	groups := testGroups()
	cases := map[string]InstallOptions{
		"unowned dlc":     {"dlc": []any{"deluxe"}},
		"unknown value":   {"language": "xx"},
		"unknown dlc":     {"dlc": []any{"nope"}},
		"wrong type":      {"subs": "yes"},
		"list for select": {"language": []any{"en"}},
		"non-string item": {"packs": []any{42}},
	}
	for name, choices := range cases {
		if _, err := ResolveOptions(groups, choices, true); err == nil {
			t.Errorf("%s: strict resolve accepted %v", name, choices)
		}
	}
	_, err := ResolveOptions(groups, InstallOptions{"dlc": []any{"deluxe"}}, true)
	if !errors.Is(err, ErrNotOwned) {
		t.Fatalf("unowned dlc error = %v, want ErrNotOwned", err)
	}
}

func TestUnownedButInstalledDLCStaysSelectable(t *testing.T) {
	got, err := ResolveOptions(testGroups(), InstallOptions{"dlc": []any{"old", "pass"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got["dlc"].([]string), []string{"pass", "old"}) {
		t.Fatalf("dlc = %v, want the store's order", got["dlc"])
	}
}

func TestLenientDropsWhatNoLongerFits(t *testing.T) {
	got, err := ResolveOptions(testGroups(), InstallOptions{
		"language": "xx", "dlc": []any{"deluxe", "pass"}, "subs": "yes", "gone": true,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got["language"] != "en" || got["subs"] != true || hasKey(got, "gone") {
		t.Fatalf("lenient resolve = %v", got)
	}
	if !slices.Equal(got["dlc"].([]string), []string{"pass"}) {
		t.Fatalf("dlc = %v, want only the owned one", got["dlc"])
	}
}

func hasKey(m InstallOptions, k string) bool { _, ok := m[k]; return ok }

func TestOptionsSizeCountsOnlyChosenValues(t *testing.T) {
	groups := testGroups()
	choices, err := ResolveOptions(groups, InstallOptions{"dlc": []any{"pass", "old"}, "packs": []any{"hd"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := OptionsSize(groups, choices); got != 100+7+300 {
		t.Fatalf("size = %d, want 407", got)
	}
	// The same size after a round trip through JSON-shaped values.
	trip := InstallOptions{"packs": []any{"docs", "hd"}}
	if got := OptionsSize(groups, trip); got != 303 {
		t.Fatalf("round-tripped size = %d, want 303", got)
	}
}
