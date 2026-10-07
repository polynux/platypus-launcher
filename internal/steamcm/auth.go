// Authentication sessions over the CM (phase 6): QR and password logins
// that mint the SteamClient refresh token the downloader logs on with.
//
// Protocol facts (steammessages_auth.steamclient.proto, SteamDatabase/
// Protobufs; EMsg numbers from enums_clientserver.proto — never from
// memory, and no SteamKit2 code is translated here):
//
//   - GetPasswordRSAPublicKey (account_name = 1) returns publickey_mod = 1,
//     publickey_exp = 2 (hex strings) and timestamp = 3. The password is
//     RSA-encrypted with PKCS#1 v1.5 and sent base64-encoded.
//   - BeginAuthSessionViaQR (device_friendly_name = 1, platform_type = 2,
//     device_details = 3, website_id = 4) returns client_id = 1,
//     challenge_url = 2, request_id = 3, interval = 4 (float),
//     allowed_confirmations = 5, version = 6.
//   - BeginAuthSessionViaCredentials (device_friendly_name = 1,
//     account_name = 2, encrypted_password = 3, encryption_timestamp = 4,
//     remember_login = 5, platform_type = 6, persistence = 7,
//     website_id = 8, device_details = 9, guard_data = 10, ...) returns
//     client_id = 1, request_id = 2, interval = 3 (float),
//     allowed_confirmations = 4, steamid = 5, ... . Each allowed
//     confirmation carries confirmation_type = 1 and
//     associated_message = 2.
//   - PollAuthSessionStatus (client_id = 1, request_id = 2) returns
//     new_client_id = 1, new_challenge_url = 2, refresh_token = 3,
//     access_token = 4, had_remote_interaction = 5, account_name = 6,
//     new_guard_data = 7. An empty reply (no refresh token, no new
//     challenge) means "not approved yet": wait interval seconds and poll
//     again. Expired sessions, wrong codes and rate limits arrive as the
//     service header EResult.
//   - UpdateAuthSessionWithSteamGuardCode (client_id = 1, steamid = 2
//     fixed64, code = 3, code_type = 4) submits an email or authenticator
//     code. EAuthSessionGuardType: EmailCode = 2, DeviceCode (TOTP) = 3,
//     DeviceConfirmation (mobile approve) = 4.
//
// Device-friendly name is "Hydra Launcher" and platform type is
// SteamClient (1), so the minted refresh token works with the CM logon
// the library and downloader already use. ESessionPersistence_Persistent
// is 1; website_id "Client" matches what the real client sends (the
// proto default is "Unknown").
//
// Errors are CMError with EResult set; callers must not retry
// RateLimitExceeded (84) — the CM throttles whole accounts, and a loop
// only extends the ban.
package steamcm

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"time"

	"hydra-launcher/internal/depot"
)

// Device identity for auth sessions.
const (
	AuthDeviceName = "Hydra Launcher"
	// EAuthTokenPlatformType_SteamClient.
	AuthPlatformSteamClient = 1
	// ESessionPersistence_Persistent.
	AuthPersistencePersistent = 1
	// EAuthSessionGuardType values.
	GuardNone               = 1
	GuardEmailCode          = 2
	GuardDeviceCode         = 3
	GuardDeviceConfirmation = 4
	GuardEmailConfirmation  = 5
)

// AllowedConfirmation is one way Steam offers to approve a session.
type AllowedConfirmation struct {
	Type    int
	Message string
}

// QRBegin is the answer to BeginAuthSessionViaQR.
type QRBegin struct {
	ClientID     uint64
	ChallengeURL string
	RequestID    []byte
	Interval     float32
	Version      int32
	Allowed      []AllowedConfirmation
}

// CredentialsBegin is the answer to BeginAuthSessionViaCredentials.
type CredentialsBegin struct {
	ClientID uint64
	Request  []byte
	Interval float32
	Allowed  []AllowedConfirmation
	SteamID  uint64
}

// PollStatus is the answer to PollAuthSessionStatus.
type PollStatus struct {
	NewClientID          uint64
	NewChallengeURL      string
	RefreshToken         string
	AccessToken          string
	HadRemoteInteraction bool
	AccountName          string
	NewGuardData         string
}

// DialUnauth connects to a CM with ClientHello only (no logon) for the
// Authentication service calls, which run before any token exists. The
// returned session has no steamid/sessionid; Call omits them from the
// header, like the real client's pre-logon service calls.
func DialUnauth(ctx context.Context) (*Session, error) {
	servers, tls, err := cmCandidates(ctx)
	if err != nil {
		return nil, err
	}
	var errs []string
	for _, endpoint := range servers {
		s, err := dialUnauthOne(ctx, endpoint, tls)
		if err != nil {
			if ce, ok := err.(*CMError); ok && ce.EResult != 0 {
				return nil, err
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

func dialUnauthOne(ctx context.Context, endpoint string, tls bool) (*Session, error) {
	scheme := "wss"
	if !tls {
		scheme = "ws"
	}
	u := scheme + "://" + endpoint + "/cmsocket/"
	// Reuse dialOne's websocket setup via a minimal inline dial: it only
	// needs the hello, not the token logon.
	conn, err := dialWebsocket(ctx, u)
	if err != nil {
		return nil, err
	}
	s := &Session{conn: conn}
	n, err := randInt()
	if err != nil {
		conn.Close()
		return nil, err
	}
	s.jobID = n
	if err := s.hello(); err != nil {
		conn.Close()
		return nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	return s, nil
}

// parseAllowed decodes the repeated AllowedConfirmation messages.
func parseAllowed(raw [][]byte) []AllowedConfirmation {
	var out []AllowedConfirmation
	for _, b := range raw {
		if m, err := depot.Decode(b); err == nil {
			out = append(out, AllowedConfirmation{Type: int(m.Int(1)), Message: string(m.Bytes(2))})
		}
	}
	return out
}

// GetPasswordRSAPublicKey fetches the RSA key that encrypts the password
// for accountName.
func (s *Session) GetPasswordRSAPublicKey(accountName string, timeout time.Duration) (mod, exp string, timestamp uint64, err error) {
	e := &depot.Enc{}
	e.Str(1, accountName)
	body, err := s.Call("Authentication.GetPasswordRSAPublicKey#1", e.Bytes(), timeout)
	if err != nil {
		return "", "", 0, err
	}
	m, err := depot.Decode(body)
	if err != nil {
		return "", "", 0, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	return string(m.Bytes(1)), string(m.Bytes(2)), m.Int(3), nil
}

// EncryptPassword RSA-encrypts the password with PKCS#1 v1.5 and returns
// the base64 the Begin call wants. mod/exp are the hex strings the key
// call returns.
func EncryptPassword(password, modHex, expHex string) (string, error) {
	modHex = strings.TrimSpace(modHex)
	expHex = strings.TrimSpace(expHex)
	n := new(big.Int)
	if _, ok := n.SetString(modHex, 16); !ok {
		// Some servers pad with whitespace; retry stripped of leading
		// zeros is the same value — SetString already tolerates it, so a
		// failure here is a real format error.
		return "", fmt.Errorf("bad RSA modulus from Steam")
	}
	ev := new(big.Int)
	if _, ok := ev.SetString(expHex, 16); !ok {
		if _, ok := ev.SetString(expHex, 10); !ok {
			return "", fmt.Errorf("bad RSA exponent from Steam")
		}
	}
	if !ev.IsInt64() {
		return "", fmt.Errorf("bad RSA exponent from Steam")
	}
	pub := &rsa.PublicKey{N: n, E: int(ev.Int64())}
	enc, err := rsa.EncryptPKCS1v15(rand.Reader, pub, []byte(password))
	if err != nil {
		return "", fmt.Errorf("couldn't encrypt the password: %w", err)
	}
	return base64.StdEncoding.EncodeToString(enc), nil
}

// BeginQR starts a QR login session.
func (s *Session) BeginQR(timeout time.Duration) (*QRBegin, error) {
	e := &depot.Enc{}
	e.Str(1, AuthDeviceName)
	e.Varint(2, AuthPlatformSteamClient)
	e.Str(4, "Client")
	body, err := s.Call("Authentication.BeginAuthSessionViaQR#1", e.Bytes(), timeout)
	if err != nil {
		return nil, err
	}
	m, err := depot.Decode(body)
	if err != nil {
		return nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	out := &QRBegin{
		ClientID:     m.Int(1),
		ChallengeURL: string(m.Bytes(2)),
		RequestID:    append([]byte{}, m.Bytes(3)...),
		Version:      int32(m.Int(6)),
		Allowed:      parseAllowed(m.AllBytes(5)),
	}
	if f := m.AllInts(4); len(f) > 0 {
		out.Interval = float32FromBits(uint32(f[0]))
	}
	if out.Interval <= 0 {
		out.Interval = 5
	}
	if out.ClientID == 0 || out.ChallengeURL == "" {
		return nil, &CMError{Msg: "Steam's QR login response was empty"}
	}
	return out, nil
}

// BeginCredentials starts a password login session. encryptedPassword is
// the base64 from EncryptPassword; timestamp comes from the RSA key call.
func (s *Session) BeginCredentials(account, encryptedPassword string, timestamp uint64, timeout time.Duration) (*CredentialsBegin, error) {
	e := &depot.Enc{}
	e.Str(1, AuthDeviceName)
	e.Str(2, account)
	e.Str(3, encryptedPassword)
	e.Varint(4, timestamp)
	e.Varint(5, 1) // remember_login
	e.Varint(6, AuthPlatformSteamClient)
	e.Varint(7, AuthPersistencePersistent)
	e.Str(8, "Client")
	body, err := s.Call("Authentication.BeginAuthSessionViaCredentials#1", e.Bytes(), timeout)
	if err != nil {
		return nil, describeAuthError("login", err)
	}
	m, err := depot.Decode(body)
	if err != nil {
		return nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	out := &CredentialsBegin{
		ClientID: m.Int(1),
		Request:  append([]byte{}, m.Bytes(2)...),
		SteamID:  m.Int(5),
		Allowed:  parseAllowed(m.AllBytes(4)),
	}
	if f := m.AllInts(3); len(f) > 0 {
		out.Interval = float32FromBits(uint32(f[0]))
	}
	if out.Interval <= 0 {
		out.Interval = 5
	}
	if out.ClientID == 0 {
		return nil, &CMError{Msg: "Steam's login response was empty"}
	}
	return out, nil
}

// PollStatus polls one auth session. An empty result (no refresh token,
// no new challenge) means "not approved yet".
func (s *Session) PollStatus(clientID uint64, requestID []byte, timeout time.Duration) (*PollStatus, error) {
	e := &depot.Enc{}
	e.Varint(1, clientID)
	e.BytesField(2, requestID)
	body, err := s.Call("Authentication.PollAuthSessionStatus#1", e.Bytes(), timeout)
	if err != nil {
		return nil, describeAuthError("login", err)
	}
	m, err := depot.Decode(body)
	if err != nil {
		return nil, &CMError{Msg: fmt.Sprintf("Steam CM connection dropped: %v", err)}
	}
	return &PollStatus{
		NewClientID:          m.Int(1),
		NewChallengeURL:      string(m.Bytes(2)),
		RefreshToken:         string(m.Bytes(3)),
		AccessToken:          string(m.Bytes(4)),
		HadRemoteInteraction: m.Int(5) != 0,
		AccountName:          string(m.Bytes(6)),
		NewGuardData:         string(m.Bytes(7)),
	}, nil
}

// UpdateGuardCode submits an email or authenticator code for a password
// session. codeType is GuardEmailCode (2) or GuardDeviceCode/TOTP (3).
func (s *Session) UpdateGuardCode(clientID, steamID uint64, code string, codeType int, timeout time.Duration) error {
	e := &depot.Enc{}
	e.Varint(1, clientID)
	e.Fixed64(2, steamID)
	e.Str(3, code)
	if codeType != 0 {
		e.Varint(4, uint64(codeType))
	}
	_, err := s.Call("Authentication.UpdateAuthSessionWithSteamGuardCode#1", e.Bytes(), timeout)
	if err != nil {
		return describeAuthError("code", err)
	}
	return nil
}

// describeAuthError rewrites the raw EResult into the message the login
// dialog shows. EResult 84 (rate limit) is never retried by callers.
func describeAuthError(op string, err error) error {
	ce, ok := err.(*CMError)
	if !ok {
		return err
	}
	switch ce.EResult {
	case 5: // InvalidPassword: wrong password or wrong guard code
		if op == "code" {
			return &CMError{Msg: "Steam rejected the code: wrong Steam Guard code", EResult: ce.EResult, Auth: ce.Auth}
		}
		return &CMError{Msg: "Steam rejected the login: wrong account name or password", EResult: ce.EResult, Auth: ce.Auth}
	case 27, 26: // Expired, Revoked: session is gone, start over
		return &CMError{Msg: "the Steam login session expired; try again", EResult: ce.EResult, Auth: ce.Auth}
	case 84: // RateLimitExceeded: stop, no retry loop
		return &CMError{Msg: "Steam is rate-limiting logins (EResult 84); wait a while and try again", EResult: ce.EResult, Auth: ce.Auth}
	case 85: // AccountLoginDeniedNeedTwoFactor: caller must supply a code
		return &CMError{Msg: "Steam needs a Steam Guard code for this account", EResult: ce.EResult, Auth: ce.Auth}
	case 0:
		return err
	default:
		return &CMError{Msg: "Steam rejected the " + op + " with " + EResultName(ce.EResult), EResult: ce.EResult, Auth: ce.Auth}
	}
}

// GuardNeeded reports which confirmation a credentials session wants:
// an email/TOTP code (returns its type), a mobile confirmation (returns
// GuardDeviceConfirmation with needCode=false), or nothing yet.
func GuardNeeded(allowed []AllowedConfirmation) (codeType int, needCode, needConfirm bool) {
	for _, a := range allowed {
		switch a.Type {
		case GuardEmailCode:
			return GuardEmailCode, true, false
		case GuardDeviceCode:
			// Prefer email when both are offered; the caller
			// falls back to device code when email is absent.
			if codeType == 0 {
				codeType = GuardDeviceCode
				needCode = true
			}
		case GuardDeviceConfirmation, GuardEmailConfirmation:
			needConfirm = true
		}
	}
	if needCode {
		return codeType, true, needConfirm
	}
	return 0, false, needConfirm
}
