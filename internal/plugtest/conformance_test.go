package plugtest_test

// One conformance test per bundled plugin. Each wires only what's specific to
// it — how to build it, where its fake CLI is, how its login flow works and
// which game to drive through install/launch — and the shared Suite checks the
// whole protocol the plugin declares (see plugtest.go).

import (
	"os"
	"path/filepath"
	"testing"

	"hydra-launcher/internal/plugtest"
	"hydra-launcher/internal/types"
)

// echo builds from source and answers everything itself, no external CLI.
func TestConformanceEcho(t *testing.T) {
	plugtest.Suite{
		ID:      "echo",
		CopyDir: "../../plugins/echo",
		BuildCmd: []string{"go", "build", "-o", "plugins/echo/bin/hydra-echo",
			"hydra-launcher/plugins/echo"},
		BuildOut: "plugins/echo/bin/hydra-echo",
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
	state := filepath.Join(t.TempDir(), "gogdl-state.json")
	// The plugin's GOG API calls (library, details) answer from this file.
	api := filepath.Join(t.TempDir(), "gog-api.json")
	os.WriteFile(api, []byte(`{"totalPages": 1, "products": [{"id": 1234, "title": "Super Game", "isGame": true, "image": "//img.gog/box"}],
  "_test": {"id": 1234, "description": {"lead": "A super game."}, "genres": [{"name": "Action"}], "downloads": {"installers": [{"os": "linux", "language": "en-US"}]}}}`), 0o644)
	plugtest.Suite{
		ID:      "gog-gogdl",
		CopyDir: "../../plugins/gog-gogdl",
		FakeCLI: map[string]string{"gogdl": "../../plugins/gog-gogdl/testdata/gogdl"},
		Env:     map[string]string{"FAKE_GOGDL_STATE": state, "HYDRA_GOG_API_FILE": api},
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

// steam-standalone wraps DepotDownloader; the fake mimics its prompts,
// account.config login-key persistence, manifest sizes and progress lines,
// and the plugin's Steam Web API calls answer from a file.
func TestConformanceSteamStandalone(t *testing.T) {
	state := filepath.Join(t.TempDir(), "dd-state.json")
	api := filepath.Join(t.TempDir(), "steam-api.json")
	os.WriteFile(api, []byte(`{
  "response": {"games": [{"appid": 4321, "name": "Standalone Game"}]},
  "_test": {"4321": {"success": true, "data": {"type": "game", "name": "Standalone Game",
    "short_description": "A standalone game.", "genres": [{"description": "Action"}],
    "screenshots": [{"path_full": "https://cdn.steam/4321_ss.jpg"}],
    "developers": ["Dev"], "publishers": ["Pub"], "release_date": {"date": "2019-09-20"}}}}
}`), 0o644)
	plugtest.Suite{
		ID:      "steam-standalone",
		CopyDir: "../../plugins/steam-standalone",
		FakeCLI: map[string]string{"depotdownloader": "../../plugins/steam-standalone/testdata/depotdownloader"},
		Env:     map[string]string{"FAKE_DD_STATE": state, "HYDRA_STEAM_API_FILE": api},
		Login: func(t *testing.T, s *plugtest.Session) {
			var st types.AuthStatus
			err := s.Call("auth.complete", map[string]any{
				"fields": map[string]string{"username": "HydraUser", "password": "correct-horse", "guard": "ABC12"}}, &st)
			if err != nil || !st.LoggedIn {
				t.Fatalf("login: %+v, %v", st, err)
			}
		},
		PickGame: func(t *testing.T, games []types.Game) types.Game {
			return pick(t, games, "4321")
		},
	}.Conformance(t)
}
