package plugtest_test

// One conformance test per bundled plugin. Each wires only what's specific to
// it — how to build it, where its fake CLI is, how its login flow works and
// which game to drive through install/launch — and the shared Suite checks the
// whole protocol the plugin declares (see plugtest.go).

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"platypus/internal/plugins"
	"platypus/internal/plugtest"
	"platypus/internal/testenv"
	"platypus/internal/types"
	"platypus/plugins/steam-standalone/fakesteam"
)

// The full protocol suites drive real plugin processes (Python plugins,
// fake CLIs, chunk-paced downloads): slow, opt-in (issue #40).

// echo builds from source and answers everything itself, no external CLI.
func TestConformanceEcho(t *testing.T) {
	testenv.RequireSlow(t)
	plugtest.Suite{
		ID:      "echo",
		CopyDir: "../../plugins/echo",
		BuildCmd: []string{"go", "build", "-o", "plugins/echo/bin/platypus-echo",
			"platypus/plugins/echo"},
		BuildOut: "plugins/echo/bin/platypus-echo",
		// Echo's uninstall is instant, so there is nothing for job.cancel to stop.
		NoUninstallCancel: true,
		Login: func(t *testing.T, s *plugtest.Session) {
			var st types.AuthStatus
			if err := s.Call("auth.complete", map[string]any{"fields": map[string]string{"code": "letmein"}}, &st); err != nil || !st.LoggedIn {
				t.Fatalf("login: %+v, %v", st, err)
			}
		},
		PickGame: func(t *testing.T, games []types.Game) types.Game {
			return pick(t, games, "tiny")
		},
	}.Conformance(t)
}

// echoAllCapabilities declares every capability in the registry (the unused
// ones too), then lets initialize switch some off and report features.
func echoAllCapabilities(env map[string]string) plugtest.Suite {
	return plugtest.Suite{
		ID:      "echo",
		CopyDir: "../../plugins/echo",
		BuildCmd: []string{"go", "build", "-o", "plugins/echo/bin/platypus-echo",
			"platypus/plugins/echo"},
		BuildOut: "plugins/echo/bin/platypus-echo",
		Env:      env,
		EditManifest: func(m map[string]any) {
			var names []string
			for _, c := range plugins.Registry() {
				names = append(names, c.Name)
			}
			m["capabilities"] = names
		},
	}
}

func echoCalls(t *testing.T, s *plugtest.Session) map[string]int {
	t.Helper()
	var calls map[string]int
	if err := s.Call("calls", nil, &calls); err != nil {
		t.Fatal(err)
	}
	return calls
}

// With every capability on, echo answers every method the registry lists.
func TestHandshakeEchoEveryCapabilityOn(t *testing.T) {
	sess := echoAllCapabilities(nil).Handshake(t)
	calls := echoCalls(t, sess)
	for _, c := range plugins.Registry() {
		if slices.Contains([]string{"library", "install", "launch", "auth", "discard"}, c.Name) {
			continue
		}
		for _, m := range c.Methods {
			if calls[m] == 0 {
				t.Errorf("%s: the suite never probed %s", c.Name, m)
			}
		}
	}
}

// Capabilities declared in the manifest but off at initialize are never
// sent a request, features are reported only for what is on.
func TestHandshakeEchoSwitchedOffCapabilities(t *testing.T) {
	sess := echoAllCapabilities(map[string]string{
		"ECHO_OFF":      "versions,verify,social",
		"ECHO_FEATURES": `{"install-options":{"dlc":true,"languages":false}}`,
	}).Handshake(t)
	calls := echoCalls(t, sess)
	for _, m := range []string{"versions.list", "game.verify", "social.start"} {
		if calls[m] != 0 {
			t.Errorf("%s reached the plugin although its capability is off", m)
		}
	}
	if calls["game.options"] == 0 {
		t.Error("game.options (install-options is on) wasn't probed")
	}
}

// A plugin that speaks only 1.5 is never sent social.*: the capability is
// introduced by 1.6, so it stays off at runtime although the manifest lists it.
func TestSocialStaysOffForAProtocol15Plugin(t *testing.T) {
	s := echoAllCapabilities(map[string]string{"ECHO_PROTOCOLS": "1.0,1.1,1.2,1.3,1.4,1.5"})
	s.EditManifest = func(m map[string]any) {
		m["protocol"] = map[string]any{"versions": []string{"1.0", "1.1", "1.2", "1.3", "1.4", "1.5"}}
	}
	sess := s.Handshake(t)
	if sess.Has("social") {
		t.Fatal("social is on for a plugin that settled on 1.5")
	}
	var raw map[string]any
	if err := sess.Call("social.start", map[string]any{}, &raw); err == nil {
		t.Fatal("social.start reached a 1.5 plugin")
	}
	if n := echoCalls(t, sess)["social.start"]; n != 0 {
		t.Fatalf("the plugin received social.start %d time(s)", n)
	}
}

// A plugin from before negotiation: no protocol in the answer.
func TestHandshakeEchoWithoutNegotiation(t *testing.T) {
	echoAllCapabilities(map[string]string{"ECHO_NO_PROTOCOL": "1"}).Handshake(t)
}

// A plugin that speaks only 1.0 and says so negotiates 1.0 with a Platypus that
// speaks 1.1 too.
func TestHandshakeEchoOlderPlugin(t *testing.T) {
	s := echoAllCapabilities(map[string]string{"ECHO_PROTOCOLS": "1.0"})
	edit := s.EditManifest
	s.EditManifest = func(m map[string]any) {
		edit(m)
		m["protocol"] = map[string]any{"versions": []string{"1.0"}}
	}
	s.Handshake(t)
}

func pick(t *testing.T, games []types.Game, storeID string) types.Game {
	t.Helper()
	for _, g := range games {
		if g.StoreAppID == storeID {
			return g
		}
	}
	t.Fatalf("library has no game %q", storeID)
	return types.Game{}
}

// epic-legendary wraps the real legendary CLI, replaced here by a fake that
// keeps its state in FAKE_LEGENDARY_STATE.
func TestConformanceEpic(t *testing.T) {
	testenv.RequireSlow(t)
	state := filepath.Join(t.TempDir(), "legendary-state.json")
	plugtest.Suite{
		ID:      "epic-legendary",
		CopyDir: "../../plugins/epic-legendary",
		FakeCLI: map[string]string{"legendary": "../../plugins/epic-legendary/testdata/legendary"},
		Env:     map[string]string{"FAKE_LEGENDARY_STATE": state},
		Login: func(t *testing.T, s *plugtest.Session) {
			var st types.AuthStatus
			err := s.Call("auth.complete", map[string]any{
				"fields": map[string]string{"code": "abcdef0123456789abcdef0123456789"}}, &st)
			if err != nil || !st.LoggedIn {
				t.Fatalf("login: %+v, %v", st, err)
			}
		},
		PickGame: func(t *testing.T, games []types.Game) types.Game {
			return pick(t, games, "Goose")
		},
	}.Conformance(t)
}

// gog-gogdl keeps its own install index in the plugin's data dir; the fake
// gogdl drives it without the network.
func TestConformanceGog(t *testing.T) {
	testenv.RequireSlow(t)
	state := filepath.Join(t.TempDir(), "gogdl-state.json")
	// The plugin's GOG API calls (library, details) answer from this file.
	api := filepath.Join(t.TempDir(), "gog-api.json")
	os.WriteFile(api, []byte(`{"totalPages": 1, "products": [{"id": 1234, "title": "Super Game", "isGame": true, "image": "//img.gog/box"}],
  "_test": {"id": 1234, "description": {"lead": "A super game."}, "genres": [{"name": "Action"}], "downloads": {"installers": [{"os": "linux", "language": "en-US"}]}}}`), 0o644)
	plugtest.Suite{
		ID:      "gog-gogdl",
		CopyDir: "../../plugins/gog-gogdl",
		FakeCLI: map[string]string{"gogdl": "../../plugins/gog-gogdl/testdata/gogdl"},
		Env:     map[string]string{"FAKE_GOGDL_STATE": state, "PLATYPUS_GOG_API_FILE": api},
		// The API file answers every id; unknown-id details is unit-tested in
		// plugins/gog-gogdl.
		NoUnknownDetails: true,
		Login: func(t *testing.T, s *plugtest.Session) {
			var st types.AuthStatus
			err := s.Call("auth.complete", map[string]any{
				"fields": map[string]string{
					"code": "https://embed.gog.com/on_login_success?origin=client&code=Xa1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"}}, &st)
			if err != nil || !st.LoggedIn {
				t.Fatalf("login: %+v, %v", st, err)
			}
		},
		PickGame: func(t *testing.T, games []types.Game) types.Game {
			return pick(t, games, "1234")
		},
	}.Conformance(t)
}

// amazon-nile wraps nile; the fake keeps its state in FAKE_NILE_STATE and the
// login is a redirect address carrying the authorization code.
func TestConformanceAmazon(t *testing.T) {
	testenv.RequireSlow(t)
	state := filepath.Join(t.TempDir(), "nile-state.json")
	plugtest.Suite{
		ID:      "amazon-nile",
		CopyDir: "../../plugins/amazon-nile",
		FakeCLI: map[string]string{"nile": "../../plugins/amazon-nile/testdata/nile"},
		Env:     map[string]string{"FAKE_NILE_STATE": state},
		Login: func(t *testing.T, s *plugtest.Session) {
			var st types.AuthStatus
			err := s.Call("auth.complete", map[string]any{
				"fields": map[string]string{
					"code": "https://www.amazon.com/?openid.oa2.authorization_code=AQCODE123&openid.oa2.token_type=bearer"}}, &st)
			if err != nil || !st.LoggedIn {
				t.Fatalf("login: %+v, %v", st, err)
			}
		},
		PickGame: func(t *testing.T, games []types.Game) types.Game {
			return pick(t, games, "amzn1.tt.1")
		},
	}.Conformance(t)
}

// startFakeCM launches the fake CM server and returns the exec.Cmd (its
// lifetime is tied to t.Cleanup) plus the "host:port" endpoint to connect to.
func startFakeCM(t *testing.T, path string) (*exec.Cmd, string) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	var out syncBuf
	cmd := exec.Command("python3", path)
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Skipf("fake CM unavailable: %v", err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		s := out.String()
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			return cmd, "127.0.0.1:" + strings.TrimSpace(s[:i])
		}
	}
	t.Fatal("fake CM didn't print a port")
	return cmd, ""
}

// steam-standalone logs in and downloads through platypus-steam-dl (built
// from source) against a fake CM and a fake CDN (see fakesteam); the
// plugin's Steam Web API calls answer from a file.
func TestConformanceSteamStandalone(t *testing.T) {
	testenv.RequireSlow(t)
	state := filepath.Join(t.TempDir(), "dd-state.json")
	// Several chunks served one at a time: the download has progress steps
	// and a window in which job.cancel can land.
	os.WriteFile(state, []byte(`{"extra_files": 8, "chunk_delay_ms": 150}`), 0o644)
	api := filepath.Join(t.TempDir(), "steam-api.json")
	os.WriteFile(api, []byte(`{
  "response": {"games": [{"appid": 4321, "name": "Standalone Game"}]},
  "_test": {"4321": {"success": true, "data": {"type": "game", "name": "Standalone Game",
    "short_description": "A standalone game.", "genres": [{"description": "Action"}],
    "screenshots": [{"path_full": "https://cdn.steam/4321_ss.jpg"}],
    "developers": ["Dev"], "publishers": ["Pub"], "release_date": {"date": "2019-09-20"}}}}
}`), 0o644)
	cmState := filepath.Join(t.TempDir(), "cm-state.json")
	os.WriteFile(cmState, []byte(`{}`), 0o644)
	// The fake CM must run while the plugin process does; its port is set
	// per-run, so it can't come in through Suite.Env. The state path must be
	// exported here too — Suite.Env applies only at plugin start.
	t.Setenv("FAKE_CM_STATE", cmState)
	_, endpoint := startFakeCM(t, "../../plugins/steam-standalone/testdata/fake_cm.py")
	// platypus-steam-dl comes from a PATH copy (the copied plugin tree has no
	// built binary); it was built once in TestMain, before any test could
	// override HOME. The fake CDN serves the fixture depot and the fake CM
	// points the downloader at it.
	t.Setenv("PATH", prebuiltDLDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_DD_STATE", state)
	fix := fakesteam.Build(fakesteam.ReadKnobs())
	cdn := fakesteam.StartCDN(t, fix)
	fakesteam.WriteCMState(t, cmState, cdn, fix)
	plugtest.Suite{
		ID:      "steam-standalone",
		CopyDir: "../../plugins/steam-standalone",
		// Steam's store answers success:false for delisted apps and unknown ids
		// alike, so an unknown id gets an empty page, not an error.
		NoUnknownDetails: true,
		Env: map[string]string{"FAKE_DD_STATE": state, "PLATYPUS_STEAM_API_FILE": api,
			"PLATYPUS_STEAM_CM_SERVER": endpoint, "FAKE_CM_STATE": cmState},
		Login: func(t *testing.T, s *plugtest.Session) {
			var st types.AuthStatus
			err := s.Call("auth.complete", map[string]any{
				"fields": map[string]string{"username": "PlatypusUser", "password": "correct-horse", "guard": "ABC12"}}, &st)
			if err != nil || !st.LoggedIn {
				t.Fatalf("login: %+v, %v", st, err)
			}
		},
		PickGame: func(t *testing.T, games []types.Game) types.Game {
			return pick(t, games, "4321")
		},
	}.Conformance(t)
}

// syncBuf is a bytes.Buffer safe to read while os/exec's copy goroutine writes.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// Every state echo's cloud can be put in answers the shapes the protocol defines.
func TestConformanceEchoCloudStates(t *testing.T) {
	for _, state := range []string{"synced", "pending", "conflict", "conflict2", "unsupported", "offline", "error"} {
		t.Run(state, func(t *testing.T) {
			s := echoCloudSuite(map[string]string{"ECHO_CLOUD": state})
			s.CloudConformance(t)
		})
	}
	t.Run("client running", func(t *testing.T) {
		echoCloudSuite(map[string]string{"ECHO_CLOUD": "pending", "ECHO_CLOUD_CLIENT": "1"}).CloudConformance(t)
	})
}

func echoCloudSuite(env map[string]string) plugtest.Suite {
	return plugtest.Suite{
		ID:      "echo",
		CopyDir: "../../plugins/echo",
		BuildCmd: []string{"go", "build", "-o", "plugins/echo/bin/platypus-echo",
			"platypus/plugins/echo"},
		BuildOut: "plugins/echo/bin/platypus-echo",
		Env:      env,
		Login: func(t *testing.T, s *plugtest.Session) {
			var st types.AuthStatus
			if err := s.Call("auth.complete", map[string]any{"fields": map[string]string{"code": "letmein"}}, &st); err != nil || !st.LoggedIn {
				t.Fatalf("login: %+v, %v", st, err)
			}
		},
		PickGame: func(t *testing.T, games []types.Game) types.Game { return pick(t, games, "tiny") },
	}
}

// A plugin that speaks only 1.1 is never sent cloud.*: the capability stays
// off at runtime although the manifest lists it.
func TestCloudSavesStayOffForAProtocol11Plugin(t *testing.T) {
	s := echoCloudSuite(map[string]string{"ECHO_PROTOCOLS": "1.0,1.1"})
	s.EditManifest = func(m map[string]any) {
		m["protocol"] = map[string]any{"versions": []string{"1.0", "1.1"}}
	}
	sess := s.Handshake(t)
	if sess.Has("cloud-saves") {
		t.Fatal("cloud-saves is on for a plugin that settled on 1.1")
	}
	var raw map[string]any
	if err := sess.Call("cloud.status", map[string]any{"id": "tiny", "ctx": types.CloudContext{OS: "linux", InstallDir: "/x"}}, &raw); err == nil {
		t.Fatal("cloud.status reached a 1.1 plugin")
	}
	if n := echoCalls(t, sess)["cloud.status"]; n != 0 {
		t.Fatalf("the plugin received cloud.status %d time(s)", n)
	}
}
