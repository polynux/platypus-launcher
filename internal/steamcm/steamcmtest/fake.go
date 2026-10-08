// Package steamcmtest is the importable fake Steam CM for tests that
// drive persistent sessions: the Go fake for internal/steamcm's Client
// (and anything else that wants a CM on localhost), with the pushes and
// kicks phase 0 of the cloud/social plan needs. It grew out of the
// fakeCM in internal/steamcm's test files; knobs here focus on
// long-lived sessions: heartbeats from the logon response, pushed
// messages interleaved with service replies, a dropped socket for
// reconnect tests and duplicate-logon-id kicks (34/6).
//
// No Valve network: everything runs on httptest over plain ws://.
package steamcmtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"hydra-launcher/internal/depot"
	"hydra-launcher/internal/steamcm"
)

// Push is one unsolicited message the fake can send a client.
type Push struct {
	EMsg uint32
	Body []byte
	// After counts service calls: 0 = right after the logon response,
	// 1 = after the first service request is received (the push goes
	// out before that call's reply, so it lands between the client's
	// request and the reply it waits for), 2 = after the second, ...
	After int
}

// Config holds the fake's knobs; zero values give a plain session.
type Config struct {
	// LogonEresult is the EResult of the logon response (0 = 1/OK).
	LogonEresult int
	// HeartbeatSeconds goes into the logon response (0 = 5, like the
	// real CM's ballpark).
	HeartbeatSeconds uint64
	// SendServerHeartbeat sends one CM-initiated ClientHeartBeat (703)
	// between the logon response and the first service reply, like the
	// existing fake's send_heartbeat knob.
	SendServerHeartbeat bool
	// DuplicateKick, when true, kicks an existing session that logs on
	// again with the same login id: the old connection gets
	// ClientLoggedOff with EResult 34 and the socket closes, like the
	// real CM's LogonSessionReplaced.
	DuplicateKick bool
	// DropAfterLogon closes the socket abruptly right after a
	// successful logon response (no close frame): a dropped connection
	// for reconnect tests. It fires once per Fake — the next logon
	// succeeds, so the client's reconnect can be observed end to end.
	DropAfterLogon bool
	// Kick sends ClientLoggedOff with this EResult (34 or 6) after
	// KickAfter service calls, then closes. 0 = never.
	Kick int
	// KickAfter counts service calls before the kick (0 = immediately
	// after logon).
	KickAfter int
	// RetryLogonEresult, when set, rejects every logon with this
	// EResult (response eresult and a close), like a CM refusing a
	// revoked/expired token. Set between test steps.
	RetryLogonEresult int
	// LogonDelay makes the fake sleep before answering a logon
	// (response or rejection), so a test can drop the socket or Close()
	// while the reconnect logon is still in flight. Applies to every
	// logon on this fake, including Connect's first one — tests that
	// need a fast first logon must call SetConfig right after Connect
	// returns.
	LogonDelay time.Duration
	// Pushes is the scripted push queue (friends list, persona state,
	// playing session state, cloud change notification, ...).
	Pushes []Push
	// ServiceEresult is the EResult stamped on service replies (0 = 1).
	ServiceEresult int
}

// Fake is one fake CM server: multiple client connections can be
// served at once, sharing the login-id registry used for duplicate
// kicks.
type Fake struct {
	t   *testing.T
	cfg Config

	mu     sync.Mutex
	events []string
	logins loginRegistry // live connections per login id
	srv    *httptest.Server
}

// New starts the fake and registers its cleanup; the return value is
// the "host:port" the client dials (HYDRA_STEAM_CM_SERVER shape).
func New(t *testing.T, cfg Config) *Fake {
	t.Helper()
	f := &Fake{t: t, cfg: cfg}
	up := websocket.Upgrader{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		f.serve(&fakeConn{Conn: c})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

// Host is the "host:port" endpoint.
func (f *Fake) Host() string {
	return f.srv.URL[len("http://"):]
}

// SetConfig swaps knobs between test steps.
func (f *Fake) SetConfig(fn func(*Config)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(&f.cfg)
}

func (f *Fake) record(ev string) {
	f.mu.Lock()
	f.events = append(f.events, ev)
	f.mu.Unlock()
}

// Events returns a copy of everything recorded so far.
func (f *Fake) Events() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.events...)
}

// EventCount is the number of recorded events.
func (f *Fake) EventCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.events)
}

// WaitFor polls until pred over the event list holds or the timeout
// passes, so tests do not sleep for fixed durations.
func (f *Fake) WaitFor(timeout time.Duration, pred func([]string) bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if pred(f.Events()) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// logonBody fields (CMsgClientLogon): obfuscated_private_ip = 11 (a
// CMsgIPAddress whose v4 = 1) and the deprecated plain field 2.
func loginIDOf(body []byte) uint32 {
	m, err := depot.Decode(body)
	if err != nil {
		return 0
	}
	if id := m.Int(2); id != 0 {
		return uint32(id)
	}
	if obf := m.Bytes(11); len(obf) > 0 {
		if om, err := depot.Decode(obf); err == nil {
			return uint32(om.Int(1))
		}
	}
	return 0
}

// tokenBody mints an access-token reply for GenerateAccessTokenForApp,
// like the existing fakes, so Call works without extra configuration.
func tokenBody(sub uint64) []byte {
	if sub == 0 {
		sub = 76561198
	}
	e := &depot.Enc{}
	e.Str(1, fakeToken(sub, 3600))
	return e.Bytes()
}

// serve handles one connection, mirroring the real CM: hello, logon
// (with duplicate-id kick), logon response, then the message loop.
func (f *Fake) serve(c *fakeConn) {
	f.record("connect")
	cfg := f.snapshot()

	var logonHdr depot.Msg
	var logonBody []byte
	for {
		emsg, hdr, body, err := readOne(c)
		if err != nil {
			return
		}
		switch emsg {
		case steamcm.EMsgClientHello:
			continue
		case steamcm.EMsgServiceMethodCallNonAuthed:
			// Unauthenticated Authentication calls (login flow), like
			// the real pre-logon surface. Answered with a token reply.
			target := string(hdr.Bytes(12))
			f.record("service_unauth " + target)
			job := hdr.Int(10)
			c.write(steamcm.PackEMsg(
				steamcm.EMsgServiceMethodResponse,
				steamcm.HeaderReply(hdr.Int(1), int32(hdr.Int(2)), job, 1),
				tokenBody(hdr.Int(1))))
			continue
		case steamcm.EMsgServiceMethodCallFromClient:
			f.record("rejected 151 before logon")
			_ = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(1000, ""))
			return
		case steamcm.EMsgClientLogOff:
			f.record("logoff")
			return
		case steamcm.EMsgClientLogon:
			logonHdr = hdr
			logonBody = body
		default:
			f.record(fmt.Sprintf("unexpected %d", emsg))
			continue
		}
		break
	}

	steamID := logonHdr.Int(1)
	loginID := loginIDOf(logonBody)
	f.record(fmt.Sprintf("logon %d loginid=%d", steamID, loginID))

	if cfg.LogonDelay > 0 {
		time.Sleep(cfg.LogonDelay)
		cfg = f.snapshot() // knobs may have changed while we slept
	}

	eres := cfg.LogonEresult
	if eres == 0 {
		eres = 1
	}
	if cfg.RetryLogonEresult != 0 {
		// Token refused on this attempt (revoked, expired, ...).
		eres = cfg.RetryLogonEresult
	}

	// Duplicate logon id: the newcomer takes the slot; the old
	// connection is kicked with ClientLoggedOff (34) and closed.
	if eres == 1 {
		if victim := f.logins.claim(loginID, c); victim != nil && cfg.DuplicateKick {
			f.record(fmt.Sprintf("kick 34 loginid=%d", loginID))
			off := &depot.Enc{}
			off.Varint(1, 34)
			hdr := &depot.Enc{}
			hdr.Fixed64(1, steamID)
			hdr.Varint(2, 4711)
			victim.write(steamcm.PackEMsg(steamcm.EMsgClientLoggedOff, hdr.Bytes(), off.Bytes()))
			victim.close()
		}
	}
	defer f.logins.release(loginID, c)

	sessHdr := &depot.Enc{}
	sessHdr.Fixed64(1, steamID)
	sessHdr.Varint(2, 4711)

	resp := &depot.Enc{}
	resp.Varint(1, uint64(eres))
	hb := cfg.HeartbeatSeconds
	if hb == 0 {
		hb = 5
	}
	resp.Varint(3, hb) // heartbeat_seconds
	c.write(steamcm.PackEMsg(751, sessHdr.Bytes(), resp.Bytes()))
	if eres != 1 {
		_ = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(1000, ""))
		return
	}

	if cfg.DropAfterLogon && f.takeDrop() {
		// Abrupt drop: no close frame, like a dead socket. Fires once.
		return
	}

	// Pushes scheduled for "after logon" go now.
	serviceCalls := 0
	f.sendDue(c, cfg, 0, steamID)
	if cfg.SendServerHeartbeat {
		c.write(steamcm.PackEMsg(steamcm.EMsgClientHeartBeat, sessHdr.Bytes(), nil))
	}

	for {
		emsg, hdr, body, err := readOne(c)
		if err != nil {
			return
		}
		switch emsg {
		case steamcm.EMsgClientHeartBeat:
			// A client-initiated heartbeat: record it; no reply comes
			// from the real CM for these.
			f.record("heartbeat")
		case steamcm.EMsgClientLogOff:
			f.record("logoff")
			off := &depot.Enc{}
			off.Varint(1, 1)
			c.write(steamcm.PackEMsg(steamcm.EMsgClientLoggedOff, sessHdr.Bytes(), off.Bytes()))
			return
		case steamcm.EMsgServiceMethodCallFromClient:
			serviceCalls++
			target := string(hdr.Bytes(12))
			job := hdr.Int(10)
			f.record("service " + target)
			// Pushes scheduled for this call go out BEFORE the reply,
			// so they land between the client's request and the reply.
			f.sendDue(c, cfg, serviceCalls, steamID)
			eres := cfg.ServiceEresult
			if eres == 0 {
				eres = 1
			}
			replyBody := tokenBody(hdr.Int(1))
			_ = body
			c.write(steamcm.PackEMsg(
				steamcm.EMsgServiceMethodResponse,
				steamcm.HeaderReply(steamID, int32(hdr.Int(2)), job, eres),
				replyBody))
			if cfg.Kick != 0 && serviceCalls >= cfg.KickAfter {
				f.record(fmt.Sprintf("kick %d loginid=%d", cfg.Kick, loginID))
				off := &depot.Enc{}
				off.Varint(1, uint64(cfg.Kick))
				c.write(steamcm.PackEMsg(steamcm.EMsgClientLoggedOff, sessHdr.Bytes(), off.Bytes()))
				c.write(websocket.FormatCloseMessage(1000, ""))
				return
			}
		default:
			f.record(fmt.Sprintf("unexpected %d", emsg))
		}
	}
}

// sendDue sends every push whose After matches count.
func (f *Fake) sendDue(c *fakeConn, cfg Config, count int, steamID uint64) {
	for _, p := range cfg.Pushes {
		if p.After != count {
			continue
		}
		f.record(fmt.Sprintf("push %d", p.EMsg))
		c.write(steamcm.PackEMsg(p.EMsg, steamcm.Header(steamID, 4711, 0, ""), p.Body))
	}
}

func (f *Fake) snapshot() Config {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cfg
}

// takeDrop consumes the one-shot DropAfterLogon.
func (f *Fake) takeDrop() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cfg.DropAfterLogon {
		f.cfg.DropAfterLogon = false
		return true
	}
	return false
}

// readOne reads one binary message and unwraps Multi envelopes into a
// queue per connection.
func readOne(c *fakeConn) (uint32, depot.Msg, []byte, error) {
	for {
		op, data, err := c.ReadMessage()
		if err != nil {
			return 0, nil, nil, err
		}
		if op != websocket.BinaryMessage {
			continue
		}
		emsg, hdr, body, err := steamcm.UnpackEMsg(data)
		if err != nil {
			return 0, nil, nil, err
		}
		if emsg == steamcm.EMsgMulti {
			continue // the persistent fake doesn't need Multi replay
		}
		return emsg, hdr, body, nil
	}
}

// fakeToken mints a JWT-shaped access token (only sub/exp matter).
func fakeToken(steamID uint64, ttl int) string {
	now := time.Now().Unix()
	seg := func(v any) string {
		raw, _ := json.Marshal(v)
		return base64Encode(raw)
	}
	return seg(map[string]string{"typ": "JWT", "alg": "EdDSA"}) + "." +
		seg(map[string]any{"iss": "steam", "sub": fmt.Sprint(steamID), "aud": []string{"web"},
			"exp": now + int64(ttl), "iat": now, "nbf": now}) +
		"." + seg(map[string]bool{"sig": true})
}
