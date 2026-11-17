// Invite tests: the BBCode card parser, the 7005/800/entry-3 pushes, the
// 120 s dedupe and the rich-presence join data, all against the fake CM.
// Invite and connect-string contents stay out of the fake's log; the tests
// assert on ids, kinds and counts only.
package main

import (
	"encoding/json"
	"testing"
	"time"

	"platypus/internal/depot"
	"platypus/internal/steamcm"
	"platypus/internal/steamcm/steamcmtest"
)

func TestParseChatInviteTable(t *testing.T) {
	lobby := invite{app: 47700, kind: "lobby", lobby: "109775241000000001"}
	connect := invite{app: 620, kind: "connect", connect: "+connect 1.2.3.4:27015"}
	for _, tc := range []struct {
		name string
		text string
		want invite
		ok   bool
	}{
		{"lobby", `[lobbyinvite appid="47700" lobbyid="109775241000000001"][/lobbyinvite]`, lobby, true},
		{"connect", `[gameinvite appid="620" connect="+connect 1.2.3.4:27015"]`, connect, true},
		{"source2", `[gameinvite  appid="570" connect="+invite 30095787509687845 -launchsource2"]`, invite{app: 570, kind: "connect", connect: "+invite 30095787509687845 -launchsource2"}, true},
		{"upper", `[GameInvite AppID="620" Connect="+connect 1.2.3.4:27015"]`, connect, true},
		{"unquoted", `[gameinvite appid=620 connect=+connect]`, invite{app: 620, kind: "connect", connect: "+connect"}, true},
		{"quotedBracket", `[gameinvite appid="620" connect="a]b"]`, invite{app: 620, kind: "connect", connect: "a]b"}, true},
		{"escapedQuote", `[gameinvite appid="620" connect="a\"b"]`, invite{app: 620, kind: "connect", connect: `a"b`}, true},
		{"firstWins", `hi [gameinvite appid="1" connect="x"] [lobbyinvite appid="2" lobbyid="3"]`, invite{app: 1, kind: "connect", connect: "x"}, true},
		{"remoteplay", `[gameinvite appid="620" connect="+connect 1.2.3.4:27015" remoteplay="x"]`, invite{}, false},
		{"noApp", `[gameinvite connect="+connect 1.2.3.4:27015"]`, invite{}, false},
		{"zeroApp", `[gameinvite appid="0" connect="x"]`, invite{}, false},
		{"badApp", `[gameinvite appid="abc" connect="x"]`, invite{}, false},
		{"lobbyNeedsID", `[lobbyinvite appid="47700"]`, invite{}, false},
		{"connectNeedsFrag", `[gameinvite appid="620"]`, invite{}, false},
		{"unclosed", `[gameinvite appid="620" connect="x"`, invite{}, false},
		{"plain", `see you at 8`, invite{}, false},
		{"chatRoom", `[chatinvite appid="620"]`, invite{}, false},
	} {
		got, ok := parseChatInvite(tc.text)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%s: parseChatInvite = %+v, %v; want %+v, %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func inviteToGameBody(dest, src uint64, connect, remote string) []byte {
	e := &depot.Enc{}
	e.Fixed64(1, dest)
	e.Fixed64(2, src)
	if connect != "" {
		e.Str(3, connect)
	}
	if remote != "" {
		e.Str(4, remote)
	}
	return e.Bytes()
}

func chatInviteBody(invited, chat, patron uint64, roomType uint64, gameID uint64) []byte {
	e := &depot.Enc{}
	e.Fixed64(1, invited)
	e.Fixed64(2, chat)
	e.Fixed64(3, patron)
	e.Varint(4, roomType)
	e.Fixed64(7, gameID)
	return e.Bytes()
}

// personaSpec gains the join data: rpConnect fills the rich presence
// "connect" key (field 71) and lobby the game_lobby_id (field 73).
func TestSocialInvitePushes(t *testing.T) {
	cf := &chatFake{convs: map[uint64][]chatMsg{}, nextOrd: map[uint64]uint64{}, unread: map[uint64]int{}}
	full := friendsListBody(false, friendEntry(testF1, 3), friendEntry(testF2, 3))
	persona := personaBody(
		personaSpec{id: testF1, state: 1, app: 620, name: "Ana Ruiz"},
		personaSpec{id: testF2, state: 1, name: "Bo Lindqvist"},
		personaSpec{id: testSelfID, state: 1, name: "Self Player"},
	)
	const lobbyID = uint64(109775241000000001)
	f := useSocialFake(t, steamcmtest.Config{
		ServiceHandler: familySubscribeHandler(cf),
		Pushes: []steamcmtest.Push{
			{EMsg: steamcm.EMsgClientFriendsList, Body: full},
			{EMsg: steamcm.EMsgClientPersonaState, Body: persona},
			// A direct join with its routing app, then the same push
			// again (dropped), one without a game (dropped) and a
			// Remote Play invite (dropped).
			{EMsg: steamcm.EMsgClientInviteToGame, RoutingAppID: 620, Body: inviteToGameBody(testSelfID, testF1, "+connect 9.9.9.9:27015", "")},
			{EMsg: steamcm.EMsgClientInviteToGame, RoutingAppID: 620, Body: inviteToGameBody(testSelfID, testF1, "+connect 9.9.9.9:27015", "")},
			{EMsg: steamcm.EMsgClientInviteToGame, Body: inviteToGameBody(testSelfID, testF1, "+connect 9.9.9.9:27015", "")},
			{EMsg: steamcm.EMsgClientInviteToGame, RoutingAppID: 620, Body: inviteToGameBody(testSelfID, testF1, "+connect 9.9.9.9:27015", "stream")},
			// A lobby invite, then a chat-room invite (dropped) and a
			// mod game_id (dropped).
			{EMsg: steamcm.EMsgClientChatInvite, Body: chatInviteBody(testSelfID, lobbyID, testF2, 3, 47700)},
			{EMsg: steamcm.EMsgClientChatInvite, Body: chatInviteBody(testSelfID, lobbyID, testF2, 1, 47700)},
			{EMsg: steamcm.EMsgClientChatInvite, Body: chatInviteBody(testSelfID, lobbyID, testF2, 3, 47700|uint64(1)<<24)},
			// Chat cards: an entry-3 lobby card for F1 (newest wins),
			// an entry-1 connect card for F2, and our own echo (dropped).
			{EMsg: steamcm.EMsgServiceMethodResponse, Target: "FriendMessagesClient.IncomingMessage#1",
				Body: incomingBody(testF1, 3, `[lobbyinvite appid="47700" lobbyid="109775241000000002"]`, 1791400010, 1, false)},
			{EMsg: steamcm.EMsgServiceMethodResponse, Target: "FriendMessagesClient.IncomingMessage#1",
				Body: incomingBody(testF2, 1, `[gameinvite appid="620" connect="+connect 1.2.3.4:27015"]`, 1791400020, 2, false)},
			{EMsg: steamcm.EMsgServiceMethodResponse, Target: "FriendMessagesClient.IncomingMessage#1",
				Body: incomingBody(testF1, 3, `[lobbyinvite appid="47700" lobbyid="109775241000000003"]`, 1791400030, 3, true)},
		},
	})
	_ = f
	p := startSocial(t, "")
	defer p.stop(9)
	type gotInvite struct {
		FriendID string `json:"friendId"`
		Name     string `json:"friendName"`
		AppID    string `json:"appId"`
		Kind     string `json:"kind"`
		Connect  string `json:"connect"`
		LobbyID  string `json:"lobbyId"`
	}
	want := []gotInvite{
		{FriendID: "76561198000000002", Name: "Ana Ruiz", AppID: "620", Kind: "connect", Connect: "+connect 9.9.9.9:27015"},
		{FriendID: "76561198000000003", Name: "Bo Lindqvist", AppID: "47700", Kind: "lobby", LobbyID: "109775241000000001"},
		{FriendID: "76561198000000002", Name: "Ana Ruiz", AppID: "47700", Kind: "lobby", LobbyID: "109775241000000002"},
		{FriendID: "76561198000000003", Name: "Bo Lindqvist", AppID: "620", Kind: "connect", Connect: "+connect 1.2.3.4:27015"},
	}
	for i, w := range want {
		raw := p.nextEvent("invite", 25*time.Second)
		var got gotInvite
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("invite %d: %v", i, err)
		}
		if got != w {
			t.Fatalf("invite %d = %+v, want %+v", i, got, w)
		}
	}
	// One invite per friend: the newest card replaced the 7005 and the
	// 800 invite.
	p.send("state", nil, 1)
	raw, _ := p.nextReply(1, 15*time.Second)
	var snap struct {
		Invites []gotInvite `json:"invites"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("state: %v", err)
	}
	if len(snap.Invites) != 2 {
		t.Fatalf("state invites = %+v, want one per friend", snap.Invites)
	}
	byID := map[string]gotInvite{}
	for _, iv := range snap.Invites {
		byID[iv.FriendID] = iv
	}
	if byID["76561198000000002"] != want[2] || byID["76561198000000003"] != want[3] {
		t.Fatalf("state invites = %+v", snap.Invites)
	}
}

// The same invite inside 120 s is dropped; after the window it emits again.
// A different invite from the same friend replaces, never appends.
func TestSocialInviteDedupeWindow(t *testing.T) {
	old := inviteNow
	now := int64(1791400000)
	inviteNow = func() int64 { return now }
	defer func() { inviteNow = old }()
	s := &socialSession{invites: map[uint64]*invite{}, inviteSeen: map[string]int64{}}
	first := invite{friend: testF1, app: 620, kind: "connect", connect: "+connect 9.9.9.9:27015"}
	s.addInvite(first)
	s.addInvite(first)
	if len(s.invites) != 1 || len(s.inviteSeen) != 1 {
		t.Fatalf("repeat inside the window kept %d invites", len(s.invites))
	}
	now += inviteRepeatWindow + 1
	s.addInvite(first)
	if len(s.invites) != 1 || len(s.inviteSeen) != 1 {
		t.Fatalf("repeat past the window kept %d invites", len(s.invites))
	}
	if s.invites[testF1].at != now {
		t.Fatalf("invite at = %d, want %d", s.invites[testF1].at, now)
	}
	s.addInvite(invite{friend: testF1, app: 47700, kind: "lobby", lobby: "109775241000000001"})
	if len(s.invites) != 1 || s.invites[testF1].kind != "lobby" {
		t.Fatalf("newest invite did not replace: %+v", s.invites[testF1])
	}
}

// Rich presence "connect" and game_lobby_id render as the friend's join
// data; a friend with neither stays unjoinable.
func TestSocialPersonaJoinData(t *testing.T) {
	persona := func(connect string, lobby uint64) []byte {
		inner := &depot.Enc{}
		inner.Fixed64(1, testF1)
		inner.Varint(3, 620)
		if connect != "" {
			kv := &depot.Enc{}
			kv.Str(1, "connect")
			kv.Str(2, connect)
			inner.BytesField(71, kv.Bytes())
			other := &depot.Enc{}
			other.Str(1, "status")
			other.Str(2, "In game")
			inner.BytesField(71, other.Bytes())
		}
		if lobby != 0 {
			inner.Fixed64(73, lobby)
		}
		outer := &depot.Enc{}
		outer.BytesField(2, inner.Bytes())
		return outer.Bytes()
	}
	s := &socialSession{friends: map[uint64]*friendInfo{testF1: {id: testF1}}, selfID: testSelfID}
	s.onPersonaState(persona("+connect 10.0.0.1:27015", 109775241000000001))
	f := s.friends[testF1]
	if f.gameConnect != "+connect 10.0.0.1:27015" || f.gameLobby != "109775241000000001" {
		t.Fatalf("join data = %q %q", f.gameConnect, f.gameLobby)
	}
	g, ok := renderFriend(f)["game"].(map[string]any)
	if !ok || g["connect"] != "+connect 10.0.0.1:27015" || g["lobbyId"] != "109775241000000001" || g["joinable"] != true {
		t.Fatalf("rendered game = %v", renderFriend(f)["game"])
	}
	s.onPersonaState(persona("", 0))
	f = s.friends[testF1]
	if f.gameConnect != "" || f.gameLobby != "" {
		t.Fatalf("cleared presence kept join data: %q %q", f.gameConnect, f.gameLobby)
	}
}
