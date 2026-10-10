// Close vs reconnect race (review bug 1): Close() during a reconnect
// whose logon is still in flight used to (a) return at once because it
// closed c.done itself before waiting and (b) leak the freshly logged-on
// session — run() stored the new conn and went Online after the stop, and
// the fake kept the id claimed forever. Fixed: only run() closes done;
// Close waits for run() to return; reconnects check stop after the logon
// and log off instead of storing the session.
package steamcm_test

import (
	"context"
	"testing"
	"time"

	"platypus/internal/steamcm"
	"platypus/internal/steamcm/steamcmtest"
)

func TestClientCloseDuringReconnect(t *testing.T) {
	f := steamcmtest.New(t, steamcmtest.Config{DropAfterLogon: true, LogonDelay: 700 * time.Millisecond})
	useFake(t, f)
	c, err := steamcm.Connect(context.Background(), fastConfig(steamcm.LoginRoleSocial))
	if err != nil {
		t.Fatal(err)
	}
	if !f.WaitFor(5*time.Second, func(evs []string) bool { return countLogons(evs) >= 1 }) {
		t.Fatalf("first logon never recorded: %v", f.Events())
	}

	// The fake sleeps LogonDelay after recording each logon, so
	// waiting for the SECOND "logon" event pins us to the moment the
	// reconnect logon is in flight: Close() must land inside it.
	// (Waiting for StateReconnecting instead races the ~20ms backoff,
	// where Close would trivially win before any logon runs.)
	if !f.WaitFor(5*time.Second, func(evs []string) bool { return countLogons(evs) >= 2 }) {
		t.Fatalf("reconnect logon never started: %v", f.Events())
	}

	// Close() while run() sits inside dialAndLogon: it must return only
	// after run() has unwound, the late logon has been undone (logoff
	// out, slot released) and the state is Offline.
	closed := make(chan struct{})
	go func() { c.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close() hung during the in-flight reconnect logon")
	}
	if st := c.State(); st != steamcm.StateOffline {
		t.Fatalf("state after close: %v (want offline)", st)
	}
	if !f.WaitFor(5*time.Second, func(evs []string) bool {
		return f.LiveSessions() == 0
	}) {
		t.Fatalf("leaked session after close: live=%d events=%v", f.LiveSessions(), f.Events())
	}
	// Call after close: a clean offline error, no hang.
	if _, err := c.Call("Authentication.GenerateAccessTokenForApp#1", nil, 2*time.Second); err == nil {
		t.Fatal("call on closed client returned no error")
	}
}

// Close on an online client still logs off and releases the fake's slot
// (the pre-existing behaviour the slow-logon test could regress).
func TestClientCloseOnline(t *testing.T) {
	f := steamcmtest.New(t, steamcmtest.Config{})
	useFake(t, f)
	c, err := steamcm.Connect(context.Background(), fastConfig(steamcm.LoginRoleSocial))
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	if !f.WaitFor(5*time.Second, func(evs []string) bool {
		for _, ev := range evs {
			if ev == "logoff" {
				return true
			}
		}
		return false
	}) {
		t.Fatalf("no logoff recorded after close: %v", f.Events())
	}
	// The fake's slot releases when its serve goroutine returns, which
	// is a moment after the logoff reply; poll briefly for it.
	if !f.WaitFor(2*time.Second, func([]string) bool { return f.LiveSessions() == 0 }) {
		t.Fatalf("live sessions after close: %d", f.LiveSessions())
	}
}
