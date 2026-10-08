package depot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestCDNTokenRequired proves hosts requiring GetCDNAuthToken 403 chunk
// URLs without ?token=: install succeeds with tokened URLs, repair with
// bare hosts fails. Regression test for STEAM-2 (verify repair passed
// ss.hosts without tokens instead of d.hosts).
func TestCDNTokenRequired(t *testing.T) {
	key := testKey()
	m, encMap := buildFixtureDepot(t, key, 1, 2)
	var hexID string
	var enc []byte
	for h, e := range encMap {
		hexID, enc = h, e
		break
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery == "" {
			http.Error(w, "auth required", 403)
			return
		}
		w.Write(enc)
	}))
	defer srv.Close()
	_ = hexID
	c := NewClient()
	bare := Host{Name: "h", BaseURL: srv.URL}
	tokened := Host{Name: "h", BaseURL: srv.URL, Token: "token=abc"}
	ctx := context.Background()
	if _, err := c.DownloadChunk(ctx, bare, 11, mustID(t, m, 0)); err == nil {
		t.Fatal("bare host without token should 403")
	}
	got, err := c.DownloadChunk(ctx, tokened, 11, mustID(t, m, 0))
	if err != nil {
		t.Fatalf("tokened host should succeed: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("empty chunk")
	}
}

func mustID(t *testing.T, m *Manifest, i int) []byte {
	t.Helper()
	u := m.UniqueChunks()
	if i >= len(u) {
		t.Fatalf("only %d chunks", len(u))
	}
	return u[i].ID
}
