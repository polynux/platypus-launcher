package main

import (
	"testing"

	"hydra-launcher/internal/types"
)

func TestNewAntiCheatKeepsTheCatalogueBadgeAndAddsOnlyNewNames(t *testing.T) {
	have := []types.CompatBadge{{Kind: "anticheat", Label: "Easy Anti-Cheat", Detail: "Linux: supported."}}
	add := []types.CompatBadge{{Kind: "anticheat", Label: "easy anti-cheat"}, {Kind: "anticheat", Label: "BattlEye"}}
	got := newAntiCheat(have, add)
	if len(got) != 1 || got[0].Label != "BattlEye" {
		t.Fatalf("got %+v, want only BattlEye", got)
	}
}
