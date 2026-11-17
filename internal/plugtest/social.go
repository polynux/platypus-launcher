package plugtest

import (
	"encoding/json"
	"slices"
	"testing"

	"platypus/internal/jsonrpc"
)

// The social capability (protocol 1.6, docs/plugins/protocol.md, "Social"):
// the keys the requests and the snapshot may carry, and the states core relies
// on. A plugin may leave out its own profile and report no game.
var (
	SocialStateKeys      = []string{"connection", "self", "friends", "invites"}
	SocialFriendKeys     = []string{"id", "name", "avatarUrl", "state", "game", "lastSeen"}
	SocialGameKeys       = []string{"id", "title", "storeId", "joinable", "connect", "lobbyId"}
	SocialSelfKeys       = []string{"name", "avatarUrl", "state"}
	SocialInviteKeys     = []string{"from", "game", "kind", "connect", "lobbyId", "at"}
	SocialInviteFromKeys = []string{"id", "name"}
	SocialStatuses       = []string{"online", "away", "busy", "snooze", "invisible"}
	socialFriendSt       = []string{"online", "offline", "away", "busy", "snooze", "looking", "invisible"}
)

// checkSocial holds a plugin offering `social` to the request and snapshot
// shapes: a bad status or a bad playing id is -32602, social.state answers the
// documented keys, a friend has an id, a name and a known state, and
// setPlaying takes a store game id or null. gameID is the picked library
// game's store id: a store with numeric ids (Steam's appids) can't take the
// fixture name another store uses.
func (s *Session) checkSocial(t *testing.T, gameID string) {
	t.Helper()
	s.wantCode("social.setStatus", map[string]any{}, jsonrpc.CodeInvalidParams)
	s.wantCode("social.setStatus", map[string]any{"state": "lurking"}, jsonrpc.CodeInvalidParams)
	s.wantCode("social.setPlaying", map[string]any{"id": 42}, jsonrpc.CodeInvalidParams)

	if err := s.call("social.start", map[string]any{}, nil); err != nil {
		t.Fatalf("social.start: %v", err)
	}
	var raw json.RawMessage
	if err := s.call("social.state", map[string]any{}, &raw); err != nil {
		t.Fatalf("social.state: %v", err)
	}
	checkKeys(t, "social.state", raw, SocialStateKeys)
	var snap struct {
		Connection *struct {
			State string `json:"state"`
		} `json:"connection"`
		Self    json.RawMessage   `json:"self"`
		Friends []json.RawMessage `json:"friends"`
		Invites []json.RawMessage `json:"invites"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("social.state: %v", err)
	}
	if snap.Connection == nil || !slices.Contains(socialConnStates, snap.Connection.State) {
		t.Errorf("social.state has no known connection state: %s", raw)
	}
	if len(snap.Self) > 0 && string(snap.Self) != "null" {
		checkKeys(t, "social.state self", snap.Self, SocialSelfKeys)
	}
	for _, f := range snap.Friends {
		checkFriend(t, f)
	}
	for _, iv := range snap.Invites {
		checkInvite(t, iv)
	}

	if err := s.call("social.setStatus", map[string]any{"state": "away"}, nil); err != nil {
		t.Fatalf("social.setStatus: %v", err)
	}
	if err := s.call("social.setPlaying", map[string]any{"id": nil}, nil); err != nil {
		t.Fatalf("social.setPlaying null: %v", err)
	}
	if err := s.call("social.setPlaying", map[string]any{"id": gameID}, nil); err != nil {
		t.Fatalf("social.setPlaying %q: %v", gameID, err)
	}
	if err := s.call("social.setPlaying", map[string]any{"id": nil}, nil); err != nil {
		t.Fatalf("social.setPlaying null: %v", err)
	}
	if err := s.call("social.stop", map[string]any{}, nil); err != nil {
		t.Fatalf("social.stop: %v", err)
	}
}

var socialConnStates = []string{"connecting", "online", "reconnecting", "replaced", "offline", "loggedOut"}

// checkFriend holds one entry of a friends list to the Friend shape.
func checkFriend(t *testing.T, raw json.RawMessage) {
	t.Helper()
	checkKeys(t, "friend", raw, SocialFriendKeys)
	var f struct {
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		State string          `json:"state"`
		Game  json.RawMessage `json:"game"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("friend: %v", err)
	}
	if f.ID == "" || f.Name == "" || !slices.Contains(socialFriendSt, f.State) {
		t.Errorf("friend without an id, a name or a known state: %s", raw)
	}
	if len(f.Game) > 0 && string(f.Game) != "null" {
		checkKeys(t, "friend game", f.Game, SocialGameKeys)
	}
}

// checkInvite holds one pending invite to the Invite shape (1.24): who
// invites to what, how to join, and when.
func checkInvite(t *testing.T, raw json.RawMessage) {
	t.Helper()
	checkKeys(t, "invite", raw, SocialInviteKeys)
	var iv struct {
		From json.RawMessage `json:"from"`
		Game json.RawMessage `json:"game"`
		Kind string          `json:"kind"`
	}
	if err := json.Unmarshal(raw, &iv); err != nil {
		t.Fatalf("invite: %v", err)
	}
	if len(iv.From) == 0 || string(iv.From) == "null" {
		t.Errorf("invite without a sender: %s", raw)
	} else {
		checkKeys(t, "invite sender", iv.From, SocialInviteFromKeys)
	}
	if len(iv.Game) == 0 || string(iv.Game) == "null" {
		t.Errorf("invite without a game: %s", raw)
	} else {
		checkKeys(t, "invite game", iv.Game, SocialGameKeys)
	}
	if iv.Kind != "connect" && iv.Kind != "lobby" {
		t.Errorf("invite with kind %q, want connect or lobby: %s", iv.Kind, raw)
	}
}
