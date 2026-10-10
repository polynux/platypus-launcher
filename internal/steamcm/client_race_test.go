// Data race on the heartbeat header (review bug 3): heartbeatLoop built
// header(c.steamID, c.sessionID, ...) after releasing c.mu, while run()
// rewrote c.sessionID on every reconnect. Frequent drops (the fake drops
// after every logon) plus 1 s heartbeats make the two goroutines collide
// reliably; run with -race.
package steamcm_test

import (
	"context"
	"testing"
	"time"

	"platypus/internal/steamcm"
	"platypus/internal/steamcm/steamcmtest"
)

func TestClientHeartbeatRacesWithReconnect(t *testing.T) {
	f := steamcmtest.New(t, steamcmtest.Config{
		HeartbeatSeconds: 1,
		// Online windows of 1.1 s (1 s heartbeats fire inside them),
		// then the socket dies and run() reconnects — the reconnect
		// path rewrites c.sessionID a heartbeat header build apart.
		DropAfter: 1100 * time.Millisecond,
	})
	useFake(t, f)
	c, err := steamcm.Connect(context.Background(), fastConfig(steamcm.LoginRoleSocial))
	if err != nil {
		t.Fatal(err)
	}
	// Several reconnect cycles (~40-80 ms each) across several 1 s
	// heartbeat beats: under -race any unlocked c.sessionID read in
	// the heartbeat header trips here.
	if !f.WaitFor(10*time.Second, func(evs []string) bool {
		n := 0
		for _, ev := range evs {
			if ev == "heartbeat" {
				n++
			}
		}
		return n >= 2
	}) {
		t.Fatalf("no heartbeats recorded: %v", f.Events())
	}
	c.Close()
}
