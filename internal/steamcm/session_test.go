package steamcm

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func testToken(steamID uint64) string {
	// A JWT-shaped refresh token; only sub and exp matter here.
	return "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJzdWIiOiI3NjU2MTE5OCJ9." +
		"SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJVadQssw5c"
}

var currentFake *fakeCM

func startFake(t *testing.T) (*fakeCM, string) {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	f, host := startFakeCM(t, fakeConfig{
		DepotKey:    key,
		AppTokens:   map[uint32]uint64{4321: 12345},
		ProductKV:   "\"depots\"\n{\n\"11\"\n{\n\"manifests\"\n{\n\"public\"\n{\n\"gid\"\t\t\"999\"\n}\n}\n}\n}\n",
		Servers:     []Server{{Type: "SteamCache", Host: "cache1-par1.steamcontent.com", UseHTTPS: true, WeightedLoad: 1}, {Type: "CDN", Host: "cache2-par1.steamcontent.com", UseHTTPS: true, WeightedLoad: 2}},
		RequestCode: 4242,
		CDNToken:    "cdn-token",
		BetaOK:      true,
		Licenses:    []uint32{77},
	})
	currentFake = f
	return f, host
}

func setLogonEresult(t *testing.T, v int) {
	t.Helper()
	currentFake.mu.Lock()
	currentFake.cfg.LogonEresult = v
	currentFake.mu.Unlock()
}

func setServiceEresult(t *testing.T, v int) {
	t.Helper()
	currentFake.mu.Lock()
	currentFake.cfg.ServiceEresult = v
	currentFake.mu.Unlock()
}

func setWrapMulti(t *testing.T, v bool) {
	t.Helper()
	currentFake.mu.Lock()
	currentFake.cfg.WrapLogonMulti = v
	currentFake.mu.Unlock()
}

func setHeartbeat(t *testing.T, v bool) {
	t.Helper()
	currentFake.mu.Lock()
	currentFake.cfg.SendHeartbeat = v
	currentFake.mu.Unlock()
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}

func readLine(r interface{ Read([]byte) (int, error) }, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total : total+1])
		total += n
		if n > 0 && buf[total-1] == '\n' {
			break
		}
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func withFakeCM(t *testing.T, hook string) {
	t.Helper()
	t.Setenv("HYDRA_STEAM_CM_SERVER", hook)
}

func TestExchangeHappyPath(t *testing.T) {
	f, host := startFake(t)
	withFakeCM(t, host)
	_ = f
	tok, err := Exchange(context.Background(), testToken(76561198))
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" {
		t.Fatal("empty access token")
	}
	if exp := TokenExpiry(tok); time.Until(exp) < time.Hour-time.Minute {
		t.Fatalf("token ttl too short: %v", exp)
	}
}

func TestExchangeErrorsAreHonest(t *testing.T) {
	_, host := startFake(t)
	withFakeCM(t, host)
	setLogonEresult(t, 27) // Expired
	_, err := Exchange(context.Background(), testToken(76561198))
	ce, ok := err.(*CMError)
	if !ok || !ce.Auth {
		t.Fatalf("Expired must be auth: %v", err)
	}
	if !strings.Contains(err.Error(), "Expired") {
		t.Fatalf("message must name the EResult: %v", err)
	}
	setLogonEresult(t, 20) // ServiceUnavailable stays ordinary
	_, err = Exchange(context.Background(), testToken(76561198))
	if ce, ok := err.(*CMError); !ok || ce.Auth {
		t.Fatalf("ServiceUnavailable must NOT be auth: %v", err)
	}
	setLogonEresult(t, 1)
	setServiceEresult(t, 15) // AccessDenied on exchange is not "logged out"
	_, err = Exchange(context.Background(), testToken(76561198))
	if ce, ok := err.(*CMError); !ok || ce.Auth {
		t.Fatalf("AccessDenied on exchange must NOT be auth: %v", err)
	}
	setServiceEresult(t, 1)
}

func TestMultiAndHeartbeat(t *testing.T) {
	f, host := startFake(t)
	withFakeCM(t, host)
	setWrapMulti(t, true)
	setHeartbeat(t, true)
	if _, err := Exchange(context.Background(), testToken(76561198)); err != nil {
		t.Fatalf("multi-wrapped logon: %v", err)
	}
	// The fake records the reply when its read loop gets to it; poll
	// briefly rather than assuming it already has.
	deadline := time.Now().Add(5 * time.Second)
	for !contains(f.log(), "heartbeat_reply") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !contains(f.log(), "heartbeat_reply") {
		t.Fatalf("heartbeat ignored: %v", f.log())
	}
	setWrapMulti(t, false)
	setHeartbeat(t, false)
}

func TestContentRPCs(t *testing.T) {
	_, host := startFake(t)
	withFakeCM(t, host)
	ctx := context.Background()
	s, err := Dial(ctx, testToken(76561198))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	key, err := s.DepotKey(11, 4321, 0)
	if err != nil || len(key) != 32 {
		t.Fatalf("depot key: %v %d", err, len(key))
	}
	// Licences arrive after logon and are consumed by the first call.
	if len(s.Licenses()) != 1 || s.Licenses()[0].PackageID != 77 {
		t.Fatalf("licenses: %+v", s.Licenses())
	}
	toks, err := s.PICSAccessTokens([]uint32{4321}, nil, 0)
	if err != nil || toks[4321] != 12345 {
		t.Fatalf("pics tokens: %v %+v", err, toks)
	}
	kv, _, err := s.ProductInfo(4321, 12345, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := kv.String("depots", "11", "manifests", "public", "gid"); got != "999" {
		t.Fatalf("product info gid: %q", got)
	}
	servers, err := s.ServersForSteamPipe(14, 20, 0)
	if err != nil || len(servers) != 2 {
		t.Fatalf("servers: %v %+v", err, servers)
	}
	if !servers[0].UseHTTPS || servers[0].Host == "" {
		t.Fatalf("server fields: %+v", servers[0])
	}
	code, err := s.ManifestRequestCode(4321, 11, 999, "public", "", 0)
	if err != nil || code != 4242 {
		t.Fatalf("request code: %v %d", code, err)
	}
	token, _, err := s.CDNAuthToken(11, "cache1-par1.steamcontent.com", 4321, 0)
	if err != nil || token != "cdn-token" {
		t.Fatalf("cdn token: %v %q", err, token)
	}
	if err := s.CheckBetaPassword(4321, "secret", 0); err != nil {
		t.Fatalf("beta password: %v", err)
	}
}

// Interop: the Go client against the Python fake CM (the protocol the
// plugin's tests drive). Skipped without python3.
func TestExchangeAgainstPythonFakeCM(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	state := t.TempDir() + "/cm-state.json"
	os.WriteFile(state, []byte(`{}`), 0o644)
	t.Setenv("FAKE_CM_STATE", state)
	cmd := exec.Command("python3", "../../plugins/steam-standalone/testdata/fake_cm.py")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Skipf("fake CM unavailable: %v", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Skipf("fake CM unavailable: %v", err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	buf := make([]byte, 64)
	n, _ := readLine(out, buf)
	if n == 0 {
		t.Skip("fake CM didn't print a port")
	}
	endpoint := "127.0.0.1:" + strings.TrimSpace(string(buf[:n]))
	withFakeCM(t, endpoint)
	tok, err := Exchange(context.Background(), testToken(76561198))
	if err != nil {
		t.Fatal(err)
	}
	if tok == "" {
		t.Fatal("empty token from python fake")
	}
}
