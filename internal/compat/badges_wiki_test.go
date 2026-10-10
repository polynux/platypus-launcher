package compat

import (
	"testing"
	"time"

	"platypus/internal/types"
)

var wikiChecked = time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)

func kinds(bs []types.CompatBadge) []string {
	out := []string{}
	for _, b := range bs {
		out = append(out, b.Kind+":"+b.Label)
	}
	return out
}

func TestWikiDRMBadgesShowCurrentAndRemovedWithTheSource(t *testing.T) {
	rec := WikiRecord{
		DRM:     []string{"Steam", "Denuvo Anti-Tamper"},
		Online:  "always",
		Removed: []RemovedDRM{{Name: "Denuvo", Date: "2023-04-10"}},
	}
	got := WikiDRMBadges(rec, wikiChecked, false)
	want := []string{"drm:Steam", "denuvo:Denuvo Anti-Tamper", "online:Always online", "drm-removed:Denuvo removed"}
	if !equalStrings(kinds(got), want) {
		t.Fatalf("badges %v, want %v", kinds(got), want)
	}
	for _, b := range got {
		if b.Source != "PCGamingWiki" || b.CheckedAt != wikiChecked.Unix() {
			t.Fatalf("badge %+v: want the PCGamingWiki source and the check date", b)
		}
	}
}

func TestWikiDRMBadgesYieldToThePluginsOwnDRMButKeepHistory(t *testing.T) {
	rec := WikiRecord{DRM: []string{"Steam"}, Online: "always", Removed: []RemovedDRM{{Name: "Denuvo", Date: "2023-04-10"}}}
	got := WikiDRMBadges(rec, wikiChecked, true)
	want := []string{"drm-removed:Denuvo removed"}
	if !equalStrings(kinds(got), want) {
		t.Fatalf("badges %v, want %v", kinds(got), want)
	}
}

func TestWikiAntiCheatBadgesNameTheSystem(t *testing.T) {
	got := WikiAntiCheatBadges(WikiRecord{AntiCheat: []string{"Easy Anti-Cheat", "BattlEye"}}, wikiChecked)
	want := []string{"anticheat:Easy Anti-Cheat", "anticheat:BattlEye"}
	if !equalStrings(kinds(got), want) {
		t.Fatalf("badges %v, want %v", kinds(got), want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
