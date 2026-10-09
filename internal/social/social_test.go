package social

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"hydra-launcher/internal/jsonrpc"
)

// fakeBackend is a plugin that records what it is sent. It answers social.state
// with snapshot and optional methods with -32601 when missing is set.
type fakeBackend struct {
	mu       sync.Mutex
	offers   bool
	snapshot any
	missing  map[string]bool
	fail     map[string]error
	calls    []fakeCall
	views    chan View
	// hook, when set, runs for every call before it is answered; a non-nil
	// error it returns is the answer.
	hook func(method string) error
}

type fakeCall struct {
	plugin string
	method string
	params json.RawMessage
}

func newFake() *fakeBackend {
	return &fakeBackend{offers: true, missing: map[string]bool{}, fail: map[string]error{}, views: make(chan View, 64)}
}

func (f *fakeBackend) Offers(context.Context, string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.offers
}

func (f *fakeBackend) Call(_ context.Context, plugin, method string, params, result any) error {
	raw, _ := json.Marshal(params)
	f.mu.Lock()
	f.calls = append(f.calls, fakeCall{plugin, method, raw})
	missing, fail, snap := f.missing[method], f.fail[method], f.snapshot
	f.mu.Unlock()
	if f.hook != nil {
		if err := f.hook(method); err != nil {
			return err
		}
	}
	if missing {
		return &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "method not found: " + method}
	}
	if fail != nil {
		return fail
	}
	if method == MethodState && result != nil && snap != nil {
		data, _ := json.Marshal(snap)
		return json.Unmarshal(data, result)
	}
	return nil
}

// sent returns the methods received, in order, optionally filtered.
func (f *fakeBackend) sent(method string) []json.RawMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []json.RawMessage
	for _, c := range f.calls {
		if c.method == method {
			out = append(out, c.params)
		}
	}
	return out
}

func (f *fakeBackend) count(method string) int { return len(f.sent(method)) }

func newManager(f *fakeBackend, enabled bool) *Manager {
	return NewManager(Options{
		Backend:    f,
		Enabled:    func(string) bool { return enabled },
		Emit:       func(v View) { f.views <- v },
		MinBackoff: time.Millisecond,
		MaxBackoff: 5 * time.Millisecond,
	})
}

// waitFor polls cond until it holds or the test times out.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func friend(id, name, state string) Friend { return Friend{ID: id, Name: name, State: state} }

func TestStartSeedsTheViewFromTheSnapshot(t *testing.T) {
	f := newFake()
	f.snapshot = map[string]any{
		"connection": Connection{State: ConnOnline},
		"self":       Self{Name: "me", State: "online"},
		"friends":    []Friend{friend("1", "Ana", "online"), friend("2", "Bo", "offline")},
	}
	m := newManager(f, true)
	m.Start("echo")
	defer m.StopAll()

	waitFor(t, "a connected view", func() bool {
		v := m.View("echo")
		return v.Connection.State == ConnOnline && len(v.Friends) == 2
	})
	v := m.View("echo")
	if !v.Running || !v.Enabled || v.Self == nil || v.Self.Name != "me" {
		t.Fatalf("view after start: %+v", v)
	}
	if f.count(MethodStart) != 1 || f.count(MethodState) != 1 {
		t.Fatalf("start %d, state %d", f.count(MethodStart), f.count(MethodState))
	}
}

func TestFullPushReplacesAndIncrementalMerges(t *testing.T) {
	f := newFake()
	m := newManager(f, true)
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "the first view", func() bool { return len(m.View("echo").Connection.State) > 0 })

	m.Notify("echo", NoteFriends, json.RawMessage(`{"full":true,"friends":[{"id":"1","name":"Ana","state":"online"},{"id":"2","name":"Bo","state":"offline"}]}`))
	m.Notify("echo", NoteFriends, json.RawMessage(`{"full":false,"friends":[{"id":"2","name":"Bo","state":"online","game":{"id":"p.echo.tiny","title":"Echo Tiny"}},{"id":"3","name":"Cy","state":"away"}]}`))
	v := m.View("echo")
	if len(v.Friends) != 3 || v.Friends[1].State != "online" || v.Friends[1].Game == nil || v.Friends[1].Game.Title != "Echo Tiny" {
		t.Fatalf("merged friends: %+v", v.Friends)
	}

	m.Notify("echo", NoteFriends, json.RawMessage(`{"full":true,"friends":[{"id":"9","name":"Zed","state":"busy"}]}`))
	if v := m.View("echo"); len(v.Friends) != 1 || v.Friends[0].ID != "9" {
		t.Fatalf("a full push should replace the list: %+v", v.Friends)
	}
}

func TestNotificationsIgnoreUnknownAndBadPayloads(t *testing.T) {
	f := newFake()
	m := newManager(f, true)
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "the first view", func() bool { return len(m.View("echo").Connection.State) > 0 })
	m.Notify("echo", "social.nonsense", json.RawMessage(`{}`))
	m.Notify("echo", NoteConnection, json.RawMessage(`{"message":"no state"}`))
	m.Notify("echo", NoteFriends, json.RawMessage(`not json`))
	m.Notify("nobody", NoteFriends, json.RawMessage(`{"full":true,"friends":[]}`))
	if v := m.View("echo"); v.Connection.State == "" || v.Friends == nil {
		t.Fatalf("bad payloads changed the view: %+v", v)
	}
}

func TestPlayingIsSentForTheGameOfTheRightPlugin(t *testing.T) {
	f := newFake()
	m := newManager(f, true)
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "connected", func() bool { return f.count(MethodState) == 1 })

	m.GameStarted("other", "p.other.42", "42") // another store: not subscribed here
	m.GameStarted("echo", "p.echo.tiny", "")   // no store id
	m.GameStarted("echo", "p.echo.tiny", "tiny")
	waitFor(t, "setPlaying tiny", func() bool { return f.count(MethodSetPlaying) == 1 })
	if got := string(f.sent(MethodSetPlaying)[0]); got != `{"id":"tiny"}` {
		t.Fatalf("setPlaying: %s", got)
	}

	m.GameStopped("p.echo.other") // not the game recorded: nothing changes
	m.GameStopped("p.echo.tiny")
	waitFor(t, "setPlaying null", func() bool { return f.count(MethodSetPlaying) == 2 })
	if got := string(f.sent(MethodSetPlaying)[1]); got != `{"id":null}` {
		t.Fatalf("setPlaying after stop: %s", got)
	}
}

// A game started while friends are off is sent when they turn on.
func TestPlayingIsSentWhenFriendsTurnOnMidGame(t *testing.T) {
	f := newFake()
	m := newManager(f, true)
	m.GameStarted("echo", "p.echo.tiny", "tiny")
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "setPlaying tiny", func() bool { return f.count(MethodSetPlaying) == 1 })
	if got := string(f.sent(MethodSetPlaying)[0]); got != `{"id":"tiny"}` {
		t.Fatalf("setPlaying: %s", got)
	}
}

// With the per-plugin setting off, the game is not sent, and turning it off
// while sent clears it on the plugin.
func TestPlayingFollowsTheShowPlayingSetting(t *testing.T) {
	f := newFake()
	var show atomic.Bool
	show.Store(false)
	m := NewManager(Options{
		Backend:     f,
		Enabled:     func(string) bool { return true },
		ShowPlaying: func(string) bool { return show.Load() },
		Emit:        func(View) {},
	})
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "connected", func() bool { return f.count(MethodState) == 1 })
	m.GameStarted("echo", "p.echo.tiny", "tiny")
	time.Sleep(10 * time.Millisecond)
	if n := f.count(MethodSetPlaying); n != 0 {
		t.Fatalf("setPlaying sent %d times with the setting off", n)
	}

	show.Store(true)
	m.Refresh("echo")
	waitFor(t, "setPlaying tiny", func() bool { return f.count(MethodSetPlaying) == 1 })
	show.Store(false)
	m.Refresh("echo")
	waitFor(t, "setPlaying null", func() bool { return f.count(MethodSetPlaying) == 2 })
	if got := string(f.sent(MethodSetPlaying)[1]); got != `{"id":null}` {
		t.Fatalf("setPlaying after the setting went off: %s", got)
	}
}

func TestPlayingMissingOptionalMethodIsNotAnError(t *testing.T) {
	f := newFake()
	f.missing[MethodSetPlaying] = true
	m := newManager(f, true)
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "connected", func() bool { return f.count(MethodState) == 1 })
	m.GameStarted("echo", "p.echo.tiny", "tiny")
	waitFor(t, "setPlaying sent", func() bool { return f.count(MethodSetPlaying) == 1 })
	m.GameStopped("p.echo.tiny")
	waitFor(t, "null sent", func() bool { return f.count(MethodSetPlaying) == 2 })
}

func TestPlayingIsReplayedAfterARestart(t *testing.T) {
	f := newFake()
	m := newManager(f, true)
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "connected", func() bool { return f.count(MethodState) == 1 })
	m.GameStarted("echo", "p.echo.tiny", "tiny")
	waitFor(t, "setPlaying", func() bool { return f.count(MethodSetPlaying) == 1 })

	m.Exited("echo") // the plugin process went away
	waitFor(t, "a second start", func() bool { return f.count(MethodStart) == 2 })
	waitFor(t, "the game replayed", func() bool { return f.count(MethodSetPlaying) == 2 })
	if got := string(f.sent(MethodSetPlaying)[1]); got != `{"id":"tiny"}` {
		t.Fatalf("replayed playing: %s", got)
	}
}

func TestExitRestartsWithBackoffAndReconnects(t *testing.T) {
	f := newFake()
	m := newManager(f, true)
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "connected", func() bool { return f.count(MethodState) == 1 })

	m.Exited("echo")
	waitFor(t, "restart", func() bool { return f.count(MethodStart) == 2 })
	m.Exited("echo")
	waitFor(t, "second restart", func() bool { return f.count(MethodStart) == 3 })
}

func TestOfferWithdrawnKeepsRetrying(t *testing.T) {
	f := newFake()
	f.offers = false
	m := newManager(f, true)
	m.Start("echo")
	defer m.StopAll()
	time.Sleep(20 * time.Millisecond)
	if f.count(MethodStart) != 0 {
		t.Fatal("social.start sent to a plugin that doesn't offer social")
	}
	f.mu.Lock()
	f.offers = true
	f.mu.Unlock()
	waitFor(t, "start once offered", func() bool { return f.count(MethodStart) == 1 })
}

func TestReplacedStopsTheRestartLoop(t *testing.T) {
	f := newFake()
	m := newManager(f, true)
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "connected", func() bool { return f.count(MethodState) == 1 })

	m.Notify("echo", NoteConnection, json.RawMessage(`{"state":"replaced"}`))
	m.Exited("echo")
	time.Sleep(20 * time.Millisecond)
	if n := f.count(MethodStart); n != 1 {
		t.Fatalf("a replaced session restarted %d times", n-1)
	}
	if v := m.View("echo"); v.Connection.State != ConnReplaced {
		t.Fatalf("connection after replaced: %+v", v.Connection)
	}
}

func TestStopAsksThePluginToStopAndClearsTheView(t *testing.T) {
	f := newFake()
	m := newManager(f, true)
	m.Start("echo")
	waitFor(t, "connected", func() bool { return f.count(MethodState) == 1 })
	m.Notify("echo", NoteFriends, json.RawMessage(`{"full":true,"friends":[{"id":"1","name":"Ana","state":"online"}]}`))

	m.Stop("echo")
	if f.count(MethodStop) != 1 {
		t.Fatalf("social.stop sent %d times", f.count(MethodStop))
	}
	v := m.View("echo")
	if v.Running || len(v.Friends) != 0 || v.Connection.State != ConnOffline {
		t.Fatalf("view after stop: %+v", v)
	}
	if m.Running("echo") {
		t.Fatal("still subscribed after stop")
	}
	m.Stop("echo") // idempotent
}

func TestStopOfAnUnconnectedSubscriptionSendsNothing(t *testing.T) {
	f := newFake()
	f.offers = false
	m := newManager(f, true)
	m.Start("echo")
	m.Stop("echo")
	if f.count(MethodStop) != 0 {
		t.Fatal("social.stop sent to a plugin that never started")
	}
}

func TestSetStatusNeedsAConnectionAndAKnownStatus(t *testing.T) {
	f := newFake()
	m := newManager(f, true)
	ctx := context.Background()
	if err := m.SetStatus(ctx, "echo", "online"); err == nil {
		t.Fatal("status with no subscription should fail")
	}
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "connected", func() bool { return f.count(MethodState) == 1 })

	if err := m.SetStatus(ctx, "echo", "lurking"); err == nil {
		t.Fatal("unknown status accepted")
	}
	if err := m.SetStatus(ctx, "echo", "busy"); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if got := string(f.sent(MethodSetStatus)[0]); got != `{"state":"busy"}` {
		t.Fatalf("setStatus params: %s", got)
	}
	f.missing[MethodSetStatus] = true
	if err := m.SetStatus(ctx, "echo", "away"); err == nil {
		t.Fatalf("a store without setStatus should refuse with a message, got %v", err)
	}
}

func TestEnabledFollowsTheSetting(t *testing.T) {
	f := newFake()
	m := newManager(f, false)
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "a view", func() bool { return m.View("echo").Plugin == "echo" })
	if v := m.View("echo"); v.Enabled {
		t.Fatalf("enabled should come from the setting: %+v", v)
	}
	if v := m.View("other"); v.Running || v.Enabled || v.Friends == nil {
		t.Fatalf("a plugin without a subscription: %+v", v)
	}
}

func TestNothingIsSentWithoutSubscription(t *testing.T) {
	f := newFake()
	m := newManager(f, true)
	m.GameStarted("echo", "p.echo.tiny", "tiny")
	m.Exited("echo")
	m.Notify("echo", NoteFriends, json.RawMessage(`{"full":true,"friends":[]}`))
	if len(f.calls) != 0 {
		t.Fatalf("calls without a subscription: %v", f.calls)
	}
}

// restartWaits records the wait before each restart after a session ends.
func restartWaits(m *Manager, waits *[]string, mu *sync.Mutex) {
	m.opt.Logf = func(format string, args ...any) {
		if strings.HasPrefix(format, "social %s: restarting") {
			mu.Lock()
			*waits = append(*waits, fmt.Sprint(args[1]))
			mu.Unlock()
		}
	}
}

// Short sessions back off: the wait doubles from one restart to the next.
func TestShortSessionsBackOff(t *testing.T) {
	f := newFake()
	var mu sync.Mutex
	var waits []string
	m := NewManager(Options{
		Backend:    f,
		Emit:       func(View) {},
		MinBackoff: time.Millisecond,
		MaxBackoff: 8 * time.Millisecond,
	})
	restartWaits(m, &waits, &mu)
	m.Start("echo")
	defer m.StopAll()
	for n := 2; n <= 4; n++ {
		waitFor(t, "a connected session", func() bool { return f.count(MethodState) == n-1 })
		m.Exited("echo")
		waitFor(t, "a restart", func() bool { return f.count(MethodStart) == n })
	}
	mu.Lock()
	defer mu.Unlock()
	if got := strings.Join(waits, " "); got != "1ms 2ms 4ms" {
		t.Fatalf("restart waits: %q, want doubling from 1ms", got)
	}
}

// A session that lasted past StableAfter counts as healthy: the wait starts again at the minimum.
func TestLongSessionResetsBackoff(t *testing.T) {
	f := newFake()
	var mu sync.Mutex
	var waits []string
	m := NewManager(Options{
		Backend:     f,
		Emit:        func(View) {},
		MinBackoff:  time.Millisecond,
		MaxBackoff:  8 * time.Millisecond,
		StableAfter: time.Nanosecond,
	})
	restartWaits(m, &waits, &mu)
	m.Start("echo")
	defer m.StopAll()
	for n := 2; n <= 4; n++ {
		waitFor(t, "a connected session", func() bool { return f.count(MethodState) == n-1 })
		time.Sleep(time.Millisecond)
		m.Exited("echo")
		waitFor(t, "a restart", func() bool { return f.count(MethodStart) == n })
	}
	mu.Lock()
	defer mu.Unlock()
	if got := strings.Join(waits, " "); got != "1ms 1ms 1ms" {
		t.Fatalf("restart waits: %q, want the minimum after stable sessions", got)
	}
}

// A plugin that started but whose snapshot failed is still stopped when the
// subscription ends: social.start succeeded, so the process has social on.
func TestStopStopsAPluginWhoseStateFailed(t *testing.T) {
	f := newFake()
	f.fail[MethodState] = errors.New("store is down")
	m := newManager(f, true)
	m.Start("echo")
	waitFor(t, "a start", func() bool { return f.count(MethodStart) >= 1 })
	m.Stop("echo")
	if n := f.count(MethodStop); n != 1 {
		t.Fatalf("social.stop sent %d times after a failed social.state, want 1", n)
	}
}

// A crash during a failed connect leaves an exit behind. It must not end the
// next session, which is healthy.
func TestExitFromAFailedConnectDoesNotEndTheNextSession(t *testing.T) {
	f := newFake()
	f.snapshot = map[string]any{"connection": Connection{State: ConnOnline}}
	var mu sync.Mutex
	var states []string
	m := NewManager(Options{Backend: f, MinBackoff: time.Millisecond, MaxBackoff: time.Millisecond,
		Emit: func(v View) {
			mu.Lock()
			states = append(states, v.Connection.State)
			mu.Unlock()
		}})
	failed := false
	f.hook = func(method string) error {
		if method == MethodState && !failed {
			failed = true
			m.Exited("echo") // the process died while this attempt was failing
			return errors.New("store is down")
		}
		return nil
	}
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "connected", func() bool { return m.View("echo").Connection.State == ConnOnline })
	time.Sleep(30 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if slices.Contains(states, ConnReconnecting) {
		t.Fatalf("the healthy session ended by a stale exit; states %v", states)
	}
}

// A plugin that reports replaced and then comes back online is restarted when
// it exits: the pause lasts only until the plugin connects again.
func TestReconnectedPluginIsRestartedWhenItExits(t *testing.T) {
	f := newFake()
	m := newManager(f, true)
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "connected", func() bool { return f.count(MethodState) == 1 })

	m.Notify("echo", NoteConnection, json.RawMessage(`{"state":"replaced"}`))
	m.Notify("echo", NoteConnection, json.RawMessage(`{"state":"online"}`))
	m.Exited("echo")
	waitFor(t, "a restart", func() bool { return f.count(MethodStart) == 2 })
}

// Emits of one plugin's view never overlap: a change made while an emit is
// running waits for it, so the UI can't receive an older view last.
func TestEmitsOfAPluginDoNotOverlap(t *testing.T) {
	f := newFake()
	f.offers = false
	var gate atomic.Bool
	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	m := NewManager(Options{Backend: f, Emit: func(View) {
		if gate.Load() {
			entered <- struct{}{}
			<-release
		}
	}})
	m.Start("echo")
	defer m.StopAll()
	// Runs first on failure too, so the emits held here can finish.
	defer func() { gate.Store(false); close(release) }()

	gate.Store(true)
	go m.Notify("echo", NoteFriends, json.RawMessage(`{"full":true,"friends":[{"id":"1","name":"Ana"}]}`))
	<-entered
	go m.Notify("echo", NoteFriends, json.RawMessage(`{"full":false,"friends":[{"id":"2","name":"Bo"}]}`))
	time.Sleep(20 * time.Millisecond)
	if n := len(entered); n != 0 {
		t.Fatalf("%d emits ran while another was still running", n)
	}
}

// Every view made carries a sequence number that rises, so a UI can tell an
// older view from a newer one.
func TestViewsCarryARisingSequence(t *testing.T) {
	f := newFake()
	m := newManager(f, true)
	m.Start("echo")
	defer m.StopAll()
	first := m.View("echo").Seq
	second := m.View("echo").Seq
	if first == 0 || second <= first {
		t.Fatalf("sequence %d then %d, want rising", first, second)
	}
}

// A game goes to the plugin it was launched from, even when another plugin's
// id is a dotted prefix of its key.
func TestPlayingGoesToTheOwningPlugin(t *testing.T) {
	f := newFake()
	m := newManager(f, true)
	m.Start("steam")
	m.Start("steam.beta")
	defer m.StopAll()
	waitFor(t, "both connected", func() bool { return f.count(MethodState) == 2 })

	m.GameStarted("steam.beta", "p.steam.beta.1", "1")
	waitFor(t, "setPlaying", func() bool { return f.count(MethodSetPlaying) == 1 })
	for _, c := range f.calls {
		if c.method == MethodSetPlaying && c.plugin != "steam.beta" {
			t.Fatalf("setPlaying for steam.beta went to %s: %s", c.plugin, c.params)
		}
	}
}
