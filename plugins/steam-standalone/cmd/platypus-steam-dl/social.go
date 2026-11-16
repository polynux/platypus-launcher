// Social subcommand: the persistent CM session behind the plugin's
// social.* and chat.* methods (protocol 1.6/1.9, docs/plans/steam-cloud-social.md
// phases 5-6). It logs on with the social logon id (a persistent session must
// not kick one-shot depot/auth jobs, plan D8), keeps the friends list and
// persona states, and answers chat over FriendMessages.
//
// Stdin starts with one request line:
//
//	{"refresh_token": "...", "status": "online"}
//
// status is the own persona sent after logon (online|away|busy|snooze|
// invisible, default online); "none" stays passive and never sends
// ClientChangeStatus, so a read-only live check can read the friends list
// without changing what friends see. The token may be omitted when the
// plugin data dir holds one (tokenFromFile).
//
// Further stdin lines are commands:
//
//	{"id": 1, "op": "waitReady"}
//	{"id": 2, "op": "state"}
//	{"id": 3, "op": "setStatus", "params": {"state": "away"}}
//	{"id": 4, "op": "setPlaying", "params": {"id": "322500"}}
//	{"id": 5, "op": "history", "params": {"friendId": "7656...", "limit": 50}}
//	{"id": 6, "op": "send", "params": {"friendId": "7656...", "text": "...", "clientId": "..."}}
//	{"id": 7, "op": "markRead", "params": {"friendId": "7656...", "upTo": "1700000000-3"}}
//	{"id": 8, "op": "unreadCounts"}
//	{"id": 9, "op": "stop"}
//
// Stdout carries replies ({"id": N, "result": ...} or {"id": N, "error":
// {"code", "message"}}) and push events ({"event": "connection|self|
// friends|message|typing", "params": ...}). Message text is never
// logged; only counts and ids reach any log.
package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"platypus/internal/depot"
	"platypus/internal/steamcm"
)

// Persona states (EPersonaState) and the protocol names they map to.
var personaToState = map[uint64]string{
	0: "offline",
	1: "online",
	2: "busy",
	3: "away",
	4: "snooze",
	5: "looking",
	6: "looking",
	7: "invisible",
}

var stateToPersona = map[string]uint64{
	"online":    1,
	"busy":      2,
	"away":      3,
	"snooze":    4,
	"invisible": 7,
}

// requestFriendDataFlags asks for everything node-steam-user's getPersonas
// asks for except Metadata (32), which SteamKit marks removed.
const requestFriendDataFlags = 1 | 2 | 4 | 8 | 16 | 64 | 128 | 256 | 512 | 1024 | 2048 | 4096 | 8192 | 16384

// defaultSocialStatus is the persona a fresh session reports: Online, as
// the Steam client does (user decision, issue #7).
const defaultSocialStatus = "online"

const (
	// readyTimeout bounds the initial sync waitReady answers with.
	readyTimeout = 10 * time.Second
	// chatPageCap is the most history returns in one page.
	chatPageCap = 100
	// defaultHistoryLimit is the page size when the caller names none.
	defaultHistoryLimit = 50
)

type socialRequest struct {
	RefreshToken string `json:"refresh_token"`
	Status       string `json:"status"`
}

type socialCommand struct {
	ID     int64           `json:"id"`
	Op     string          `json:"op"`
	Params json.RawMessage `json:"params"`
}

type socialError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Codes mirror the JSON-RPC codes the plugin answers with.
const (
	codeInvalidParams = -32602
	codeStoreError    = -32000
	codeAuthRequired  = -32001
)

type friendInfo struct {
	id       uint64
	name     string
	avatar   string
	state    string
	gameApp  uint32
	gameName string
	lastSeen int64
	hasData  bool
}

type sentMsg struct {
	id     string
	sentAt int64
}

type socialSession struct {
	mu       sync.Mutex
	client   *steamcm.Client
	selfID   uint64
	status   string
	selfName string
	selfAva  string
	friends  map[uint64]*friendInfo
	gotList  bool
	// playing is what the user plays on this store (0 = none); blocked is
	// set while another session holds the playing slot.
	playingDesired uint32
	playingSent    uint32
	blocked        bool
	playingApp     uint32
	sent           map[string]sentMsg
	// inSend counts the in-flight SendMessage calls per friend, so our
	// own echo is dropped before core has the send's id to dedupe by.
	inSend map[uint64]int
	// sentEcho marks confirmed sends (friend/id) whose echo is dropped.
	sentEcho map[string]bool
	// requested tracks ids a ClientRequestFriendData already asked for,
	// so the waitReady loop doesn't re-ask every poll.
	requested map[uint64]bool
	ready     chan struct{}
	readyOnce sync.Once
	// readyDone remembers the first waitReady finished; later calls
	// answer immediately. A reconnect's full list still merges and the
	// monitor still resyncs around it.
	readyDone bool
	outMu     sync.Mutex
}

func runSocial(ctx context.Context) error {
	br := bufio.NewReader(os.Stdin)
	line, err := br.ReadString('\n')
	if err != nil && len(line) == 0 {
		return fmt.Errorf("couldn't read the social request: %v", err)
	}
	var req socialRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		return fmt.Errorf("bad social request: %v", err)
	}
	token := req.RefreshToken
	if token == "" {
		token = tokenFromFile()
	}
	if token == "" {
		emitSocialErr(0, codeAuthRequired, "log in to Steam first")
		return fmt.Errorf("no refresh token")
	}
	status := req.Status
	if status == "" {
		status = defaultSocialStatus
	}
	if status != "none" {
		if _, ok := stateToPersona[status]; !ok {
			emitSocialErr(0, codeInvalidParams, fmt.Sprintf("invalid params: state %q is not a status", status))
			return fmt.Errorf("bad initial status")
		}
	}
	s := &socialSession{
		status:    status,
		friends:   map[uint64]*friendInfo{},
		sent:      map[string]sentMsg{},
		inSend:    map[uint64]int{},
		sentEcho:  map[string]bool{},
		requested: map[uint64]bool{},
		ready:     make(chan struct{}),
	}
	// Known before Connect: the friends list and persona pushes ride in
	// with the logon response, and their handlers need our own id.
	s.selfID, _ = steamcm.SteamIDFromToken(token)
	cfg := steamcm.ClientConfig{
		RefreshToken: token,
		Role:         steamcm.LoginRoleSocial,
		MachineID:    steamcm.StableMachineID(os.Getenv("PLATYPUS_PLUGIN_DATA")),
		// Handlers ride on the config so pushes batched with the
		// logon response are routed, not dropped.
		OnPush: map[uint32]func(depot.Msg, []byte){
			steamcm.EMsgClientFriendsList:         func(_ depot.Msg, body []byte) { s.onFriendsList(body) },
			steamcm.EMsgClientPersonaState:        func(_ depot.Msg, body []byte) { s.onPersonaState(body) },
			steamcm.EMsgClientPlayingSessionState: func(_ depot.Msg, body []byte) { s.onPlayingState(body) },
			steamcm.EMsgServiceMethodResponse:     func(hdr depot.Msg, body []byte) { s.onService(hdr, body) },
			// FamilyGroupsClient.NotifyRunningApps#1 shares EMsg 146
			// with the cloud change notification; onFamilyRunning
			// keeps only that target_job_name.
			steamcm.EMsgCloudNotifyAppStateChange: func(hdr depot.Msg, body []byte) { s.onFamilyRunning(hdr, body) },
		},
	}
	c, err := steamcm.Connect(ctx, cfg)
	if err != nil {
		if ce, ok := err.(*steamcm.CMError); ok && ce.Auth {
			emitSocialErr(0, codeAuthRequired, "log in to Steam first (the stored login was rejected)")
		} else {
			emitSocialErr(0, codeStoreError, shortSocialErr(err))
		}
		return err
	}
	s.client = c
	// The session's steamid after logon wins over the token's parse when
	// they differ (issue #189); the subscribe reads it from the client.
	if id := c.SteamID(); id != 0 {
		s.selfID = id
	}

	// Reconnects resend what the session needs; the monitor also reports
	// connection changes and ends the process on replaced/loggedOut.
	go s.monitor(ctx)

	if s.status != "none" {
		if err := s.sendStatus(s.status); err != nil {
			c.Close()
			emitSocialErr(0, codeStoreError, shortSocialErr(err))
			return err
		}
	}
	s.emit("connection", map[string]any{"state": "online"})
	// Ask Steam to push the family running list to this session; the push
	// carries the take-back signal the plugin warns about. Never gates the
	// session: waitReady and the command loop run without it.
	go s.subscribeFamilyRunning()

	sc := bufio.NewScanner(br)
	sc.Buffer(make([]byte, 256*1024), 1024*1024)
	for sc.Scan() {
		var cmd socialCommand
		if err := json.Unmarshal(sc.Bytes(), &cmd); err != nil {
			continue
		}
		if cmd.Op == "stop" {
			emitSocialResult(cmd.ID, map[string]any{})
			c.Close()
			return nil
		}
		s.handle(cmd)
	}
	c.Close()
	return nil
}

// socialExit ends the process when the session is replaced or logged out;
// tests stub it so a kicked session doesn't kill the test binary.
var socialExit = os.Exit

func (s *socialSession) monitor(ctx context.Context) {
	last := steamcm.StateOnline
	t := time.NewTicker(250 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		st := s.client.State()
		if st == last {
			continue
		}
		last = st
		switch st {
		case steamcm.StateReconnecting:
			s.emit("connection", map[string]any{"state": "reconnecting"})
		case steamcm.StateOnline:
			// Every return to Online restores what the drop may have
			// lost: own status, persona for every known friend, game.
			s.resync()
			s.emit("connection", map[string]any{"state": "online"})
		case steamcm.StateReplaced:
			s.emit("connection", map[string]any{"state": "replaced"})
			s.client.Close()
			socialExit(0)
			return
		case steamcm.StateOffline:
			// run() exits Offline on a refused token (loggedOut) and
			// on Close (handled by the command loop, not here).
			state := "offline"
			if err := s.client.Err(); err != nil {
				if ce, ok := err.(*steamcm.CMError); ok && ce.Auth {
					state = "loggedOut"
				}
			}
			s.emit("connection", map[string]any{"state": state})
			socialExit(0)
			return
		}
	}
}

// resync restores the session after a reconnect: own status, persona data
// for every known friend, the current game, and the family running-apps
// push (Steam only pushes to a session that asked with send_running_apps,
// so every reconnect re-asks).
func (s *socialSession) resync() {
	s.mu.Lock()
	status := s.status
	var ids []uint64
	for id := range s.friends {
		ids = append(ids, id)
	}
	desired := s.playingDesired
	s.mu.Unlock()
	if status != "none" {
		_ = s.sendStatus(status)
	}
	if len(ids) > 0 {
		ids = append(ids, s.selfID)
		_ = s.client.Send(steamcm.EMsgClientRequestFriendData, requestFriendData(ids))
	}
	s.mu.Lock()
	s.playingSent = 0 // force a resend below
	s.mu.Unlock()
	_ = s.sendPlaying(desired, true)
	// The reconnect event goes out without it; the re-ask runs behind.
	go s.subscribeFamilyRunning()
}

func (s *socialSession) handle(cmd socialCommand) {
	var result map[string]any
	var cerr *socialError
	switch cmd.Op {
	case "waitReady":
		result, cerr = s.waitReady()
	case "state":
		result = s.snapshot()
	case "setStatus":
		result, cerr = s.opSetStatus(cmd.Params)
	case "setPlaying":
		result, cerr = s.opSetPlaying(cmd.Params)
	case "history":
		result, cerr = s.opHistory(cmd.Params)
	case "send":
		result, cerr = s.opSend(cmd.Params)
	case "markRead":
		result, cerr = s.opMarkRead(cmd.Params)
	case "unreadCounts":
		result, cerr = s.opUnreadCounts()
	default:
		cerr = &socialError{Code: codeInvalidParams, Message: fmt.Sprintf("invalid params: unknown op %q", cmd.Op)}
	}
	if cerr != nil {
		emitSocialErr(cmd.ID, cerr.Code, cerr.Message)
		return
	}
	if result == nil {
		result = map[string]any{}
	}
	emitSocialResult(cmd.ID, result)
}

func emitSocialResult(id int64, result map[string]any) {
	raw, _ := json.Marshal(map[string]any{"id": id, "result": result})
	fmt.Println(string(raw))
}

func emitSocialErr(id int64, code int, msg string) {
	raw, _ := json.Marshal(map[string]any{"id": id, "error": map[string]any{"code": code, "message": msg}})
	fmt.Println(string(raw))
}

func (s *socialSession) emit(event string, params map[string]any) {
	s.outMu.Lock()
	defer s.outMu.Unlock()
	raw, _ := json.Marshal(map[string]any{"event": event, "params": params})
	fmt.Println(string(raw))
}

func shortSocialErr(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, ": "); i >= 0 {
		msg = msg[i+2:]
	}
	return msg
}

// requestFriendData builds ClientRequestFriendData (815): flags + ids.
func requestFriendData(ids []uint64) []byte {
	e := &depot.Enc{}
	e.Varint(1, requestFriendDataFlags)
	for _, id := range ids {
		e.Fixed64(2, id)
	}
	return e.Bytes()
}

// sendStatus sends ClientChangeStatus (716) for a protocol state name.
func (s *socialSession) sendStatus(state string) error {
	persona, ok := stateToPersona[state]
	if !ok {
		return fmt.Errorf("invalid params: state %q is not a status", state)
	}
	e := &depot.Enc{}
	e.Varint(1, persona)
	return s.client.Send(steamcm.EMsgClientChangeStatus, e.Bytes())
}

// sendPlaying reports the game (0 clears it) unless another session
// holds the playing slot; it never forces or kicks (plan D9/R11).
func (s *socialSession) sendPlaying(appID uint32, force bool) error {
	s.mu.Lock()
	blocked := s.blocked && s.playingApp != 0
	if !force && appID == s.playingSent {
		s.mu.Unlock()
		return nil
	}
	if blocked {
		s.mu.Unlock()
		return nil
	}
	// A push that arrived with the logon batch runs before Connect
	// returns, so s.client isn't set yet: the monitor's resync and the
	// waitReady loop send it once the session exists.
	cl := s.client
	if cl == nil {
		s.mu.Unlock()
		return nil
	}
	s.playingSent = appID
	s.mu.Unlock()
	e := &depot.Enc{}
	if appID != 0 {
		g := &depot.Enc{}
		g.Fixed64(2, uint64(appID))
		e.MsgField(1, g)
	}
	return cl.Send(steamcm.EMsgClientGamesPlayed, e.Bytes())
}

// onFriendsList handles ClientFriendsList (767): full replaces, incremental
// merges; only relationship 3 (friend) with an individual SteamID stays.
func (s *socialSession) onFriendsList(body []byte) {
	m, err := depot.Decode(body)
	if err != nil {
		return
	}
	incr := m.Int(1) != 0
	s.mu.Lock()
	if !incr {
		s.friends = map[uint64]*friendInfo{}
		s.requested = map[uint64]bool{}
	}
	var fresh []uint64
	removed := false
	for _, raw := range m.AllBytes(2) {
		fm, err := depot.Decode(raw)
		if err != nil {
			continue
		}
		id := fm.Int(1)
		if id == 0 || steamIDType(id) != 1 {
			continue
		}
		if fm.Int(2) != 3 {
			// An incremental drop for a known id removes it; a
			// full list simply omits it.
			if incr {
				if _, ok := s.friends[id]; ok {
					delete(s.friends, id)
					delete(s.requested, id)
					removed = true
				}
			}
			continue
		}
		if _, ok := s.friends[id]; !ok {
			s.friends[id] = &friendInfo{id: id, state: "offline"}
			fresh = append(fresh, id)
		}
	}
	s.gotList = true
	s.readyOnce.Do(func() { close(s.ready) })
	// Persona for new ids (plus our own, for the self name/avatar).
	req := append(append([]uint64{}, fresh...), s.selfID)
	var out []map[string]any
	switch {
	case removed:
		out = s.friendsLocked()
	case incr:
		for _, id := range fresh {
			out = append(out, renderFriend(s.friends[id]))
		}
	default:
		out = s.friendsLocked()
	}
	cl := s.client
	full := !incr || removed
	if cl != nil {
		for _, id := range req {
			s.requested[id] = true
		}
	}
	s.mu.Unlock()
	// A push batched with the logon response runs before Connect
	// returns (s.client unset); waitReady asks for the same data.
	if cl != nil {
		_ = cl.Send(steamcm.EMsgClientRequestFriendData, requestFriendData(req))
	}
	s.emit("friends", map[string]any{"full": full, "friends": out})
}

// friendsLocked renders the whole list, oldest id first.
func (s *socialSession) friendsLocked() []map[string]any {
	ids := make([]uint64, 0, len(s.friends))
	for id := range s.friends {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := []map[string]any{}
	for _, id := range ids {
		out = append(out, renderFriend(s.friends[id]))
	}
	return out
}

func renderFriend(f *friendInfo) map[string]any {
	m := map[string]any{
		"id":    strconv.FormatUint(f.id, 10),
		"name":  f.name,
		"state": f.state,
	}
	if m["name"] == "" {
		m["name"] = strconv.FormatUint(f.id, 10)
	}
	if f.avatar != "" {
		m["avatarUrl"] = f.avatar
	}
	if f.gameApp != 0 || f.gameName != "" {
		g := map[string]any{}
		if f.gameApp != 0 {
			g["appid"] = strconv.FormatUint(uint64(f.gameApp), 10)
		}
		if f.gameName != "" {
			g["title"] = f.gameName
		}
		m["game"] = g
	}
	if f.lastSeen != 0 {
		m["lastSeen"] = f.lastSeen
	}
	return m
}

// steamIDType returns the SteamID account type (bits 52-55): 1 individual,
// 7 clan.
func steamIDType(id uint64) uint64 {
	return (id >> 52) & 0xF
}

// onPersonaState handles ClientPersonaState (766): names, avatars, states,
// games and last-seen times for known friends, plus our own profile.
func (s *socialSession) onPersonaState(body []byte) {
	m, err := depot.Decode(body)
	if err != nil {
		return
	}
	s.mu.Lock()
	var changed []*friendInfo
	selfChanged := false
	for _, raw := range m.AllBytes(2) {
		fm, err := depot.Decode(raw)
		if err != nil {
			continue
		}
		id := fm.Int(1)
		if id == 0 {
			continue
		}
		name := string(fm.Bytes(15))
		ava := fm.Bytes(31)
		state := personaToState[fm.Int(2)]
		if state == "" {
			state = "offline"
		}
		app := uint32(fm.Int(3))
		gameID := fm.Int(56)
		if app == 0 && gameID != 0 {
			app = uint32(gameID & 0xFFFFFFFF)
		}
		gameName := string(fm.Bytes(55))
		seen := int64(fm.Int(47))
		if seen == 0 {
			seen = int64(fm.Int(45))
		}
		if id == s.selfID {
			if name != "" {
				s.selfName = name
			}
			if len(ava) == 20 {
				s.selfAva = avatarURL(ava)
			}
			selfChanged = true
			continue
		}
		f, ok := s.friends[id]
		if !ok {
			continue
		}
		f.hasData = true
		if name != "" {
			f.name = name
		}
		if len(ava) == 20 {
			f.avatar = avatarURL(ava)
		}
		f.state = state
		f.gameApp = app
		f.gameName = gameName
		f.lastSeen = seen
		changed = append(changed, f)
	}
	var out []map[string]any
	for _, f := range changed {
		out = append(out, renderFriend(f))
	}
	var self map[string]any
	if selfChanged {
		self = s.selfLocked()
	}
	s.mu.Unlock()
	if self != nil {
		s.emit("self", self)
	}
	if len(out) > 0 {
		s.emit("friends", map[string]any{"full": false, "friends": out})
	}
}

// onPlayingState handles ClientPlayingSessionState (9600): another session
// playing blocks our report until the slot frees.
func (s *socialSession) onPlayingState(body []byte) {
	m, err := depot.Decode(body)
	if err != nil {
		return
	}
	blocked := m.Int(2) != 0
	app := uint32(m.Int(3))
	s.mu.Lock()
	was := s.blocked && s.playingApp != 0
	s.blocked = blocked
	s.playingApp = app
	desired := s.playingDesired
	s.mu.Unlock()
	if was && !blocked {
		_ = s.sendPlaying(desired, true)
	}
}

// familySubscribeTimeout bounds each subscribe call: the running list is
// supplementary, so a silent Steam never stalls the startup or a resync.
const familySubscribeTimeout = 5 * time.Second

// subscribeFamilyRunning asks Steam to push the family running list to this
// persistent session: the group id via GetFamilyGroupForUser (the steamid
// is a uint64 varint, like the group id), then GetFamilyGroup with
// send_running_apps. Steam pushes FamilyGroupsClient.NotifyRunningApps#1
// now and again on every start/stop, each push the full current list.
// Failures stay silent: the running list is supplementary, and a user in
// no group simply gets no push. No ids reach any log. Runs in its own
// goroutine: the session startup and the reconnect event never wait for it.
func (s *socialSession) subscribeFamilyRunning() {
	if s.client == nil {
		return
	}
	// The session's steamid after logon, not the token's parse; with
	// none there is nothing to subscribe (issue #189).
	id := s.client.SteamID()
	if id == 0 {
		return
	}
	fu := &depot.Enc{}
	fu.Varint(1, id)
	raw, err := s.client.Call("FamilyGroups.GetFamilyGroupForUser#1", fu.Bytes(), familySubscribeTimeout)
	if err != nil {
		return
	}
	m, err := depot.Decode(raw)
	if err != nil {
		return
	}
	if m.Int(1) == 0 {
		return
	}
	g := &depot.Enc{}
	g.Varint(1, m.Int(1))
	g.Varint(2, 1) // send_running_apps
	_, _ = s.client.Call("FamilyGroups.GetFamilyGroup#1", g.Bytes(), familySubscribeTimeout)
}

// onFamilyRunning forwards one NotifyRunningApps push as a familyRunning
// event: the full running list ([{appid, player, owner}], steamids as
// strings), each push replacing the last. Malformed pushes are dropped
// silently; ids never reach any log.
func (s *socialSession) onFamilyRunning(hdr depot.Msg, body []byte) {
	if string(hdr.Bytes(12)) != notifyRunningApps {
		return
	}
	running := make([]map[string]any, 0)
	for _, e := range decodeRunningApps(body) {
		running = append(running, map[string]any{
			"appid":  e.appID,
			"player": strconv.FormatUint(e.player, 10),
			"owner":  strconv.FormatUint(e.owner, 10),
		})
	}
	s.emit("familyRunning", map[string]any{"running": running})
}

// onService routes service notifications: incoming chat messages and typing.
// A notification with local_echo set is our own send coming back: it is
// dropped while the send is still in flight (core has no id to dedupe by
// yet) or when it matches a send this session confirmed, and otherwise
// reported as sent by us (another of our devices). Local echoes never
// touch unread counts.
func (s *socialSession) onService(hdr depot.Msg, body []byte) {
	if string(hdr.Bytes(12)) != "FriendMessagesClient.IncomingMessage#1" {
		return
	}
	m, err := depot.Decode(body)
	if err != nil {
		return
	}
	friend := m.Int(1)
	if friend == 0 {
		return
	}
	entry := m.Int(2)
	fid := strconv.FormatUint(friend, 10)
	if entry == 2 {
		s.emit("typing", map[string]any{"friendId": fid})
		return
	}
	if entry != 1 {
		return
	}
	text := string(m.Bytes(4))
	if plain := m.Bytes(8); len(plain) > 0 {
		text = string(plain) // message_no_bbcode when present
	}
	ts := int64(m.Int(5))
	ord := m.Int(6)
	id := fmt.Sprintf("%d-%d", ts, ord)
	from := fid
	if m.Int(7) != 0 {
		s.mu.Lock()
		_, inFlight := s.inSend[friend]
		_, confirmed := s.sentEcho[fid+"/"+id]
		s.mu.Unlock()
		if inFlight || confirmed {
			return
		}
		from = "self"
	}
	s.emit("message", map[string]any{
		"friendId": fid,
		"message": map[string]any{
			"id": id, "from": from, "text": text, "sentAt": ts * 1000,
		},
	})
}

func avatarURL(hash []byte) string {
	return "https://avatars.cloudflare.steamstatic.com/" + hex.EncodeToString(hash) + "_full.jpg"
}

// selfLocked renders our own profile; callers hold s.mu.
func (s *socialSession) selfLocked() map[string]any {
	m := map[string]any{"name": s.selfName, "state": s.status}
	if m["name"] == "" {
		m["name"] = strconv.FormatUint(s.selfID, 10)
	}
	if s.selfAva != "" {
		m["avatarUrl"] = s.selfAva
	}
	return m
}

// waitReady blocks until the friends list arrived and every known friend
// has persona data (or readyTimeout passes), backfills missing names from
// Player.GetPlayerLinkDetails, and answers the initial snapshot. Later
// calls answer immediately: the first sync is done.
func (s *socialSession) waitReady() (map[string]any, *socialError) {
	s.mu.Lock()
	if s.readyDone {
		snap := map[string]any{
			"connection": map[string]any{"state": connState(s.client.State())},
			"self":       s.selfLocked(),
			"friends":    s.friendsLocked(),
		}
		s.mu.Unlock()
		return snap, nil
	}
	s.mu.Unlock()
	select {
	case <-s.ready:
	case <-time.After(readyTimeout):
	}
	deadline := time.Now().Add(readyTimeout)
	for {
		s.mu.Lock()
		pending := 0
		var ask []uint64
		for id, f := range s.friends {
			if !f.hasData {
				pending++
				if !s.requested[id] {
					ask = append(ask, id)
				}
			}
		}
		if s.selfName == "" && !s.requested[s.selfID] {
			ask = append(ask, s.selfID)
		}
		done := s.gotList && pending == 0
		cl := s.client
		if cl != nil {
			for _, id := range ask {
				s.requested[id] = true
			}
		}
		s.mu.Unlock()
		// The list push batched with the logon may have run before
		// Connect returned (no request went out); ask here.
		if cl != nil && len(ask) > 0 {
			_ = cl.Send(steamcm.EMsgClientRequestFriendData, requestFriendData(ask))
		}
		if done || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	s.backfillNames()
	s.mu.Lock()
	s.readyDone = true
	s.mu.Unlock()
	return s.snapshot(), nil
}

// backfillNames fetches display names for friends (and ourselves) that got
// no persona push, from Player.GetPlayerLinkDetails#1.
func (s *socialSession) backfillNames() {
	s.mu.Lock()
	var missing []uint64
	for id, f := range s.friends {
		if f.name == "" {
			missing = append(missing, id)
		}
	}
	needSelf := s.selfName == ""
	s.mu.Unlock()
	if len(missing) == 0 && !needSelf {
		return
	}
	want := append(append([]uint64{}, missing...), s.selfID)
	e := &depot.Enc{}
	for _, id := range want {
		e.Fixed64(1, id)
	}
	resp, err := s.client.Call("Player.GetPlayerLinkDetails#1", e.Bytes(), 15*time.Second)
	if err != nil {
		return
	}
	m, err := depot.Decode(resp)
	if err != nil {
		return
	}
	names := map[uint64]string{}
	for _, raw := range m.AllBytes(1) {
		im, err := depot.Decode(raw)
		if err != nil || len(im.AllBytes(1)) == 0 {
			continue
		}
		pub, err := depot.Decode(im.AllBytes(1)[0])
		if err != nil {
			continue
		}
		if id := pub.Int(1); id != 0 {
			if name := string(pub.Bytes(10)); name != "" {
				names[id] = name
			}
		}
	}
	if len(names) == 0 {
		return
	}
	s.mu.Lock()
	for id, f := range s.friends {
		if f.name == "" {
			if n, ok := names[id]; ok {
				f.name = n
				f.hasData = true
			}
		}
	}
	if n, ok := names[s.selfID]; ok && s.selfName == "" {
		s.selfName = n
	}
	s.mu.Unlock()
}

// snapshot renders the current connection, self and friends.
func (s *socialSession) snapshot() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]any{
		"connection": map[string]any{"state": connState(s.client.State())},
		"self":       s.selfLocked(),
		"friends":    s.friendsLocked(),
	}
}

// connState maps the session state to the protocol's connection states.
func connState(st steamcm.ClientState) string {
	switch st {
	case steamcm.StateOnline:
		return "online"
	case steamcm.StateReconnecting:
		return "reconnecting"
	case steamcm.StateReplaced:
		return "replaced"
	default:
		return "offline"
	}
}

func (s *socialSession) opSetStatus(raw json.RawMessage) (map[string]any, *socialError) {
	var p struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(raw, &p); err != nil || p.State == "" {
		return nil, &socialError{Code: codeInvalidParams, Message: "invalid params: state is required"}
	}
	if _, ok := stateToPersona[p.State]; !ok {
		return nil, &socialError{Code: codeInvalidParams, Message: fmt.Sprintf("invalid params: state %q is not a status", p.State)}
	}
	if err := s.sendStatus(p.State); err != nil {
		return nil, &socialError{Code: codeStoreError, Message: shortSocialErr(err)}
	}
	s.mu.Lock()
	s.status = p.State
	self := s.selfLocked()
	s.mu.Unlock()
	s.emit("self", self)
	return map[string]any{}, nil
}

func (s *socialSession) opSetPlaying(raw json.RawMessage) (map[string]any, *socialError) {
	var p struct {
		ID *string `json:"id"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &socialError{Code: codeInvalidParams, Message: "invalid params: " + err.Error()}
	}
	var app uint32
	if p.ID != nil {
		n, err := strconv.ParseUint(*p.ID, 10, 32)
		if err != nil {
			return nil, &socialError{Code: codeInvalidParams, Message: fmt.Sprintf("invalid params: id %q is not a game id", *p.ID)}
		}
		app = uint32(n)
	}
	s.mu.Lock()
	s.playingDesired = app
	s.mu.Unlock()
	if err := s.sendPlaying(app, false); err != nil {
		return nil, &socialError{Code: codeStoreError, Message: shortSocialErr(err)}
	}
	return map[string]any{}, nil
}

// parseMsgID splits the "timestamp-ordinal" ids history hands out.
func parseMsgID(id string) (uint64, uint64, bool) {
	ts, ord, ok := strings.Cut(id, "-")
	if !ok {
		return 0, 0, false
	}
	t, err1 := strconv.ParseUint(ts, 10, 32)
	o, err2 := strconv.ParseUint(ord, 10, 32)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return t, o, true
}

func (s *socialSession) knownFriend(id string) (uint64, bool) {
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.friends[n]
	return n, ok
}

func (s *socialSession) opHistory(raw json.RawMessage) (map[string]any, *socialError) {
	var p struct {
		FriendID string `json:"friendId"`
		Before   string `json:"before"`
		Limit    int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &socialError{Code: codeInvalidParams, Message: "invalid params: " + err.Error()}
	}
	friend, ok := s.knownFriend(p.FriendID)
	if !ok {
		return nil, &socialError{Code: codeInvalidParams, Message: fmt.Sprintf("invalid params: no friend %q", p.FriendID)}
	}
	limit := p.Limit
	if limit <= 0 {
		limit = defaultHistoryLimit
	}
	limit = min(limit, chatPageCap)
	e := &depot.Enc{}
	e.Fixed64(1, s.selfID)
	e.Fixed64(2, friend)
	e.Varint(3, uint64(limit))
	if p.Before == "" {
		e.Varint(4, 1) // most_recent_conversation
	} else {
		ts, ord, ok := parseMsgID(p.Before)
		if !ok {
			return nil, &socialError{Code: codeInvalidParams, Message: fmt.Sprintf("invalid params: before %q is not a message id", p.Before)}
		}
		e.Varint(8, ts)  // time_last
		e.Varint(9, ord) // ordinal_last
	}
	resp, err := s.client.Call("FriendMessages.GetRecentMessages#1", e.Bytes(), 30*time.Second)
	if err != nil {
		return nil, socialCallErr(err)
	}
	m, err := depot.Decode(resp)
	if err != nil {
		return nil, &socialError{Code: codeStoreError, Message: "Steam sent a history Platypus can't read"}
	}
	type msg struct {
		ts   uint64
		ord  uint64
		body map[string]any
	}
	var msgs []msg
	for _, r := range m.AllBytes(1) {
		fm, err := depot.Decode(r)
		if err != nil {
			continue
		}
		ts, ord := fm.Int(2), fm.Int(4)
		from := p.FriendID
		if fm.Int(1) == uint64(uint32(s.selfID)) {
			from = "self"
		}
		msgs = append(msgs, msg{ts: ts, ord: ord, body: map[string]any{
			"id":     fmt.Sprintf("%d-%d", ts, ord),
			"from":   from,
			"text":   string(fm.Bytes(3)),
			"sentAt": int64(ts) * 1000,
		}})
	}
	sort.Slice(msgs, func(i, j int) bool {
		if msgs[i].ts != msgs[j].ts {
			return msgs[i].ts < msgs[j].ts
		}
		return msgs[i].ord < msgs[j].ord
	})
	out := []map[string]any{}
	for _, mm := range msgs {
		out = append(out, mm.body)
	}
	return map[string]any{"messages": out, "more": m.Int(4) != 0}, nil
}

func (s *socialSession) opSend(raw json.RawMessage) (map[string]any, *socialError) {
	var p struct {
		FriendID string `json:"friendId"`
		Text     string `json:"text"`
		ClientID string `json:"clientId"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &socialError{Code: codeInvalidParams, Message: "invalid params: " + err.Error()}
	}
	friend, ok := s.knownFriend(p.FriendID)
	if !ok {
		return nil, &socialError{Code: codeInvalidParams, Message: fmt.Sprintf("invalid params: no friend %q", p.FriendID)}
	}
	if p.ClientID == "" {
		return nil, &socialError{Code: codeInvalidParams, Message: "invalid params: clientId is required"}
	}
	if p.Text == "" || len([]rune(p.Text)) > 4096 {
		return nil, &socialError{Code: codeInvalidParams, Message: "invalid params: text must be 1 to 4096 characters"}
	}
	s.mu.Lock()
	if first, ok := s.sent[p.ClientID]; ok {
		s.mu.Unlock()
		return map[string]any{"id": first.id, "sentAt": first.sentAt}, nil
	}
	// Bound the dedupe tables; a clientId older than the cap sends twice.
	if len(s.sent) > 2000 {
		s.sent = map[string]sentMsg{}
		s.sentEcho = map[string]bool{}
	}
	s.inSend[friend]++
	s.mu.Unlock()
	e := &depot.Enc{}
	e.Fixed64(1, friend)
	e.Varint(2, 1) // ChatMsg
	e.Str(3, p.Text)
	e.Varint(5, 1) // echo_to_sender
	e.Str(8, p.ClientID)
	resp, err := s.client.Call("FriendMessages.SendMessage#1", e.Bytes(), 30*time.Second)
	s.mu.Lock()
	if n := s.inSend[friend] - 1; n <= 0 {
		delete(s.inSend, friend)
	} else {
		s.inSend[friend] = n
	}
	s.mu.Unlock()
	if err != nil {
		return nil, socialCallErr(err)
	}
	m, err := depot.Decode(resp)
	if err != nil {
		return nil, &socialError{Code: codeStoreError, Message: "Steam sent a send reply Platypus can't read"}
	}
	ts, ord := m.Int(2), m.Int(3)
	id := fmt.Sprintf("%d-%d", ts, ord)
	sentAt := int64(ts) * 1000
	s.mu.Lock()
	s.sent[p.ClientID] = sentMsg{id: id, sentAt: sentAt}
	s.sentEcho[p.FriendID+"/"+id] = true
	s.mu.Unlock()
	return map[string]any{"id": id, "sentAt": sentAt}, nil
}

func (s *socialSession) opMarkRead(raw json.RawMessage) (map[string]any, *socialError) {
	var p struct {
		FriendID string `json:"friendId"`
		UpTo     string `json:"upTo"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, &socialError{Code: codeInvalidParams, Message: "invalid params: " + err.Error()}
	}
	friend, ok := s.knownFriend(p.FriendID)
	if !ok {
		return nil, &socialError{Code: codeInvalidParams, Message: fmt.Sprintf("invalid params: no friend %q", p.FriendID)}
	}
	ts, _, ok := parseMsgID(p.UpTo)
	if !ok {
		return nil, &socialError{Code: codeInvalidParams, Message: fmt.Sprintf("invalid params: upTo %q is not a message id", p.UpTo)}
	}
	e := &depot.Enc{}
	e.Fixed64(1, friend)
	e.Varint(2, ts)
	// AckMessage returns NoResponse: fire and forget, never wait.
	if err := s.client.Notify("FriendMessages.AckMessage#1", e.Bytes()); err != nil {
		return nil, &socialError{Code: codeStoreError, Message: shortSocialErr(err)}
	}
	return map[string]any{}, nil
}

func (s *socialSession) opUnreadCounts() (map[string]any, *socialError) {
	e := &depot.Enc{}
	e.Varint(2, 1) // only_sessions_with_messages
	resp, err := s.client.Call("FriendMessages.GetActiveMessageSessions#1", e.Bytes(), 30*time.Second)
	if err != nil {
		return nil, socialCallErr(err)
	}
	m, err := depot.Decode(resp)
	if err != nil {
		return nil, &socialError{Code: codeStoreError, Message: "Steam sent unread counts Platypus can't read"}
	}
	type row struct {
		fid string
		n   int
	}
	var rows []row
	for _, r := range m.AllBytes(1) {
		sm, err := depot.Decode(r)
		if err != nil {
			continue
		}
		acct := uint32(sm.Int(1))
		if acct == 0 {
			continue
		}
		// Individual SteamIDs rebuild from the account id.
		fid := strconv.FormatUint(0x0110000100000000|uint64(acct), 10)
		rows = append(rows, row{fid: fid, n: int(sm.Int(4))})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].fid < rows[j].fid })
	out := []map[string]any{}
	for _, r := range rows {
		out = append(out, map[string]any{"friendId": r.fid, "unread": r.n})
	}
	return map[string]any{"counts": out}, nil
}

// callErr maps a CM call failure to a helper error: refused tokens become
// auth-required, everything else a store error with Steam's message.
func socialCallErr(err error) *socialError {
	if ce, ok := err.(*steamcm.CMError); ok && ce.Auth {
		return &socialError{Code: codeAuthRequired, Message: "log in to Steam first (the stored login was rejected)"}
	}
	return &socialError{Code: codeStoreError, Message: shortSocialErr(err)}
}
