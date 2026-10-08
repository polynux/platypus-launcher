// Package steamcm is a Go port of what plugins/steam-standalone/bin/steam_cm.py
// does (WebSocket CM, hello, logon with the refresh token, Multi/gzip,
// heartbeats, service calls), plus depot keys, PICS access tokens and
// product info, licences, the server list, request codes and CDN tokens.
//
// Message layouts follow SteamKit2 and node-steam-session; EMsg numbers come
// from steam/enums_clientserver.proto and field numbers from the .proto
// files the plan lists (SteamDatabase/Protobufs), never from memory.
package steamcm

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
)

// EMsg numbers (steam/enums_clientserver.proto).
const (
	EMsgMulti                       = 1
	EMsgCloudNotifyAppStateChange   = 146
	EMsgClientPersonaState          = 766
	EMsgClientFriendsList           = 767
	EMsgClientPlayingSessionState   = 9600
	EMsgServiceMethodResponse       = 147
	EMsgServiceMethodCallFromClient = 151
	// Unauthenticated service calls (before any logon) go out as 9804,
	// not 151: the real CM closes the socket (1000) on a 151 from a
	// client that never logged on. The reply is still 147, matched by
	// jobid_target like authed calls.
	EMsgServiceMethodCallNonAuthed  = 9804
	EMsgClientHeartBeat             = 703
	EMsgClientLogOff                = 706
	EMsgClientLicenseList           = 780
	EMsgClientLoggedOff             = 757
	EMsgClientGetDepotDecryptionKey = 5438
	EMsgClientCheckAppBetaPassword  = 5450
	EMsgClientLogon                 = 5514
	EMsgClientPICSProductInfo       = 8903
	EMsgClientPICSAccessToken       = 8905
	EMsgClientHello                 = 9805
)

// Logon constants, matching SteamKit2's SteamUser (protocol 65580) and
// node-steam-user (package 1771, LinuxUnknown), like steam_cm.py.
const (
	ProtocolVersion      = 65580
	ClientPackageVersion = 1771
	// EOSType.LinuxUnknown as uint32.
	ClientOSLinux = uint32(0xFFFFFF35)
	// The obfuscated private ip doubles as the login id. The base is
	// fixed so one-shot logons differ from the official client's
	// (derived from its private ip); per-role values and the one-shot
	// random pick (LoginIDFor) keep Hydra's own sessions from kicking
	// each other (plan D8, research R1).
	LoginIDBase = 2718281
	MachineName = "Hydra"
)

// LoginRole names what a logon session is for: each role gets its own
// logon id so that overlapping Hydra sessions (a persistent social or
// cloud session next to one-shot depot/auth jobs) never replace each
// other (LogonSessionReplaced, EResult 34).
type LoginRole int

const (
	// One-shot callers (library listing, depot download jobs, token
	// exchange): one random value per process.
	LoginRoleOneShot LoginRole = iota
	LoginRoleSocial
	LoginRoleCloud
	LoginRoleContent
)

var (
	loginIDOnce sync.Once
	shotLoginID uint32
)

// LoginIDFor returns the obfuscated_private_ip value for a role. Role
// values sit well above the base so a collision with the official
// client's id (derived from its real private ip) stays as unlikely as
// the base itself; the one-shot value is one random pick per process.
func LoginIDFor(role LoginRole) uint32 {
	switch role {
	case LoginRoleSocial:
		return LoginIDBase + 0x01000000
	case LoginRoleCloud:
		return LoginIDBase + 0x02000000
	case LoginRoleContent:
		return LoginIDBase + 0x03000000
	default:
		loginIDOnce.Do(func() {
			n := make([]byte, 4)
			rand.Read(n)
			v := binary.BigEndian.Uint32(n)
			// Keep it distinct from the base and the role values.
			if v == 0 || v == LoginIDBase || v >= LoginIDBase+0x01000000 && v <= LoginIDBase+0x03000000 {
				v = LoginIDBase - 1
			}
			shotLoginID = v
		})
		return shotLoginID
	}
}

// logonTaken marks the EResults that say "another session holds this
// slot": LogonSessionReplaced 34 (the logon result) and
// LoggedInElsewhere 6 (ClientLoggedOff eresult). A session that sees
// one must back off, not reconnect-fight (research §3.2).
func logonTaken(eresult int) bool {
	return eresult == 34 || eresult == 6
}

// EResults worth naming for users (steammessages + SteamKit2's EResult).
var eresultNames = map[int]string{
	1:  "OK",
	2:  "Fail",
	3:  "NoConnection",
	5:  "InvalidPassword",
	8:  "InvalidParam",
	15: "AccessDenied",
	20: "ServiceUnavailable",
	26: "Revoked",
	27: "Expired",
	48: "TryAnotherCM",
	84: "RateLimitExceeded",
	85: "AccountLoginDeniedNeedTwoFactor",
	87: "AccountLoginDeniedThrottle",
}

// EResultName names an EResult for error messages.
func EResultName(e int) string {
	if n, ok := eresultNames[e]; ok {
		return n
	}
	return fmt.Sprintf("EResult %d", e)
}

// tokenRejected marks EResults that mean "Steam says this login/token is
// no good": InvalidPassword 5, Revoked 26 and Expired 27. AccessDenied
// 15 is deliberately absent: on a service call it usually means "no
// permission for this app", not "logged out" (see session_test).
// Callers map these to auth-required; everything else is an ordinary
// error.
func tokenRejected(eresult int) bool {
	return eresult == 5 || eresult == 26 || eresult == 27
}

// logonTokenRejected marks the EResults that mean the token itself was
// refused at LOGON time — there, AccessDenied 15 is the CM's answer to
// a revoked token (a service-call AccessDenied means app permissions,
// which is why tokenRejected above stays narrower). The persistent
// Client uses this to stop reconnecting on a dead token.
func logonTokenRejected(eresult int) bool {
	return tokenRejected(eresult) || eresult == 15
}

// CMError is a failed CM exchange. EResult is the server's result (or 0
// for connection-level failures); Auth marks token-rejected results.
type CMError struct {
	Msg     string
	EResult int
	Auth    bool
}

func (e *CMError) Error() string { return e.Msg }
