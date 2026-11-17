package social

import (
	"encoding/json"
	"strings"
	"testing"
)

func inviteFor(friend, name, storeID, kind, connect, lobby string) Invite {
	iv := Invite{Kind: kind, Connect: connect, LobbyID: lobby}
	iv.From.ID, iv.From.Name = friend, name
	iv.Game.StoreID, iv.Game.Title = storeID, "Game "+storeID
	return iv
}

func startInvites(t *testing.T) *Manager {
	t.Helper()
	f := newFake()
	f.snapshot = map[string]any{"connection": Connection{State: ConnOnline}, "friends": []Friend{}}
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

// Core cuts over-long payloads down to size and fills a missing name with
// the id, like the rest of a friend's data.
func TestInviteNotifyCaps(t *testing.T) {
	m := startInvites(t)
	defer m.StopAll()
	iv := inviteFor("1", "", strings.Repeat("9", 100), "connect", strings.Repeat("c", 300), strings.Repeat("l", 100))
	raw, _ := json.Marshal(iv)
	m.Notify("echo", NoteInvite, raw)
	v := m.View("echo")
	if len(v.Invites) != 1 {
		t.Fatalf("invites = %+v", v.Invites)
	}
	got := v.Invites[0]
	if got.From.Name != "1" {
		t.Fatalf("name = %q, want the id", got.From.Name)
	}
	if len(got.Connect) != 256 || len(got.Game.StoreID) != 64 || len(got.LobbyID) != 64 {
		t.Fatalf("caps = %d %d %d", len(got.Connect), len(got.Game.StoreID), len(got.LobbyID))
	}
}

// The snapshot seeds the pending invites, cleaned and one per friend, so a
// UI restart does not lose them.
func TestInviteSeededFromSnapshot(t *testing.T) {
	f := newFake()
	f.snapshot = map[string]any{
		"connection": Connection{State: ConnOnline},
		"friends":    []Friend{},
		"invites": []Invite{
			inviteFor("1", "Ana", "620", "connect", "+connect 9.9.9.9:27015", ""),
			inviteFor("1", "Ana", "47700", "lobby", "", "109775241000000001"),
			inviteFor("", "Nobody", "620", "connect", "+connect 9.9.9.9:27015", ""),
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
