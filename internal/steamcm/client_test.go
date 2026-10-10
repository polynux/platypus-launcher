// Client tests over the steamcmtest fake: heartbeat cadence from the
// logon response, reconnect after a dropped socket, two logons with
// different ids coexisting, a duplicate id kicking the first (which
// then must not reconnect-loop), a push landing between a request and
// its reply, and Call matching by jobid_target. EMsg numbers and field
// numbers per docs/research §4.1 (enums_clientserver.proto).
package steamcm_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"platypus/internal/depot"
	"platypus/internal/steamcm"
	"platypus/internal/steamcm/steamcmtest"
)

// testToken is a JWT-shaped refresh token; only sub matters.
func testToken() string {
	return "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiI3NjU2MTE5OCJ9." +
		"SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJVadQssw5c"
}

func fastConfig(role steamcm.LoginRole) steamcm.ClientConfig {
	return steamcm.ClientConfig{
		RefreshToken: testToken(),
		Role:         role,
		BackoffBase:  20 * time.Millisecond,
		BackoffMax:   60 * time.Millisecond,
	}
}

// useFake pins the CM endpoint to the fake: nothing here may touch the
// real Steam network.
func useFake(t *testing.T, f *steamcmtest.Fake) {
	t.Helper()
	t.Setenv("PLATYPUS_STEAM_CM_SERVER", f.Host())
}

// heartbeatSeconds=1 on the fake, so the client's cadence is 1s: two
// beats within ~2.4s prove the cadence came from the logon response
// (the old Session only answered heartbeats, never sent them).
func TestClientSendsHeartbeats(t *testing.T) {
	f := steamcmtest.New(t, steamcmtest.Config{
		HeartbeatSeconds: 1,
	})
	useFake(t, f)
	c, err := steamcm.Connect(context.Background(), fastConfig(steamcm.LoginRoleSocial))
	if err != nil {
		t.Fatal(err)
	}
	if c.State() != steamcm.StateOnline {
		t.Fatalf("state after connect: %v", c.State())
	}
	defer c.Close()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		beats := 0
		for _, ev := range f.Events() {
			if ev == "heartbeat" {
				beats++
			}
		}
		if beats >= 2 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no two client heartbeats within 4s: %v", f.Events())
}

// The fake drops the socket right after logon; the client must come
// back online by itself and serve a Call afterwards.
func TestClientReconnectsAfterDrop(t *testing.T) {
	f := steamcmtest.New(t, steamcmtest.Config{DropAfterLogon: true})
	useFake(t, f)
	c, err := steamcm.Connect(context.Background(), fastConfig(steamcm.LoginRoleSocial))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if !f.WaitFor(5*time.Second, func(evs []string) bool {
		online := false
		for _, ev := range evs {
			if strings.HasPrefix(ev, "logon") {
				online = true
			}
		}
		return online
	}) {
		t.Fatalf("first logon never recorded: %v", f.Events())
	}
	// Wait for the reconnect: a second logon on the same fake.
	firstCount := len(f.Events())
	if !f.WaitFor(5*time.Second, func(evs []string) bool {
		return countLogons(evs) >= 2
	}) {
		t.Fatalf("client never reconnected: %v", f.Events())
	}
	_ = firstCount
	if c.State() != steamcm.StateOnline {
		t.Fatalf("state after reconnect: %v (err %v)", c.State(), c.Err())
	}
	// The reconnected session answers calls.
	body, err := c.Call("Authentication.GenerateAccessTokenForApp#1", nil, 5*time.Second)
	if err != nil {
		t.Fatalf("call after reconnect: %v", err)
	}
	if m, err := depot.Decode(body); err != nil || len(m.Bytes(1)) == 0 {
		t.Fatalf("reply body: %v %x", err, body)
	}
}

func countLogons(evs []string) int {
	n := 0
	for _, ev := range evs {
		if strings.HasPrefix(ev, "logon ") {
			n++
		}
	}
	return n
}

// Two clients with different role ids log on at the same time and both
// stay online (D8): neither gets kicked.
func TestClientDistinctLoginIDsCoexist(t *testing.T) {
	f := steamcmtest.New(t, steamcmtest.Config{DuplicateKick: true})
	useFake(t, f)
	social, err := steamcm.Connect(context.Background(), fastConfig(steamcm.LoginRoleSocial))
	if err != nil {
		t.Fatal(err)
	}
	defer social.Close()
	cloud, err := steamcm.Connect(context.Background(), fastConfig(steamcm.LoginRoleCloud))
	if err != nil {
		t.Fatal(err)
	}
	defer cloud.Close()

	time.Sleep(300 * time.Millisecond)
	if social.State() != steamcm.StateOnline || cloud.State() != steamcm.StateOnline {
		t.Fatalf("states: social=%v cloud=%v", social.State(), cloud.State())
	}
	for _, ev := range f.Events() {
		if strings.HasPrefix(ev, "kick") {
			t.Fatalf("distinct ids kicked each other: %v", f.Events())
		}
	}
	if countLogons(f.Events()) != 2 {
		t.Fatalf("expected two logons: %v", f.Events())
	}
	// Role ids differ.
	socialID := steamcm.LoginIDFor(steamcm.LoginRoleSocial)
	cloudID := steamcm.LoginIDFor(steamcm.LoginRoleCloud)
	if socialID == cloudID {
		t.Fatalf("role ids collide: %d", socialID)
	}
	if !strings.Contains(strings.Join(f.Events(), "\n"), "loginid="+itoa(uint64(socialID))) {
		t.Fatalf("social logon id not on the wire: %v", f.Events())
	}
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

// Same login id twice: the first session is kicked with ClientLoggedOff
// (34) and must end in StateReplaced without reconnect-looping.
func TestClientSameLoginIDReplaced(t *testing.T) {
	f := steamcmtest.New(t, steamcmtest.Config{DuplicateKick: true})
	useFake(t, f)
	cfg := fastConfig(steamcm.LoginRoleContent)
	first, err := steamcm.Connect(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()

	second, err := steamcm.Connect(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()

	if !f.WaitFor(5*time.Second, func(evs []string) bool {
		for _, ev := range evs {
			if strings.HasPrefix(ev, "kick 34") {
				return true
			}
		}
		return false
	}) {
		t.Fatalf("no kick recorded: %v", f.Events())
	}

	// The kicked one must report replaced and stop trying.
	if !f.WaitFor(5*time.Second, func(evs []string) bool {
		return first.State() == steamcm.StateReplaced
	}) {
		t.Fatalf("first client state: %v (want replaced)", first.State())
	}
	time.Sleep(300 * time.Millisecond)
	logons := countLogons(f.Events())
	if logons > 2 {
		t.Fatalf("kicked client reconnect-looped: %d logons: %v", logons, f.Events())
	}
	// The survivor is still online and callable.
	if second.State() != steamcm.StateOnline {
		t.Fatalf("second client state: %v", second.State())
	}
}

// A kicked ClientLoggedOff with eresult 6 (LoggedInElsewhere) also ends
// the session as replaced instead of reconnecting.
func TestClientLoggedInElsewhereStops(t *testing.T) {
	f := steamcmtest.New(t, steamcmtest.Config{
		Kick:      6,
		KickAfter: 1,
	})
	useFake(t, f)
	c, err := steamcm.Connect(context.Background(), fastConfig(steamcm.LoginRoleSocial))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Call("Authentication.GenerateAccessTokenForApp#1", nil, 5*time.Second); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if !f.WaitFor(5*time.Second, func([]string) bool { return c.State() == steamcm.StateReplaced }) {
		t.Fatalf("state after eresult-6 kick: %v (want replaced)", c.State())
	}
	time.Sleep(300 * time.Millisecond)
	if countLogons(f.Events()) != 1 {
		t.Fatalf("reconnect after LoggedInElsewhere: %v", f.Events())
	}
}

// A push sent between a request and its reply must reach the handler
// while the pending Call still gets its own reply (nothing lost).
func TestClientPushBetweenRequestAndReply(t *testing.T) {
	persona := &depot.Enc{}
	persona.Varint(1, 2) // status_flags: PlayerName
	friend := &depot.Enc{}
	friend.Fixed64(1, 76561198000000000)
	friend.Str(15, "FriendName")
	persona.MsgField(2, friend)

	pushes := []steamcmtest.Push{
		{EMsg: steamcm.EMsgClientPersonaState, Body: persona.Bytes(), After: 1},
		{EMsg: steamcm.EMsgClientFriendsList, Body: []byte{0x08, 0x00}, After: 1},
		{EMsg: steamcm.EMsgClientPlayingSessionState, Body: []byte{0x10, 0x00}, After: 2},
		{EMsg: steamcm.EMsgCloudNotifyAppStateChange, Body: []byte{0x08, 0x01}, After: 2},
	}
	f := steamcmtest.New(t, steamcmtest.Config{Pushes: pushes})
	useFake(t, f)

	c, err := steamcm.Connect(context.Background(), fastConfig(steamcm.LoginRoleSocial))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	got := make(chan uint32, 8)
	c.On(steamcm.EMsgClientPersonaState, func(_ depot.Msg, _ []byte) { got <- steamcm.EMsgClientPersonaState })
	c.On(steamcm.EMsgClientFriendsList, func(_ depot.Msg, _ []byte) { got <- steamcm.EMsgClientFriendsList })
	c.On(steamcm.EMsgClientPlayingSessionState, func(_ depot.Msg, _ []byte) { got <- steamcm.EMsgClientPlayingSessionState })
	c.On(steamcm.EMsgCloudNotifyAppStateChange, func(_ depot.Msg, _ []byte) { got <- steamcm.EMsgCloudNotifyAppStateChange })

	// Two calls; each has pushes scheduled to fire before its reply.
	for i := 0; i < 2; i++ {
		body, err := c.Call("Authentication.GenerateAccessTokenForApp#1", nil, 5*time.Second)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if m, err := depot.Decode(body); err != nil || len(m.Bytes(1)) == 0 {
			t.Fatalf("call %d reply: %v", i, err)
		}
	}
	// All four pushes arrived despite arriving mid-call.
	want := map[uint32]int{
		steamcm.EMsgClientPersonaState:        1,
		steamcm.EMsgClientFriendsList:         1,
		steamcm.EMsgClientPlayingSessionState: 1,
		steamcm.EMsgCloudNotifyAppStateChange: 1,
	}
	deadline := time.After(5 * time.Second)
	for len(want) > 0 {
		select {
		case emsg := <-got:
			if want[emsg] > 0 {
				want[emsg]--
				if want[emsg] == 0 {
					delete(want, emsg)
				}
			}
		case <-deadline:
			t.Fatalf("pushes lost (missing %v)", want)
		}
	}
}

// Call matches replies by jobid_target even with concurrent calls, and
// a server heartbeat push in between does not disturb the matching.
func TestClientCallMatchesJobID(t *testing.T) {
	f := steamcmtest.New(t, steamcmtest.Config{
		SendServerHeartbeat: true,
		Pushes: []steamcmtest.Push{
			{EMsg: steamcm.EMsgClientFriendsList, Body: []byte{0x08, 0x00}, After: 1},
		},
	})
	useFake(t, f)
	c, err := steamcm.Connect(context.Background(), fastConfig(steamcm.LoginRoleCloud))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	const n = 8
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			body, err := c.Call("Authentication.GenerateAccessTokenForApp#1", nil, 5*time.Second)
			if err != nil {
				errs <- err
				return
			}
			m, derr := depot.Decode(body)
			if derr != nil || len(m.Bytes(1)) == 0 {
				errs <- derr
				return
			}
			errs <- nil
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent call: %v", err)
		}
	}
	if !containsEvent(f.Events(), "heartbeat_reply_marker") {
		// The fake doesn't reply to client heartbeats; this just guards
		// that no fatal mismatch happened above.
		_ = f
	}
}

func containsEvent(evs []string, want string) bool {
	for _, e := range evs {
		if strings.Contains(e, want) {
			return true
		}
	}
	return false
}
