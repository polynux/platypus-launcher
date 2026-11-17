package social

import (
	"regexp"
	"strconv"
	"strings"
)

// inviteTTLSeconds is how long a pending invite lives: 15 minutes. Core is
// the authority on expiry; the helper and the UI agree on the same window.
const inviteTTLSeconds = int64(15 * 60)

// maxPendingInvites caps one subscription's pending invites; beyond it the
// oldest invite makes room. Invites are one per friend, so this only trips
// on a misbehaving store.
const maxPendingInvites = 64

var (
	// A Steam id: 1-20 digits (uint64s are at most 20 digits).
	joinUintRe = regexp.MustCompile(`^\d{1,20}$`)
	// A join host: a hostname/IPv4 or a bracketed IPv6.
	joinHostRe = regexp.MustCompile(`^(?:[A-Za-z0-9.-]+|\[[0-9A-Fa-f:.]+\])$`)
	// A TCP port: 1-5 digits, range-checked after the match.
	joinPortRe = regexp.MustCompile(`^\d{1,5}$`)
	// A server password: 1-64 chars, no spaces, never +/- leading.
	joinPasswordRe = regexp.MustCompile(`^[A-Za-z0-9._~!@#$%^&*-]{1,64}$`)
)

// ParseJoinArgs validates a join against the strict allowlist (protocol
// 1.24) and returns the argv to launch with. kind is "connect" (a join
// command line) or "lobby" (a Steam lobby id). Anything else is refused:
// the remote friend controls launch argv, so only these exact token shapes
// ever reach an exec.
func ParseJoinArgs(kind, connect, lobby string) ([]string, bool) {
	if kind == "lobby" {
		id, ok := parseJoinUint(lobby)
		if !ok {
			return nil, false
		}
		return []string{"+connect_lobby", id}, true
	}
	if kind != "connect" {
		return nil, false
	}
	if len(connect) == 0 || len(connect) > maxConnectLen {
		return nil, false
	}
	toks := strings.Fields(connect)
	switch {
	case len(toks) == 2 && toks[0] == "+connect":
		if !validJoinAddr(toks[1]) {
			return nil, false
		}
		return []string{"+connect", toks[1]}, true
	case len(toks) == 4 && toks[0] == "+connect" && toks[2] == "+password":
		if !validJoinAddr(toks[1]) || !validJoinPassword(toks[3]) {
			return nil, false
		}
		return []string{"+connect", toks[1], "+password", toks[3]}, true
	case len(toks) == 2 && toks[0] == "+invite":
		id, ok := parseJoinUint(toks[1])
		if !ok {
			return nil, false
		}
		return []string{"+invite", id}, true
	case len(toks) == 3 && toks[0] == "+invite" && toks[2] == "-launchsource2":
		id, ok := parseJoinUint(toks[1])
		if !ok {
			return nil, false
		}
		return []string{"+invite", id, "-launchsource2"}, true
	default:
		return nil, false
	}
}

// parseJoinUint reads a Steam id: 1-20 digits that fit a uint64 and are not
// zero. Longer input (lobby ids past 20 digits) never fits.
func parseJoinUint(s string) (string, bool) {
	if !joinUintRe.MatchString(s) {
		return "", false
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || n == 0 {
		return "", false
	}
	return strconv.FormatUint(n, 10), true
}

// validJoinAddr checks host[:port][/password]: a hostname/IPv4 or bracketed
// IPv6, an optional 1-65535 port, and an optional server password.
func validJoinAddr(addr string) bool {
	hostport, pw, slash := strings.Cut(addr, "/")
	if slash && !validJoinPassword(pw) {
		return false
	}
	host, port, colon := cutJoinHostPort(hostport)
	if !joinHostRe.MatchString(host) {
		return false
	}
	if !colon {
		return true
	}
	if !joinPortRe.MatchString(port) {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n >= 1 && n <= 65535
}

// cutJoinHostPort splits host[:port], keeping a bracketed IPv6 address whole.
func cutJoinHostPort(hostport string) (host, port string, hasPort bool) {
	if strings.HasPrefix(hostport, "[") {
		end := strings.Index(hostport, "]")
		if end < 0 {
			return "", "", false
		}
		rest := hostport[end+1:]
		if rest == "" {
			return hostport[:end+1], "", false
		}
		if !strings.HasPrefix(rest, ":") {
			return "", "", true
		}
		return hostport[:end+1], rest[1:], true
	}
	if strings.Count(hostport, ":") > 1 {
		return "", "", true
	}
	h, p, c := strings.Cut(hostport, ":")
	return h, p, c
}

// validJoinPassword checks the server password: 1-64 chars, never +/-
// leading (the class already excludes + everywhere).
func validJoinPassword(pw string) bool {
	return joinPasswordRe.MatchString(pw) && !strings.HasPrefix(pw, "-")
}
