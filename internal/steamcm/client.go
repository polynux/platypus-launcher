// Persistent CM client: a long-lived logon session next to the one-shot
// Session. One reader goroutine owns the socket; service replies route
// by jobid_target, pushes go to a handler registry, client-initiated
// heartbeats go out at the logon response's heartbeat_seconds, and a
// dropped socket reconnects with backoff + jitter. LogonSessionReplaced
// (34) and LoggedInElsewhere (6) end the loop with State "replaced"
// instead of fighting the other session (plan D8, research §3.2).
//
// The wire shapes (framing, logon, service calls) are the ones Session
// uses; Call keeps Session.Call's signature so depot and auth code
// stay untouched.
package steamcm

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"hydra-launcher/internal/depot"
)

// ClientState mirrors social.connection's states (plan, social.*).
type ClientState int

const (
	StateConnecting ClientState = iota
	StateOnline
	StateReconnecting
	StateReplaced
	StateOffline
)

func (s ClientState) String() string {
	switch s {
	case StateConnecting:
		return "connecting"
	case StateOnline:
		return "online"
	case StateReconnecting:
		return "reconnecting"
	case StateReplaced:
		return "replaced"
	default:
		return "offline"
	}
}

// ClientConfig configures a persistent Client.
type ClientConfig struct {
	RefreshToken string
	// Role picks the logon id (LoginIDFor); the zero value is the
	// per-process random one-shot id, which a persistent session must
	// not use — set Social/Cloud/Content explicitly.
	Role LoginRole
	// MachineID is the machine_id blob; StableMachineID(dataDir) builds
	// the per-install one. Empty = a random id (not recommended for
	// persistent sessions).
	MachineID []byte
	// Backoff base and cap for reconnect waits; defaults 500ms/16s.
	// Tests shrink these.
	BackoffBase, BackoffMax time.Duration
	// CallTimeout is Call's default when timeout == 0 (30s, like
	// Session.Call).
	CallTimeout time.Duration
}

// Client is one persistent logon session to one CM. All methods are
// safe for concurrent use.
type Client struct {
	cfg    ClientConfig
	role   LoginRole
	loginID uint32
	steamID uint64

	mu        sync.Mutex
	state     ClientState
	handlers  map[uint32][]func(depot.Msg, []byte)
	pending   map[uint64]chan callResult
	sessionID int32
	jobID     uint64
	conn      *websocket.Conn
	writeMu   sync.Mutex // one writer at a time (gorilla/websocket)
	// hbSeconds is the cadence the logon response asked for.
	hbSeconds uint64
	// stop closes the run loop; doneWaiters fire once it ends.
	stop     chan struct{}
	doneOnce sync.Once
	done     chan struct{}
	lastErr  error
}

type callResult struct {
	body    []byte
	eresult int
	errMsg  string
	err     error
}

// PushHandler registers fn for every push with the given EMsg. Register
// before Connect; handlers run on the reader goroutine and must not
// block (copy what they need and return).
func (c *Client) On(emsg uint32, fn func(hdr depot.Msg, body []byte)) {
	c.mu.Lock()
	c.handlers[emsg] = append(c.handlers[emsg], fn)
	c.mu.Unlock()
}

// State reports the current connection state.
func (c *Client) State() ClientState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// Err returns the error that ended the session (nil while running).
func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastErr
}

// Connect dials, logs on and starts the reader, heartbeat and reconnect
// goroutines. It returns once the first logon response arrived: nil
// means online, an error means the first logon failed (including
// replaced, whose CMError carries EResult 34/6).
func Connect(ctx context.Context, cfg ClientConfig) (*Client, error) {
	steamID, err := SteamIDFromToken(cfg.RefreshToken)
	if err != nil {
		return nil, err
	}
	if cfg.BackoffBase <= 0 {
		cfg.BackoffBase = 500 * time.Millisecond
	}
	if cfg.BackoffMax <= 0 {
		cfg.BackoffMax = 16 * time.Second
	}
	if cfg.CallTimeout <= 0 {
		cfg.CallTimeout = 30 * time.Second
	}
	c := &Client{
		cfg:      cfg,
		role:     cfg.Role,
		loginID:  LoginIDFor(cfg.Role),
		steamID:  steamID,
		handlers: map[uint32][]func(depot.Msg, []byte){},
		pending:  map[uint64]chan callResult{},
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	conn, sessionID, hbSeconds, err := c.dialAndLogon(ctx)
	if err != nil {
		c.mu.Lock()
		c.state = StateOffline
		c.lastErr = err
		c.mu.Unlock()
		return nil, err
	}
	c.mu.Lock()
	c.conn = conn
	c.sessionID = sessionID
	c.hbSeconds = hbSeconds
	c.state = StateOnline
	c.mu.Unlock()
	go c.heartbeatLoop()
	go c.run()
	return c, nil
}

// dialAndLogon does one hello+logon round trip and returns the live
// connection, the session id and the heartbeat cadence.
func (c *Client) dialAndLogon(ctx context.Context) (*websocket.Conn, int32, uint64, error) {
	servers, tls, err := cmCandidates(ctx)
	if err != nil {
		return nil, 0, 0, err
	}
	var errs []string
	for _, endpoint := range servers {
		scheme := "wss"
		if !tls {
			scheme = "ws"
		}
		u := scheme + "://" + endpoint + "/cmsocket/"
		conn, err := dialWebsocket(ctx, u)
		if err != nil {
			errs = append(errs, endpoint+": "+err.Error())
			continue
		}
		sessionID, hb, lerr := c.logonOn(conn, cfgToken(c.cfg))
		if lerr != nil {
			conn.Close()
			if ce, ok := lerr.(*CMError); ok && ce.EResult != 0 {
				return nil, 0, 0, lerr // the CM answered: its result is the answer
			}
			errs = append(errs, endpoint+": "+lerr.Error())
			continue
		}
		return conn, sessionID, hb, nil
	}
	if len(errs) == 0 {
		errs = []string{"no servers"}
	}
	msg := ""
	for i, e := range errs {
		if i >= 3 {
			break
		}
		if i > 0 {
			msg += "; "
		}
		msg += e
	}
	return nil, 0, 0, &CMError{Msg: "couldn't reach any Steam CM server (" + msg + ")"}
}

func cfgToken(cfg ClientConfig) string { return cfg.RefreshToken }

// logonOn sends hello+logon on conn and waits for the logon response,
// absorbing pushes the way Session.logon does.
func (c *Client) logonOn(conn *websocket.Conn, refreshToken string) (int32, uint64, error) {
	if err := conn.WriteMessage(websocket.BinaryMessage,
		packEMsg(EMsgClientHello, header(0, 0, 0, ""), func() []byte {
			e := &depot.Enc{}
			e.Varint(1, ProtocolVersion)
			return e.Bytes()
		}())); err != nil {
		return 0, 0, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	machine := c.cfg.MachineID
	if len(machine) == 0 {
		machine = randomMachineID()
	}
	if err := conn.WriteMessage(websocket.BinaryMessage,
		packEMsg(EMsgClientLogon, header(c.steamID, 0, 0, ""),
			logonBody(refreshToken, c.loginID, machine))); err != nil {
		return 0, 0, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			return 0, 0, &CMError{Msg: "Steam didn't answer the CM logon within 30 seconds"}
		}
		if err := conn.SetReadDeadline(deadline); err != nil {
			return 0, 0, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
		}
		op, data, err := conn.ReadMessage()
		if err != nil {
			return 0, 0, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
		}
		if op != websocket.BinaryMessage {
			continue
		}
		emsg, hdr, body, err := unpackEMsg(data)
		if err != nil {
			return 0, 0, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
		}
		if emsg == EMsgMulti {
			inner, err := multiUnwrap(body)
			if err != nil {
				return 0, 0, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
			}
			for _, m := range inner {
				iemsg, ihdr, ibody, err := unpackEMsg(m)
				if err != nil {
					return 0, 0, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
				}
				if iemsg == 751 {
					sid, hb, lerr := parseLogonResponse(ihdr, ibody)
					if lerr != nil {
						return 0, 0, lerr
					}
					return sid, hb, nil
				}
			}
			continue
		}
		if emsg == 751 {
			sid, hb, lerr := parseLogonResponse(hdr, body)
			if lerr != nil {
				return 0, 0, lerr
			}
			return sid, hb, nil
		}
		// Anything else before the response (licence list, ...): ignore
		// here; the server resends what matters after logon.
	}
}

func parseLogonResponse(hdr depot.Msg, body []byte) (int32, uint64, error) {
	m, err := depot.Decode(body)
	if err != nil {
		return 0, 0, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	eresult := int(m.Int(1))
	if eresult == 2 {
		if h := hdr.Int(13); h != 0 {
			eresult = int(h) // some CMs only set the header eresult
		}
	}
	sessionID := int32(hdr.Int(2))
	if eresult != 1 {
		suffix := ""
		auth := tokenRejected(eresult)
		if !auth {
			suffix = " while logging on to its CM server"
		}
		return 0, 0, &CMError{
			Msg:     "Steam rejected the login with " + EResultName(eresult) + suffix,
			EResult: eresult, Auth: auth,
		}
	}
	return sessionID, m.Int(3), nil
}

// run owns the connection for its whole life: reader loop, heartbeat
// ticker, reconnects. It exits when Stop is called or the session is
// replaced.
func (c *Client) run() {
	defer c.doneOnce.Do(func() { close(c.done) })
	backoff := c.cfg.BackoffBase
	for {
		c.readLoop()

		c.mu.Lock()
		st := c.state
		c.mu.Unlock()
		if st == StateReplaced {
			return
		}
		select {
		case <-c.stop:
			c.setState(StateOffline)
			return
		default:
		}

		// Fail every in-flight call: its reply died with the socket.
		c.failPending(&CMError{Msg: "Steam CM connection dropped"})

		c.setState(StateReconnecting)
		wait := backoff + time.Duration(rand.Int63n(int64(backoff)/2+1)) // + jitter [0, 50%)
		c.mu.Lock()
		c.lastErr = nil
		c.mu.Unlock()
		select {
		case <-time.After(wait):
		case <-c.stop:
			c.setState(StateOffline)
			return
		}
		backoff *= 2
		if backoff > c.cfg.BackoffMax {
			backoff = c.cfg.BackoffMax
		}

		conn, sessionID, hb, err := c.dialAndLogon(context.Background())
		if err != nil {
			c.mu.Lock()
			c.lastErr = err
			c.mu.Unlock()
			// Replaced while reconnecting: give up for good.
			if ce, ok := err.(*CMError); ok && logonTaken(ce.EResult) {
				c.setState(StateReplaced)
				return
			}
			continue
		}
		c.mu.Lock()
		c.conn = conn
		c.sessionID = sessionID
		c.hbSeconds = hb
		c.state = StateOnline
		c.mu.Unlock()
		backoff = c.cfg.BackoffBase
	}
}

func (c *Client) setState(s ClientState) {
	c.mu.Lock()
	c.state = s
	c.mu.Unlock()
}

// readLoop reads until the socket dies, routing messages: replies by
// jobid_target, pushes to the handler registry, heartbeats silently.
func (c *Client) readLoop() {
	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return
		}
		_ = conn.SetReadDeadline(time.Time{})
		op, data, err := conn.ReadMessage()
		if err != nil {
			return // socket dead: reconnect logic takes over
		}
		if op != websocket.BinaryMessage {
			continue
		}
		msgs := [][]byte{data}
		for len(msgs) > 0 {
			emsg, hdr, body, err := unpackEMsg(msgs[0])
			msgs = msgs[1:]
			if err != nil {
				return
			}
			if emsg == EMsgMulti {
				inner, err := multiUnwrap(body)
				if err != nil {
					return
				}
				msgs = append(inner, msgs...)
				continue
			}
			c.route(emsg, hdr, body)
		}
	}
}

// route sends one message where it belongs.
func (c *Client) route(emsg uint32, hdr depot.Msg, body []byte) {
	switch emsg {
	case EMsgClientHeartBeat:
		return // nothing to answer; the client sends its own
	case EMsgClientLoggedOff:
		// 757 with eresult 6 (LoggedInElsewhere) or 34: another session
		// took this slot. Back off; never fight (research §3.2).
		if m, err := depot.Decode(body); err == nil {
			if logonTaken(int(m.Int(1))) {
				c.mu.Lock()
				c.state = StateReplaced
				c.lastErr = &CMError{Msg: "Steam replaced this session: " + EResultName(int(m.Int(1))), EResult: int(m.Int(1))}
				c.mu.Unlock()
				c.failPending(&CMError{Msg: "Steam replaced this session", EResult: int(m.Int(1))})
				c.closeConn()
			}
		}
		return
	}
	// Service reply? Match by jobid_target.
	if job := hdr.Int(11); job != 0 {
		c.mu.Lock()
		ch := c.pending[job]
		if ch != nil {
			delete(c.pending, job)
		}
		c.mu.Unlock()
		if ch != nil {
			eresult := int(hdr.Int(13))
			if eresult == 0 {
				eresult = 1 // the fake CM omits it on success; real CMs set 1
			}
			res := callResult{body: body, eresult: eresult}
			if eresult != 1 {
				res.errMsg = ""
				if e := hdr.Bytes(14); len(e) > 0 {
					res.errMsg = ": " + string(e)
				}
			}
			ch <- res
			return
		}
	}
	// Otherwise: a push. Handlers, then forget.
	c.mu.Lock()
	handlers := append([]func(depot.Msg, []byte){}, c.handlers[emsg]...)
	c.mu.Unlock()
	for _, fn := range handlers {
		fn(hdr, body)
	}
}

func (c *Client) failPending(err error) {
	c.mu.Lock()
	pending := c.pending
	c.pending = map[uint64]chan callResult{}
	c.mu.Unlock()
	for _, ch := range pending {
		ch <- callResult{err: err}
	}
}

func (c *Client) closeConn() {
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

// heartbeatLoop sends ClientHeartBeat every hb seconds until stop; the
// cadence comes from the logon response's heartbeat_seconds.
func (c *Client) heartbeatLoop() {
	for {
		c.mu.Lock()
		hb := c.hbSeconds
		c.mu.Unlock()
		if hb == 0 {
			hb = 5
		}
		select {
		case <-time.After(time.Duration(hb) * time.Second):
		case <-c.stop:
			return
		case <-c.done:
			return
		}
		c.mu.Lock()
		conn := c.conn
		state := c.state
		c.mu.Unlock()
		if conn == nil || state != StateOnline {
			continue
		}
		c.writeMu.Lock()
		err := conn.WriteMessage(websocket.BinaryMessage,
			packEMsg(EMsgClientHeartBeat, header(c.steamID, c.sessionID, 0, ""), nil))
		c.writeMu.Unlock()
		_ = err
	}
}

// Call makes one ServiceMethodCallFromClient request and returns the
// response body — same shape and signature as Session.Call. Replies
// match by jobid_target; pushes that arrive meanwhile are routed to
// handlers, never swallowed.
func (c *Client) Call(service string, body []byte, timeout time.Duration) ([]byte, error) {
	if timeout == 0 {
		timeout = c.cfg.CallTimeout
	}
	c.mu.Lock()
	if c.state != StateOnline || c.conn == nil {
		st := c.state
		c.mu.Unlock()
		return nil, &CMError{Msg: "Steam session is " + st.String() + "; not sending " + service}
	}
	c.jobID++
	job := c.jobID
	hdr := header(c.steamID, c.sessionID, job, service)
	ch := make(chan callResult, 1)
	c.pending[job] = ch
	conn := c.conn
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, job)
		c.mu.Unlock()
	}()

	c.writeMu.Lock()
	err := conn.WriteMessage(websocket.BinaryMessage,
		packEMsg(EMsgServiceMethodCallFromClient, hdr, body))
	c.writeMu.Unlock()
	if err != nil {
		return nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case res := <-ch:
		if res.err != nil {
			return nil, res.err
		}
		if res.eresult != 1 {
			return nil, &CMError{
				Msg:     "Steam rejected the request with " + EResultName(res.eresult) + res.errMsg,
				EResult: res.eresult, Auth: tokenRejected(res.eresult),
			}
		}
		return res.body, nil
	case <-timer.C:
		return nil, &CMError{Msg: fmt.Sprintf("Steam didn't answer %s within %v", service, timeout)}
	case <-c.stop:
		return nil, &CMError{Msg: "Steam session closed while calling " + service}
	}
}

// Close logs off and ends the session. Idempotent.
func (c *Client) Close() {
	c.doneOnce.Do(func() { close(c.done) })
	c.mu.Lock()
	if c.stopped() {
		c.mu.Unlock()
		return
	}
	close(c.stop)
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	if conn != nil {
		c.writeMu.Lock()
		err := conn.WriteMessage(websocket.BinaryMessage,
			packEMsg(EMsgClientLogOff, header(c.steamID, c.sessionID, 0, ""), nil))
		c.writeMu.Unlock()
		_ = err
		_ = conn.Close()
	}
	c.failPending(&CMError{Msg: "Steam session closed"})
	<-c.done
}

func (c *Client) stopped() bool {
	select {
	case <-c.stop:
		return true
	default:
		return false
	}
}
