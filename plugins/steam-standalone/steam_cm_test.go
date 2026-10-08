// CM token-exchange behaviour, through the plugin's real JSON-RPC surface
// and the fake CM server (testdata/fake_cm.py): honest error codes, the
// access-token cache, heartbeats, Multi-wrapped responses and the persona
// name. The wire protocol itself is exercised for real — only the CM and
// DepotDownloader endpoints are fakes.
package steam_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hydra-launcher/internal/jsonrpc"
)

// setCMState edits the fake CM's knobs (logon_eresult, service_eresult,
// wrap_logon_multi, send_heartbeat, token_ttl).
func (e *env) setCMState(knob string, value any) {
	e.t.Helper()
	state := os.Getenv("FAKE_CM_STATE")
	var st map[string]any
	if data, err := os.ReadFile(state); err == nil {
		json.Unmarshal(data, &st)
	}
	if st == nil {
		st = map[string]any{}
	}
	st[knob] = value
	data, _ := json.Marshal(st)
	if err := os.WriteFile(state, data, 0o644); err != nil {
		e.t.Fatal(err)
	}
}

// cmLog returns what the fake CM saw on the wire.
func (e *env) cmLog() string {
	e.t.Helper()
	data, err := os.ReadFile(os.Getenv("FAKE_CM_STATE") + ".log")
	if err != nil {
		return ""
	}
	return string(data)
}

// rpcCode extracts the JSON-RPC error code from a failed call.
func (e *env) rpcCode(err error) int {
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) {
		e.t.Fatalf("not a jsonrpc error: %v", err)
	}
	return rpcErr.Code
}

// A rejected-but-real session: Steam (the fake CM) says Expired, the Library
// must call it auth-required — but ServiceUnavailable must stay a normal
// error with the real message, so the Library's banner can't say "logged
// out" for a flaky CM.
func TestSteamCMErrorsAreHonest(t *testing.T) {
	e := setup(t)
	e.login()

	// The login warms the access-token cache (persona resolution); force
	// a fresh exchange so the knob flips below take effect.
	e.expireAccessCache()

	e.setCMState("logon_eresult", 27) // Expired
	err := e.call("library.list", nil, nil)
	if code := e.rpcCode(err); code != -32001 {
		t.Fatalf("Expired must be auth-required (-32001), got %d: %v", code, err)
	}
	if !strings.Contains(err.Error(), "Expired") {
		t.Fatalf("message must name the EResult: %v", err)
	}

	e.setCMState("logon_eresult", 20) // ServiceUnavailable
	err = e.call("library.list", nil, nil)
	if code := e.rpcCode(err); code == -32001 {
		t.Fatalf("ServiceUnavailable must NOT be auth-required: %v", err)
	}

	// The API-side token exchange being denied is also not "logged out":
	// the token itself is fine, Steam refused to derive one.
	e.setCMState("logon_eresult", 1)
	e.setCMState("service_eresult", 15) // AccessDenied
	err = e.call("library.list", nil, nil)
	if code := e.rpcCode(err); code == -32001 {
		t.Fatalf("AccessDenied on exchange must NOT be auth-required: %v", err)
	}
	if !strings.Contains(err.Error(), "AccessDenied") {
		t.Fatalf("message must name the EResult: %v", err)
	}
}

// The happy path: logon, one CM round trip, the access token cached in the
// plugin's data folder, the library served and the persona name reported.
// A second library.list must not touch the CM again (cache until exp).
func TestSteamCMExchangeAndCache(t *testing.T) {
	e := setup(t)
	// The ttl knob must be in place before the login: login resolves the
	// persona, which mints and caches the access token (the default lifetime
	// would cache a ~1h exp that the expiry assert below sits within a
	// second of — a flake under load, with the 7200 knob never observed).
	e.setCMState("token_ttl", 7200)
	e.login()

	if lib := e.library(); len(lib) == 0 {
		t.Fatal("empty library")
	}
	if st := readJSON(t, filepath.Join(e.pluginData(), "steam-access.json")); st["access_token"] == "" {
		t.Fatalf("access token not cached: %v", st)
	} else if st["exp"].(float64) < float64(time.Now().Add(time.Hour).Unix()) {
		t.Fatalf("cache expiry too near: %v", st["exp"])
	}
	first := strings.Count(e.cmLog(), "\nconnect")

	// Persona name comes from GetPlayerSummaries via the _routes hook.
	var st struct {
		LoggedIn bool
		User     string `json:"user"`
	}
	if err := e.call("auth.status", nil, &st); err != nil || !st.LoggedIn || st.User != "TestPersona" {
		t.Fatalf("auth.status: %+v, %v", st, err)
	}

	// Second list: served from the cache, no new CM connection.
	if lib := e.library(); len(lib) == 0 {
		t.Fatal("empty library from cache")
	}
	if got := strings.Count(e.cmLog(), "\nconnect"); got != first {
		t.Fatalf("cache miss: %d CM connections after two library.list calls", got)
	}

	// Wire sanity: the plugin really logged on, called the service and
	// logged off again.
	log := e.cmLog()
	for _, want := range []string{"logon ", "service Authentication.GenerateAccessTokenForApp#1", "logoff"} {
		if !strings.Contains(log, want) {
			t.Fatalf("CM log missing %q: %s", want, log)
		}
	}
	if strings.Contains(log, "unexpected") {
		t.Fatalf("plugin surprised the fake CM: %s", log)
	}

	// A fresh access token arrives when the cached one expires: rewrite the
	// cached token to one already past its exp, then the next library.list
	// re-contacts the CM — which the fake now rejects, proving the contact
	// happened.
	e.setCMState("logon_eresult", 27)
	cache := readJSON(t, filepath.Join(e.pluginData(), "steam-access.json"))
	expired := strings.SplitN(cache["access_token"].(string), ".", 3)
	payload := `{"exp": ` + fmt.Sprint(time.Now().Add(-time.Minute).Unix()) + `, "sub": "76561198"}`
	expired[1] = base64.RawURLEncoding.EncodeToString([]byte(payload))
	cache["access_token"] = strings.Join(expired, ".")
	writeJSON(t, filepath.Join(e.pluginData(), "steam-access.json"), cache)
	err := e.call("library.list", nil, nil)
	if err == nil {
		t.Fatal("expired cache must re-exchange (and the fake rejects it)")
	}
}

// Robust framing: a logon response that only arrives inside a Multi (gzip)
// envelope, plus a heartbeat from the CM that must be answered on the wire.
func TestSteamCMMultiAndHeartbeat(t *testing.T) {
	e := setup(t)
	e.login()

	// The login warms the access-token cache; force a fresh exchange so
	// the Multi-wrapped logon and heartbeat below are actually exercised.
	e.expireAccessCache()

	e.setCMState("wrap_logon_multi", true)
	e.setCMState("send_heartbeat", true)
	if lib := e.library(); len(lib) == 0 {
		t.Fatal("empty library with Multi-wrapped logon response")
	}
	if !strings.Contains(e.cmLog(), "heartbeat_reply") {
		t.Fatalf("plugin ignored the CM heartbeat: %s", e.cmLog())
	}
}

// expireAccessCache rewrites the cached access token with a past exp so
// the next call forces a fresh CM exchange (logins warm the cache while
// resolving the persona).
func (e *env) expireAccessCache() {
	e.t.Helper()
	cache := readJSON(e.t, filepath.Join(e.pluginData(), "steam-access.json"))
	expired := strings.SplitN(cache["access_token"].(string), ".", 3)
	payload := `{"exp": ` + fmt.Sprint(time.Now().Add(-time.Minute).Unix()) + `, "sub": "76561198"}`
	expired[1] = base64.RawURLEncoding.EncodeToString([]byte(payload))
	cache["access_token"] = strings.Join(expired, ".")
	writeJSON(e.t, filepath.Join(e.pluginData(), "steam-access.json"), cache)
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		t.Fatalf("unreadable %s", path)
	}
	return m
}

func writeJSON(t *testing.T, path string, m any) {
	t.Helper()
	data, _ := json.Marshal(m)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
