package app

import (
	"sync"

	"platypus/internal/types"
)

// Login-state cache (issue #28): asking a plugin whether the user is logged
// in costs it a store round trip (~1 s), and the Plugins view asked on every
// visit. PluginAuthStatus therefore keeps the last AuthStatus of every plugin
// and serves it immediately, refreshing it in the background; the UI updates
// when the refresh finds a change (the "auth" event). The zero value is ready
// to use.
//
// Invalidation is explicit: a login, a logout or an auth completion writes the
// fresh state straight into the cache, and anything that may change what the
// plugin answers without going through those (approve, enable, retry, rescan,
// a process restart or a crash) drops the entry, so the next read asks the
// plugin again.
type authCache struct {
	mu sync.Mutex
	// entries is the last known login state by plugin id. Failures are never
	// cached: a failed read must not pin a stale state.
	entries map[string]types.AuthStatus
	// inflight is the auth.status call currently running by plugin id, slow
	// or background: never two concurrent auth.status calls for one plugin.
	inflight map[string]*authFlight
	// gen counts, by plugin id, the writes and invalidations that don't come
	// from a round trip (login, logout, approve, restart…). A round trip
	// stores its answer only if gen didn't move while it ran: a check that
	// started before a logout must not put the old "logged in" back.
	gen map[string]uint64
}

// authFlight is one auth.status round trip. A background refresh has no
// waiter: its caller already answered from the cache.
type authFlight struct {
	gen    uint64 // auth.gen[id] when the round trip started
	done   chan struct{}
	status types.AuthStatus
	err    error
}

// PluginAuthStatus asks the plugin whether the user is logged in. It starts
// the plugin if needed.
//
// A known state is served immediately and refreshed in the background, so
// revisiting the Plugins view never waits on the store; only the first read
// (or one after an invalidation) blocks on the plugin. Concurrent reads share
// one round trip.
func (a *App) PluginAuthStatus(id string) (types.AuthStatus, error) {
	a.auth.mu.Lock()
	if a.auth.inflight == nil {
		a.auth.inflight = map[string]*authFlight{}
	}
	if st, ok := a.auth.entries[id]; ok {
		if _, busy := a.auth.inflight[id]; !busy {
			f := &authFlight{gen: a.auth.gen[id], done: make(chan struct{})}
			a.auth.inflight[id] = f
			go a.refreshAuthStatus(id, f)
		}
		a.auth.mu.Unlock()
		return st, nil
	}
	f, leader := a.auth.join(id)
	a.auth.mu.Unlock()
	if !leader {
		// Another read is already asking the plugin: wait for its answer
		// instead of asking again. It is bounded by authTimeout.
		<-f.done
		return f.status, f.err
	}
	var st types.AuthStatus
	err := a.authCall(id, "auth.status", nil, &st)
	a.auth.mu.Lock()
	if err == nil && a.auth.gen[id] == f.gen {
		a.auth.set(id, st)
	}
	f.status, f.err = st, err
	close(f.done)
	delete(a.auth.inflight, id)
	a.auth.mu.Unlock()
	return st, err
}

// join returns the running round trip for id, creating it when this caller is
// the first. Call with auth.mu held.
func (c *authCache) join(id string) (*authFlight, bool) {
	if f, ok := c.inflight[id]; ok {
		return f, false
	}
	if c.inflight == nil {
		c.inflight = map[string]*authFlight{}
	}
	f := &authFlight{gen: c.gen[id], done: make(chan struct{})}
	c.inflight[id] = f
	return f, true
}

// bump marks a write or invalidation that isn't a round trip's answer, so a
// round trip running meanwhile discards its older answer. Call with auth.mu held.
func (c *authCache) bump(id string) {
	if c.gen == nil {
		c.gen = map[string]uint64{}
	}
	c.gen[id]++
}

// set stores a fresh state. Call with auth.mu held.
func (c *authCache) set(id string, st types.AuthStatus) {
	if c.entries == nil {
		c.entries = map[string]types.AuthStatus{}
	}
	c.entries[id] = st
}

// refreshAuthStatus re-reads a cached state in the background and reports it
// on the "auth" event when it changed. f is id's inflight entry.
func (a *App) refreshAuthStatus(id string, f *authFlight) {
	var st types.AuthStatus
	err := a.authCall(id, "auth.status", nil, &st)
	a.auth.mu.Lock()
	changed := false
	if err == nil && a.auth.gen[id] != f.gen {
		// A login, logout or invalidation happened meanwhile: this answer
		// predates it and must not replace what it stored.
		// A read that joined this flight still gets the plugin's answer.
		f.status = st
		close(f.done)
		delete(a.auth.inflight, id)
		a.auth.mu.Unlock()
		return
	}
	if err == nil {
		if prev, ok := a.auth.entries[id]; !ok || prev != st {
			changed = true
		}
		a.auth.set(id, st)
	}
	f.status, f.err = st, err
	close(f.done)
	delete(a.auth.inflight, id)
	a.auth.mu.Unlock()
	if err != nil {
		a.warnf("%s: login check failed: %v", id, err)
		return
	}
	if changed {
		a.emitAuth(id, st)
	}
}

// setAuthStatus stores a state the plugin just returned itself (a login, a
// logout, an auth completion): it is fresh, so no round trip follows. A
// change reaches the UI on the "auth" event, the same as a refresh finding one.
func (a *App) setAuthStatus(id string, st types.AuthStatus) {
	a.auth.mu.Lock()
	prev, ok := a.auth.entries[id]
	a.auth.bump(id)
	a.auth.set(id, st)
	a.auth.mu.Unlock()
	if !ok || prev != st {
		a.emitAuth(id, st)
	}
}

// invalidateAuth drops one plugin's cached login state, so the next read asks
// the plugin again. Call it wherever the plugin may answer differently now.
func (a *App) invalidateAuth(id string) {
	a.auth.mu.Lock()
	defer a.auth.mu.Unlock()
	a.auth.bump(id)
	delete(a.auth.entries, id)
}

// invalidateAuthAll drops every cached login state: the plugin set itself may
// have changed, so no entry can be trusted.
func (a *App) invalidateAuthAll() {
	a.auth.mu.Lock()
	defer a.auth.mu.Unlock()
	for id := range a.auth.entries {
		a.auth.bump(id)
	}
	for id := range a.auth.inflight {
		a.auth.bump(id)
	}
	a.auth.entries = map[string]types.AuthStatus{}
}

// emitAuth reports a plugin's login state to the UI. Tests build an App
// without a host: without one there is nobody to tell.
func (a *App) emitAuth(id string, st types.AuthStatus) {
	if a.host == nil {
		return
	}
	a.host.Emit("auth", types.AuthEvent{Plugin: id, Status: st})
}
