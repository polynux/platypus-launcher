package testenv_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"platypus/internal/testenv"
)

func TestGuardRefusesNonLoopback(t *testing.T) {
	if !testenv.Guarded() {
		t.Skip("guard lifted (PLATYPUS_TEST_NETWORK=1?)")
	}
	req, _ := http.NewRequest("GET", "https://example.com/", nil)
	if _, err := http.DefaultTransport.RoundTrip(req); err == nil {
		t.Fatal("non-loopback request was not refused")
	}
}

func TestGuardAllowsLoopback(t *testing.T) {
	if !testenv.Guarded() {
		t.Skip("guard lifted (PLATYPUS_TEST_NETWORK=1?)")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("loopback request refused: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}
}

func TestDialRefusesNonLoopback(t *testing.T) {
	if !testenv.Guarded() {
		t.Skip("guard lifted (PLATYPUS_TEST_NETWORK=1?)")
	}
	if _, err := testenv.DialContext(t.Context(), "tcp", "example.com:80"); err == nil {
		t.Fatal("non-loopback dial was not refused")
	}
}
