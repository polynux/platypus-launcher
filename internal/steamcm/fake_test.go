// Fake CM for the steamcm tests: a real WebSocket CM speaking the
// session's protocol on localhost (no Valve network). It mirrors
// plugins/steam-standalone/testdata/fake_cm.py's knobs where they overlap
// (logon_eresult, service_eresult, wrap_logon_multi, token_ttl,
// send_heartbeat) and adds the content RPCs phase 2 needs.
package steamcm

import (
	"bytes"
	"crypto/rsa"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"hydra-launcher/internal/depot"
)

type fakeConfig struct {
	LogonEresult   int
	ServiceEresult int
	WrapLogonMulti bool
	SendHeartbeat  bool
	MixedMulti     bool
	TokenTTL       int
	DepotKey       []byte
	AppTokens      map[uint32]uint64
	ProductKV      string
	Servers        []Server
	RequestCode    uint64
	CDNToken       string
	BetaOK         bool
	Licenses       []uint32
	// Auth knobs (phase 6): QR/password logins without Valve's network.
	AuthAccount         string
	AuthSteamID         uint64
	AuthPassword        string
	AuthGuard           string // "none", "email", "totp", "confirm"
	AuthExpectedCode    string
	AuthQRApproved      bool
	AuthMobileApproved  bool
	AuthQRRefresh       bool
	AuthBeginEresult    int
	AuthPollEresult     int
	AuthChallengeURL    string
	AuthNewChallengeURL string
}

func (c fakeConfig) AuthSteamIDOr(def uint64) uint64 {
	if c.AuthSteamID != 0 {
		return c.AuthSteamID
	}
	return def
}

// packOld builds an old-format (non-protobuf) message: the EMsg without
// the proto bit and an opaque body, like the live CM's 798/782 messages.
func packOld(emsg uint32, body []byte) []byte {
	out := make([]byte, 8, 8+len(body))
	binary.LittleEndian.PutUint32(out, emsg)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(body)))
	return append(out, body...)
}

type fakeCM struct {
	t      *testing.T
	mu     sync.Mutex
	cfg    fakeConfig
	events []string
	// Auth state (phase 6): RSA key for password encryption and live sessions.
	rsaKey       *rsa.PrivateKey
	authSessions map[uint64]*fakeAuthSession
	nextID       uint64
}

func (f *fakeCM) record(ev string) {
	f.mu.Lock()
	f.events = append(f.events, ev)
	f.mu.Unlock()
}

func (f *fakeCM) log() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.events...)
}

func (f *fakeCM) getCfg() fakeConfig {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cfg
}

func fakeToken(steamID uint64, ttl int) string {
	if ttl == 0 {
		ttl = 3600
	}
	now := time.Now().Unix()
	seg := func(v any) string {
		raw, _ := json.Marshal(v)
		return base64.RawURLEncoding.EncodeToString(raw)
	}
	return seg(map[string]string{"typ": "JWT", "alg": "EdDSA"}) + "." +
		seg(map[string]any{"iss": "steam", "sub": fmt.Sprint(steamID), "aud": []string{"web"}, "exp": now + int64(ttl), "iat": now, "nbf": now}) + "." +
		seg(map[string]bool{"sig": true})
}

func startFakeCM(t *testing.T, cfg fakeConfig) (*fakeCM, string) {
	t.Helper()
	f := &fakeCM{t: t, cfg: cfg}
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		f.serve(c)
	}))
	t.Cleanup(srv.Close)
	// httptest URL is http://127.0.0.1:port; the client wants host:port.
	host := srv.URL[len("http://"):]
	return f, host
}

func (f *fakeCM) serve(c *websocket.Conn) {
	f.record("connect")
	cfg := f.getCfg()
	// Hello + logon, or unauthenticated Authentication service calls
	// (phase 6 logins run before any token exists, so they never log on).
	var logonHdr depot.Msg
	for {
		op, data, err := c.ReadMessage()
		if err != nil {
			return
		}
		if op != websocket.BinaryMessage {
			continue
		}
		emsg, hdr, body, err := unpackEMsg(data)
		if err != nil {
			return
		}
		if emsg == EMsgClientHello {
			continue
		}
		if emsg == EMsgServiceMethodCallFromClient {
			target := string(hdr.Bytes(12))
			job := hdr.Int(10)
			if len(target) >= 15 && target[:15] == "Authentication." {
				f.answerAuth(c, target, job, hdr.Int(1), body, f.getCfg())
				continue
			}
		}
		if emsg == EMsgClientLogOff {
			f.record("logoff")
			return
		}
		if emsg == EMsgClientLogon {
			logonHdr = hdr
			break
		}
		f.record(fmt.Sprintf("unexpected %d", emsg))
	}
	steamID := logonHdr.Int(1)
	f.record(fmt.Sprintf("logon %d", steamID))
	eres := cfg.LogonEresult
	if eres == 0 {
		eres = 1
	}
	hdr := &depot.Enc{}
	hdr.Fixed64(1, steamID)
	hdr.Varint(2, 4711)
	resp := &depot.Enc{}
	resp.Varint(1, uint64(eres))
	resp.Varint(3, 5)
	msg := packEMsg(751, hdr.Bytes(), resp.Bytes())
	if cfg.WrapLogonMulti {
		msg = multiWrap(msg)
	}
	_ = c.WriteMessage(websocket.BinaryMessage, msg)
	if eres != 1 {
		_ = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(1000, ""))
		return
	}
	// Licences arrive after logon, like the real CM. With MixedMulti the
	// licence is batched behind an old-format (non-protobuf) message in
	// one Multi, the way live Multis mix formats: the client must
	// tolerate the old message and still find the licence behind it.
	if cfg.MixedMulti {
		old := packOld(798, bytes.Repeat([]byte{0xAB}, 20))
		lic := &depot.Enc{}
		for _, p := range cfg.Licenses {
			l := &depot.Enc{}
			l.Varint(1, uint64(p))
			lic.MsgField(2, l)
		}
		_ = c.WriteMessage(websocket.BinaryMessage,
			multiWrap(old, packEMsg(EMsgClientLicenseList, hdr.Bytes(), lic.Bytes())))
	} else if len(cfg.Licenses) > 0 {
		lic := &depot.Enc{}
		for _, p := range cfg.Licenses {
			l := &depot.Enc{}
			l.Varint(1, uint64(p))
			lic.MsgField(2, l)
		}
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgClientLicenseList, hdr.Bytes(), lic.Bytes()))
	}
	sentHeartbeat := false
	for {
		op, data, err := c.ReadMessage()
		if err != nil {
			return
		}
		if op != websocket.BinaryMessage {
			continue
		}
		emsg, hdr, body, err := unpackEMsg(data)
		if err != nil {
			return
		}
		switch emsg {
		case EMsgClientHeartBeat:
			f.record("heartbeat_reply")
		case EMsgClientLogOff:
			f.record("logoff")
			off := &depot.Enc{}
			off.Varint(1, 1)
			lh := &depot.Enc{}
			lh.Fixed64(1, steamID)
			lh.Varint(2, 4711)
			_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgClientLoggedOff, lh.Bytes(), off.Bytes()))
			return
		case EMsgServiceMethodCallFromClient:
			target := string(hdr.Bytes(12))
			job := hdr.Int(10)
			f.record("service " + target)
			if cfg.SendHeartbeat && !sentHeartbeat {
				sentHeartbeat = true
				hb := &depot.Enc{}
				hb.Fixed64(1, steamID)
				hb.Varint(2, 4711)
				_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgClientHeartBeat, hb.Bytes(), nil))
			}
			f.answerService(c, target, job, steamID, body, cfg)
			if cfg.ServiceEresult != 0 && cfg.ServiceEresult != 1 {
				return
			}
		case EMsgClientGetDepotDecryptionKey, EMsgClientPICSAccessToken, EMsgClientPICSProductInfo, EMsgClientCheckAppBetaPassword:
			job := hdr.Int(10)
			f.answerDirect(c, emsg, job, steamID, body, cfg)
		default:
			f.record(fmt.Sprintf("unexpected %d", emsg))
		}
	}
}

func (f *fakeCM) replyHdr(steamID, job uint64, eresult int) []byte {
	h := &depot.Enc{}
	h.Fixed64(1, steamID)
	h.Varint(2, 4711)
	h.Fixed64(11, job)
	h.Varint(13, uint64(eresult))
	return h.Bytes()
}

func (f *fakeCM) answerService(c *websocket.Conn, target string, job, steamID uint64, body []byte, cfg fakeConfig) {
	if f.answerAuth(c, target, job, steamID, body, cfg) {
		return
	}
	eres := cfg.ServiceEresult
	if eres == 0 {
		eres = 1
	}
	var resp []byte
	switch target {
	case "Authentication.GenerateAccessTokenForApp#1":
		if eres == 1 {
			m, _ := depot.Decode(body)
			sub := m.Int(2)
			if sub == 0 {
				sub = steamID
			}
			e := &depot.Enc{}
			e.Str(1, fakeToken(sub, cfg.TokenTTL))
			resp = e.Bytes()
		}
	case "ContentServerDirectory.GetServersForSteamPipe#1":
		if eres == 1 {
			e := &depot.Enc{}
			for _, sv := range cfg.Servers {
				se := &depot.Enc{}
				se.Str(1, sv.Type)
				se.Varint(3, uint64(uint32(sv.CellID)))
				se.Varint(4, 10)
				se.Fixed32(5, bitsOf(sv.WeightedLoad))
				se.Varint(6, 1)
				se.Str(8, sv.Host)
				if sv.VHost != "" {
					se.Str(9, sv.VHost)
				}
				se.Str(12, "https")
				for _, a := range sv.AllowedAppIDs {
					se.Varint(13, uint64(a))
				}
				e.MsgField(1, se)
			}
			resp = e.Bytes()
		}
	case "ContentServerDirectory.GetManifestRequestCode#1":
		if eres == 1 {
			e := &depot.Enc{}
			e.Varint(1, cfg.RequestCode)
			resp = e.Bytes()
		}
	case "ContentServerDirectory.GetCDNAuthToken#1":
		if eres == 1 {
			e := &depot.Enc{}
			e.Str(1, cfg.CDNToken)
			e.Varint(2, uint64(time.Now().Add(time.Hour).Unix()))
			resp = e.Bytes()
		}
	default:
		eres = 2
	}
	_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, eres), resp))
	_ = binary.LittleEndian
}

func (f *fakeCM) answerDirect(c *websocket.Conn, emsg uint32, job, steamID uint64, body []byte, cfg fakeConfig) {
	switch emsg {
	case EMsgClientGetDepotDecryptionKey:
		m, _ := depot.Decode(body)
		e := &depot.Enc{}
		e.Varint(1, 1)
		e.Varint(2, m.Int(1))
		key := cfg.DepotKey
		if len(key) == 0 {
			key = make([]byte, 32)
		}
		e.BytesField(3, key)
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(emsg+1, f.replyHdr(steamID, job, 1), e.Bytes()))
	case EMsgClientPICSAccessToken:
		e := &depot.Enc{}
		for app, tok := range cfg.AppTokens {
			te := &depot.Enc{}
			te.Varint(1, uint64(app))
			te.Varint(2, tok)
			e.MsgField(3, te)
		}
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(emsg+1, f.replyHdr(steamID, job, 1), e.Bytes()))
	case EMsgClientPICSProductInfo:
		m, _ := depot.Decode(body)
		var appID uint64
		for _, a := range m.AllBytes(2) {
			if am, err := depot.Decode(a); err == nil {
				appID = am.Int(1)
			}
		}
		e := &depot.Enc{}
		ae := &depot.Enc{}
		ae.Varint(1, appID)
		ae.Varint(2, 42)
		ae.BytesField(5, []byte(cfg.ProductKV))
		e.MsgField(1, ae)
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(emsg+1, f.replyHdr(steamID, job, 1), e.Bytes()))
	case EMsgClientCheckAppBetaPassword:
		eres := 1
		if !cfg.BetaOK {
			eres = 2
		}
		e := &depot.Enc{}
		e.Varint(1, uint64(eres))
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(emsg+1, f.replyHdr(steamID, job, eres), e.Bytes()))
	}
}
