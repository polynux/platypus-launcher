// Fake CM authentication service (phase 6 tests): QR and password logins
// without Valve's network. Protocol facts match auth.go (field numbers
// from steammessages_auth.steamclient.proto).
package steamcm

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"hydra-launcher/internal/depot"
)

// Auth knobs for the fake (see fakeConfigAuth below).
type fakeAuthSession struct {
	clientID     uint64
	requestID    []byte
	account      string
	steamID      uint64
	mode         string // "qr" or "password"
	polls        int
	guard        string // "none", "email", "totp", "confirm"
	expectedCode string
	challengeURL string
	refreshed    bool
	approved     bool // set by UpdateGuardCode success
}

func (f *fakeCM) ensureRSA() *rsa.PrivateKey {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rsaKey == nil {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			panic(err)
		}
		f.rsaKey = key
	}
	return f.rsaKey
}

func (f *fakeCM) nextClientID() uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	if f.nextID < 1000 {
		f.nextID = 1000
	}
	return f.nextID
}

func (f *fakeCM) storeAuth(s *fakeAuthSession) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.authSessions == nil {
		f.authSessions = map[uint64]*fakeAuthSession{}
	}
	f.authSessions[s.clientID] = s
}

func (f *fakeCM) getAuth(clientID uint64) *fakeAuthSession {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.authSessions[clientID]
}

// authDefaults fills the account/steamid/challenge defaults.
func authDefaults(cfg fakeConfig) (account string, steamID uint64, challenge string) {
	account = cfg.AuthAccount
	if account == "" {
		account = "TestUser"
	}
	steamID = cfg.AuthSteamID
	if steamID == 0 {
		steamID = 76561198
	}
	challenge = cfg.AuthChallengeURL
	if challenge == "" {
		challenge = "https://steamcommunity.com/qr/TESTCHALLENGE"
	}
	return account, steamID, challenge
}

func (f *fakeCM) answerAuth(c *websocket.Conn, target string, job, steamID uint64, body []byte, cfg fakeConfig) bool {
	switch target {
	case "Authentication.GetPasswordRSAPublicKey#1":
		return f.answerRSAKey(c, job, steamID, body)
	case "Authentication.BeginAuthSessionViaQR#1":
		return f.answerBeginQR(c, job, steamID, body, cfg)
	case "Authentication.BeginAuthSessionViaCredentials#1":
		return f.answerBeginCreds(c, job, steamID, body, cfg)
	case "Authentication.PollAuthSessionStatus#1":
		return f.answerPoll(c, job, steamID, body, cfg)
	case "Authentication.UpdateAuthSessionWithSteamGuardCode#1":
		return f.answerUpdateCode(c, job, steamID, body, cfg)
	default:
		return false
	}
}

func (f *fakeCM) answerRSAKey(c *websocket.Conn, job, steamID uint64, body []byte) bool {
	key := f.ensureRSA()
	mod := fmt.Sprintf("%x", key.N)
	exp := fmt.Sprintf("%x", key.E)
	e := &depot.Enc{}
	e.Str(1, mod)
	e.Str(2, exp)
	e.Varint(3, uint64(time.Now().Unix()))
	_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, 1), e.Bytes()))
	f.record("service Authentication.GetPasswordRSAPublicKey#1")
	return true
}

func (f *fakeCM) answerBeginQR(c *websocket.Conn, job, steamID uint64, body []byte, cfg fakeConfig) bool {
	f.record("service Authentication.BeginAuthSessionViaQR#1")
	if cfg.AuthBeginEresult != 0 && cfg.AuthBeginEresult != 1 {
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, cfg.AuthBeginEresult), nil))
		return true
	}
	account, _, challenge := authDefaults(cfg)
	clientID := f.nextClientID()
	reqID := make([]byte, 16)
	rand.Read(reqID)
	sess := &fakeAuthSession{clientID: clientID, requestID: append([]byte{}, reqID...), account: account, steamID: cfg.AuthSteamIDOr(76561198), mode: "qr", challengeURL: challenge}
	f.storeAuth(sess)
	e := &depot.Enc{}
	e.Varint(1, clientID)
	e.Str(2, challenge)
	e.BytesField(3, reqID)
	e.Fixed32(4, bitsOf(0.2)) // fast polls in tests
	allow := &depot.Enc{}
	allow.Varint(1, GuardDeviceConfirmation)
	allow.Str(2, "Confirm via the Steam mobile app")
	e.MsgField(5, allow)
	e.Varint(6, 2)
	_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, 1), e.Bytes()))
	return true
}

func (f *fakeCM) answerBeginCreds(c *websocket.Conn, job, steamID uint64, body []byte, cfg fakeConfig) bool {
	f.record("service Authentication.BeginAuthSessionViaCredentials#1")
	if cfg.AuthBeginEresult != 0 && cfg.AuthBeginEresult != 1 {
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, cfg.AuthBeginEresult), nil))
		return true
	}
	m, _ := depot.Decode(body)
	account := string(m.Bytes(2))
	encB64 := string(m.Bytes(3))
	expectedPass := cfg.AuthPassword
	if expectedPass == "" {
		expectedPass = "correct-horse"
	}
	// Decrypt and compare, like the real server checks the password.
	key := f.ensureRSA()
	raw, err := base64.StdEncoding.DecodeString(encB64)
	if err != nil {
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, 5), nil))
		return true
	}
	plain, err := rsa.DecryptPKCS1v15(rand.Reader, key, raw)
	if err != nil || string(plain) != expectedPass {
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, 5), nil))
		return true
	}
	if account == "" {
		account, _, _ = authDefaults(cfg)
	}
	steamIDOut := cfg.AuthSteamIDOr(76561198)
	guard := cfg.AuthGuard
	if guard == "" {
		guard = "none"
	}
	expCode := cfg.AuthExpectedCode
	if expCode == "" {
		if guard == "totp" {
			expCode = "123456"
		} else {
			expCode = "ABC12"
		}
	}
	clientID := f.nextClientID()
	reqID := make([]byte, 16)
	rand.Read(reqID)
	sess := &fakeAuthSession{clientID: clientID, requestID: append([]byte{}, reqID...), account: account, steamID: steamIDOut, mode: "password", guard: guard, expectedCode: expCode}
	// "none" means the password alone suffices: next poll issues the token.
	if guard == "none" {
		sess.approved = true
	}
	f.storeAuth(sess)
	e := &depot.Enc{}
	e.Varint(1, clientID)
	e.BytesField(2, reqID)
	e.Fixed32(3, bitsOf(0.2))
	switch guard {
	case "email":
		allow := &depot.Enc{}
		allow.Varint(1, GuardEmailCode)
		allow.Str(2, "Enter the code emailed to you")
		e.MsgField(4, allow)
	case "totp":
		allow := &depot.Enc{}
		allow.Varint(1, GuardDeviceCode)
		allow.Str(2, "Enter the authenticator code")
		e.MsgField(4, allow)
	case "confirm":
		allow := &depot.Enc{}
		allow.Varint(1, GuardDeviceConfirmation)
		allow.Str(2, "Confirm in the mobile app")
		e.MsgField(4, allow)
	}
	e.Varint(5, steamIDOut)
	_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, 1), e.Bytes()))
	return true
}

func (f *fakeCM) answerPoll(c *websocket.Conn, job, steamID uint64, body []byte, cfg fakeConfig) bool {
	m, _ := depot.Decode(body)
	clientID := m.Int(1)
	f.record(fmt.Sprintf("service Authentication.PollAuthSessionStatus#1 client=%d", clientID))
	if cfg.AuthPollEresult != 0 && cfg.AuthPollEresult != 1 {
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, cfg.AuthPollEresult), nil))
		return true
	}
	sess := f.getAuth(clientID)
	if sess == nil {
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, 27), nil))
		return true
	}
	f.mu.Lock()
	sess.polls++
	polls := sess.polls
	f.mu.Unlock()
	e := &depot.Enc{}
	// QR challenge refresh: first poll hands a new URL, like Steam expiring it.
	if sess.mode == "qr" && cfg.AuthQRRefresh && !sess.refreshed {
		f.mu.Lock()
		sess.refreshed = true
		f.mu.Unlock()
		newURL := cfg.AuthNewChallengeURL
		if newURL == "" {
			newURL = "https://steamcommunity.com/qr/REFRESHED"
		}
		f.mu.Lock()
		sess.challengeURL = newURL
		f.mu.Unlock()
		e.Str(2, newURL)
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, 1), e.Bytes()))
		return true
	}
	approved := false
	switch sess.mode {
	case "qr":
		if f.getCfg().AuthQRApproved {
			approved = true
		}
	case "password":
		if sess.approved {
			approved = true
		} else if sess.guard == "confirm" && f.getCfg().AuthMobileApproved {
			approved = true
		}
	}
	_ = polls
	if !approved {
		// Empty reply: not approved yet, poll again.
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, 1), e.Bytes()))
		return true
	}
	ttl := cfg.TokenTTL
	if ttl == 0 {
		ttl = 3600
	}
	refresh := fakeRefreshToken(sess.steamID, ttl)
	e.Str(3, refresh)
	e.Str(6, sess.account)
	_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, 1), e.Bytes()))
	return true
}

func (f *fakeCM) answerUpdateCode(c *websocket.Conn, job, steamID uint64, body []byte, cfg fakeConfig) bool {
	m, _ := depot.Decode(body)
	clientID := m.Int(1)
	code := string(m.Bytes(3))
	f.record(fmt.Sprintf("service Authentication.UpdateAuthSessionWithSteamGuardCode#1 client=%d", clientID))
	sess := f.getAuth(clientID)
	if sess == nil {
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, 27), nil))
		return true
	}
	if code != sess.expectedCode {
		_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, 5), nil))
		return true
	}
	f.mu.Lock()
	sess.approved = true
	f.mu.Unlock()
	e := &depot.Enc{}
	_ = c.WriteMessage(websocket.BinaryMessage, packEMsg(EMsgServiceMethodResponse, f.replyHdr(steamID, job, 1), e.Bytes()))
	return true
}

// fakeRefreshToken mints a SteamClient-shaped refresh JWT for auth tests:
// like the real token it carries sub (steamid) and a long exp, but its
// aud marks it as a client token the CM logon accepts.
func fakeRefreshToken(steamID uint64, ttl int) string {
	return fakeToken(steamID, ttl)
}
