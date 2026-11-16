// Social subcommand tests: the helper's long-lived CM session against the
// importable fake CM (internal/steamcm/steamcmtest). Friends list filtering
// and incremental merges, persona mapping, the chat service calls, reconnect
// re-requests and the replaced stop. Message text never reaches the fake's
// log; the tests assert on counts, ids and states only.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"platypus/internal/depot"
	"platypus/internal/steamcm"
	"platypus/internal/steamcm/steamcmtest"
)

var (
	testSelfID = uint64(76561198000000001)
	testF1     = uint64(76561198000000002)
	testF2     = uint64(76561198000000003)
	testF3     = uint64(76561198000000004)
)

func testClanID() uint64 { return (1 << 56) | (7 << 52) | 12345 }

func testToken(sub uint64) string {
	seg := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	return seg(`{"alg":"none"}`) + "." +
		seg(fmt.Sprintf(`{"sub":"%d"}`, sub)) + ".sig"
}

func friendEntry(id uint64, rel uint64) []byte {
	e := &depot.Enc{}
	e.Fixed64(1, id)
	e.Varint(2, rel)
	return e.Bytes()
}

func friendsListBody(incr bool, entries ...[]byte) []byte {
	e := &depot.Enc{}
	if incr {
		e.Varint(1, 1)
	}
	for _, en := range entries {
		e.BytesField(2, en)
	}
	return e.Bytes()
}

type personaSpec struct {
	id    uint64
	state uint64
	app   uint32
	name  string
	ava   byte // avatar hash filler byte; 0 = no avatar
	seen  uint64
}

func personaBody(specs ...personaSpec) []byte {
	e := &depot.Enc{}
	for _, p := range specs {
		f := &depot.Enc{}
		f.Fixed64(1, p.id)
		f.Varint(2, p.state)
		if p.app != 0 {
			f.Varint(3, uint64(p.app))
		}
		if p.name != "" {
			f.Str(15, p.name)
		}
		if p.ava != 0 {
			h := make([]byte, 20)
			for i := range h {
				h[i] = p.ava
			}
			f.BytesField(31, h)
		}
		if p.seen != 0 {
			f.Varint(47, p.seen)
		}
		e.BytesField(2, f.Bytes())
	}
	return e.Bytes()
}

func playingBody(blocked bool, app uint32) []byte {
	e := &depot.Enc{}
	if blocked {
		e.Varint(2, 1)
	}
	if app != 0 {
		e.Varint(3, uint64(app))
	}
	return e.Bytes()
}

func incomingBody(friend uint64, entry uint64, text string, ts, ord uint64, localEcho bool) []byte {
	e := &depot.Enc{}
	e.Fixed64(1, friend)
	e.Varint(2, entry)
	if text != "" {
		e.Str(4, text)
	}
	e.Fixed32(5, uint32(ts))
	e.Varint(6, ord)
	if localEcho {
		e.Varint(7, 1)
	}
	return e.Bytes()
}

// chatFake models the store side of FriendMessages: history per friend and
// unread counts. All methods are safe for concurrent use.
type chatFake struct {
	mu      sync.Mutex
	convs   map[uint64][]chatMsg
	nextOrd map[uint64]uint64
	unread  map[uint64]int
	sends   int
	acks    []uint64
	names   map[uint64]string
	ts      uint64
}

type chatMsg struct {
	fromSelf bool
	text     string
	ts       uint64
	ord      uint64
}

func (c *chatFake) handler(target string, body []byte) ([]byte, int) {
	m, _ := depot.Decode(body)
	switch target {
	case "Player.GetPlayerLinkDetails#1":
		e := &depot.Enc{}
		for _, id := range m.AllInts(1) {
			pub := &depot.Enc{}
			pub.Fixed64(1, id)
			if n := c.names[id]; n != "" {
				pub.Str(10, n)
			}
			inner := &depot.Enc{}
			inner.MsgField(1, pub)
			e.MsgField(1, inner)
		}
		return e.Bytes(), 1
	case "FriendMessages.SendMessage#1":
		friend := m.Int(1)
		text := string(m.Bytes(3))
		c.mu.Lock()
		c.sends++
		c.ts++
		ord := c.nextOrd[friend] + 1
		c.nextOrd[friend] = ord
		c.convs[friend] = append(c.convs[friend], chatMsg{fromSelf: true, text: text, ts: c.ts, ord: ord})
		ts := c.ts
		c.mu.Unlock()
		e := &depot.Enc{}
		e.Varint(2, ts)
		e.Varint(3, ord)
		return e.Bytes(), 1
	case "FriendMessages.GetRecentMessages#1":
		friend := m.Int(2)
		limit := int(m.Int(3))
		if limit <= 0 {
			limit = 50
		}
		recent := m.Int(4) != 0
		tl, ol := m.Int(8), m.Int(9)
		c.mu.Lock()
		conv := append([]chatMsg{}, c.convs[friend]...)
		c.mu.Unlock()
		if !recent {
			// Page older than (time_last, ordinal_last).
			kept := conv[:0:0]
			for _, mm := range conv {
				if mm.ts < tl || (mm.ts == tl && mm.ord < ol) {
					kept = append(kept, mm)
				}
			}
			conv = kept
		}
		more := false
		if len(conv) > limit {
			more = true
			conv = conv[len(conv)-limit:]
		}
		e := &depot.Enc{}
		for _, mm := range conv {
			fm := &depot.Enc{}
			if mm.fromSelf {
				fm.Varint(1, uint64(uint32(testSelfID)))
			} else {
				fm.Varint(1, uint64(uint32(friend)))
			}
			fm.Varint(2, mm.ts)
			fm.Str(3, mm.text)
			fm.Varint(4, mm.ord)
			e.MsgField(1, fm)
		}
		if more {
			e.Varint(4, 1)
		}
		return e.Bytes(), 1
	case "FriendMessages.GetActiveMessageSessions#1":
		c.mu.Lock()
		defer c.mu.Unlock()
		e := &depot.Enc{}
		for f, n := range c.unread {
			if n <= 0 {
				continue
			}
			sm := &depot.Enc{}
			sm.Varint(1, uint64(uint32(f)))
			sm.Varint(4, uint64(n))
			e.MsgField(1, sm)
		}
		return e.Bytes(), 1
	case "FriendMessages.AckMessage#1":
		c.mu.Lock()
		c.acks = append(c.acks, m.Int(1))
		c.unread[m.Int(1)] = 0
		c.mu.Unlock()
		return []byte{}, 1
	}
	return []byte{}, 1
}

// socialProc drives runSocial in-process with piped stdin/stdout.
type socialProc struct {
	t     *testing.T
	in    *os.File
	lines chan string
	done  chan error
}

func startSocial(t *testing.T, status string) *socialProc {
	t.Helper()
	oldExit := socialExit
	socialExit = func(code int) {}
	t.Cleanup(func() { socialExit = oldExit })
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = inR, outW
	t.Cleanup(func() { os.Stdin, os.Stdout = oldIn, oldOut })
	p := &socialProc{t: t, in: inW, lines: make(chan string, 256), done: make(chan error, 1)}
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(make([]byte, 256*1024), 1024*1024)
		for sc.Scan() {
			p.lines <- sc.Text()
		}
	}()
	go func() { p.done <- runSocial(context.Background()) }()
	req := map[string]any{"refresh_token": testToken(testSelfID)}
	if status != "" {
		req["status"] = status
	}
	raw, _ := json.Marshal(req)
	fmt.Fprintln(inW, string(raw))
	return p
}

func (p *socialProc) send(op string, params map[string]any, id int64) {
	p.t.Helper()
	raw, _ := json.Marshal(map[string]any{"id": id, "op": op, "params": params})
	fmt.Fprintln(p.in, string(raw))
}

// stop ends the session and settles: the monitor's last tick (250 ms)
// may still emit into this pipe after Close, so drain past it before the
// next test swaps the shared stdout.
func (p *socialProc) stop(id int64) {
	p.t.Helper()
	p.send("stop", nil, id)
	p.nextReply(id, 10*time.Second)
	time.Sleep(500 * time.Millisecond)
	for {
		select {
		case <-p.lines:
		default:
			return
		}
	}
}

type socialLine struct {
	ID     *int64          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Event  string          `json:"event"`
	Params json.RawMessage `json:"params"`
}

func (p *socialProc) nextReply(id int64, timeout time.Duration) (json.RawMessage, []socialLine) {
	p.t.Helper()
	var events []socialLine
	deadline := time.Now().Add(timeout)
	for {
		rest := time.Until(deadline)
		if rest <= 0 {
			p.t.Fatalf("no reply %d within %v", id, timeout)
		}
		select {
		case line := <-p.lines:
			var sl socialLine
			if err := json.Unmarshal([]byte(line), &sl); err != nil {
				p.t.Fatalf("bad helper line %q: %v", line, err)
			}
			if sl.Event != "" {
				events = append(events, sl)
				continue
			}
			if sl.ID != nil && *sl.ID == id {
				if sl.Error != nil {
					p.t.Fatalf("op %d failed %d: %s", id, sl.Error.Code, sl.Error.Message)
				}
				return sl.Result, events
			}
		case <-time.After(rest):
			p.t.Fatalf("no reply %d within %v", id, timeout)
		}
	}
}

// nextEvent waits for one event of the named kind.
func (p *socialProc) nextEvent(kind string, timeout time.Duration) json.RawMessage {
	p.t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		rest := time.Until(deadline)
		if rest <= 0 {
			p.t.Fatalf("no event %q within %v", kind, timeout)
		}
		select {
		case line := <-p.lines:
			var sl socialLine
			if err := json.Unmarshal([]byte(line), &sl); err != nil {
				p.t.Fatalf("bad helper line %q: %v", line, err)
			}
			if sl.Event == kind {
				return sl.Params
			}
		case <-time.After(rest):
			p.t.Fatalf("no event %q within %v", kind, timeout)
		}
	}
}

func useSocialFake(t *testing.T, cfg steamcmtest.Config) *steamcmtest.Fake {
	t.Helper()
	t.Setenv("PLATYPUS_PLUGIN_DATA", t.TempDir())
	f := steamcmtest.New(t, cfg)
	t.Setenv("PLATYPUS_STEAM_CM_SERVER", f.Host())
	return f
}

func friendIDs(v any) []string {
	m := v.(map[string]any)
	ids := []string{}
	for _, f := range m["friends"].([]any) {
		ids = append(ids, f.(map[string]any)["id"].(string))
	}
	return ids
}

func TestSocialFriendsFullAndIncremental(t *testing.T) {
	cf := &chatFake{convs: map[uint64][]chatMsg{}, nextOrd: map[uint64]uint64{},
		unread: map[uint64]int{}, names: map[uint64]string{testF2: "Bo Lindqvist"}}
	full := friendsListBody(false,
		friendEntry(testF1, 3), friendEntry(testF2, 3),
		friendEntry(76561198000000009, 1), friendEntry(76561198000000010, 2),
		friendEntry(testClanID(), 3))
	persona := personaBody(
		personaSpec{id: testF1, state: 1, app: 322500, name: "Ana Ruiz", ava: 0xAB, seen: 1791398361},
		personaSpec{id: testF2, state: 0, seen: 1791398000}, // no name: link details backfills
		personaSpec{id: testSelfID, state: 1, name: "Self Player", ava: 0xCD},
	)
	incr := friendsListBody(true, friendEntry(testF3, 3))
	incrPersona := personaBody(personaSpec{id: testF3, state: 3, name: "Cy Okafor"})
	f := useSocialFake(t, steamcmtest.Config{
		ServiceHandler: cf.handler,
		Pushes: []steamcmtest.Push{
			{EMsg: steamcm.EMsgClientFriendsList, Body: full},
			{EMsg: steamcm.EMsgClientPersonaState, Body: persona},
			{EMsg: steamcm.EMsgClientFriendsList, Body: incr, After: 1},
			{EMsg: steamcm.EMsgClientPersonaState, Body: incrPersona, After: 1},
		},
	})
	_ = f
	p := startSocial(t, "")
	p.send("waitReady", nil, 1)
	raw, _ := p.nextReply(1, 25*time.Second)
	var snap struct {
		Connection struct {
			State string `json:"state"`
		} `json:"connection"`
		Self struct {
			Name  string `json:"name"`
			State string `json:"state"`
		} `json:"self"`
		Friends []struct {
			ID        string         `json:"id"`
			Name      string         `json:"name"`
			State     string         `json:"state"`
			AvatarUrl string         `json:"avatarUrl"`
			Game      map[string]any `json:"game"`
			LastSeen  int64          `json:"lastSeen"`
		} `json:"friends"`
	}
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatalf("waitReady: %v", err)
	}
	// Blocked, pending and clan ids are filtered. The backfill lookup for
	// F2's name is a service call, so the scripted incremental push may
	// already have landed: F3 may or may not be here yet.
	byID := map[string]int{}
	for i, fr := range snap.Friends {
		byID[fr.ID] = i
	}
	for _, id := range []string{"76561198000000002", "76561198000000003"} {
		if _, ok := byID[id]; !ok {
			t.Fatalf("friends = %v, want F1 and F2", friendIDs(map[string]any{"friends": toAnySlice(snap.Friends)}))
		}
	}
	if len(snap.Friends) > 3 {
		t.Fatalf("friends = %v, want at most F1 F2 F3", friendIDs(map[string]any{"friends": toAnySlice(snap.Friends)}))
	}
	f1 := snap.Friends[byID["76561198000000002"]]
	if f1.Name != "Ana Ruiz" || f1.State != "online" {
		t.Fatalf("F1 = %+v", f1)
	}
	if !strings.HasPrefix(f1.AvatarUrl, "https://avatars.cloudflare.steamstatic.com/") || !strings.HasSuffix(f1.AvatarUrl, "_full.jpg") {
		t.Fatalf("F1 avatar %q", f1.AvatarUrl)
	}
	if f1.Game["appid"] != "322500" {
		t.Fatalf("F1 game = %v", f1.Game)
	}
	f2 := snap.Friends[byID["76561198000000003"]]
	if f2.Name != "Bo Lindqvist" || f2.State != "offline" || f2.LastSeen != 1791398000 {
		t.Fatalf("F2 = %+v", f2)
	}
	if snap.Self.Name != "Self Player" || snap.Self.State != "invisible" {
		t.Fatalf("self = %+v", snap.Self)
	}
	// The scripted incremental pushes rode in on the backfill lookup; the
	// new friend lands in state (the events already went out on stdout).
	p.send("history", map[string]any{"friendId": "76561198000000002"}, 2)
	hraw, _ := p.nextReply(2, 15*time.Second)
	var hist struct {
		Messages []any `json:"messages"`
		More     bool  `json:"more"`
	}
	if err := json.Unmarshal(hraw, &hist); err != nil || len(hist.Messages) != 0 || hist.More {
		t.Fatalf("history = %s", hraw)
	}
	found := false
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && !found {
		p.send("state", nil, 3)
		sraw, _ := p.nextReply(3, 10*time.Second)
		var st struct {
			Friends []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"friends"`
		}
		json.Unmarshal(sraw, &st)
		for _, fr := range st.Friends {
			if fr.ID == "76561198000000004" && fr.Name == "Cy Okafor" {
				found = true
			}
		}
		if !found {
			time.Sleep(200 * time.Millisecond)
		}
	}
	if !found {
		t.Fatal("incremental friend never landed in state")
	}
	p.stop(9)
}

func toAnySlice[T any](in []T) []any {
	out := make([]any, len(in))
	for i, v := range in {
		raw, _ := json.Marshal(v)
		var m any
		json.Unmarshal(raw, &m)
		out[i] = m
	}
	return out
}

func socialTestPushes() []steamcmtest.Push {
	full := friendsListBody(false, friendEntry(testF1, 3), friendEntry(testF2, 3))
	persona := personaBody(
		personaSpec{id: testF1, state: 1, name: "Ana Ruiz"},
		personaSpec{id: testF2, state: 1, name: "Bo Lindqvist"},
		personaSpec{id: testSelfID, state: 1, name: "Self Player"},
	)
	return []steamcmtest.Push{
		{EMsg: steamcm.EMsgClientFriendsList, Body: full},
		{EMsg: steamcm.EMsgClientPersonaState, Body: persona},
		{EMsg: steamcm.EMsgServiceMethodResponse, Target: "FriendMessagesClient.IncomingMessage#1",
			Body: incomingBody(testF2, 1, "hey", 1791400000, 7, false), After: 2},
		{EMsg: steamcm.EMsgServiceMethodResponse, Target: "FriendMessagesClient.IncomingMessage#1",
			Body: incomingBody(testF2, 2, "", 1791400001, 0, false), After: 3},
	}
}

// waitReadyReply runs the start handshake and returns the snapshot.
func waitReadyReply(p *socialProc, id int64) map[string]any {
	p.t.Helper()
	p.send("waitReady", nil, id)
	raw, _ := p.nextReply(id, 25*time.Second)
	var snap map[string]any
	if err := json.Unmarshal(raw, &snap); err != nil {
		p.t.Fatalf("waitReady: %v", err)
	}
	return snap
}

func TestSocialChatRoundTrip(t *testing.T) {
	cf := &chatFake{convs: map[uint64][]chatMsg{}, nextOrd: map[uint64]uint64{},
		unread: map[uint64]int{testF2: 3},
		names:  map[uint64]string{}}
	f := useSocialFake(t, steamcmtest.Config{ServiceHandler: cf.handler, Pushes: socialTestPushes()})
	_ = f
	p := startSocial(t, "")
	waitReadyReply(p, 1)

	// Validation first: unknown friends, empty text, missing clientId.
	for i, op := range []struct {
		op     string
		params map[string]any
		code   int
	}{
		{"history", map[string]any{"friendId": "76561198000000099"}, -32602},
		{"history", nil, -32602},
		{"send", map[string]any{"friendId": "76561198000000002", "text": "", "clientId": "c0"}, -32602},
		{"send", map[string]any{"friendId": "76561198000000002", "text": "x", "clientId": ""}, -32602},
		{"send", map[string]any{"friendId": "76561198000000099", "text": "x", "clientId": "c0"}, -32602},
		{"send", map[string]any{"friendId": "76561198000000002", "text": "no client"}, -32602},
		{"markRead", map[string]any{"friendId": "76561198000000099", "upTo": "1-1"}, -32602},
		{"markRead", map[string]any{"friendId": "76561198000000002", "upTo": "bogus"}, -32602},
		{"setStatus", map[string]any{"state": "lurking"}, -32602},
		{"setStatus", nil, -32602},
		{"setPlaying", map[string]any{"id": 42}, -32602},
	} {
		id := int64(10 + i)
		raw, _ := json.Marshal(map[string]any{"id": id, "op": op.op, "params": op.params})
		fmt.Fprintln(p.in, string(raw))
		deadline := time.Now().Add(10 * time.Second)
		for {
			rest := time.Until(deadline)
			if rest <= 0 {
				t.Fatalf("no reply for %s %v", op.op, op.params)
			}
			select {
			case line := <-p.lines:
				var sl socialLine
				json.Unmarshal([]byte(line), &sl)
				if sl.Event != "" || sl.ID == nil || *sl.ID != id {
					continue
				}
				if sl.Error == nil || sl.Error.Code != op.code {
					t.Fatalf("%s %v: got %+v, want code %d", op.op, op.params, sl, op.code)
				}
				goto next
			case <-time.After(rest):
				t.Fatalf("no reply for %s %v", op.op, op.params)
			}
		}
	next:
	}

	// Send is idempotent per clientId: the store sees one message.
	p.send("send", map[string]any{"friendId": "76561198000000002", "text": "hello", "clientId": "c1"}, 30)
	sraw, _ := p.nextReply(30, 15*time.Second)
	var sent struct {
		ID     string `json:"id"`
		SentAt int64  `json:"sentAt"`
	}
	json.Unmarshal(sraw, &sent)
	if sent.ID == "" || sent.SentAt == 0 {
		t.Fatalf("send = %s", sraw)
	}
	p.send("send", map[string]any{"friendId": "76561198000000002", "text": "hello", "clientId": "c1"}, 31)
	araw, _ := p.nextReply(31, 15*time.Second)
	var again struct {
		ID     string `json:"id"`
		SentAt int64  `json:"sentAt"`
	}
	json.Unmarshal(araw, &again)
	if again != sent {
		t.Fatalf("repeat send = %+v, want %+v", again, sent)
	}
	cf.mu.Lock()
	n := cf.sends
	cf.mu.Unlock()
	if n != 1 {
		t.Fatalf("store saw %d sends, want 1", n)
	}

	// The sent message is in the history afterwards, oldest first.
	p.send("history", map[string]any{"friendId": "76561198000000002", "limit": 5}, 32)
	hraw, evs := p.nextReply(32, 15*time.Second)
	var hist struct {
		Messages []struct {
			ID     string `json:"id"`
			From   string `json:"from"`
			Text   string `json:"text"`
			SentAt int64  `json:"sentAt"`
		} `json:"messages"`
		More bool `json:"more"`
	}
	json.Unmarshal(hraw, &hist)
	if len(hist.Messages) != 1 || hist.Messages[0].ID != sent.ID || hist.Messages[0].From != "self" {
		t.Fatalf("history = %s", hraw)
	}
	// The scripted incoming message (After 2) rode in before this reply.
	var incoming *socialLine
	for i := range evs {
		if evs[i].Event == "message" {
			incoming = &evs[i]
		}
	}
	if incoming == nil {
		t.Fatalf("no incoming message among %d events", len(evs))
	}
	var im struct {
		FriendID string `json:"friendId"`
		Message  struct {
			ID     string `json:"id"`
			From   string `json:"from"`
			SentAt int64  `json:"sentAt"`
		} `json:"message"`
	}
	json.Unmarshal(incoming.Params, &im)
	if im.FriendID != "76561198000000003" || im.Message.From != im.FriendID || im.Message.SentAt != 1791400000*1000 {
		t.Fatalf("incoming = %s", incoming.Params)
	}
	// The typing notice (After 3) arrives around the next call.
	p.send("history", map[string]any{"friendId": "76561198000000003"}, 33)
	_, evs2 := p.nextReply(33, 15*time.Second)
	typed := false
	for i := range evs2 {
		if evs2[i].Event == "typing" {
			typed = true
		}
	}
	if !typed {
		t.Fatalf("no typing notice among %d events", len(evs2))
	}

	// markRead acks without waiting for a reply (NoResponse service).
	p.send("markRead", map[string]any{"friendId": "76561198000000002", "upTo": sent.ID}, 34)
	p.nextReply(34, 15*time.Second)
	deadline := time.Now().Add(5 * time.Second)
	for {
		cf.mu.Lock()
		acked := len(cf.acks) > 0
		cf.mu.Unlock()
		if acked || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	cf.mu.Lock()
	acked := append([]uint64{}, cf.acks...)
	cf.mu.Unlock()
	if len(acked) != 1 || acked[0] != testF1 {
		t.Fatalf("acks = %v", acked)
	}

	// The unread snapshot carries the store's counts.
	p.send("unreadCounts", nil, 35)
	uraw, _ := p.nextReply(35, 15*time.Second)
	var uc struct {
		Counts []struct {
			FriendID string `json:"friendId"`
			Unread   int    `json:"unread"`
		} `json:"counts"`
	}
	json.Unmarshal(uraw, &uc)
	if len(uc.Counts) != 1 || uc.Counts[0].FriendID != "76561198000000003" || uc.Counts[0].Unread != 3 {
		t.Fatalf("unreadCounts = %s", uraw)
	}

	// setStatus and setPlaying reach the wire (716/742 recorded).
	p.send("setStatus", map[string]any{"state": "away"}, 36)
	p.nextReply(36, 15*time.Second)
	p.send("setPlaying", map[string]any{"id": "322500"}, 37)
	p.nextReply(37, 15*time.Second)
	p.send("setPlaying", nil, 38)
	p.nextReply(38, 15*time.Second)
	p.stop(39)
	if !f.WaitFor(5*time.Second, func(evs []string) bool {
		var s716, s742 int
		for _, e := range evs {
			if e == "emsg 716" {
				s716++
			}
			if e == "emsg 742" {
				s742++
			}
		}
		return s716 >= 2 && s742 >= 2
	}) {
		t.Fatalf("status/playing not on the wire: %v", f.Events())
	}
}

// Our own send echoed back (local_echo) is dropped while the send is in
// flight and when it matches the confirmed send; an unmatched echo (sent
// from another of our devices) arrives as from self, with no unread.
func TestSocialLocalEcho(t *testing.T) {
	cf := &chatFake{convs: map[uint64][]chatMsg{}, nextOrd: map[uint64]uint64{},
		unread: map[uint64]int{}, names: map[uint64]string{}}
	f := useSocialFake(t, steamcmtest.Config{
		ServiceHandler: cf.handler,
		Pushes: []steamcmtest.Push{
			{EMsg: steamcm.EMsgClientFriendsList, Body: friendsListBody(false, friendEntry(testF1, 3))},
			{EMsg: steamcm.EMsgClientPersonaState, Body: personaBody(
				personaSpec{id: testF1, state: 1, name: "Ana Ruiz"},
				personaSpec{id: testSelfID, state: 1, name: "Self Player"})},
			// The first send gets ts 1, ord 1: this echo lands
			// mid-call and is dropped before core has the id.
			{EMsg: steamcm.EMsgServiceMethodResponse, Target: "FriendMessagesClient.IncomingMessage#1",
				Body: incomingBody(testF1, 1, "hello", 1, 1, true), After: 1},
			// The same echo after the reply matches the confirmed
			// send and is dropped too.
			{EMsg: steamcm.EMsgServiceMethodResponse, Target: "FriendMessagesClient.IncomingMessage#1",
				Body: incomingBody(testF1, 1, "hello", 1, 1, true), After: 2},
			// An echo from our other device arrives as from self.
			{EMsg: steamcm.EMsgServiceMethodResponse, Target: "FriendMessagesClient.IncomingMessage#1",
				Body: incomingBody(testF1, 1, "elsewhere", 1791399999, 9, true), After: 3},
		},
	})
	_ = f
	p := startSocial(t, "")
	waitReadyReply(p, 1)

	hasEvent := func(evs []socialLine, kind string) *socialLine {
		for i := range evs {
			if evs[i].Event == kind {
				return &evs[i]
			}
		}
		return nil
	}
	p.send("send", map[string]any{"friendId": "76561198000000002", "text": "hello", "clientId": "c1"}, 30)
	sraw, evs := p.nextReply(30, 15*time.Second)
	var sent struct {
		ID string `json:"id"`
	}
	json.Unmarshal(sraw, &sent)
	if sent.ID != "1-1" {
		t.Fatalf("send = %s", sraw)
	}
	if m := hasEvent(evs, "message"); m != nil {
		t.Fatalf("in-flight echo shown: %s", m.Params)
	}
	p.send("history", map[string]any{"friendId": "76561198000000002"}, 31)
	_, evs = p.nextReply(31, 15*time.Second)
	if m := hasEvent(evs, "message"); m != nil {
		t.Fatalf("matched echo shown: %s", m.Params)
	}
	p.send("history", map[string]any{"friendId": "76561198000000002"}, 32)
	_, evs = p.nextReply(32, 15*time.Second)
	incoming := hasEvent(evs, "message")
	if incoming == nil {
		t.Fatalf("unmatched echo missing among %d events", len(evs))
	}
	var im struct {
		FriendID string `json:"friendId"`
		Message  struct {
			ID     string `json:"id"`
			From   string `json:"from"`
			Text   string `json:"text"`
			SentAt int64  `json:"sentAt"`
		} `json:"message"`
	}
	json.Unmarshal(incoming.Params, &im)
	if im.Message.From != "self" || im.Message.ID != "1791399999-9" || im.Message.Text != "elsewhere" {
		t.Fatalf("echo = %s", incoming.Params)
	}
	if u := hasEvent(evs, "unread"); u != nil {
		t.Fatalf("echo raised unread: %s", u.Params)
	}
	p.stop(33)
}

// A later waitReady answers immediately: the first sync is remembered, so
// the names lookup runs once no matter how often the plugin asks.
func TestSocialWaitReadyOnce(t *testing.T) {
	cf := &chatFake{convs: map[uint64][]chatMsg{}, nextOrd: map[uint64]uint64{},
		unread: map[uint64]int{}, names: map[uint64]string{}}
	f := useSocialFake(t, steamcmtest.Config{
		ServiceHandler: cf.handler,
		Pushes: []steamcmtest.Push{
			{EMsg: steamcm.EMsgClientFriendsList, Body: friendsListBody(false, friendEntry(testF1, 3))},
			// No persona: the first waitReady times out and backfills
			// the name over Player.GetPlayerLinkDetails.
		},
	})
	p := startSocial(t, "")
	waitReadyReply(p, 1)
	calls := 0
	for _, e := range f.Events() {
		if strings.HasPrefix(e, "service Player.GetPlayerLinkDetails") {
			calls++
		}
	}
	if calls != 1 {
		t.Fatalf("lookups after first waitReady = %d, want 1", calls)
	}
	start := time.Now()
	p.send("waitReady", nil, 2)
	p.nextReply(2, 10*time.Second)
	if dt := time.Since(start); dt > 5*time.Second {
		t.Fatalf("second waitReady took %v", dt)
	}
	calls = 0
	for _, e := range f.Events() {
		if strings.HasPrefix(e, "service Player.GetPlayerLinkDetails") {
			calls++
		}
	}
	if calls != 1 {
		t.Fatalf("lookups after second waitReady = %d, want 1", calls)
	}
	p.stop(3)
}

func TestSocialReconnectRerequests(t *testing.T) {
	cf := &chatFake{convs: map[uint64][]chatMsg{}, nextOrd: map[uint64]uint64{},
		unread: map[uint64]int{}, names: map[uint64]string{}}
	f := useSocialFake(t, steamcmtest.Config{
		ServiceHandler: cf.handler,
		Pushes: []steamcmtest.Push{
			{EMsg: steamcm.EMsgClientFriendsList, Body: friendsListBody(false, friendEntry(testF1, 3))},
			{EMsg: steamcm.EMsgClientPersonaState, Body: personaBody(
				personaSpec{id: testF1, state: 1, name: "Ana Ruiz"},
				personaSpec{id: testSelfID, state: 1, name: "Self Player"})},
		},
		// The first logon serves normally, then the socket dies: the
		// client reconnects on its own and the second logon re-requests.
		DropAfter: 300 * time.Millisecond,
	})
	p := startSocial(t, "")
	snap := waitReadyReply(p, 1)
	if friends, _ := snap["friends"].([]any); len(friends) != 1 {
		t.Fatalf("friends = %v", friends)
	}
	// The drop reconnects on its own; the second logon re-requests data
	// and resyncs the status and the (empty) playing report.
	if !f.WaitFor(10*time.Second, func(evs []string) bool {
		var e815, e716, e742 int
		for _, e := range evs {
			switch e {
			case "emsg 815":
				e815++
			case "emsg 716":
				e716++
			case "emsg 742":
				e742++
			}
		}
		return e815 >= 2 && e716 >= 2 && e742 >= 1
	}) {
		t.Fatalf("no resync after reconnect: %v", f.Events())
	}
	p.stop(2)
}

// A friends list batched in the same Multi as the logon response must
// arrive: the handlers are registered before the first logon, and the
// batch remainder is routed once online.
func TestSocialBatchedLogonBatch(t *testing.T) {
	cf := &chatFake{convs: map[uint64][]chatMsg{}, nextOrd: map[uint64]uint64{},
		unread: map[uint64]int{}, names: map[uint64]string{}}
	f := useSocialFake(t, steamcmtest.Config{
		ServiceHandler: cf.handler,
		Pushes: []steamcmtest.Push{
			{EMsg: steamcm.EMsgClientFriendsList, Body: friendsListBody(false, friendEntry(testF1, 3))},
			{EMsg: steamcm.EMsgClientPersonaState, Body: personaBody(
				personaSpec{id: testF1, state: 1, name: "Ana Ruiz"},
				personaSpec{id: testSelfID, state: 1, name: "Self Player"})},
		},
		BatchLogon: true,
	})
	_ = f
	p := startSocial(t, "")
	snap := waitReadyReply(p, 1)
	friends, _ := snap["friends"].([]any)
	if len(friends) != 1 {
		t.Fatalf("friends = %v", friends)
	}
	if name := friends[0].(map[string]any)["name"]; name != "Ana Ruiz" {
		t.Fatalf("batched persona missing: %v", friends)
	}
	p.stop(2)
}

func TestSocialReplacedStopsLoop(t *testing.T) {
	exits := make(chan int, 2)
	cf := &chatFake{convs: map[uint64][]chatMsg{}, nextOrd: map[uint64]uint64{},
		unread: map[uint64]int{}, names: map[uint64]string{}}
	f := useSocialFake(t, steamcmtest.Config{
		ServiceHandler: cf.handler,
		Pushes: []steamcmtest.Push{
			{EMsg: steamcm.EMsgClientFriendsList, Body: friendsListBody(false, friendEntry(testF1, 3))},
			{EMsg: steamcm.EMsgClientPersonaState, Body: personaBody(
				personaSpec{id: testF1, state: 1, name: "Ana Ruiz"},
				personaSpec{id: testSelfID, state: 1, name: "Self Player"})},
		},
		Kick: 6, KickAfter: 1,
	})
	_ = f
	p := startSocial(t, "")
	// The driver stubs the exit; tap it here to observe the stop.
	socialExit = func(code int) { exits <- code }
	waitReadyReply(p, 1)
	// The kick needs a service call to land on; history is one.
	p.send("history", map[string]any{"friendId": "76561198000000002"}, 2)
	p.nextReply(2, 15*time.Second)
	params := p.nextEvent("connection", 10*time.Second)
	var conn struct {
		State string `json:"state"`
	}
	json.Unmarshal(params, &conn)
	if conn.State != "replaced" {
		t.Fatalf("connection = %s, want replaced", params)
	}
	select {
	case code := <-exits:
		if code != 0 {
			t.Fatalf("exit code %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no exit after replaced")
	}
	// No reconnect fight: exactly one logon happened.
	n := 0
	for _, e := range f.Events() {
		if strings.HasPrefix(e, "logon ") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("logons = %d, want 1", n)
	}
}

func TestSocialPassiveSendsNoStatus(t *testing.T) {
	cf := &chatFake{convs: map[uint64][]chatMsg{}, nextOrd: map[uint64]uint64{},
		unread: map[uint64]int{}, names: map[uint64]string{}}
	f := useSocialFake(t, steamcmtest.Config{
		ServiceHandler: cf.handler,
		Pushes: []steamcmtest.Push{
			{EMsg: steamcm.EMsgClientFriendsList, Body: friendsListBody(false, friendEntry(testF1, 3))},
			{EMsg: steamcm.EMsgClientPersonaState, Body: personaBody(
				personaSpec{id: testF1, state: 1, name: "Ana Ruiz"},
				personaSpec{id: testSelfID, state: 1, name: "Self Player"})},
		},
	})
	p := startSocial(t, "none")
	snap := waitReadyReply(p, 1)
	if friends, _ := snap["friends"].([]any); len(friends) != 1 {
		t.Fatalf("friends = %v", friends)
	}
	for _, e := range f.Events() {
		if e == "emsg 716" {
			t.Fatalf("passive session sent ClientChangeStatus: %v", f.Events())
		}
	}
	p.stop(2)
}

func TestSocialPlayingBlocked(t *testing.T) {
	cf := &chatFake{convs: map[uint64][]chatMsg{}, nextOrd: map[uint64]uint64{},
		unread: map[uint64]int{}, names: map[uint64]string{}}
	f := useSocialFake(t, steamcmtest.Config{
		ServiceHandler: cf.handler,
		Pushes: []steamcmtest.Push{
			{EMsg: steamcm.EMsgClientFriendsList, Body: friendsListBody(false, friendEntry(testF1, 3))},
			{EMsg: steamcm.EMsgClientPersonaState, Body: personaBody(
				personaSpec{id: testF1, state: 1, name: "Ana Ruiz"},
				personaSpec{id: testSelfID, state: 1, name: "Self Player"})},
			{EMsg: steamcm.EMsgClientPlayingSessionState, Body: playingBody(true, 999)},
			// The slot frees on the next service call.
			{EMsg: steamcm.EMsgClientPlayingSessionState, Body: playingBody(false, 0), After: 1},
		},
	})
	p := startSocial(t, "")
	waitReadyReply(p, 1)
	p.send("setPlaying", map[string]any{"id": "322500"}, 2)
	p.nextReply(2, 15*time.Second)
	time.Sleep(300 * time.Millisecond)
	for _, e := range f.Events() {
		if e == "emsg 742" {
			t.Fatalf("reported while blocked: %v", f.Events())
		}
	}
	// A service call lets the unblock push through; the game is reported.
	p.send("history", map[string]any{"friendId": "76561198000000002"}, 3)
	p.nextReply(3, 15*time.Second)
	if !f.WaitFor(5*time.Second, func(evs []string) bool {
		for _, e := range evs {
			if e == "emsg 742" {
				return true
			}
		}
		return false
	}) {
		t.Fatalf("no report after unblock: %v", f.Events())
	}
	p.stop(4)
}
