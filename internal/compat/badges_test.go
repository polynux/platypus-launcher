package compat

import (
	"testing"
	"time"
)

func TestLocalBadgesSayPresenceNotEnforcement(t *testing.T) {
	got := LocalBadges([]LocalHit{
		{Kind: "anticheat", Label: "BattlEye", Name: "BEClient_x64.dll"},
		{Kind: "eos", Label: "Epic Online Services", Name: "EOSBootstrapper.exe"},
	}, time.Unix(1, 0))
	if len(got) != 2 || got[0].Source != "Installed files" {
		t.Fatalf("local badges %+v", got)
	}
	if got[1].Detail == "" || got[1].Kind != "eos" {
		t.Fatalf("eos badge %+v", got[1])
	}
}
