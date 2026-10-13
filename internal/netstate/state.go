// Package netstate tracks whether the machine looks online (issue #44): the
// core never disables Play because of the network, but background fetches
// skip their work while offline, cloud checks are skipped with a note, and
// the library serves its last good listing with a stale warning.
//
// Any core outbound request reports its outcome: a network-class error marks
// the state offline, any success marks it online. While offline, at most one
// probe runs on a 5 s → 15 s → 60 s capped backoff, only while a UI client
// is connected. The probe is a TCP dial with a 1 s timeout to the host of
// the last failed request — never a third-party host — and is injectable
// for tests. One event fires per transition.
package netstate

import (
	"errors"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// Delays is the probe backoff while offline: 5 s, then 15 s, then 60 s,
// capped at the last step.
var Delays = []time.Duration{5 * time.Second, 15 * time.Second, 60 * time.Second}

// ProbeTimeout is the TCP dial deadline of one probe.
const ProbeTimeout = time.Second

// Snapshot is what GetNetState returns: whether the machine looks online,
// and when the current state began (unix seconds, 0 when never changed).
type Snapshot struct {
	Online bool  `json:"online"`
	Since  int64 `json:"since"`
}

// IsNetworkError reports whether err is a network-class failure: a DNS
// error, an operation error (dial, connect, read), or a timeout. Any core
// outbound request that fails this way marks the state offline; anything
// else (HTTP 500, a bad payload, a refused… no: refused is an OpError and
// counts, so failing loopback servers simulate offline in tests) leaves
// the state alone.
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return true
	}
	var op *net.OpError
	if errors.As(err, &op) {
		return true
	}
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return true
	}
	return os.IsTimeout(err)
}

// State is the offline tracker. The zero value is ready: online, no UI
// client, the default dial probe. Tests inject a fake probe and short
// delays; production wires OnTransition to the "netstate" core event.
type State struct {
	mu              sync.Mutex
	online          bool
	since           int64
	hasSince        bool
	lastAddr        string
	clientConnected bool
	backoff         int
	timer           *time.Timer
	probe           func(addr string) bool
	onChange        func(online bool)
	now             func() int64
	delays          []time.Duration
}

// New returns a tracker with the default dial probe.
func New() *State { return &State{online: true} }

// SetProbe replaces the dial function (tests). A nil probe restores the
// default TCP dial.
func (s *State) SetProbe(probe func(addr string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probe = probe
}

// SetDelays replaces the probe backoff (tests).
func (s *State) SetDelays(delays []time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delays = delays
}

// OnTransition registers the callback fired once per transition (the core
// emits its "netstate" event from it). It runs on the reporter's goroutine.
func (s *State) OnTransition(fn func(online bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onChange = fn
}

// SetClientConnected tells the tracker whether a UI client is connected:
// probes only run while one is. The core sets it on initialize and clears
// it on shutdown.
func (s *State) SetClientConnected(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clientConnected = connected
	if connected && !s.online && s.timer == nil && s.lastAddr != "" {
		s.scheduleLocked()
	}
	if !connected {
		s.stopLocked()
	}
}

// Online reports whether the machine looks online.
func (s *State) Online() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.online
}

// Snapshot returns the current state for GetNetState.
func (s *State) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Snapshot{Online: s.online, Since: s.since}
}

// ObserveSuccess marks the state online: any successful core request clears
// a previous offline marking, with one event on the transition.
func (s *State) ObserveSuccess() {
	s.mu.Lock()
	if s.online {
		s.mu.Unlock()
		return
	}
	s.online = true
	s.backoff = 0
	s.stopLocked()
	since := s.stampLocked()
	cb := s.onChange
	s.mu.Unlock()
	if cb != nil {
		cb(true)
	}
	_ = since
}

// ObserveError marks the state offline when err is network-class, remembering
// addr (host or host:port) for the probe. Non-network errors change nothing,
// so background work never flaps on server errors. One event fires per
// transition.
func (s *State) ObserveError(err error, addr string) {
	if !IsNetworkError(err) {
		return
	}
	s.mu.Lock()
	if addr != "" {
		s.lastAddr = addr
	}
	if !s.online {
		s.mu.Unlock()
		return
	}
	s.online = false
	s.backoff = 0
	s.stampLocked()
	if s.clientConnected && s.lastAddr != "" {
		s.scheduleLocked()
	}
	cb := s.onChange
	s.mu.Unlock()
	if cb != nil {
		cb(false)
	}
}

// ProbeOnce runs one probe round now (tests and the timer): while offline
// with a remembered address, dial it; success marks the state online. It
// reports whether the state flipped to online.
func (s *State) ProbeOnce() bool {
	s.mu.Lock()
	if s.online || s.lastAddr == "" {
		s.mu.Unlock()
		return false
	}
	probe := s.probe
	addr := s.lastAddr
	s.mu.Unlock()
	ok := dialProbe
	if probe != nil {
		ok = probe
	}
	if !ok(addr) {
		s.mu.Lock()
		if !s.online && s.lastAddr == addr {
			if s.backoff < len(s.delaysOrDefault())-1 {
				s.backoff++
			}
			if s.clientConnected && s.timer == nil {
				s.scheduleLocked()
			}
		}
		s.mu.Unlock()
		return false
	}
	s.ObserveSuccess()
	return true
}

// LastAddr returns the host of the last failed request (tests).
func (s *State) LastAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastAddr
}

func (s *State) nowUnix() int64 {
	if s.now != nil {
		return s.now()
	}
	return time.Now().Unix()
}

func (s *State) stampLocked() int64 {
	s.since = s.nowUnix()
	s.hasSince = true
	return s.since
}

func (s *State) delaysOrDefault() []time.Duration {
	if len(s.delays) > 0 {
		return s.delays
	}
	return Delays
}

func (s *State) scheduleLocked() {
	delays := s.delaysOrDefault()
	d := delays[s.backoff]
	if s.backoff >= len(delays) {
		d = delays[len(delays)-1]
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(d, func() {
		s.mu.Lock()
		s.timer = nil
		offline := !s.online
		connected := s.clientConnected
		s.mu.Unlock()
		if offline && connected {
			s.ProbeOnce()
		}
	})
}

func (s *State) stopLocked() {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

// dialProbe is the default probe: a TCP dial with a 1 s timeout to addr.
// addr without a port dials 443. No third-party host is ever involved: addr
// is always the host of the last failed request.
func dialProbe(addr string) bool {
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "443")
	}
	c, err := net.DialTimeout("tcp", addr, ProbeTimeout)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// Transport is an http.RoundTripper that reports each request's outcome to
// the tracker: a network-class error marks offline, any success (even an
// HTTP 500, which proves the network works) marks online. Wrap every core
// HTTP client with it, so any outbound request feeds the same state.
type Transport struct {
	State *State
	Base  http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (t Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if t.State == nil {
		return resp, err
	}
	addr := ""
	if req != nil && req.URL != nil {
		addr = req.URL.Host
	}
	if err != nil {
		t.State.ObserveError(err, addr)
		return resp, err
	}
	t.State.ObserveSuccess()
	return resp, nil
}

// WrapClient returns a client like c whose requests feed s. A nil client
// means a default one with the given timeout.
func WrapClient(c *http.Client, s *State, timeout time.Duration) *http.Client {
	base := http.DefaultTransport
	var checkRedirect func(*http.Request, []*http.Request) error
	if c != nil {
		if c.Transport != nil {
			base = c.Transport
		}
		checkRedirect = c.CheckRedirect
		if c.Timeout != 0 {
			timeout = c.Timeout
		}
	}
	return &http.Client{
		Transport:     Transport{State: s, Base: base},
		CheckRedirect: checkRedirect,
		Timeout:       timeout,
	}
}
