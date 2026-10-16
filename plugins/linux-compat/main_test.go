package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"platypus/internal/compat"
)

type ratingLookupFunc func(context.Context, string) (compat.Summary, error)

func (f ratingLookupFunc) Summary(ctx context.Context, id string) (compat.Summary, error) {
	return f(ctx, id)
}

func TestInitializeNegotiatesCompatFactsProtocol(t *testing.T) {
	params, _ := json.Marshal(map[string]any{"protocols": []string{"1.17"}})
	resp := handle(context.Background(), message{Method: "initialize", Params: params})
	result := resp.Result.(map[string]any)
	if result["protocol"] != "1.17" {
		t.Fatalf("protocol = %v, want 1.17", result["protocol"])
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

	params, _ := json.Marshal(lookupParams{Games: []game{{ID: "gog_1", Store: "gog", StoreAppID: "1", SteamAppID: "620"}}})
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

func TestLookupReportsFewAndNoReports(t *testing.T) {
	old := proton
	proton = ratingLookupFunc(func(_ context.Context, id string) (compat.Summary, error) {
		if id == "620" {
			return compat.Summary{Total: 2}, nil
		}
		return compat.Summary{}, compat.ErrNotFound
	})
	t.Cleanup(func() { proton = old })

	params, _ := json.Marshal(lookupParams{Games: []game{{ID: "few", SteamAppID: "620"}, {ID: "none", SteamAppID: "621"}}})
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
		{ID: "one", SteamAppID: "620"},
		{ID: "two", SteamAppID: "621"},
		{ID: "three", SteamAppID: "622"},
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
		{ID: "one", SteamAppID: "620"},
		{ID: "two", SteamAppID: "621"},
		{ID: "three", SteamAppID: "622"},
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

	params, _ := json.Marshal(lookupParams{Games: []game{{ID: "one", SteamAppID: "620"}}})
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
		{ID: "one", SteamAppID: "620"},
		{ID: "two", SteamAppID: "621"},
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
