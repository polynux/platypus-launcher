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

func TestOfflineFactDoesNotSatisfyOnlineAndReplacesSnapshot(t *testing.T) {
	now := time.Now()
	rating := plugins.CompatFact{ID: "g", Badges: []plugins.CompatBadge{{Kind: "rating", Label: "Platinum", Source: "ProtonDB"}}}
	a := &App{compatFacts: &compatFactsCache{path: filepath.Join(t.TempDir(), "compat-facts-cache.json"), facts: map[string]map[string]compatFactEntry{"linux-compat": {
		"g": {Fact: rating, SteamAppID: "620", Expires: now.Add(time.Hour), Online: true, InstallDir: "/games/g"},
	}}}}
	if compatFactFresh(a.compatFacts.facts["linux-compat"]["g"], "620", "/games/g", false, false, now) != true {
		t.Fatal("online result should satisfy offline lookup")
	}
	if compatFactFresh(compatFactEntry{Fact: rating, SteamAppID: "620", Expires: now.Add(time.Hour), InstallDir: "/games/g"}, "620", "/games/g", true, false, now) {
		t.Fatal("offline result satisfied online lookup")
	}
	a.compatFacts.mu.Lock()
	a.cacheCompatFactResultLocked("linux-compat", []plugins.CompatLookupGame{{ID: "g", SteamAppID: "620", InstallDir: "/games/g", Online: false}}, []plugins.CompatFact{{ID: "g", Badges: []plugins.CompatBadge{{Kind: "anticheat", Label: "BattlEye", Source: "Installed files"}}, TTLSeconds: 600}}, now)
	a.compatFacts.mu.Unlock()
	fact, _ := a.cachedProviderFact("g", "620")
	if len(fact.Badges) != 1 || fact.Badges[0].Label != "BattlEye" {
		t.Fatalf("offline snapshot = %+v", fact.Badges)
	}
}

// An answer fetched with only the protection switch on is an online snapshot
// (issue #150): it must not be replaced by the next offline answer, and it
// goes stale as soon as the ratings switch comes on.
func TestProtectionOnlyAnswerIsAnOnlineSnapshot(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := &App{compatFacts: &compatFactsCache{path: filepath.Join(t.TempDir(), "compat-facts-cache.json"), facts: map[string]map[string]compatFactEntry{"linux-compat": {}}}}
	req := plugins.CompatLookupGame{ID: "g", SteamAppID: "620", InstallDir: "/games/g", Online: false, ProtectionOnline: true}
	a.compatFacts.mu.Lock()
	a.cacheCompatFactResultLocked("linux-compat", []plugins.CompatLookupGame{req}, []plugins.CompatFact{{ID: "g", Badges: []plugins.CompatBadge{{Kind: "anticheat", Label: "BattlEye", Source: "AreWeAntiCheatYet"}}, TTLSeconds: 7 * 24 * 3600}}, now)
	a.compatFacts.mu.Unlock()
	entry := a.compatFacts.facts["linux-compat"]["g"]
	if !lookupOnline(req) || !entry.protection() {
		t.Fatal("an answer fetched with protection online was saved as an offline snapshot")
	}
	if !compatFactFresh(entry, "620", "/games/g", false, true, now.Add(time.Minute)) {
		t.Fatal("the protection-only answer was not fresh for the next protection lookup")
	}
	if compatFactFresh(entry, "620", "/games/g", true, true, now.Add(time.Minute)) {
		t.Fatal("the protection-only answer stayed fresh once ratings were switched on")
	}
}

// Switching a source on makes an answer that came from the other one stale
// (issue #150): otherwise the new facts would wait for the provider's TTL, up
// to 7 days. Entries saved before the split followed the one old switch.
func TestEnablingACompatSourceMakesTheAnswerStale(t *testing.T) {
	t.Parallel()
	now := time.Now()
	fact := plugins.CompatFact{ID: "g", Badges: []plugins.CompatBadge{{Kind: "rating", Label: "Platinum", Source: "ProtonDB"}}}
	fetched, notFetched := true, false

	a := &App{compatFacts: &compatFactsCache{path: filepath.Join(t.TempDir(), "compat-facts-cache.json"), facts: map[string]map[string]compatFactEntry{"linux-compat": {}}}}
	ratingsOnly := plugins.CompatLookupGame{ID: "g", SteamAppID: "620", Online: true, ProtectionOnline: false}
	a.compatFacts.mu.Lock()
	a.cacheCompatFactResultLocked("linux-compat", []plugins.CompatLookupGame{ratingsOnly}, []plugins.CompatFact{fact}, now)
	a.compatFacts.mu.Unlock()
	entry := a.compatFacts.facts["linux-compat"]["g"]
	if entry.protection() {
		t.Fatal("the cache recorded protection as fetched without the switch")
	}
	if !compatFactFresh(entry, "620", "", true, false, now.Add(time.Minute)) {
		t.Fatal("a ratings answer is not fresh for the next ratings lookup")
	}
	if compatFactFresh(entry, "620", "", true, true, now.Add(time.Minute)) {
		t.Fatal("a ratings-only answer stayed fresh once protection was switched on")
	}

	protectionOnly := compatFactEntry{Fact: fact, SteamAppID: "620", Expires: now.Add(time.Hour), Online: false, Protection: &fetched}
	if !compatFactFresh(protectionOnly, "620", "", false, true, now) {
		t.Fatal("a protection answer is not fresh for the next protection lookup")
	}
	if compatFactFresh(protectionOnly, "620", "", true, true, now) {
		t.Fatal("a protection-only answer stayed fresh once ratings were switched on")
	}

	// Before the split the entry had no Protection flag: it followed Online.
	old := compatFactEntry{Fact: fact, SteamAppID: "620", Expires: now.Add(time.Hour), Online: true}
	if !compatFactFresh(old, "620", "", true, true, now) {
		t.Fatal("an old online answer is not fresh while both switches are on")
	}
	oldOff := compatFactEntry{Fact: fact, SteamAppID: "620", Expires: now.Add(time.Hour), Online: false, Protection: &notFetched}
	if compatFactFresh(oldOff, "620", "", false, true, now) {
		t.Fatal("an old offline answer is fresh once protection is switched on")
	}
}
