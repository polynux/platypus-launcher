// CM session: connects, sends ClientHello+ClientLogon, then serves
// Call requests until Close. Responses can arrive wrapped in Multi (gzip)
// envelopes; incoming heartbeats are answered inline.
package steamcm

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"platypus/internal/depot"
)

// Session is one logon session to one CM.
type Session struct {
	steamID   uint64
	sessionID int32
	jobID     uint64
	loginID   uint32
	conn      *websocket.Conn
	licenses  []License
	pending   [][]byte   // batched messages waiting for the next read
	mu        sync.Mutex // serialises concurrent service calls (STEAM-5)
	// authed is false before any logon (DialUnauth): service calls go
	// out as EMsg 9804 with no steamid/sessionid, like the real
	// client's pre-logon calls.
	authed bool
}

// License is one entry of CMsgClientLicenseList.
type License struct {
	PackageID uint32
}

// Licenses returns the licence list the CM sent after logon.
func (s *Session) Licenses() []License { return s.licenses }

// cmCandidates lists WebSocket CM endpoints: the PLATYPUS_STEAM_CM_SERVER
// test hook ("host:port", plain ws) or Valve's directory (best load first).
func cmCandidates(ctx context.Context) ([]string, bool, error) {
	if hook := os.Getenv("PLATYPUS_STEAM_CM_SERVER"); hook != "" {
		return []string{hook}, false, nil
	}
	req, _ := http.NewRequestWithContext(ctx, "GET",
		"https://api.steampowered.com/ISteamDirectory/GetCMListForConnect/v1/?cellid=0&maxcount=20&cmtype=websockets", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, &CMError{Msg: fmt.Sprintf("couldn't reach any Steam CM server (%v)", err)}
	}
	defer resp.Body.Close()
	var doc struct {
		Response struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
			Servers []struct {
				Endpoint string `json:"endpoint"`
				Type     string `json:"type"`
				Realm    string `json:"realm"`
			} `json:"serverlist"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, false, &CMError{Msg: fmt.Sprintf("couldn't reach any Steam CM server (%v)", err)}
	}
	if !doc.Response.Success {
		return nil, false, &CMError{Msg: fmt.Sprintf("Steam's CM list request failed: %s", doc.Response.Message)}
	}
	var out []string
	for _, s := range doc.Response.Servers {
		if s.Type == "websockets" && s.Realm == "steamglobal" {
			out = append(out, s.Endpoint)
		}
	}
	if len(out) == 0 {
		return nil, false, &CMError{Msg: "Steam returned no WebSocket CM servers to connect to"}
	}
	return out, true, nil
}

// Dial connects to one CM and logs on with the refresh token. The token
// never leaves this package's memory except on the wire; callers pass it
// in, never through argv, and it is never logged.
func Dial(ctx context.Context, refreshToken string) (*Session, error) {
	steamID, err := SteamIDFromToken(refreshToken)
	if err != nil {
		return nil, err
	}
	servers, tls, err := cmCandidates(ctx)
	if err != nil {
		return nil, err
	}
	var errs []string
	for _, endpoint := range servers {
		s, err := dialOne(ctx, endpoint, tls, steamID, refreshToken)
		if err != nil {
			if ce, ok := err.(*CMError); ok && ce.EResult != 0 {
				return nil, err // the CM answered: its result is the answer
			}
			errs = append(errs, endpoint+": "+err.Error())
			continue
		}
		return s, nil
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
	return nil, &CMError{Msg: "couldn't reach any Steam CM server (" + msg + ")"}
}

func dialWebsocket(ctx context.Context, rawURL string) (*websocket.Conn, error) {
	dialer := websocket.Dialer{HandshakeTimeout: 15 * time.Second}
	conn, _, err := dialer.DialContext(ctx, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// machineIDForDial picks the machine id for a one-shot dial: stable
// under PLATYPUS_PLUGIN_DATA when the plugin data dir is known, otherwise
// a random one (the old behaviour).
func machineIDForDial() []byte { return StableMachineID(os.Getenv("PLATYPUS_PLUGIN_DATA")) }

func randInt() (uint64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1<<30))
	if err != nil {
		return 0, err
	}
	return n.Uint64() + 1, nil
}

func (s *Session) hello() error {
	return s.conn.WriteMessage(websocket.BinaryMessage,
		packEMsg(EMsgClientHello, header(0, 0, 0, ""), func() []byte {
			e := &depot.Enc{}
			e.Varint(1, ProtocolVersion)
			return e.Bytes()
		}()))
}

func dialOne(ctx context.Context, endpoint string, tls bool, steamID uint64, refreshToken string) (*Session, error) {
	scheme := "wss"
	if !tls {
		scheme = "ws"
	}
	u := url.URL{Scheme: scheme, Host: endpoint, Path: "/cmsocket/"}
	conn, err := dialWebsocket(ctx, u.String())
	if err != nil {
		return nil, err
	}
	s := &Session{steamID: steamID, conn: conn, authed: true, loginID: LoginIDFor(LoginRoleOneShot)}
	n, err := randInt()
	if err != nil {
		conn.Close()
		return nil, err
	}
	s.jobID = n
	if err := s.logon(ctx, refreshToken); err != nil {
		conn.Close()
		return nil, err
	}
	return s, nil
}

func (s *Session) logon(ctx context.Context, refreshToken string) error {
	if err := s.hello(); err != nil {
		return &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	s.jobID++
	if err := s.conn.WriteMessage(websocket.BinaryMessage,
		packEMsg(EMsgClientLogon, header(s.steamID, 0, 0, ""),
			logonBody(refreshToken, s.loginID, machineIDForDial()))); err != nil {
		return &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			return &CMError{Msg: "Steam didn't answer the CM logon within 30 seconds"}
		}
		emsg, hdr, body, err := s.readOne(deadline)
		if err != nil {
			return err
		}
		if emsg == EMsgClientLogon {
			// The CM echoes our logon; ignore it.
			_ = hdr
			_ = body
			continue
		}
		// CMsgClientLogonResponse has no fixed EMsg on the wire in the
		// Python port's view: it arrives as 751. Match by number.
		if emsg == 751 {
			m, err := depot.Decode(body)
			if err != nil {
				return &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
			}
			eresult := int(m.Int(1))
			if eresult == 2 {
				if h := hdr.Int(13); h != 0 {
					eresult = int(h) // some CMs only set the header eresult
				}
			}
			s.sessionID = int32(hdr.Int(2))
			if eresult != 1 {
				auth := tokenRejected(eresult)
				suffix := ""
				if !auth {
					suffix = " while logging on to its CM server"
				}
				return &CMError{
					Msg:     "Steam rejected the login with " + EResultName(eresult) + suffix,
					EResult: eresult, Auth: auth,
				}
			}
			// Licences arrive right after logon; Calls consume them via
			// sideMessage, so Dial returns without waiting (a read here
			// would need a timeout, and a timed-out websocket read
			// breaks the connection for later reads).
			return nil
		}
		s.sideMessage(emsg, hdr, body)
	}
}

func (s *Session) sideMessage(emsg uint32, hdr depot.Msg, body []byte) {
	switch emsg {
	case EMsgClientHeartBeat:
		_ = s.conn.WriteMessage(websocket.BinaryMessage,
			packEMsg(EMsgClientHeartBeat, header(s.steamID, s.sessionID, 0, ""), nil))
	case EMsgClientLicenseList:
		if m, err := depot.Decode(body); err == nil {
			for _, l := range m.AllBytes(2) {
				if lm, err := depot.Decode(l); err == nil {
					s.licenses = append(s.licenses, License{PackageID: uint32(lm.Int(1))})
				}
			}
		}
	}
}

// readOne reads one binary message, unwrapping Multi envelopes. Extra
// messages batched in one envelope wait in the session queue for the next
// call instead of being dropped: the logon response routinely arrives
// behind other messages in the same Multi.
func (s *Session) readOne(deadline time.Time) (uint32, depot.Msg, []byte, error) {
	if len(s.pending) > 0 {
		m := s.pending[0]
		s.pending = s.pending[1:]
		emsg, hdr, body, err := unpackEMsg(m)
		if err != nil {
			return 0, nil, nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
		}
		return emsg, hdr, body, nil
	}
	if err := s.conn.SetReadDeadline(deadline); err != nil {
		return 0, nil, nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	for {
		op, data, err := s.conn.ReadMessage()
		if err != nil {
			return 0, nil, nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
		}
		if op != websocket.BinaryMessage {
			continue // text frames and pings: nothing Steam sends here concerns us
		}
		msgs := [][]byte{data}
		for len(msgs) > 0 {
			emsg, hdr, body, err := unpackEMsg(msgs[0])
			msgs = msgs[1:]
			if err != nil {
				return 0, nil, nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
			}
			if emsg == EMsgMulti {
				inner, err := multiUnwrap(body)
				if err != nil {
					return 0, nil, nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
				}
				msgs = append(inner, msgs...)
				continue
			}
			s.pending = append(s.pending, msgs...)
			return emsg, hdr, body, nil
		}
	}
}

// Call makes one ServiceMethodCallFromClient request and returns the
// response body. Replies match by jobid_target, like node-steam-session.
// Before any logon the call goes out as 9804 NonAuthed with no steamid
// or sessionid in the header, just target_job_name and jobid_source.
func (s *Session) Call(service string, body []byte, timeout time.Duration) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	s.jobID++
	job := s.jobID
	hdr := header(s.steamID, s.sessionID, job, service)
	emsg := uint32(EMsgServiceMethodCallFromClient)
	if !s.authed {
		emsg = EMsgServiceMethodCallNonAuthed
	}
	if err := s.conn.WriteMessage(websocket.BinaryMessage,
		packEMsg(emsg, hdr, body)); err != nil {
		return nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return nil, &CMError{Msg: fmt.Sprintf("Steam didn't answer %s within %v", service, timeout)}
		}
		emsg, hdr, msgBody, err := s.readOne(deadline)
		if err != nil {
			return nil, err
		}
		if emsg == EMsgClientHeartBeat {
			s.sideMessage(emsg, hdr, msgBody)
			continue
		}
		if hdr.Int(11) == job { // jobid_target
			eresult := int(hdr.Int(13))
			if eresult == 0 {
				eresult = 1 // the fake CM omits it on success; real CMs set 1
				if m, err := depot.Decode(msgBody); err == nil && len(m) > 0 {
					_ = m
				}
			}
			if eresult != 1 {
				errMsg := ""
				if e := hdr.Bytes(14); len(e) > 0 {
					errMsg = ": " + string(e)
				}
				return nil, &CMError{
					Msg:     "Steam rejected the request with " + EResultName(eresult) + errMsg,
					EResult: eresult, Auth: tokenRejected(eresult),
				}
			}
			return msgBody, nil
		}
		s.sideMessage(emsg, hdr, msgBody)
	}
}

// Send logs off and closes the connection; a dropped close is fine.
func (s *Session) Close() {
	_ = s.conn.WriteMessage(websocket.BinaryMessage,
		packEMsg(EMsgClientLogOff, header(s.steamID, s.sessionID, 0, ""), nil))
	s.conn.Close()
}
