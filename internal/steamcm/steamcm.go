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
	"fmt"
)

// EMsg numbers (steam/enums_clientserver.proto).
const (
	EMsgMulti                       = 1
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
	// The obfuscated private ip doubles as the login id: a fixed value
	// keeps a logon from kicking an existing session.
	LoginID     = 2718281
	MachineName = "Hydra"
)

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
// no good". Callers map these to auth-required; everything else is an
// ordinary error.
func tokenRejected(eresult int) bool {
	return eresult == 5 || eresult == 26 || eresult == 27
}

// CMError is a failed CM exchange. EResult is the server's result (or 0
// for connection-level failures); Auth marks token-rejected results.
type CMError struct {
	Msg     string
	EResult int
	Auth    bool
}

func (e *CMError) Error() string { return e.Msg }
