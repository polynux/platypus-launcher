// Package testenv labels the test suites (issue #40): slow integration
// tests and network tests are opt-in, and the default run touches neither.
//
// Slow tests (real plugin processes, Python plugins, full protocol
// conformance suites, download pacing, long polling timeouts) call
// RequireSlow: they skip unless PLATYPUS_TEST_SLOW=1.
//
// Network tests (dialing non-loopback hosts) call RequireNetwork: they
// skip unless PLATYPUS_TEST_NETWORK=1. There are currently no network
// tests; everything answers from httptest servers, fixture API files or
// localhost fakes.
//
// The default run is guarded: importing this package installs an
// http.DefaultTransport (plus a dialer helper) that refuses non-loopback
// addresses, so a test that sneaks in a real network call fails fast with
// a clear message instead of hanging on the network. Every Go test binary
// in this repo imports this package from a testenv_guard_test.go file, so
// the guard holds suite-wide. Tests that need the network must both set
// PLATYPUS_TEST_NETWORK=1 and call RequireNetwork (which lifts the guard
// for that test's process).
package testenv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
)

// Slow reports whether slow integration tests are enabled.
func Slow() bool { return os.Getenv("PLATYPUS_TEST_SLOW") == "1" }

// Network reports whether network tests are enabled.
func Network() bool { return os.Getenv("PLATYPUS_TEST_NETWORK") == "1" }

// RequireSlow skips the test unless PLATYPUS_TEST_SLOW=1. Slow tests run
// real plugin processes (Python plugins, full conformance suites,
// download pacing) and don't belong in the default run.
func RequireSlow(t *testing.T) {
	t.Helper()
	if !Slow() {
		t.Skip("slow test: rerun with PLATYPUS_TEST_SLOW=1")
	}
}

// RequireNetwork skips the test unless PLATYPUS_TEST_NETWORK=1, and lifts
// the non-loopback guard for the test process (see below).
func RequireNetwork(t *testing.T) {
	t.Helper()
	if !Network() {
		t.Skip("network test: rerun with PLATYPUS_TEST_NETWORK=1")
	}
	liftGuard()
}

var (
	guardMu sync.Mutex
	guardOn bool
	// guardBase is the transport the guard wraps (captured at install).
	guardBase http.RoundTripper
)

// InstallGuard refuses non-loopback HTTP through http.DefaultTransport
// (and offers DialContext for custom dialers). It is installed
// automatically when this package is imported by a test binary, and is a
// no-op when PLATYPUS_TEST_NETWORK=1. Prefer the automatic install;
// TestMain helpers that need ordering control can call InstallGuard
// explicitly.
func InstallGuard() {
	guardMu.Lock()
	defer guardMu.Unlock()
	if guardOn || os.Getenv("PLATYPUS_TEST_NETWORK") == "1" {
		return
	}
	guardBase = http.DefaultTransport
	http.DefaultTransport = guardTransport{}
	guardOn = true
}

// liftGuard restores the transport captured at install. Only RequireNetwork
// calls it: a network test opts the whole test process back in.
func liftGuard() {
	guardMu.Lock()
	defer guardMu.Unlock()
	if guardOn {
		http.DefaultTransport = guardBase
		guardOn = false
	}
}

// Guarded reports whether the non-loopback guard is currently installed.
func Guarded() bool {
	guardMu.Lock()
	defer guardMu.Unlock()
	return guardOn
}

func init() { InstallGuard() }

// guardTransport wraps the default transport and refuses anything that
// isn't loopback, so the default suite can never touch the real network:
// a violation fails fast instead of hanging on a dial timeout.
type guardTransport struct{}

func (guardTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	if host == "" || !isLoopbackHost(host) {
		return nil, fmt.Errorf("testenv: non-loopback HTTP %s %s refused in the default run "+
			"(gate the test behind testenv.RequireNetwork or point it at a localhost fake)",
			req.Method, req.URL.Redacted())
	}
	base := guardBase
	if base == nil {
		return nil, fmt.Errorf("testenv: no base transport (RequireNetwork already lifted the guard?)")
	}
	if _, ok := base.(guardTransport); ok {
		return nil, fmt.Errorf("testenv: refusing to recurse into the guard transport")
	}
	return base.RoundTrip(req)
}

// DialContext is a drop-in DialContext for custom transports that refuses
// non-loopback addresses the same way the DefaultTransport guard does.
// Tests that build their own http.Transport should use it:
//
//	&http.Transport{DialContext: testenv.DialContext}
func DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("testenv: bad dial address %q: %w", addr, err)
	}
	guardMu.Lock()
	on := guardOn
	guardMu.Unlock()
	if on && !isLoopbackHost(host) {
		return nil, fmt.Errorf("testenv: non-loopback dial %s refused in the default run "+
			"(gate the test behind testenv.RequireNetwork or point it at a localhost fake)", addr)
	}
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}

func isLoopbackHost(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return ip.IsLoopback()
	}
	return false
}
