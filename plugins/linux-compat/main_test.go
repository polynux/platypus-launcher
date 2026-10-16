package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"platypus/internal/compat"
)

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
}
