package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"platypus/internal/dataplug"
	"platypus/internal/types"
)

// Login-state cache (issue #28): the Plugins view asks every plugin whether
// the user is logged in on every visit, and each ask costs the plugin a store
// round trip. PluginAuthStatus serves the last known state immediately and
// refreshes it in the background (single-flight per plugin), invalidating on
// login/logout/auth completion, plugin changes and process restarts, and
// reporting a changed refresh on the "auth" event.

// authEventHost captures the events an App emits in tests.
type authEventHost struct {
	mu     sync.Mutex
	events []types.AuthEvent
}

func (h *authEventHost) Emit(event string, data ...any) {
	if event != "auth" || len(data) == 0 {
		return
	}
	e, ok := data[0].(types.AuthEvent)
	if !ok {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, e)
}

func (h *authEventHost) Infof(string, ...any)  {}
func (h *authEventHost) Warnf(string, ...any)  {}
func (h *authEventHost) Errorf(string, ...any) {}

func (h *authEventHost) OpenFile(FileDialog) (string, error) {
	return "", errors.New("no dialog in tests")
}

func (h *authEventHost) OpenDirectory(DirectoryDialog) (string, error) {
	return "", errors.New("no dialog in tests")
}

func (h *authEventHost) authEvents() []types.AuthEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]types.AuthEvent(nil), h.events...)
}

// authEnv is a whole App around the echo test plugin with the auth capability,
// logged in, and an event-capturing host. Echo answers auth.status after
// ECHO_AUTH_STATUS_DELAY_MS when the test sets it before the plugin starts.
func authEnv(t *testing.T) (*versionsEnv, *authEventHost) {
	t.Helper()
	e := versionsApp(t, allVersions, echoCaps)
	e.a.data = dataplug.NewRegistry()
	host := &authEventHost{}
	e.a.host = host
	return e, host
}

func authCalls(t *testing.T, e *versionsEnv) int {
	t.Helper()
	return e.calls(t)["auth.status"]
}

// waitAuthCalls waits until the plugin has answered want auth.status calls.
func waitAuthCalls(t *testing.T, e *versionsEnv, want int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if n := authCalls(t, e); n >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("auth.status answered %d times, want at least %d", authCalls(t, e), want)
}

// A second PluginAuthStatus doesn't hit the plugin synchronously: with a slow
// login check the first read blocks and the second answers from the cache,
// refreshing in the background afterwards.
func TestAuthStatusSecondCallServesCache(t *testing.T) {
	t.Setenv("ECHO_AUTH_STATUS_DELAY_MS", "800")
	e, _ := authEnv(t)

	start := time.Now()
	st, err := e.a.PluginAuthStatus("echo")
	first := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if !st.LoggedIn {
		t.Fatalf("first status = %+v, want logged in (the harness logs in)", st)
	}
	if first < 500*time.Millisecond {
		t.Fatalf("first status took %v, want the plugin's slow login check", first)
	}
	if n := authCalls(t, e); n != 1 {
		t.Fatalf("first status hit the plugin %d times, want 1", n)
	}

	start = time.Now()
	again, err := e.a.PluginAuthStatus("echo")
	second := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if again != st {
		t.Fatalf("second status = %+v, want the cached %+v", again, st)
	}
	if second >= 500*time.Millisecond {
		t.Fatalf("second status took %v, want the cached answer", second)
	}
	// The background refresh the second read triggered must not have answered
	// yet: nothing hit the plugin synchronously.
	if n := authCalls(t, e); n != 1 {
		t.Fatalf("second status hit the plugin synchronously (%d calls, want 1)", n)
	}

	// The refresh does run: the plugin is asked again in the background.
	waitAuthCalls(t, e, 2)
}

// Concurrent first reads share one plugin round trip (single-flight), and all
// callers get its answer.
func TestAuthStatusSingleFlight(t *testing.T) {
	e, _ := authEnv(t)
	e.a.invalidateAuth("echo")

	const callers = 8
	results := make([]types.AuthStatus, callers)
	errs := make([]error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = e.a.PluginAuthStatus("echo")
		}(i)
	}
	wg.Wait()
	for i := 0; i < callers; i++ {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if results[i] != results[0] {
			t.Fatalf("caller %d got %+v, want %+v", i, results[i], results[0])
		}
	}
	if n := authCalls(t, e); n != 1 {
		t.Fatalf("%d callers hit the plugin %d times, want 1", callers, n)
	}
}

// Concurrent reads of a cached state trigger exactly one background refresh.
func TestAuthStatusRefreshSingleFlight(t *testing.T) {
	e, _ := authEnv(t)
	if _, err := e.a.PluginAuthStatus("echo"); err != nil {
		t.Fatal(err)
	}
	waitAuthCalls(t, e, 1)
	warm := authCalls(t, e)

	const callers = 8
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := e.a.PluginAuthStatus("echo"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	waitAuthCalls(t, e, warm+1)
	// The single background refresh has answered: wait until its flight is
	// gone, then make sure no second refresh follows. A fixed sleep would
	// miss a delayed duplicate when the machine is busy, so poll the call
	// count until it has stayed put long enough.
	waitAuthIdle(t, e)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		if n := authCalls(t, e); n != warm+1 {
			t.Fatalf("%d cached readers refreshed the plugin %d times, want 1", callers, n-warm)
		}
	}
}

// waitAuthIdle waits until no auth.status round trip is in flight.
func waitAuthIdle(t *testing.T, e *versionsEnv) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		e.a.auth.mu.Lock()
		busy := len(e.a.auth.inflight) != 0
		e.a.auth.mu.Unlock()
		if !busy {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("auth refresh still in flight after 10s")
}

// A logout and a login write the fresh state straight into the cache: the
// next read serves it without hitting the plugin. A refused login changes
// nothing.
func TestAuthLogoutAndLoginRefreshCache(t *testing.T) {
	e, _ := authEnv(t)

	st, err := e.a.PluginAuthComplete("echo", map[string]string{"code": "letmein"})
	if err != nil {
		t.Fatal(err)
	}
	if !st.LoggedIn {
		t.Fatalf("complete = %+v, want logged in", st)
	}
	before := authCalls(t, e)
	if got, err := e.a.PluginAuthStatus("echo"); err != nil || !got.LoggedIn {
		t.Fatalf("status after login = %+v, %v; plugin hits: %d (want no new hit)", got, err, authCalls(t, e)-before)
	}

	st, err = e.a.PluginLogout("echo")
	if err != nil {
		t.Fatal(err)
	}
	if st.LoggedIn {
		t.Fatalf("logout = %+v, want logged out", st)
	}
	before = authCalls(t, e)
	if got, err := e.a.PluginAuthStatus("echo"); err != nil || got.LoggedIn {
		t.Fatalf("status after logout = %+v, %v; plugin hits: %d (want no new hit)", got, err, authCalls(t, e)-before)
	}

	if _, err := e.a.PluginAuthComplete("echo", map[string]string{"code": "nope"}); err == nil {
		t.Fatal("a wrong login code must fail")
	}
	before = authCalls(t, e)
	if got, err := e.a.PluginAuthStatus("echo"); err != nil || got.LoggedIn {
		t.Fatalf("status after a refused login = %+v, %v; plugin hits: %d (want no new hit)", got, err, authCalls(t, e)-before)
	}
}

// Rescanning, retrying and a process restart drop the cached state: the next
// read asks the plugin again, synchronously.
func TestAuthInvalidatedByRescanRetryAndRestart(t *testing.T) {
	e, _ := authEnv(t)
	if _, err := e.a.PluginAuthStatus("echo"); err != nil {
		t.Fatal(err)
	}
	waitAuthCalls(t, e, 1)

	for _, change := range []struct {
		name string
		do   func() error
	}{
		{"rescan", func() error { _, err := e.a.RescanPlugins(); return err }},
		{"retry", func() error { return e.a.RetryPlugin("echo") }},
		{"restart", func() error { e.a.pluginReady("echo"); return nil }},
	} {
		t.Run(change.name, func(t *testing.T) {
			// A settled cache: the earlier background refresh has answered.
			waitAuthCalls(t, e, authCalls(t, e))
			before := authCalls(t, e)
			if err := change.do(); err != nil {
				t.Fatal(err)
			}
			if _, err := e.a.PluginAuthStatus("echo"); err != nil {
				t.Fatal(err)
			}
			// Cache was dropped: this read blocked on the plugin, so its
			// answer is already counted on return (a background refresh, if
			// any, only adds more later).
			if n := authCalls(t, e); n != before+1 {
				t.Fatalf("status after %s hit the plugin %d times, want 1", change.name, n-before)
			}
			waitAuthCalls(t, e, before+1)
		})
	}
}

// A background refresh that finds a changed state reports it on the "auth"
// event, carrying the fresh state.
func TestAuthRefreshEmitsEventOnChange(t *testing.T) {
	e, host := authEnv(t)

	if _, err := e.a.PluginAuthStatus("echo"); err != nil {
		t.Fatal(err)
	}
	waitAuthCalls(t, e, 1)
	if _, err := e.a.PluginLogout("echo"); err != nil {
		t.Fatal(err)
	}
	host.mu.Lock()
	host.events = nil
	host.mu.Unlock()

	// Log back in behind the cache's back: the plugin says logged in, the
	// cache still says logged out.
	if err := e.m.Call(context.Background(), "echo", "auth.complete",
		map[string]any{"fields": map[string]string{"code": "letmein"}}, nil); err != nil {
		t.Fatal(err)
	}
	st, err := e.a.PluginAuthStatus("echo")
	if err != nil {
		t.Fatal(err)
	}
	if st.LoggedIn {
		t.Fatalf("cached status = %+v, want the stale logged-out state first", st)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, ev := range host.authEvents() {
			if ev.Plugin == "echo" {
				if !ev.Status.LoggedIn {
					t.Fatalf("auth event = %+v, want logged in", ev)
				}
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no auth event after the refresh found a changed login state")
}

// A background check that started before a logout must not put the old
// "logged in" back when it answers after it.
func TestAuthStaleRefreshDoesNotOverwriteLogout(t *testing.T) {
	t.Setenv("ECHO_AUTH_STATUS_DELAY_MS", "400")
	e, _ := authEnv(t)

	if st, err := e.a.PluginAuthStatus("echo"); err != nil || !st.LoggedIn {
		t.Fatalf("first status = %+v, %v; want logged in", st, err)
	}
	// Serves the cache and starts a slow background check (answers "logged in").
	if _, err := e.a.PluginAuthStatus("echo"); err != nil {
		t.Fatal(err)
	}
	// The logout lands while that check is still running.
	e.a.setAuthStatus("echo", types.AuthStatus{LoggedIn: false})
	waitAuthCalls(t, e, 2)
	// Let the refresh store (or discard) its answer.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		e.a.auth.mu.Lock()
		_, busy := e.a.auth.inflight["echo"]
		e.a.auth.mu.Unlock()
		if !busy {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	e.a.auth.mu.Lock()
	got := e.a.auth.entries["echo"]
	e.a.auth.mu.Unlock()
	if got.LoggedIn {
		t.Fatalf("cached state = %+v after a logout during a refresh, want logged out", got)
	}
}
