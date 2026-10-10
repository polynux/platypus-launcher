package compat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeWiki serves the PCGW endpoints from a table: appid → page title, and
// title → wikitext. It counts the requests it gets.
func fakeWiki(t *testing.T, pages map[string]string, titles map[string]string) (*httptest.Server, *atomic.Int32, *[]string) {
	t.Helper()
	var hits atomic.Int32
	var agents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		agents = append(agents, r.Header.Get("User-Agent"))
		switch r.URL.Path {
		case "/api/appid.php":
			title, ok := pages[r.URL.Query().Get("appid")]
			if !ok {
				http.Redirect(w, r, "/index.php?search=x", http.StatusFound)
				return
			}
			http.Redirect(w, r, "/wiki/"+url.PathEscape(title), http.StatusFound)
		case "/w/api.php":
			title := r.URL.Query().Get("page")
			text, ok := titles[title]
			if !ok {
				_, _ = w.Write([]byte(`{"error":{"code":"missingtitle"}}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"parse": map[string]any{
				"title":    title,
				"wikitext": map[string]string{"*": text},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits, &agents
}

func portal2Wikitext(t *testing.T) string { return readFixture(t, "portal_2.wiki") }

func testPCGW(srv *httptest.Server) *PCGW {
	f, _ := testFetcher(NewNoRedirectClient())
	f.UserAgent = PCGWUserAgent
	f.Interval = 0
	f.Jitter = 0
	return &PCGW{Fetch: f, AppIDURL: srv.URL + "/api/appid.php?appid=", API: srv.URL + "/w/api.php"}
}

func TestPCGWLookupResolvesTheTitleAndReadsThePage(t *testing.T) {
	srv, hits, agents := fakeWiki(t,
		map[string]string{"620": "Portal 2"},
		map[string]string{"Portal 2": portal2Wikitext(t)})
	p := testPCGW(srv)
	rec, err := p.Lookup(context.Background(), "620")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Title != "Portal 2" || len(rec.DRM) != 1 || rec.DRM[0] != "Steam" {
		t.Fatalf("record %+v", rec)
	}
	if hits.Load() != 2 {
		t.Fatalf("%d requests, want 2: the appid redirect, then the parse (the redirect target is not fetched)", hits.Load())
	}
	for _, ua := range *agents {
		if ua != PCGWUserAgent {
			t.Fatalf("user agent %q, want the documented one", ua)
		}
	}
	if !strings.Contains(PCGWUserAgent, PCGWContact) {
		t.Fatalf("user agent %q must name the contact", PCGWUserAgent)
	}
}

func TestPCGWLookupMissingAndWrongPageAreNotFound(t *testing.T) {
	srv, _, _ := fakeWiki(t,
		map[string]string{"620": "Portal 2"},
		map[string]string{"Portal 2": portal2Wikitext(t)})
	p := testPCGW(srv)
	ctx := context.Background()
	if _, err := p.Lookup(ctx, "999999"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no page: %v, want ErrNotFound", err)
	}
	// The page for 620 says nothing about 1196590: a wrong page is not an answer.
	if _, err := p.Lookup(ctx, "1196590"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other game's page: %v, want ErrNotFound", err)
	}
	if _, err := p.Lookup(ctx, "Portal 2"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("a title is not a Steam AppID: %v", err)
	}
}

func TestPCGWFetcherFollowsTheDocumentedPoliteness(t *testing.T) {
	f := NewPCGWFetcher()
	if f.UserAgent != PCGWUserAgent || f.Concurrency != 1 || f.Interval < time.Second {
		t.Fatalf("fetcher %+v: one request at a time, about one a second", f)
	}
	if f.Backoff != time.Minute {
		t.Fatalf("backoff %v, want a minute after a 429", f.Backoff)
	}
	if f.Client.CheckRedirect == nil {
		t.Fatal("the appid redirect must not be followed")
	}
}

func TestPCGWBacksOffAMinuteOnRateLimit(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		http.Redirect(w, r, "/index.php?search=x", http.StatusFound)
	}))
	defer srv.Close()
	p := testPCGW(srv)
	f, waits := testFetcher(NewNoRedirectClient())
	f.Backoff = time.Minute
	f.Retries = 1
	f.Jitter = 0
	p.Fetch = f
	if _, err := p.Lookup(context.Background(), "620"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err %v, want ErrNotFound after the retry", err)
	}
	if len(*waits) == 0 || (*waits)[0] != time.Minute {
		t.Fatalf("waits %v, want the first wait to be a minute", *waits)
	}
}
