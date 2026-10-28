package app

import (
	"context"
	"errors"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/session"
	"platypus/internal/stores"
	"platypus/internal/types"
)

// sessionEventHost records the "session" events an App emits in tests.
type sessionEventHost struct {
	mu     sync.Mutex
	events []types.SessionEvent
}

func (h *sessionEventHost) Emit(event string, data ...any) {
	if event != "session" || len(data) == 0 {
		return
	}
	e, ok := data[0].(types.SessionEvent)
	if !ok {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, e)
}

func (h *sessionEventHost) Infof(string, ...any)  {}
func (h *sessionEventHost) Warnf(string, ...any)  {}
func (h *sessionEventHost) Errorf(string, ...any) {}

func (h *sessionEventHost) OpenFile(FileDialog) (string, error) {
	return "", errors.New("no dialog in tests")
}

func (h *sessionEventHost) OpenDirectory(DirectoryDialog) (string, error) {
	return "", errors.New("no dialog in tests")
}

func (h *sessionEventHost) snapshot() []types.SessionEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]types.SessionEvent(nil), h.events...)
}

// clientStatusStore is a client store whose client says what it is doing.
// find is the game's pid (0 while the client works); zero values mean "never
// starts", "launching", and a 50 ms start timeout.
type clientStatusStore struct {
	find    func() int
	state   string
	timeout time.Duration
}

func (clientStatusStore) ID() string { return "clientstore" }
func (clientStatusStore) Library(context.Context) ([]types.Game, error) {
	return nil, nil
}
func (clientStatusStore) Details(context.Context, types.Game) (*types.GameDetails, error) {
	return nil, nil
}
func (s clientStatusStore) Launch(context.Context, types.Game, bool) (*stores.Launch, error) {
	find, state, timeout := s.find, s.state, s.timeout
	if find == nil {
		find = func() int { return 0 }
	}
	if state == "" {
		state = "launching"
	}
	if timeout == 0 {
		timeout = 50 * time.Millisecond
	}
	return &stores.Launch{Client: &stores.ClientLaunch{
		Client:       "Steam",
		Command:      []string{"sh", "-c", "exit 0"},
		StartTimeout: timeout,
		Find:         find,
		ClientStatus: func(time.Time) (*types.ClientStatus, error) {
			return &types.ClientStatus{State: state}, nil
		},
	}}, nil
}

// launchClient starts a client launch through the App and returns the host
// that records its session events.
func launchClient(t *testing.T, a *App, store clientStatusStore) *sessionEventHost {
	t.Helper()
	host := &sessionEventHost{}
	a.host = host
	a.sessions = session.NewTracker(func(e types.SessionEvent) { a.host.Emit("session", e) })
	a.ctx = context.Background()
	a.stores = stores.NewRegistry(nil, store)
	g := types.Game{ID: "clientstore-620", Source: "clientstore", StoreAppID: "620", Title: "Portal"}
	p, err := a.prepareLaunch(g, false)
	if err != nil {
		t.Fatalf("prepareLaunch: %v", err)
	}
	cmd := exec.Command(p.args[0], p.args[1:]...)
	if _, err := a.startClient(g, cmd, p.client); err != nil {
		t.Fatalf("startClient: %v", err)
	}
	return host
}

// waitSessionEnd waits for the launch to end and returns its events.
func waitSessionEnd(t *testing.T, host *sessionEventHost, wait time.Duration) []types.SessionEvent {
	t.Helper()
	deadline := time.Now().Add(wait)
	for {
		events := host.snapshot()
		for _, e := range events {
			if e.State == "failed" || e.State == "stopped" {
				return events
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the client launch never ended: %+v", events)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Store client notices (issue #149): on, the App reports what the client is
// doing while it starts the game; off, the launch waits and says nothing.
// Either way the launch is followed and its outcome still reaches the UI.
func TestClientNoticesOffSkipsTheClientStatusEvents(t *testing.T) {
	for _, tc := range []struct {
		name    string
		notices bool
		status  bool
	}{
		{"on", true, true},
		{"off", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := nativeTestApp(t, t.TempDir())
			if err := a.settings.SetClientNotices(tc.notices); err != nil {
				t.Fatal(err)
			}
			host := launchClient(t, a, clientStatusStore{})
			events := waitSessionEnd(t, host, 10*time.Second)
			statuses := 0
			for _, e := range events {
				if e.ClientStatus != nil {
					statuses++
				}
			}
			if tc.status && statuses == 0 {
				t.Fatalf("no client status event with notices on: %+v", events)
			}
			if !tc.status && statuses != 0 {
				t.Fatalf("%d client status events with notices off: %+v", statuses, events)
			}
		})
	}
}

// Notices off only hides what the client reports: the status is still polled,
// and polling is what keeps a client that updates (or compiles shaders) past
// the start timeout from being called a failed launch (issue #149). Here the
// game appears on the fourth poll, about 6 s in, while the timeout is 3 s.
func TestClientNoticesOffStillExtendsTheLaunchDeadline(t *testing.T) {
	a := nativeTestApp(t, t.TempDir())
	if err := a.settings.SetClientNotices(false); err != nil {
		t.Fatal(err)
	}
	var polls atomic.Int32
	find := func() int {
		if polls.Add(1) == 4 {
			return 1234
		}
		return 0
	}
	host := launchClient(t, a, clientStatusStore{find: find, state: "updating", timeout: 3 * time.Second})
	events := waitSessionEnd(t, host, 30*time.Second)
	ran, failed := false, false
	for _, e := range events {
		if e.State == "running" {
			ran = true
		}
		if e.State == "failed" {
			failed = true
		}
	}
	if failed {
		t.Fatalf("notices off made a client that was updating fail after its start timeout: %+v", events)
	}
	if !ran {
		t.Fatalf("the launch never reached running: %+v", events)
	}
}

// Both switches reach the UI through GetLauncherSettings, and their setters
// save them (issues #149, #150).
func TestLauncherSettingsCarryTheClientAndProtectionSwitches(t *testing.T) {
	a := nativeTestApp(t, t.TempDir())
	got := a.launcherSettings()
	if !got.ClientNotices || !got.ProtectionOnline {
		t.Fatalf("defaults = client notices %v, protection online %v; want both on", got.ClientNotices, got.ProtectionOnline)
	}
	if _, err := a.SetClientNotices(false); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetProtectionOnline(false); err != nil {
		t.Fatal(err)
	}
	got = a.launcherSettings()
	if got.ClientNotices || got.ProtectionOnline || !got.RatingsOnline {
		t.Fatalf("after switching off = client notices %v, protection online %v, ratings online %v", got.ClientNotices, got.ProtectionOnline, got.RatingsOnline)
	}
}
