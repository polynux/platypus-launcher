package compat

import (
	"os"
	"reflect"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/pcgw/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseExamplePuzzleFixture(t *testing.T) {
	f := ParseWikitext(readFixture(t, "example_puzzle.wiki"))
	if f.SteamAppID != "900001" {
		t.Fatalf("steam appid %q", f.SteamAppID)
	}
	if !reflect.DeepEqual(f.SteamSide, []string{"900002", "900003", "900004", "900005"}) {
		t.Fatalf("side %v", f.SteamSide)
	}
	if !reflect.DeepEqual(f.DRM, []string{"Steam", "Denuvo Anti-Tamper"}) {
		t.Fatalf("drm %v", f.DRM)
	}
	if len(f.Removed) != 0 || f.Online != "" || len(f.AntiCheat) != 0 {
		t.Fatalf("unexpected facts %+v", f)
	}
}

func TestParseExampleHorrorRemovedDRM(t *testing.T) {
	f := ParseWikitext(readFixture(t, "example_horror.wiki"))
	if f.SteamAppID != "900101" {
		t.Fatalf("steam appid %q", f.SteamAppID)
	}
	if !reflect.DeepEqual(f.DRM, []string{"Steam"}) {
		t.Fatalf("current drm %v: the Mac row must not count and Denuvo was removed", f.DRM)
	}
	want := []RemovedDRM{{Name: "Denuvo", Date: "2023-04-10"}}
	if !reflect.DeepEqual(f.Removed, want) {
		t.Fatalf("removed %+v, want %+v", f.Removed, want)
	}
	if f.Online != "always" {
		t.Fatalf("online %q, want always from the offline=false note", f.Online)
	}
	if !reflect.DeepEqual(f.AntiCheat, []string{"Example Guard", "Sentinel Shield"}) {
		t.Fatalf("anticheat %v", f.AntiCheat)
	}
}

func TestParseDRMNotesAndAntiCheat(t *testing.T) {
	text := `{{Infobox game
|steam appid = 42
}}
==Availability==
{{Availability|
{{Availability/row| Steam | 42 | {{DRM|Denuvo Anti-Tamper}} | | | Windows }}
{{Availability/row| GOG.com | 1 | | | | Linux }}
}}
{{ii}} Requires {{DRM|Steam|offline=false}} and is [[Denuvo]] protected.
{{ii}} {{Removed DRM|SecuROM}} was removed from the Windows version on sometime.
{{ii}} {{Removed DRM|StarForce}} was removed from the Linux version on May 1, 2020.
==Other==
{{DRM|Ignored outside the section}}
===Middleware===
{{Middleware
|anticheat = [[Easy Anti-Cheat]], BattlEye, {{Unknown|x}}
}}`
	f := ParseWikitext(text)
	if !reflect.DeepEqual(f.DRM, []string{"Denuvo Anti-Tamper", "Steam"}) {
		t.Fatalf("drm %v", f.DRM)
	}
	if f.Online != "always" {
		t.Fatalf("online %q", f.Online)
	}
	if len(f.Removed) != 0 {
		t.Fatalf("removed %+v: undated and non-Windows removals must be left out", f.Removed)
	}
	if !reflect.DeepEqual(f.AntiCheat, []string{"Easy Anti-Cheat", "BattlEye"}) {
		t.Fatalf("anticheat %v", f.AntiCheat)
	}
}

func TestParseReadsNothingFromBrokenMarkup(t *testing.T) {
	for _, text := range []string{"", "{{Infobox game|steam appid = 7", "==Availability==\n{{Availability/row| Steam | 1 | ", "plain text"} {
		f := ParseWikitext(text)
		if f.SteamAppID != "" || len(f.DRM) != 0 || len(f.AntiCheat) != 0 {
			t.Fatalf("%q gave %+v", text, f)
		}
	}
}
