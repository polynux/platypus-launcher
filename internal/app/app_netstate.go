package app

import (
	"net/http"
	"time"

	"platypus/internal/netstate"
	"platypus/internal/types"
)

// Offline tracking (issue #44): every core outbound request reports its
// outcome to a.net, the UI reads it through GetNetState and the "netstate"
// event, and background work skips its fetches while offline.

// GetNetState returns whether the machine looks online, and when the
// current state began (unix seconds, 0 when never changed).
func (a *App) GetNetState() types.NetState {
	if a == nil || a.net == nil {
		return types.NetState{Online: true}
	}
	snap := a.net.Snapshot()
	return types.NetState{Online: snap.Online, Since: snap.Since}
}

// netOnline reports whether the machine looks online. Tests without a
// tracker (nil net) read as online.
func (a *App) netOnline() bool {
	return a == nil || a.net == nil || a.net.Online()
}

// noteNetSuccess marks the state online: any successful core request clears
// a previous offline marking.
func (a *App) noteNetSuccess() {
	if a != nil && a.net != nil {
		a.net.ObserveSuccess()
	}
}

// noteNetError marks the state offline when err is network-class (DNS,
// operation or timeout errors). addr is the request's host, remembered for
// the probe. Anything else changes nothing.
func (a *App) noteNetError(err error, addr string) {
	if a != nil && a.net != nil {
		a.net.ObserveError(err, addr)
	}
}

// netClient wraps an HTTP client so its requests feed the offline tracker.
// A nil client means a default one with the given timeout.
func (a *App) netClient(c *http.Client, timeout time.Duration) *http.Client {
	if a == nil || a.net == nil {
		if c != nil {
			return c
		}
		return &http.Client{Timeout: timeout}
	}
	return netstate.WrapClient(c, a.net, timeout)
}

// netConnectedForTests marks a UI client connected (tests): probes only run
// while one is. Production sets it in startup and clears it in shutdown.
func (a *App) netConnectedForTests(connected bool) {
	if a != nil && a.net != nil {
		a.net.SetClientConnected(connected)
	}
}
