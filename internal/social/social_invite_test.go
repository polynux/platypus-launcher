package social

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

func inviteFor(friend, name, storeID, kind, connect, lobby string) Invite {
	iv := Invite{Kind: kind, Connect: connect, LobbyID: lobby, At: time.Now().Unix()}
	iv.From.ID, iv.From.Name = friend, name
	iv.Game.StoreID, iv.Game.Title = storeID, "Game "+storeID
	return iv
}

func startInvites(t *testing.T) *Manager {
	t.Helper()
	f := newFake()
	f.snapshot = map[string]any{
		"connection": Connection{State: ConnOnline},
		"friends":    []Friend{friend("1", "Ana", "online"), friend("2", "Bo", "offline")},
	}
	m := newManager(f, true)
	m.Start("echo")
	waitFor(t, "a connected view", func() bool {
		return m.View("echo").Connection.State == ConnOnline
	})
	return m
}

// One invite per friend: the newest replaces, and friends don't share slots.
func TestInviteNotifyKeepsOnePerFriend(t *testing.T) {
	m := startInvites(t)
	defer m.StopAll()
	note := func(iv Invite) {
		t.Helper()
		raw, _ := json.Marshal(iv)
		m.Notify("echo", NoteInvite, raw)
	}
	note(inviteFor("1", "Ana", "620", "connect", "+connect 9.9.9.9:27015", ""))
	note(inviteFor("1", "Ana", "47700", "lobby", "", "109775241000000001"))
	note(inviteFor("2", "Bo", "620", "connect", "+connect 1.2.3.4:27015", ""))
	v := m.View("echo")
	if len(v.Invites) != 2 {
		t.Fatalf("invites = %+v, want one per friend", v.Invites)
	}
	if v.Invites[0].Kind != "lobby" || v.Invites[0].Game.StoreID != "47700" {
		t.Fatalf("newest did not replace: %+v", v.Invites[0])
	}
}

// An invite with no friend, no game, a bad kind or no way to join is
// dropped, as is any other unknown notification.
func TestInviteNotifyDropsBad(t *testing.T) {
	m := startInvites(t)
	defer m.StopAll()
	bad := []Invite{
		inviteFor("", "Nobody", "620", "connect", "+connect 9.9.9.9:27015", ""),
		inviteFor("1", "Ana", "", "connect", "+connect 9.9.9.9:27015", ""),
		inviteFor("1", "Ana", "620", "stream", "+connect 9.9.9.9:27015", ""),
		inviteFor("1", "Ana", "620", "connect", "", ""),
		inviteFor("1", "Ana", "620", "lobby", "", ""),
	}
	for _, iv := range bad {
		raw, _ := json.Marshal(iv)
		m.Notify("echo", NoteInvite, raw)
	}
	m.Notify("echo", "social.presence", json.RawMessage(`{}`))
	if v := m.View("echo"); len(v.Invites) != 0 {
		t.Fatalf("invites = %+v, want none", v.Invites)
	}
}

// Over-long fragments are dropped at the boundary, never clipped; a missing
// name still falls back to the id.
func TestInviteNotifyDropsOverlong(t *testing.T) {
	m := startInvites(t)
	defer m.StopAll()
	bad := []Invite{
		inviteFor("1", "Ana", "620", "connect", strings.Repeat("c", 300), ""),
		inviteFor("1", "Ana", strings.Repeat("9", 100), "connect", "+connect 9.9.9.9:27015", ""),
		inviteFor("1", "Ana", "620", "lobby", "", strings.Repeat("1", 100)),
	}
	for _, iv := range bad {
		raw, _ := json.Marshal(iv)
		m.Notify("echo", NoteInvite, raw)
	}
	good := inviteFor("1", "", "620", "connect", "+connect 9.9.9.9:27015", "")
	raw, _ := json.Marshal(good)
	m.Notify("echo", NoteInvite, raw)
	v := m.View("echo")
	if len(v.Invites) != 1 || v.Invites[0].From.Name != "1" {
		t.Fatalf("invites = %+v, want the one good invite with the id as name", v.Invites)
	}
}

// The join allowlist (protocol 1.24, B1): exact token shapes, nothing else.
func TestParseJoinArgsTable(t *testing.T) {
	for _, tc := range []struct {
		name           string
		kind           string
		connect, lobby string
		want           []string
	}{
		{"host", "connect", "+connect example.com", "", []string{"+connect", "example.com"}},
		{"hostPort", "connect", "+connect 1.2.3.4:27015", "", []string{"+connect", "1.2.3.4:27015"}},
		{"bareHost", "connect", "+connect 1.2.3.4", "", []string{"+connect", "1.2.3.4"}},
		{"password", "connect", "+connect 1.2.3.4:27015 +password s3cret", "", []string{"+connect", "1.2.3.4:27015", "+password", "s3cret"}},
		{"slashPassword", "connect", "+connect 1.2.3.4:27015/secret", "", []string{"+connect", "1.2.3.4:27015/secret"}},
		{"ipv6", "connect", "+connect [2001:db8::1]:27015", "", []string{"+connect", "[2001:db8::1]:27015"}},
		{"ipv6NoPort", "connect", "+connect [::1]", "", []string{"+connect", "[::1]"}},
		{"invite", "connect", "+invite 30095787509687845", "", []string{"+invite", "30095787509687845"}},
		{"inviteSource2", "connect", "+invite 30095787509687845 -launchsource2", "", []string{"+invite", "30095787509687845", "-launchsource2"}},
		{"lobby", "lobby", "", "109775241000000001", []string{"+connect_lobby", "109775241000000001"}},
		{"insecure", "connect", "-insecure", "", nil},
		{"dashHost", "connect", "+connect -insecure", "", nil},
		{"dashHostPort", "connect", "+connect -game:27015", "", nil},
		{"dashPassword", "connect", "+connect 1.2.3.4 +password -insecure", "", nil},
		{"dotHost", "connect", "+connect .hidden", "", nil},
		{"exec", "connect", "+exec x", "", nil},
		{"extraToken", "connect", "+connect a b", "", nil},
		{"execAfter", "connect", "+connect 1.2.3.4:27015 +exec x", "", nil},
		{"portOverflow", "connect", "+connect 1.2.3.4:99999", "", nil},
		{"portZero", "connect", "+connect 1.2.3.4:0", "", nil},
		{"badHost", "connect", "+connect ho st", "", nil},
		{"badHostChars", "connect", "+connect ho_st!", "", nil},
		{"unbracketedV6", "connect", "+connect ::1", "", nil},
		{"pwLeadingDash", "connect", "+connect h:1 +password -x", "", nil},
		{"pwLeadingPlus", "connect", "+connect h:1 +password +x", "", nil},
		{"pwTooLong", "connect", "+connect h:1 +password " + strings.Repeat("p", 65), "", nil},
		{"lobby21", "lobby", "", "1" + strings.Repeat("0", 20), nil},
		{"lobbyZero", "lobby", "", "0", nil},
		{"lobbyAlpha", "lobby", "", "abc", nil},
		{"kindCase", "Connect", "+connect 1.2.3.4:27015", "", nil},
		{"empty", "connect", "", "", nil},
		{"badKind", "stream", "+connect 1.2.3.4:27015", "", nil},
		{"lobbyNoID", "lobby", "", "", nil},
	} {
		got, ok := ParseJoinArgs(tc.kind, tc.connect, tc.lobby)
		if tc.want == nil {
			if ok {
				t.Errorf("%s: accepted %q, want refusal", tc.name, tc.connect+tc.lobby)
			}
			continue
		}
		if !ok {
			t.Errorf("%s: refused, want %q", tc.name, tc.want)
			continue
		}
		if strings.Join(got, " ") != strings.Join(tc.want, " ") {
			t.Errorf("%s: argv = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// The snapshot seeds the pending invites, cleaned and one per friend, so a
// UI restart does not lose them.
func TestInviteSeededFromSnapshot(t *testing.T) {
	f := newFake()
	f.snapshot = map[string]any{
		"connection": Connection{State: ConnOnline},
		"friends":    []Friend{friend("1", "Ana", "online")},
		"invites": []Invite{
			inviteFor("1", "Ana", "620", "connect", "+connect 9.9.9.9:27015", ""),
			inviteFor("1", "Ana", "47700", "lobby", "", "109775241000000001"),
			inviteFor("", "Nobody", "620", "connect", "+connect 9.9.9.9:27015", ""),
			inviteFor("9", "Zed", "620", "connect", "+connect 9.9.9.9:27015", ""),
		},
	}
	m := newManager(f, true)
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "seeded invites", func() bool { return len(m.View("echo").Invites) > 0 })
	v := m.View("echo")
	if len(v.Invites) != 1 || v.Invites[0].Kind != "lobby" {
		t.Fatalf("invites = %+v, want the newest of friend 1", v.Invites)
	}
}

// Invites for strangers never reach the view, on notify or on seeding.
func TestInviteDropsStrangers(t *testing.T) {
	m := startInvites(t)
	defer m.StopAll()
	raw, _ := json.Marshal(inviteFor("9", "Zed", "620", "connect", "+connect 9.9.9.9:27015", ""))
	m.Notify("echo", NoteInvite, raw)
	if v := m.View("echo"); len(v.Invites) != 0 {
		t.Fatalf("stranger invite reached the view: %+v", v.Invites)
	}
}

// Below protocol 1.24 the state seeds no invites and notifications are
// dropped where they arrive.
func TestInviteStrippedBelow124(t *testing.T) {
	f := newFake()
	f.invites = false
	f.snapshot = map[string]any{
		"connection": Connection{State: ConnOnline},
		"friends":    []Friend{friend("1", "Ana", "online")},
		"invites":    []Invite{inviteFor("1", "Ana", "620", "connect", "+connect 9.9.9.9:27015", "")},
	}
	m := newManager(f, true)
	m.Start("echo")
	defer m.StopAll()
	waitFor(t, "a connected view", func() bool {
		return m.View("echo").Connection.State == ConnOnline
	})
	raw, _ := json.Marshal(inviteFor("1", "Ana", "620", "connect", "+connect 9.9.9.9:27015", ""))
	m.Notify("echo", NoteInvite, raw)
	if v := m.View("echo"); len(v.Invites) != 0 {
		t.Fatalf("pre-1.24 invites reached the view: %+v", v.Invites)
	}
}

// Pending invites expire after 15 minutes; core is the authority.
func TestInviteExpires(t *testing.T) {
	m := startInvites(t)
	defer m.StopAll()
	fresh := inviteFor("1", "Ana", "620", "connect", "+connect 9.9.9.9:27015", "")
	old := inviteFor("2", "Bo", "620", "connect", "+connect 1.2.3.4:27015", "")
	old.At = time.Now().Unix() - inviteTTLSeconds - 1
	for _, iv := range []Invite{fresh, old} {
		raw, _ := json.Marshal(iv)
		m.Notify("echo", NoteInvite, raw)
	}
	v := m.View("echo")
	if len(v.Invites) != 1 || v.Invites[0].From.ID != "1" {
		t.Fatalf("invites = %+v, want only the fresh one", v.Invites)
	}
}

// When the inviter stops playing that app, the join is gone with it.
func TestInviteDroppedWhenStoppedPlaying(t *testing.T) {
	m := startInvites(t)
	defer m.StopAll()
	raw, _ := json.Marshal(inviteFor("1", "Ana", "620", "connect", "+connect 9.9.9.9:27015", ""))
	m.Notify("echo", NoteInvite, raw)
	if v := m.View("echo"); len(v.Invites) != 1 {
		t.Fatalf("invite missing: %+v", v.Invites)
	}
	m.Notify("echo", NoteFriends, json.RawMessage(`{"full":true,"friends":[{"id":"1","name":"Ana","state":"online","game":{"id":"p.echo.other","title":"Other","storeId":"9999"}}]}`))
	if v := m.View("echo"); len(v.Invites) != 0 {
		t.Fatalf("stale invite survived the game change: %+v", v.Invites)
	}
}

// Concurrent notifies and snapshots must not race (run with -race).
func TestInviteConcurrentNotifyAndSnapshot(t *testing.T) {
	m := startInvites(t)
	defer m.StopAll()
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			for range 20 {
				raw, _ := json.Marshal(inviteFor(id, "N", "620", "connect", "+connect 9.9.9.9:27015", ""))
				m.Notify("echo", NoteInvite, raw)
			}
		}([]string{"1", "2"}[i])
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 200 {
			_ = m.View("echo")
		}
	}()
	wg.Wait()
}
