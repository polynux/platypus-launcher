// Revoked token retries forever (review bug 2): a reconnect logon
// answered with a token-rejected EResult (AccessDenied 15, Expired 27,
// InvalidPassword 5, ...) used to send run() back into the backoff
// loop, hammering the CM forever with a token Steam keeps refusing.
// Fixed: tokenRejected(eresult) on the reconnect logon ends the loop —
// state Offline, lastErr kept with Auth=true, no further attempts.
package steamcm_test

import (
	"context"
	"testing"
	"time"

	"platypus/internal/steamcm"
	"platypus/internal/steamcm/steamcmtest"
)

func TestClientTokenRejectedStopsReconnecting(t *testing.T) {
	f := steamcmtest.New(t, steamcmtest.Config{DropAfterLogon: true})
	useFake(t, f)
	c, err := steamcm.Connect(context.Background(), fastConfig(steamcm.LoginRoleSocial))
	if err != nil {
		t.Fatal(err)
	}
	if !f.WaitFor(5*time.Second, func(evs []string) bool { return countLogons(evs) >= 1 }) {
		t.Fatalf("first logon never recorded: %v", f.Events())
	}
	// From now on every logon attempt is refused: the token is revoked.
	f.SetConfig(func(cfg *steamcmtest.Config) { cfg.RetryLogonEresult = 15 })

	// The client must end offline with the auth error — and stay there.
	if !f.WaitFor(5*time.Second, func([]string) bool {
		return c.State() == steamcm.StateOffline
	}) {
		t.Fatalf("state after rejected reconnect logon: %v (want offline)", c.State())
	}
	err = c.Err()
	if err == nil {
		t.Fatal("Err() is nil after the token was rejected")
	}
	ce, ok := err.(*steamcm.CMError)
	if !ok || !ce.Auth || ce.EResult != 15 {
		t.Fatalf("Err() = %v (want CMError Auth=true EResult=15)", err)
	}

	// One reconnect attempt only: the first logon plus the refused one.
	// (Poll, because a retrying loop would keep adding logons.)
	time.Sleep(1 * time.Second)
	if !f.WaitFor(2*time.Second, func(evs []string) bool { return countLogons(evs) <= 2 }) {
		t.Fatalf("counts moved off two logons: %v", f.Events())
	}
	logons := countLogons(f.Events())
	if logons != 2 {
		t.Fatalf("expected exactly one refused retry: %d logons: %v", logons, f.Events())
	}
	select {
	case <-time.After(700 * time.Millisecond):
		// Still exactly 2 — no further attempts, no reconnect loop.
	}
	if n := countLogons(f.Events()); n != 2 {
		t.Fatalf("kept retrying after a token-rejected logon: %v", f.Events())
	}
}
