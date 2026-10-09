package compat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testFetcher is a fetcher whose waits are recorded, not slept, and whose
// jitter is zero, so the timing is exact.
func testFetcher(client *http.Client) (*Fetcher, *[]time.Duration) {
	var mu sync.Mutex
	var waits []time.Duration
	f := NewFetcher("hydra-test")
	f.Client = client
	f.Jitter = 0
	f.Now = func() time.Time { return time.Unix(1000, 0) }
	f.Sleep = func(ctx context.Context, d time.Duration) error {
		mu.Lock()
		waits = append(waits, d)
		mu.Unlock()
		return ctx.Err()
	}
	return f, &waits
}

func TestFetcherSendsHydraUserAgent(t *testing.T) {
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	f, _ := testFetcher(srv.Client())
	f.UserAgent = "Hydra-Launcher/9 (test)"
	if _, err := f.Get(context.Background(), srv.URL); err != nil {
		t.Fatal(err)
	}
	if ua != "Hydra-Launcher/9 (test)" {
		t.Fatalf("user agent %q", ua)
	}
}

func TestFetcherNotFoundIsErrNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	f, _ := testFetcher(srv.Client())
	if _, err := f.Get(context.Background(), srv.URL); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err %v, want ErrNotFound", err)
	}
}

func TestFetcherBacksOffOnServerErrorsThenGivesUp(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	f, waits := testFetcher(srv.Client())
	f.Retries = 3
	f.Backoff = 2 * time.Second
	f.Interval = 0 // no pacing waits: only the backoff is recorded
	_, err := f.Get(context.Background(), srv.URL)
	var se *StatusError
	if !errors.As(err, &se) || se.Status != http.StatusBadGateway {
		t.Fatalf("err %v, want a 502 StatusError", err)
	}
	if hits.Load() != 4 {
		t.Fatalf("%d requests, want 1 + 3 retries", hits.Load())
	}
	// Exponential: 2 s, 4 s, 8 s between the attempts.
	backoff := *waits
	want := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}
	if len(backoff) != len(want) {
		t.Fatalf("backoff waits %v, want %v", backoff, want)
	}
	for i := range want {
		if backoff[i] != want[i] {
			t.Fatalf("backoff waits %v, want %v", backoff, want)
		}
	}
}

func TestFetcherHonoursRetryAfter(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	f, waits := testFetcher(srv.Client())
	body, err := f.Get(context.Background(), srv.URL)
	if err != nil || string(body) != `{"ok":true}` {
		t.Fatalf("body %q err %v", body, err)
	}
	found := false
	for _, w := range *waits {
		if w == 30*time.Second {
			found = true
		}
	}
	if !found {
		t.Fatalf("waits %v, want the 30 s Retry-After", *waits)
	}
}

func TestFetcherPausesAfterRepeatedRateLimits(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	f, _ := testFetcher(srv.Client())
	f.Retries = 0 // one 429 per call: three calls trip the pause
	for i := 0; i < pauseAfter; i++ {
		if _, err := f.Get(context.Background(), srv.URL); err == nil {
			t.Fatal("expected a 429 error")
		}
	}
	before := hits.Load()
	if _, err := f.Get(context.Background(), srv.URL); !errors.Is(err, ErrPaused) {
		t.Fatalf("err %v, want ErrPaused", err)
	}
	if hits.Load() != before {
		t.Fatal("asked the server during the pause")
	}
}

func TestFetcherConcurrencyIsTwo(t *testing.T) {
	var cur, peak atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := cur.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		<-release
		cur.Add(-1)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	f, _ := testFetcher(srv.Client())
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = f.Get(context.Background(), srv.URL)
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for cur.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if peak.Load() != 2 {
		t.Fatalf("peak concurrency %d, want 2", peak.Load())
	}
}

func TestProtonDBRejectsNonNumericAppIDWithoutAsking(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()
	f, _ := testFetcher(srv.Client())
	p := &ProtonDB{Fetch: f, Base: srv.URL + "/"}
	for _, bad := range []string{"", "../620", "62o", "620.json"} {
		if _, err := p.Summary(context.Background(), bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
	if hits.Load() != 0 {
		t.Fatal("asked the server for a bad AppID")
	}
}
