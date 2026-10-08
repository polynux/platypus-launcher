package depot

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"
)

// STEAM-10: a stalled body must not leak a reader goroutine, and an
// over-long body must error instead of truncating silently at 64 MiB.
func TestCDNBodyTimeoutNoLeak(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(2 * time.Second) // stall past the test's body timeout
	}))
	defer srv.Close()
	c := NewClient()
	h := Host{Name: "stall", BaseURL: srv.URL}
	time.Sleep(50 * time.Millisecond) // let test servers settle
	before := runtime.NumGoroutine()
	for i := 0; i < 3; i++ {
		_, _ = c.getWithTimeouts(nil, c.url(h, "x"), 5*time.Second, 100*time.Millisecond)
	}
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before+2 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > before+2 {
		t.Fatalf("goroutines leaked: before %d, after %d", before, got)
	}
}

func TestCDNOversizedBodyErrors(t *testing.T) {
	big := make([]byte, (64<<20)+10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(big)
	}))
	defer srv.Close()
	c := NewClient()
	h := Host{Name: "big", BaseURL: srv.URL}
	if _, err := c.getWithTimeouts(nil, c.url(h, "x"), 5*time.Second, 5*time.Second); err == nil {
		t.Fatal("oversized body should error, not truncate")
	}
}
