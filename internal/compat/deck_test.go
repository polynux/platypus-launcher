package compat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// deckServer answers the Deck report from a table of app id to body and
// counts the asks.
func deckServer(t *testing.T, bodies map[string]string, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if got := r.URL.Query().Get("nAppID"); got == "" {
			t.Errorf("Deck ask without nAppID: %s", r.URL)
		}
		id := r.URL.Query().Get("nAppID")
		b, ok := bodies[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(b))
	}))
}

const (
	deckVerified = `{"success":1,"results":{"appid":367520,"resolved_category":3,"resolved_items":[
		{"display_type":4,"loc_token":"#SteamDeckVerified_TestResult_DefaultControllerConfigFullyFunctional"},
		{"display_type":4,"loc_token":"#SteamDeckVerified_TestResult_InterfaceTextIsLegible"}],
		"steam_deck_blog_url":"","extra_number":2,"extra_nested":{"a":[1,2]}}}`
	deckPlayable = `{"success":1,"results":{"appid":730,"resolved_category":2,"resolved_items":[
		{"display_type":3,"loc_token":"#SteamDeckVerified_TestResult_ControllerGlyphsDoNotMatchDeckDevice"},
		{"display_type":3,"loc_token":"#SteamDeckVerified_TestResult_InterfaceTextIsNotLegible"},
		{"display_type":4,"loc_token":"#SteamDeckVerified_TestResult_DefaultControllerConfigFullyFunctional"},
		{"display_type":9,"loc_token":"#SomethingNew_UnknownDisplayType"}]}}`
	deckUnsupported = `{"success":1,"results":{"appid":578080,"resolved_category":1,"resolved_items":[
		{"display_type":2,"loc_token":"#SteamDeckVerified_TestResult_UnsupportedAntiCheat_Other"}]}}`
	deckUnknownCategory = `{"success":1,"results":{"appid":1,"resolved_category":0,"resolved_items":[]}}`
	deckEmptyResults    = `{"success":1,"results":[]}`
	deckMissingResults  = `{"success":1}`
)

func TestSteamDeckParsesEachCategory(t *testing.T) {
	var hits atomic.Int32
	srv := deckServer(t, map[string]string{
		"367520": deckVerified, "730": deckPlayable, "578080": deckUnsupported,
	}, &hits)
	defer srv.Close()
	f, _ := testFetcher(srv.Client())
	d := &SteamDeck{Fetch: f, Base: srv.URL}

	rep, err := d.Report(context.Background(), "367520")
	if err != nil || rep.Rating != "verified" || len(rep.Notes) != 0 {
		t.Fatalf("verified = %+v err %v, want no notes", rep, err)
	}
	rep, err = d.Report(context.Background(), "730")
	if err != nil || rep.Rating != "playable" {
		t.Fatalf("playable = %+v err %v", rep, err)
	}
	if len(rep.Notes) != 2 || rep.Notes[0] != deckNoteWords["#SteamDeckVerified_TestResult_ControllerGlyphsDoNotMatchDeckDevice"] {
		t.Fatalf("playable notes = %q", rep.Notes)
	}
	rep, err = d.Report(context.Background(), "578080")
	if err != nil || rep.Rating != "unsupported" {
		t.Fatalf("unsupported = %+v err %v", rep, err)
	}
	if len(rep.Notes) != 1 {
		t.Fatalf("unsupported notes = %q", rep.Notes)
	}
}

func TestSteamDeckUnknownAndMissingAreErrNotFound(t *testing.T) {
	var hits atomic.Int32
	srv := deckServer(t, map[string]string{
		"1": deckUnknownCategory, "2": deckEmptyResults, "3": deckMissingResults,
	}, &hits)
	defer srv.Close()
	f, _ := testFetcher(srv.Client())
	d := &SteamDeck{Fetch: f, Base: srv.URL}
	for _, id := range []string{"1", "2", "3", "999"} {
		// "999" is a real 404 from the server; the rest are verdicts Steam
		// doesn't have. All are ErrNotFound.
		if _, err := d.Report(context.Background(), id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("%s err = %v, want ErrNotFound", id, err)
		}
	}
}

func TestSteamDeckToleratesExtraFieldsAndOmitsUnknownTokens(t *testing.T) {
	// deckVerified carries extra numeric/nested fields; deckPlayable an
	// unknown display type and a passing item: neither leaks into the report.
	var hits atomic.Int32
	srv := deckServer(t, map[string]string{"367520": deckVerified, "730": deckPlayable}, &hits)
	defer srv.Close()
	f, _ := testFetcher(srv.Client())
	d := &SteamDeck{Fetch: f, Base: srv.URL}
	rep, err := d.Report(context.Background(), "367520")
	if err != nil || rep.Rating != "verified" {
		t.Fatalf("with extra fields = %+v err %v", rep, err)
	}
	rep, err = d.Report(context.Background(), "730")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range rep.Notes {
		if n == "" || n == "#SomethingNew_UnknownDisplayType" {
			t.Fatalf("unknown token leaked into notes: %q", rep.Notes)
		}
	}
	if len(rep.Notes) != 2 {
		t.Fatalf("notes = %q, want the two known caveats", rep.Notes)
	}
}

func TestSteamDeckRejectsNonNumericAppIDWithoutAsking(t *testing.T) {
	var hits atomic.Int32
	srv := deckServer(t, map[string]string{}, &hits)
	defer srv.Close()
	f, _ := testFetcher(srv.Client())
	d := &SteamDeck{Fetch: f, Base: srv.URL}
	for _, bad := range []string{"", "../620", "62o", "620.json"} {
		if _, err := d.Report(context.Background(), bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
	if hits.Load() != 0 {
		t.Fatal("asked the server for a bad AppID")
	}
}

// fakeDeck answers Deck verdicts from a table and counts the asks.
type fakeDeck struct {
	reps  map[string]DeckReport
	calls atomic.Int32
}

func (f *fakeDeck) Report(_ context.Context, appID string) (DeckReport, error) {
	f.calls.Add(1)
	if r, ok := f.reps[appID]; ok {
		return r, nil
	}
	return DeckReport{}, ErrNotFound
}

func newDeckTestService(t *testing.T, online *bool) (*Service, *fakeDeck) {
	t.Helper()
	on := true
	if online != nil {
		on = *online
	}
	s, _, ck, _ := newTestService(t, &on)
	_ = ck
	fd := &fakeDeck{reps: map[string]DeckReport{
		"367520": {Rating: "verified"},
		"730":    {Rating: "playable", Notes: []string{"Some text is too small to read on the Deck."}},
	}}
	s.Deck = fd
	return s, fd
}

func TestDeckSecondCallMakesNoRequest(t *testing.T) {
	on := true
	s, fd := newDeckTestService(t, &on)
	r, ok := s.DeckRating(context.Background(), "367520")
	if !ok || !r.Found || r.Report.Rating != "verified" || r.Stale {
		t.Fatalf("first answer %+v ok %v", r, ok)
	}
	if fd.calls.Load() != 1 {
		t.Fatalf("%d asks, want 1", fd.calls.Load())
	}
	r, ok = s.DeckRating(context.Background(), "367520")
	if !ok || r.Stale || fd.calls.Load() != 1 {
		t.Fatalf("fresh hit asked again: %+v asks %d", r, fd.calls.Load())
	}
	// The cached verdict survives a reopen through the same cache data.
	if _, ok := s.Cache.Deck("367520"); !ok {
		t.Fatal("verdict not in the cache")
	}
}

func TestDeckUnknownVerdictIsCachedForADay(t *testing.T) {
	on := true
	s, fd := newDeckTestService(t, &on)
	r, ok := s.DeckRating(context.Background(), "999")
	if !ok || r.Found {
		t.Fatalf("unknown answer %+v ok %v", r, ok)
	}
	s.DeckRating(context.Background(), "999")
	if fd.calls.Load() != 1 {
		t.Fatalf("an unknown verdict was asked again within its day (%d asks)", fd.calls.Load())
	}
}

func TestDeckSwitchOffMeansNoRequest(t *testing.T) {
	on := false
	s, fd := newDeckTestService(t, &on)
	if _, ok := s.DeckRating(context.Background(), "367520"); ok {
		t.Fatal("an answer with no cache and no network")
	}
	if _, ok := s.DeckRatingCached("367520"); ok {
		t.Fatal("a cached answer with no cache and no network")
	}
	if fd.calls.Load() != 0 {
		t.Fatal("asked the network while offline")
	}
}

func TestDeckCachedNeverWaitsAndFillsTheCacheForTheNextLook(t *testing.T) {
	on := true
	s, fd := newDeckTestService(t, &on)
	s.FirstWait = time.Minute // a waiting lookup would hang the test
	if _, ok := s.DeckRatingCached("730"); ok {
		t.Fatal("an unknown app answered before any fetch")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if r, ok := s.DeckRatingCached("730"); ok {
			if r.Report.Rating != "playable" || len(r.Report.Notes) != 1 || r.Stale {
				t.Fatalf("cached verdict = %+v, want playable with its note, fresh", r)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the background fetch never filled the cache")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if n := fd.calls.Load(); n != 1 {
		t.Fatalf("Steam asked %d times, want 1", n)
	}
}
