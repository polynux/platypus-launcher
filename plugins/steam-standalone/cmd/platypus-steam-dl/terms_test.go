package main

import (
	"os"
	"reflect"
	"testing"

	"platypus/internal/steam"
)

// The fixture is a real PICS app info excerpt (Trackmania 2225070, captured
// live 2026-11-15): common/eulas holds the EULA whose version acceptance is
// keyed on.
func TestParseEULAsFromLiveAppInfo(t *testing.T) {
	raw, err := os.ReadFile("testdata/appinfo_2225070.vdf")
	if err != nil {
		t.Fatal(err)
	}
	kv, err := steam.ParseKV(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	app := kv.Get("appinfo") // what ProductInfo hands back
	if app == nil {
		t.Fatal("fixture lost its appinfo section")
	}
	got := parseEULAs(app)
	want := []eulaTerm{{
		ID:      "2225070_eula_0",
		Title:   "Trackmania EULA",
		URL:     "https://store.steampowered.com//eula/2225070_eula_0",
		Version: "4",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("eulas = %+v, want %+v", got, want)
	}
}

// common/eulas is an indexed object: keep the store's order (numerically, so
// "10" follows "2"), and drop entries acceptance couldn't be keyed on.
func TestParseEULASOrdersAndSkipsIncomplete(t *testing.T) {
	kv, err := steam.ParseKV(`"common" { "eulas" {
		"10" { "id" "second" "name" "Second" "version" "1" }
		"2"  { "id" "first" "name" "First" "version" "2" }
		"3"  { "id" "no-version" "name" "No version" }
		"4"  { "version" "1" }
	} }`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, term := range parseEULAs(kv) {
		ids = append(ids, term.ID)
	}
	if want := []string{"first", "second"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("eulas = %v, want %v", ids, want)
	}
}

// Most apps list none: game info with no common/eulas has no terms.
func TestParseEULAsNone(t *testing.T) {
	kv, err := steam.ParseKV(`"common" { "name" "Rocket League" "type" "Game" }`)
	if err != nil {
		t.Fatal(err)
	}
	if got := parseEULAs(kv); len(got) != 0 {
		t.Fatalf("eulas = %+v, want none", got)
	}
}
