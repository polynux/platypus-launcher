package netstate

import (
	"errors"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestIsNetworkError(t *testing.T) {
	dns := &net.DNSError{Err: "no such host", Name: "example.com"}
	if !IsNetworkError(dns) {
		t.Error("DNSError should be network-class")
	}
	op := &net.OpError{Op: "dial", Err: errors.New("refused")}
	if !IsNetworkError(op) {
		t.Error("OpError should be network-class")
	}
	if !IsNetworkError(os.ErrDeadlineExceeded) {
		t.Error("deadline exceeded should be network-class")
	}
	if IsNetworkError(errors.New("http 500")) {
		t.Error("a plain error should not be network-class")
	}
	if IsNetworkError(nil) {
		t.Error("nil should not be network-class")
	}
}

func TestTransitionsFireOnce(t *testing.T) {
	s := New()
	var events []bool
	s.OnTransition(func(online bool) { events = append(events, online) })
	op := &net.OpError{Op: "dial", Err: errors.New("no route")}
	s.ObserveError(op, "example.com:443")
	s.ObserveError(op, "example.com:443")
	if s.Online() {
		t.Fatal("should be offline")
	}
	snap := s.Snapshot()
	if snap.Online || snap.Since == 0 {
		t.Fatalf("snapshot: %+v", snap)
	}
	s.ObserveSuccess()
	s.ObserveSuccess()
	if !s.Online() {
		t.Fatal("should be online")
	}
	if len(events) != 2 || events[0] || !events[1] {
		t.Fatalf("one event per transition, got %v", events)
	}
}

func TestNonNetworkErrorChangesNothing(t *testing.T) {
	s := New()
	fired := false
	s.OnTransition(func(bool) { fired = true })
	s.ObserveError(errors.New("http 500"), "example.com:443")
	if !s.Online() || fired {
		t.Fatal("server errors must not mark offline")
	}
}

func TestProbeFlipsOnline(t *testing.T) {
	s := New()
	s.SetClientConnected(true)
	s.SetProbe(func(addr string) bool { return addr == "example.com:443" })
	s.ObserveError(&net.OpError{Op: "dial", Err: errors.New("x")}, "example.com:443")
	if s.Online() {
		t.Fatal("should be offline")
	}
	if !s.ProbeOnce() || !s.Online() {
		t.Fatal("a successful probe should flip online")
	}
}

func TestProbeKeepsBackoff(t *testing.T) {
	s := New()
	s.SetDelays([]time.Duration{time.Millisecond})
	s.SetClientConnected(false) // no timer while disconnected
	calls := 0
	s.SetProbe(func(string) bool { calls++; return false })
	s.ObserveError(&net.OpError{Op: "dial", Err: errors.New("x")}, "h:1")
	s.ProbeOnce()
	if s.Online() || calls != 1 {
		t.Fatalf("failed probe keeps offline (calls=%d)", calls)
	}
}

func TestProbeOnlyWhileConnected(t *testing.T) {
	s := New()
	s.SetDelays([]time.Duration{time.Hour})
	s.SetClientConnected(false)
	s.ObserveError(&net.OpError{Op: "dial", Err: errors.New("x")}, "h:1")
	s.SetClientConnected(true)
	s.SetClientConnected(false)
	if s.Online() {
		t.Fatal("should still be offline")
	}
}

func TestTransportObserves(t *testing.T) {
	s := New()
	s.SetClientConnected(true)
	failing := WrapClient(nil, s, time.Second)
	failing.Transport = Transport{State: s, Base: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, &net.OpError{Op: "dial", Err: errors.New("no route")}
	})}
	req, _ := http.NewRequest("GET", "http://example.com/x", nil)
	if _, err := failing.Do(req); err == nil {
		t.Fatal("expected the dial error")
	}
	if s.Online() || s.LastAddr() != "example.com" {
		t.Fatalf("a failed request should mark offline with the host, got %v/%q", s.Online(), s.LastAddr())
	}
	ok := WrapClient(nil, s, time.Second)
	ok.Transport = Transport{State: s, Base: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 500}, nil
	})}
	if _, err := ok.Do(req); err != nil {
		t.Fatal(err)
	}
	if !s.Online() {
		t.Fatal("any success — even a 500 — marks online")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
