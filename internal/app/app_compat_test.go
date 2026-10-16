package app

import (
	"path/filepath"
	"testing"
	"time"

	"platypus/internal/plugins"
	"platypus/internal/types"
)

func TestStoreDRMIsMappedInCore(t *testing.T) {
	trueValue := true
	got := storeDRMBadges(&types.DRM{DRMFree: &trueValue, Launcher: "ubisoft-connect"}, "GOG", time.Unix(10, 0))
	if len(got) != 2 || got[0].Kind != "drm-free" || got[1].Label != "Needs Ubisoft Connect" || got[1].Source != "GOG" {
		t.Fatalf("store DRM badges = %+v", got)
	}
}

func TestOfflineFactDoesNotSatisfyOnlineAndKeepsOnlineBadges(t *testing.T) {
	now := time.Now()
	a := &App{compatFacts: &compatFactsCache{path: filepath.Join(t.TempDir(), "compat-facts-cache.json"), facts: map[string]map[string]compatFactEntry{"linux-compat": {
		"g": {Fact: plugins.CompatFact{ID: "g", Badges: []plugins.CompatBadge{{Kind: "rating", Label: "Platinum", Source: "ProtonDB"}}}, SteamAppID: "620", Expires: now.Add(time.Hour), Online: true, InstallDir: "/games/g"},
	}}}}
	if compatFactFresh(a.compatFacts.facts["linux-compat"]["g"], "620", "/games/g", false, now) != true {
		t.Fatal("online result should satisfy offline lookup")
	}
	if compatFactFresh(compatFactEntry{Fact: plugins.CompatFact{ID: "g"}, SteamAppID: "620", Expires: now.Add(time.Hour), InstallDir: "/games/g"}, "620", "/games/g", true, now) {
		t.Fatal("offline result satisfied online lookup")
	}
	a.compatFacts.mu.Lock()
	a.cacheCompatFactResultLocked("linux-compat", []plugins.CompatLookupGame{{ID: "g", SteamAppID: "620", InstallDir: "/games/g", Online: false}}, []plugins.CompatFact{{ID: "g", Badges: []plugins.CompatBadge{{Kind: "anticheat", Label: "BattlEye", Source: "Installed files"}}, TTLSeconds: 600}}, now)
	a.compatFacts.mu.Unlock()
	fact, _ := a.cachedProviderFact("g", "620")
	if len(fact.Badges) != 2 {
		t.Fatalf("merged offline facts = %+v", fact.Badges)
	}
}
