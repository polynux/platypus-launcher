package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"platypus/internal/plugins"
	"platypus/internal/settings"
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

// compatProviderApp is an App with one compat-facts provider (echo's program)
// whose manifest promises the given protocol versions, with the ratings switch
// on and the protection switch as asked.
func compatProviderApp(t *testing.T, versions []string, protection bool) *App {
	t.Helper()
	root := t.TempDir()
	trust, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := trust.SetRatingsOnline(true); err != nil {
		t.Fatal(err)
	}
	if err := trust.SetProtectionOnline(protection); err != nil {
		t.Fatal(err)
	}
	pluginsDir := filepath.Join(root, "plugins")
	installEchoApp(t, pluginsDir)
	list, _ := json.Marshal(versions)
	manifest := `{"id":"echo","name":"Echo","version":"1.0.0","manifestVersion":1,` +
		`"protocol":{"versions":` + string(list) + `},"store":"echo","exec":"bin/platypus-echo-app",` +
		`"capabilities":["compat-facts"]}`
	if err := os.WriteFile(filepath.Join(pluginsDir, "echo", "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	m := plugins.NewManager(plugins.Options{
		Dir: pluginsDir, LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"),
		Settings: trust, Home: root,
	})
	t.Cleanup(m.Shutdown)
	if _, err := m.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := m.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	a := &App{host: stderrHost{}, settings: trust, plugins: m, optHome: root,
		compatFacts: openCompatFactsCache(filepath.Join(root, "compat-facts.json"))}
	return a
}

// queuedCompatRequest asks for one game while the provider is busy and returns
// the request core built for it: the pending batch keeps it as it will be sent.
func queuedCompatRequest(t *testing.T, a *App, game types.Game) plugins.CompatLookupGame {
	t.Helper()
	a.compatFacts.mu.Lock()
	a.compatFacts.inFlight["echo"] = true
	a.compatFacts.mu.Unlock()
	a.requestCompatFacts([]types.Game{game}, nil)
	a.compatFacts.mu.Lock()
	defer a.compatFacts.mu.Unlock()
	req, ok := a.compatFacts.pending["echo"][game.ID]
	if !ok {
		t.Fatalf("the lookup of %s was not queued: %+v", game.ID, a.compatFacts.pending["echo"])
	}
	return req
}

// A provider that predates the split gate every source on `online` (issue
// #162): it gets the intersection of both switches, so switching protection
// off keeps it off the network instead of letting it fetch everything.
func TestPreSplitCompatProviderGetsTheIntersectionOfBothSwitches(t *testing.T) {
	t.Parallel()
	a := compatProviderApp(t, []string{"1.0", "1.17"}, false)
	req := queuedCompatRequest(t, a, types.Game{ID: "g", Source: "echo", Store: "echo"})
	if req.Online || req.ProtectionOnline {
		t.Fatalf("pre-split request = online %v protection %v, want both off", req.Online, req.ProtectionOnline)
	}
}

// A provider that advertises the split (1.18) gets the two switches apart: it
// still fetches ratings while the protection switch is off.
func TestSplitCompatProviderGetsBothSwitchesApart(t *testing.T) {
	t.Parallel()
	a := compatProviderApp(t, []string{"1.0", "1.17", "1.18"}, false)
	req := queuedCompatRequest(t, a, types.Game{ID: "g", Source: "echo", Store: "echo"})
	if !req.Online || req.ProtectionOnline {
		t.Fatalf("split request = online %v protection %v, want ratings on, protection off", req.Online, req.ProtectionOnline)
	}
}

// A game asked for while a provider's batch is in flight waits for that batch
// to end and is looked up right after (S3): it must not wait for the next
// library load, which can be minutes away.
func TestQueuedCompatLookupIsAskedAfterTheRunningBatch(t *testing.T) {
	t.Parallel()
	a := compatProviderApp(t, []string{"1.0", "1.17", "1.18"}, true)
	game := types.Game{ID: "p.echo.queued", Source: "echo", Store: "echo"}

	a.compatFacts.mu.Lock()
	a.compatFacts.inFlight["echo"] = true
	a.compatFacts.mu.Unlock()
	a.requestCompatFacts([]types.Game{game}, nil)

	// The batch in flight ends with another game: the queued one follows.
	a.refreshCompatFacts("echo", []plugins.CompatLookupGame{{ID: "p.echo.running", Store: "echo"}})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.compatFacts.mu.Lock()
		entry := a.compatFacts.facts["echo"]["p.echo.queued"]
		left := len(a.compatFacts.pending["echo"])
		a.compatFacts.mu.Unlock()
		if !entry.RetryAt.IsZero() && left == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the queued game was never looked up")
}

// The compat answer carries both online switches (issue #162), so a screen can
// tell why one kind of fact is missing and the other isn't.
func TestGetGameCompatReportsBothOnlineSwitches(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetRatingsOnline(true); err != nil {
		t.Fatal(err)
	}
	if err := st.SetProtectionOnline(false); err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st}
	got := a.GetGameCompat(types.Game{ID: "g", Source: types.SourceCustom})
	if !got.RatingsOnline || got.ProtectionOnline {
		t.Fatalf("compat = ratings %v protection %v, want ratings on, protection off", got.RatingsOnline, got.ProtectionOnline)
	}
}
