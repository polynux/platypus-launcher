package compat

import (
	"testing"
	"time"

	"platypus/internal/types"
)

func boolp(b bool) *bool { return &b }

func TestStoreDRMMapsEachFieldOrNothing(t *testing.T) {
	checked := time.Unix(1_700_000_000, 0)
	all := StoreDRM(&types.DRM{
		DRMFree: boolp(true), OnlineRequired: boolp(true), Launcher: "ubisoft-connect", DenuvoLikely: boolp(true), Notes: "x",
	}, "Echo", checked)
	kinds := map[string]string{}
	for _, b := range all {
		kinds[b.Kind] = b.Label
		if b.Source != "Echo" || b.CheckedAt != checked.Unix() {
			t.Fatalf("badge %+v lacks its source or date", b)
		}
	}
	want := map[string]string{
		"drm-free": "DRM-free", "online": "Online required", "launcher": "Needs Ubisoft Connect", "denuvo": "Denuvo likely",
	}
	for k, v := range want {
		if kinds[k] != v {
			t.Errorf("%s = %q, want %q (all %+v)", k, kinds[k], v, all)
		}
	}
	// Unknown stays unknown: nil and false fields give no badge.
	if got := StoreDRM(&types.DRM{DRMFree: boolp(false), OnlineRequired: boolp(false), DenuvoLikely: boolp(false)}, "Echo", checked); len(got) != 0 {
		t.Fatalf("false or unknown facts made badges: %+v", got)
	}
	if got := StoreDRM(nil, "Echo", checked); got != nil {
		t.Fatalf("nil facts gave %+v", got)
	}
}

func TestStoreDRMKeepsUnknownLauncherNamesAsWritten(t *testing.T) {
	got := StoreDRM(&types.DRM{Launcher: "some-new-client"}, "Echo", time.Now())
	if len(got) != 1 || got[0].Label != "Needs some-new-client" {
		t.Fatalf("launcher badge %+v", got)
	}
}

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
