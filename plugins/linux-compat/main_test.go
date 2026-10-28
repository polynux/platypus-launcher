package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"platypus/internal/compat"
	"platypus/internal/types"
)

type ratingLookupFunc func(context.Context, string) (compat.Summary, error)

func (f ratingLookupFunc) Summary(ctx context.Context, id string) (compat.Summary, error) {
	return f(ctx, id)
}

type deckLookupFunc func(context.Context, string) (compat.DeckReport, error)

func (f deckLookupFunc) Report(ctx context.Context, id string) (compat.DeckReport, error) {
	return f(ctx, id)
}

type wikiLookupFunc func(context.Context, string) (compat.WikiRecord, error)

func (f wikiLookupFunc) Lookup(ctx context.Context, id string) (compat.WikiRecord, error) {
	return f(ctx, id)
}

func TestInitializeNegotiatesCompatFactsProtocol(t *testing.T) {
	params, _ := json.Marshal(map[string]any{"protocols": []string{"1.17"}})
	resp := handle(context.Background(), message{Method: "initialize", Params: params})
	result := resp.Result.(map[string]any)
	if result["protocol"] != "1.17" {
		t.Fatalf("protocol = %v, want 1.17", result["protocol"])
	}
	// 1.18 splits the online state of compat.lookup: offering it wins.
	params, _ = json.Marshal(map[string]any{"protocols": []string{"1.6", "1.17", "1.18"}})
	resp = handle(context.Background(), message{Method: "initialize", Params: params})
	result = resp.Result.(map[string]any)
	if result["protocol"] != "1.18" {
		t.Fatalf("protocol = %v, want 1.18", result["protocol"])
	}
}

func TestLookupReturnsProtonDBBadgeBySteamAppID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/reports/summaries/620.json" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"tier":"platinum","confidence":"strong","total":343}`))
	}))
	defer srv.Close()
	old := proton
	proton = &compat.ProtonDB{Fetch: compat.NewFetcher("test"), Base: srv.URL + "/api/v1/reports/summaries/"}
	defer func() { proton = old }()

	params, _ := json.Marshal(lookupParams{Games: []game{{ID: "gog_1", Store: "gog", StoreAppID: "1", SteamAppID: "620", Online: true}}})
	got, rpcErr := lookup(context.Background(), params)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	result := got.(map[string]any)["facts"].([]fact)
	if len(result) != 1 || result[0].ID != "gog_1" || len(result[0].Badges) != 1 {
		t.Fatalf("facts = %#v", got)
	}
	b := result[0].Badges[0]
	if b.Kind != "rating" || b.Label != "Platinum" || b.Tone != "platinum" || b.Source != "ProtonDB" || b.Link != "https://www.protondb.com/app/620" {
		t.Fatalf("badge = %+v", b)
	}
	if result[0].Report == nil || result[0].Report.State != "rated" || result[0].Report.Count != 343 || result[0].Report.Tier != "platinum" {
		t.Fatalf("report = %+v, want rated summary with 343 reports", result[0].Report)
	}
}

func TestOfflineLookupStillScansInstalledFiles(t *testing.T) {
	old := svc
	cache, _ := compat.OpenCache("")
	svc = &compat.Service{Cache: cache, Scan: func(root string) ([]compat.LocalHit, bool) {
		if root != "/games/test" {
			t.Fatalf("scan root = %q", root)
		}
		return []compat.LocalHit{{Kind: "anticheat", Label: "BattlEye", Name: "BEClient_x64.dll"}}, false
	}}
	defer func() { svc = old }()
	params, _ := json.Marshal(lookupParams{Games: []game{{ID: "g", SteamAppID: "620", InstallDir: "/games/test", Online: false}}})
	got, rpcErr := lookup(context.Background(), params)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	facts := got.(map[string]any)["facts"].([]fact)
	if len(facts) != 1 || len(facts[0].Badges) != 1 || facts[0].Badges[0].Source != "Installed files" {
		t.Fatalf("offline facts = %+v", facts)
	}
}

func TestLookupReturnsDeckVerdictAsGenericBadge(t *testing.T) {
	oldSvc, oldProton := svc, proton
	cache, _ := compat.OpenCache("")
	svc = &compat.Service{Cache: cache, Deck: deckLookupFunc(func(_ context.Context, id string) (compat.DeckReport, error) {
		if id != "620" {
			t.Fatalf("Deck AppID=%q", id)
		}
		return compat.DeckReport{Rating: "playable", Notes: []string{"Small text"}}, nil
	})}
	proton = ratingLookupFunc(func(context.Context, string) (compat.Summary, error) { return compat.Summary{}, compat.ErrNotFound })
	defer func() { svc, proton = oldSvc, oldProton }()
	params, _ := json.Marshal(lookupParams{Games: []game{{ID: "g", Store: "steam", StoreAppID: "620", SteamAppID: "620", Online: true}}})
	got, rpcErr := lookup(context.Background(), params)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	facts := got.(map[string]any)["facts"].([]fact)
	if len(facts) != 1 || len(facts[0].Badges) != 2 || facts[0].Badges[0].Kind != "verdict" || facts[0].Badges[1].Kind != "note" {
		t.Fatalf("Deck fact = %+v", facts)
	}
}

func TestWikiAntiCheatDeduplicatesAgainstAWACY(t *testing.T) {
	have := []badge{{Kind: "anticheat", Label: "Easy Anti-Cheat"}}
	got := newAntiCheat(have, []types.CompatBadge{{Kind: "anticheat", Label: "easy anti-cheat"}, {Kind: "anticheat", Label: "BattlEye"}})
	if len(got) != 1 || got[0].Label != "BattlEye" {
		t.Fatalf("deduped wiki anti-cheat=%+v", got)
	}
}

func TestLookupReturnsWikiCreditAndGenericProtectionBadges(t *testing.T) {
	oldSvc, oldProton := svc, proton
	cache, _ := compat.OpenCache("")
	svc = &compat.Service{Cache: cache, Wiki: wikiLookupFunc(func(context.Context, string) (compat.WikiRecord, error) {
		return compat.WikiRecord{Title: "Portal 2", DRM: []string{"Denuvo"}, Online: "always", Removed: []compat.RemovedDRM{{Name: "Old DRM", Date: "2020"}}, AntiCheat: []string{"Easy Anti-Cheat"}}, nil
	})}
	proton = ratingLookupFunc(func(context.Context, string) (compat.Summary, error) { return compat.Summary{}, compat.ErrNotFound })
	defer func() { svc, proton = oldSvc, oldProton }()
	params, _ := json.Marshal(lookupParams{Games: []game{{ID: "g", SteamAppID: "620", Online: true, ProtectionOnline: true, StoreDRM: true}}})
	got, rpcErr := lookup(context.Background(), params)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	facts := got.(map[string]any)["facts"].([]fact)
	if len(facts) != 1 {
		t.Fatalf("facts=%+v", facts)
	}
	if facts[0].Wiki == nil || facts[0].Wiki.Title != "Portal 2" || facts[0].Wiki.URL != compat.PCGWPage("Portal 2") || facts[0].Wiki.LicenceURL != compat.PCGWLicenceURL {
		t.Fatalf("wiki credit=%+v", facts[0].Wiki)
	}
	kinds := map[string]bool{}
	for _, b := range facts[0].Badges {
		kinds[b.Kind] = true
	}
	if !kinds["drm-removed"] || !kinds["anticheat"] || kinds["denuvo"] || kinds["online"] {
		t.Fatalf("wiki badges=%+v", facts[0].Badges)
	}
}

func TestLookupReportsFewAndNoReports(t *testing.T) {
	old := proton
	proton = ratingLookupFunc(func(_ context.Context, id string) (compat.Summary, error) {
		if id == "620" {
			return compat.Summary{Total: 2}, nil
		}
		return compat.Summary{}, compat.ErrNotFound
	})
	t.Cleanup(func() { proton = old })

	params, _ := json.Marshal(lookupParams{Games: []game{{ID: "few", SteamAppID: "620", Online: true}, {ID: "none", SteamAppID: "621", Online: true}}})
	got, rpcErr := lookup(context.Background(), params)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	facts := got.(map[string]any)["facts"].([]fact)
	if len(facts) != 2 || facts[0].Report == nil || facts[0].Report.State != "few" || facts[0].Report.Count != 2 {
		t.Fatalf("few reports = %+v", facts)
	}
	if facts[1].Report == nil || facts[1].Report.State != "none" {
		t.Fatalf("no reports = %+v", facts[1])
	}
}

func TestLookupKeepsSuccessfulFactsWhenOneGameFails(t *testing.T) {
	old := proton
	var calls []string
	proton = ratingLookupFunc(func(_ context.Context, id string) (compat.Summary, error) {
		calls = append(calls, id)
		if id == "621" {
			return compat.Summary{}, errors.New("temporary network error")
		}
		return compat.Summary{Tier: "gold", Confidence: "strong", Total: 10}, nil
	})
	t.Cleanup(func() { proton = old })

	params, _ := json.Marshal(lookupParams{Games: []game{
		{ID: "one", SteamAppID: "620", Online: true},
		{ID: "two", SteamAppID: "621", Online: true},
		{ID: "three", SteamAppID: "622", Online: true},
	}})
	got, rpcErr := lookup(context.Background(), params)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	facts := got.(map[string]any)["facts"].([]fact)
	if len(facts) != 2 || facts[0].ID != "one" || facts[1].ID != "three" {
		t.Fatalf("facts = %+v, want successful games one and three", facts)
	}
	if len(calls) != 3 {
		t.Fatalf("looked up %v, want to continue after the transient failure", calls)
	}
}

func TestLookupStopsOnPausedFetcherAndKeepsEarlierFacts(t *testing.T) {
	old := proton
	calls := 0
	proton = ratingLookupFunc(func(_ context.Context, id string) (compat.Summary, error) {
		calls++
		if id == "621" {
			return compat.Summary{}, compat.ErrPaused
		}
		return compat.Summary{Tier: "gold", Confidence: "strong", Total: 10}, nil
	})
	t.Cleanup(func() { proton = old })

	params, _ := json.Marshal(lookupParams{Games: []game{
		{ID: "one", SteamAppID: "620", Online: true},
		{ID: "two", SteamAppID: "621", Online: true},
		{ID: "three", SteamAppID: "622", Online: true},
	}})
	got, rpcErr := lookup(context.Background(), params)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	facts := got.(map[string]any)["facts"].([]fact)
	if len(facts) != 1 || facts[0].ID != "one" || calls != 2 {
		t.Fatalf("facts=%+v calls=%d, want only the first fact and two calls", facts, calls)
	}
}

func TestLookupReportsUnavailableWhenNoGameCouldBeFetched(t *testing.T) {
	old := proton
	proton = ratingLookupFunc(func(context.Context, string) (compat.Summary, error) {
		return compat.Summary{}, errors.New("source unavailable")
	})
	t.Cleanup(func() { proton = old })

	params, _ := json.Marshal(lookupParams{Games: []game{{ID: "one", SteamAppID: "620", Online: true}}})
	_, rpcErr := lookup(context.Background(), params)
	if rpcErr == nil || rpcErr.Code != -32002 {
		t.Fatalf("error = %+v, want -32002", rpcErr)
	}
}

func TestLookupKeepsLastFactWhenContextEnds(t *testing.T) {
	old := proton
	ctx, cancel := context.WithCancel(context.Background())
	proton = ratingLookupFunc(func(context.Context, string) (compat.Summary, error) {
		cancel()
		return compat.Summary{Tier: "gold", Confidence: "strong", Total: 10}, nil
	})
	t.Cleanup(func() {
		cancel()
		proton = old
	})

	params, _ := json.Marshal(lookupParams{Games: []game{
		{ID: "one", SteamAppID: "620", Online: true},
		{ID: "two", SteamAppID: "621", Online: true},
	}})
	got, rpcErr := lookup(ctx, params)
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	facts := got.(map[string]any)["facts"].([]fact)
	if len(facts) != 1 || facts[0].ID != "one" {
		t.Fatalf("facts = %+v, want the completed lookup", facts)
	}
}

// Protection facts have their own switch (issue #150): with it off, the wiki
// source is never asked, while ProtonDB ratings still go out.
func TestProtectionSwitchKeepsTheWikiSourceOffline(t *testing.T) {
	oldSvc, oldProton := svc, proton
	cache, _ := compat.OpenCache("")
	wikiAsked := 0
	svc = &compat.Service{Cache: cache, Wiki: wikiLookupFunc(func(context.Context, string) (compat.WikiRecord, error) {
		wikiAsked++
		return compat.WikiRecord{Title: "Portal 2", AntiCheat: []string{"Easy Anti-Cheat"}}, nil
	})}
	proton = ratingLookupFunc(func(context.Context, string) (compat.Summary, error) {
		return compat.Summary{Tier: "platinum", Confidence: "strong", Total: 343}, nil
	})
	defer func() { svc, proton = oldSvc, oldProton }()

	lookupOne := func(g game) fact {
		t.Helper()
		params, _ := json.Marshal(lookupParams{Games: []game{g}})
		got, rpcErr := lookup(context.Background(), params)
		if rpcErr != nil {
			t.Fatal(rpcErr)
		}
		facts := got.(map[string]any)["facts"].([]fact)
		if len(facts) != 1 {
			t.Fatalf("facts = %+v", facts)
		}
		return facts[0]
	}

	off := lookupOne(game{ID: "off", SteamAppID: "620", Online: true})
	if wikiAsked != 0 {
		t.Fatalf("PCGamingWiki asked %d times with protection off", wikiAsked)
	}
	if off.Report == nil || off.Report.Tier != "platinum" {
		t.Fatalf("ratings should still fetch with protection off: %+v", off.Report)
	}

	on := lookupOne(game{ID: "on", SteamAppID: "620", Online: true, ProtectionOnline: true})
	if wikiAsked != 1 {
		t.Fatalf("PCGamingWiki asked %d times with protection on, want 1", wikiAsked)
	}
	if on.Wiki == nil {
		t.Fatalf("no wiki credit with protection on: %+v", on.Badges)
	}
}
