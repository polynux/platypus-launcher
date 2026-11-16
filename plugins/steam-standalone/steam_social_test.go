// Social and chat through the real plugin, the real helper binary and the
// fake CM (testdata/fake_cm.py): friends mapping, the chat round trip and
// what the helper puts on the wire. Fast: localhost only, no Valve network.
package steam_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

const (
	socialF1 = "76561198000000021"
	socialF2 = "76561198000000022"
	socialF3 = "76561198000000025"
	selfID   = "76561198"
)

func socialKnobs() map[string]any {
	return map[string]any{
		"friends": []any{
			map[string]any{"id": socialF1, "rel": 3},
			map[string]any{"id": socialF2, "rel": 3},
			map[string]any{"id": "76561198000000023", "rel": 1},
			map[string]any{"id": "76561198000000024", "rel": 2},
			map[string]any{"id": fmt.Sprint(uint64(1)<<56 | uint64(7)<<52 | 12345), "rel": 3},
		},
		"persona_states": map[string]any{
			socialF1: map[string]any{"name": "Ana Ruiz", "state": 1, "app": 4321,
				"title": "Standalone Game", "ava": strings.Repeat("ab", 20), "seen": 1791398361},
			socialF2: map[string]any{"name": "Bo Lindqvist", "state": 3,
				"app": 9999, "title": "Something Else"},
			socialF3: map[string]any{"name": "Cy Okafor", "state": 4},
			selfID:   map[string]any{"name": "Self Player", "state": 1},
		},
		"push_body_after": map[string]any{
			"0": []any{"friends", "personas"},
			// The third service call (the second history page below)
			// carries the incremental list.
			"3": []any{"friends_incr", "personas"},
		},
		"friends_incr": []any{
			map[string]any{"id": socialF3, "rel": 3},
		},
		"chat_history": map[string]any{
			socialF1: []any{map[string]any{"ts": 1791390000, "ord": 1, "text": "hey"}},
		},
		"chat_unread": map[string]any{socialF1: 2},
	}
}

func setupSocial(t *testing.T) *env {
	t.Helper()
	e := setup(t)
	for knob, value := range socialKnobs() {
		e.setCMState(knob, value)
	}
	e.login()
	games := e.library()
	e.find(games, "4321")
	return e
}

type socialFriend struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	State     string         `json:"state"`
	AvatarUrl string         `json:"avatarUrl"`
	Game      map[string]any `json:"game"`
	LastSeen  int64          `json:"lastSeen"`
}

type socialSnapshot struct {
	Connection struct {
		State string `json:"state"`
	} `json:"connection"`
	Self struct {
		Name  string `json:"name"`
		State string `json:"state"`
	} `json:"self"`
	Friends []socialFriend `json:"friends"`
}

func (e *env) socialState() socialSnapshot {
	e.t.Helper()
	var snap socialSnapshot
	if err := e.call("social.state", map[string]any{}, &snap); err != nil {
		e.t.Fatalf("social.state: %v", err)
	}
	return snap
}

func (e *env) wantSocialCode(method string, params map[string]any, code int) {
	e.t.Helper()
	if err := e.call(method, params, nil); err == nil {
		e.t.Fatalf("%s %v: want error %d, got nil", method, params, code)
	} else if got := e.rpcCode(err); got != code {
		e.t.Fatalf("%s %v: error %d, want %d", method, params, got, code)
	}
}

// Friends map to the protocol shape, chat round-trips, and the helper puts
// the status, the game and the ack on the wire.
func TestSteamSocialChat(t *testing.T) {
	e := setupSocial(t)

	var startRes map[string]any
	if err := e.call("social.start", map[string]any{}, &startRes); err != nil {
		t.Fatalf("social.start: %v", err)
	}
	snap := e.socialState()
	if snap.Connection.State != "online" {
		t.Fatalf("connection: %+v", snap.Connection)
	}
	if len(snap.Friends) != 2 {
		t.Fatalf("friends: %+v", snap.Friends)
	}
	byID := map[string]socialFriend{}
	for _, f := range snap.Friends {
		byID[f.ID] = f
	}
	ana := byID[socialF1]
	if ana.Name != "Ana Ruiz" || ana.State != "online" || ana.LastSeen != 1791398361 {
		t.Fatalf("ana: %+v", ana)
	}
	if !strings.HasPrefix(ana.AvatarUrl, "https://avatars.cloudflare.steamstatic.com/") ||
		!strings.HasSuffix(ana.AvatarUrl, "_full.jpg") {
		t.Fatalf("ana avatar: %q", ana.AvatarUrl)
	}
	if ana.Game["id"] != "p.steam-standalone.4321" || ana.Game["title"] != "Standalone Game" {
		t.Fatalf("ana game: %v", ana.Game)
	}
	bo := byID[socialF2]
	if bo.Name != "Bo Lindqvist" || bo.State != "away" {
		t.Fatalf("bo: %+v", bo)
	}
	// App 9999 is not in the library: title only, no Platypus id.
	if _, ok := bo.Game["id"]; ok || bo.Game["title"] != "Something Else" {
		t.Fatalf("bo game: %v", bo.Game)
	}

	// Validation happens before anything reaches Steam.
	e.wantSocialCode("social.setStatus", map[string]any{}, -32602)
	e.wantSocialCode("social.setStatus", map[string]any{"state": "lurking"}, -32602)
	e.wantSocialCode("social.setPlaying", map[string]any{"id": 42}, -32602)
	e.wantSocialCode("chat.history", map[string]any{}, -32602)
	e.wantSocialCode("chat.history", map[string]any{"friendId": "76561198000000099"}, -32602)
	e.wantSocialCode("chat.send", map[string]any{"friendId": socialF1, "text": "", "clientId": "x"}, -32602)
	e.wantSocialCode("chat.send", map[string]any{"friendId": socialF1, "text": "x"}, -32602)
	e.wantSocialCode("chat.send", map[string]any{"friendId": "76561198000000099", "text": "x", "clientId": "x"}, -32602)
	e.wantSocialCode("chat.markRead", map[string]any{"friendId": "76561198000000099", "upTo": "1-1"}, -32602)

	if err := e.call("social.setStatus", map[string]any{"state": "away"}, nil); err != nil {
		t.Fatalf("setStatus: %v", err)
	}
	e.waitCMLog("status 3")
	if err := e.call("social.setPlaying", map[string]any{"id": "4321"}, nil); err != nil {
		t.Fatalf("setPlaying: %v", err)
	}
	e.waitCMLog("games_played 4321")
	if err := e.call("social.setPlaying", map[string]any{"id": nil}, nil); err != nil {
		t.Fatalf("setPlaying null: %v", err)
	}
	e.waitCMLog("games_played none")

	var hist struct {
		Messages []struct {
			ID     string `json:"id"`
			From   string `json:"from"`
			Text   string `json:"text"`
			SentAt int64  `json:"sentAt"`
		} `json:"messages"`
		More bool `json:"more"`
	}
	if err := e.call("chat.history", map[string]any{"friendId": socialF1}, &hist); err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(hist.Messages) != 1 || hist.Messages[0].From != socialF1 || hist.More {
		t.Fatalf("history: %+v", hist)
	}
	var sent struct {
		ID     string `json:"id"`
		SentAt int64  `json:"sentAt"`
	}
	if err := e.call("chat.send", map[string]any{"friendId": socialF1, "text": "plugtest hello",
		"clientId": "plugtest-1"}, &sent); err != nil {
		t.Fatalf("send: %v", err)
	}
	if sent.ID == "" || sent.SentAt == 0 {
		t.Fatalf("send: %+v", sent)
	}
	var again struct {
		ID     string `json:"id"`
		SentAt int64  `json:"sentAt"`
	}
	if err := e.call("chat.send", map[string]any{"friendId": socialF1, "text": "plugtest hello",
		"clientId": "plugtest-1"}, &again); err != nil {
		t.Fatalf("send again: %v", err)
	}
	if again != sent {
		t.Fatalf("repeat send = %+v, want %+v", again, sent)
	}
	e.waitCMLog("send " + socialF1)
	if err := e.call("chat.history", map[string]any{"friendId": socialF1, "limit": 5}, &hist); err != nil {
		t.Fatalf("history after send: %v", err)
	}
	if len(hist.Messages) != 2 || hist.Messages[1].ID != sent.ID || hist.Messages[1].From != "self" {
		t.Fatalf("history after send: %+v", hist)
	}
	if err := e.call("chat.markRead", map[string]any{"friendId": socialF1, "upTo": sent.ID}, nil); err != nil {
		t.Fatalf("markRead: %v", err)
	}
	e.waitCMLog("ack " + socialF1 + " ")
	var unread struct {
		Counts []struct {
			FriendID string `json:"friendId"`
			Unread   int    `json:"unread"`
		} `json:"counts"`
	}
	if err := e.call("chat.unreadCounts", map[string]any{}, &unread); err != nil {
		t.Fatalf("unreadCounts: %v", err)
	}
	if len(unread.Counts) != 1 || unread.Counts[0].FriendID != socialF1 || unread.Counts[0].Unread != 2 {
		t.Fatalf("unread: %+v", unread)
	}

	// The incremental list rode in on the third service call and merged.
	deadline := time.Now().Add(10 * time.Second)
	found := false
	for time.Now().Before(deadline) && !found {
		snap = e.socialState()
		for _, f := range snap.Friends {
			if f.ID == socialF3 && f.Name == "Cy Okafor" && f.State == "snooze" {
				found = true
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !found {
		t.Fatalf("incremental friend missing: %+v", snap.Friends)
	}

	if err := e.call("social.stop", map[string]any{}, nil); err != nil {
		t.Fatalf("social.stop: %v", err)
	}
	snap = e.socialState()
	if snap.Connection.State != "offline" || len(snap.Friends) != 0 {
		t.Fatalf("after stop: %+v", snap)
	}
}

// A kicked session (LoggedInElsewhere) reports replaced and never fights:
// the helper ends, the plugin stays up, and the next start logs on again.
func TestSteamSocialReplaced(t *testing.T) {
	e := setup(t)
	knobs := socialKnobs()
	knobs["kick"] = 6
	knobs["kick_after"] = 1
	for knob, value := range knobs {
		e.setCMState(knob, value)
	}
	e.login()
	e.library()
	if err := e.call("social.start", map[string]any{}, nil); err != nil {
		t.Fatalf("social.start: %v", err)
	}
	if got := e.socialState(); len(got.Friends) != 2 {
		t.Fatalf("friends: %+v", got.Friends)
	}
	// The first service call carries the kick; the helper stops its loop.
	var hist map[string]any
	if err := e.call("chat.history", map[string]any{"friendId": socialF1}, &hist); err != nil {
		t.Fatalf("history: %v", err)
	}
	e.waitCMLog("kick 6")
	// The session is over: the helper ended instead of reconnecting, so
	// the state goes back to offline with no friends.
	deadline := time.Now().Add(10 * time.Second)
	for {
		var gone socialSnapshot
		if err := e.call("social.state", map[string]any{}, &gone); err != nil {
			t.Fatalf("state after kick: %v", err)
		}
		if gone.Connection.State == "offline" && len(gone.Friends) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("session survived the kick: %+v", gone)
		}
		time.Sleep(100 * time.Millisecond)
	}
	// No reconnect fight: the helper ended instead of logging on again, so
	// a fresh start is exactly one more logon.
	before := strings.Count(e.cmLog(), "logon ")
	if err := e.call("social.start", map[string]any{}, nil); err != nil {
		t.Fatalf("social.start again: %v", err)
	}
	if got := e.socialState(); len(got.Friends) != 2 {
		t.Fatalf("friends after restart: %+v", got.Friends)
	}
	after := strings.Count(e.cmLog(), "logon ")
	if after != before+1 {
		t.Fatalf("logons went %d -> %d, want exactly one more", before, after)
	}
}
