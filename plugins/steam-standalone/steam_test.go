// Tests the Steam-standalone plugin through Hydra's real plugin path
// (manager, JSON-RPC, job queue, launch-spec validation). Logins go
// through hydra-steam-dl (built from source) against the fake CM
// (testdata/fake_cm.py); downloads go through the same binary against the
// fake CM and a fake CDN (see fakesteam). It doesn't reach Steam or
// Valve's network.
package steam_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/settings"
	"hydra-launcher/internal/stores"
	"hydra-launcher/internal/types"
	"hydra-launcher/plugins/steam-standalone/fakesteam"
)

const id = "steam-standalone"

var outMu sync.Mutex

type backend struct{ m *plugins.Manager }

func (b backend) Size(ctx context.Context, j types.Job) (plugins.InstallSize, error) {
	var s plugins.InstallSize
	err := b.m.Call(ctx, j.Source, "game.installSize", map[string]any{"id": j.StoreGameID}, &s)
	return s, err
}

func (b backend) Start(ctx context.Context, j types.Job) error {
	p := map[string]any{"id": j.StoreGameID, "jobId": j.ID}
	if j.Kind != jobs.KindUninstall {
		p["installDir"] = j.InstallDir
	}
	return b.m.Call(ctx, j.Source, "game."+j.Kind, p, nil)
}

func (b backend) Cancel(ctx context.Context, j types.Job) error {
	return b.m.Call(ctx, j.Source, "job.cancel", map[string]any{"jobId": j.ID}, nil)
}

func copyFile(t *testing.T, src, dst string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Dir(dst), 0o755)
	if err := os.WriteFile(dst, data, mode); err != nil {
		t.Fatal(err)
	}
}

type env struct {
	t           *testing.T
	root        string
	m           *plugins.Manager
	q           *jobs.Queue
	store       *stores.Plugin
	folder      string // the plugin's install folder: Games/steam
	gamesFolder string // Hydra's Games folder
	cmState     string
	fix         *fakesteam.Fixture
	cdn         *fakesteam.CDN
}

// refreshCDN rebuilds the fake content from the current knobs (FAKE_DD_STATE
// switches like bare_elf/windows_only plus FAKE_DD_STEAMWORKS) and points
// the fake CM at it. Call after changing knobs, before installing.
func (e *env) refreshCDN() {
	e.t.Helper()
	e.fix = fakesteam.Build(fakesteam.ReadKnobs())
	e.cdn = fakesteam.StartCDN(e.t, e.fix)
	fakesteam.WriteCMState(e.t, e.cmState, e.cdn, e.fix)
}

func setup(t *testing.T) *env {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	root := t.TempDir()
	if keep := os.Getenv("STEAM_TEST_KEEP"); keep != "" {
		// Debugging aid: keep the plugin root so its log survives.
		var err error
		root, err = os.MkdirTemp("", "steamtest")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if os.Getenv("STEAM_TEST_KEEP") == "" {
				os.RemoveAll(root)
			}
		})
		t.Logf("test root: %s", root)
	}
	plug := filepath.Join(root, "plugins", id)
	copyFile(t, "plugin.json", filepath.Join(plug, "plugin.json"), 0o644)
	// The whole bin folder: the exec plus the steam_cm module it imports.
	binDir := filepath.Join(plug, "bin")
	os.MkdirAll(binDir, 0o755)
	binSrc, err := os.ReadDir("bin")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range binSrc {
		if !f.IsDir() && (strings.HasSuffix(f.Name(), ".py") || f.Name() == "hydra-steam") {
			copyFile(t, filepath.Join("bin", f.Name()), filepath.Join(binDir, f.Name()), 0o755)
		}
	}
	// Fixture variants (bare_elf/windows_only plus FAKE_DD_STEAMWORKS) ride
	// FAKE_DD_STATE for historical reasons; fakesteam.ReadKnobs reads them
	// to build the fake CDN. Logins use the fake CM state instead.
	state := filepath.Join(root, "dd-state.json")
	os.WriteFile(state, []byte(`{}`), 0o644)
	t.Setenv("FAKE_DD_STATE", state)
	// The plugin's Steam Web API calls answer from this file: GetOwnedGames
	// plus the appdetails entries under "_test" per app id.
	api := filepath.Join(root, "steam-api.json")
	os.WriteFile(api, []byte(`{
	  "response": {"games": [
	    {"appid": 4321, "name": "Standalone Game"},
	    {"appid": 4322, "name": "Tool Thing"}
	  ]},
	  "_test": {
	    "4321": {"success": true, "data": {"type": "game", "name": "Standalone Game",
	      "short_description": "A standalone game.", "genres": [{"description": "Action"}],
	      "screenshots": [{"path_full": "https://cdn.steam/4321_ss.jpg"}],
	      "developers": ["Dev"], "publishers": ["Pub"], "release_date": {"date": "2019-09-20"}}},
	    "4322": {"success": true, "data": {"type": "tool", "name": "Tool Thing"}}
	  }
	}`), 0o644)
	t.Setenv("HYDRA_STEAM_API_FILE", api)

	// The CM exchange answers from testdata/fake_cm.py: a real WebSocket CM
	// speaking the plugin's protocol on localhost (no Valve network).
	cmState := filepath.Join(root, "cm-state.json")
	os.WriteFile(cmState, []byte(`{}`), 0o644)
	t.Setenv("FAKE_CM_STATE", cmState)
	cmd := exec.Command("python3", "testdata/fake_cm.py")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Skipf("fake CM unavailable: %v", err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	deadline := time.Now().Add(10 * time.Second)
	endpoint := ""
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		s := out.String()
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			endpoint = "127.0.0.1:" + strings.TrimSpace(s[:i])
			break
		}
	}
	if endpoint == "" {
		t.Skip("fake CM didn't print a port")
	}
	t.Setenv("HYDRA_STEAM_CM_SERVER", endpoint)

	sets, _ := settings.Open(filepath.Join(root, "settings.json"))
	e := &env{t: t, root: root, folder: filepath.Join(root, "Games", "steam"), gamesFolder: filepath.Join(root, "Games"), cmState: cmState}
	// hydra-steam-dl is built once in TestMain (real environment) and only
	// copied here, the way install.sh ships it into the plugin's bin/.
	// Logins and downloads both go through it, against the fake CM.
	copyFile(t, prebuiltDL, filepath.Join(binDir, "hydra-steam-dl"), 0o755)
	// The fake CDN serves the fixture depot; the fake CM points the
	// downloader at it (see fakesteam).
	e.refreshCDN()
	var q *jobs.Queue
	m := plugins.NewManager(plugins.Options{
		Dir: filepath.Join(root, "plugins"), LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: sets,
		OnNotify: func(pid, method string, params json.RawMessage) {
			var u plugins.JobUpdate
			if json.Unmarshal(params, &u) == nil {
				q.Notify(pid, method, u)
			}
		},
		OnExit: func(pid string, err error) { q.PluginExited(pid, err) },
	})
	q, _ = jobs.Open(filepath.Join(root, "jobs.json"), backend{m}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	go q.Run(ctx)
	t.Cleanup(func() { cancel(); m.Shutdown() })

	if _, err := m.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := m.Approve(id); err != nil {
		t.Fatal(err)
	}
	p, _ := m.Plugin(id)
	if p.Status != plugins.StatusReady {
		t.Fatalf("not ready: %+v", p)
	}
	e.m, e.q, e.store = m, q, stores.NewPlugin(m, p, func() []string { return m.Folders(id, e.gamesFolder) })
	return e
}

func (e *env) call(method string, params, result any) error {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return e.m.Call(ctx, id, method, params, result)
}

func (e *env) login() {
	e.t.Helper()
	var st types.AuthStatus
	if err := e.call("auth.complete", map[string]any{"fields": map[string]string{
		"username": "HydraUser", "password": "correct-horse", "guard": "ABC12"}}, &st); err != nil || !st.LoggedIn {
		e.t.Fatalf("login: %+v, %v", st, err)
	}
	if !strings.Contains(st.User, "HydraUser") && !strings.Contains(st.User, "TestPersona") {
		e.t.Fatalf("user: %+v", st)
	}
}

func (e *env) library() []types.Game {
	e.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	lib, err := e.store.Library(ctx)
	if err != nil {
		e.t.Fatalf("library: %v", err)
	}
	return lib
}

func (e *env) find(games []types.Game, appID string) *types.Game {
	e.t.Helper()
	for i := range games {
		if games[i].StoreAppID == appID {
			return &games[i]
		}
	}
	e.t.Fatalf("no game %q in library (%d games)", appID, len(games))
	return nil
}

func (e *env) waitJob(jobID string) types.Job {
	e.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		for _, x := range e.q.List() {
			if x.ID == jobID && (x.State == jobs.StateDone || x.State == jobs.StateFailed) {
				return x
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	e.t.Fatalf("job never finished: %+v", e.q.List())
	return types.Job{}
}

// libraryDisk mirrors the plugin's private data folder, for flipping launch
// modes and checking persisted state the way the game page and session
// tracker would.
// setDDState edits the fixture state JSON (behaviour knobs like
// windows_only) for the fake CDN build.
func (e *env) setDDState(knob string, value any) {
	e.t.Helper()
	state := os.Getenv("FAKE_DD_STATE")
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

func (e *env) pluginData() string {
	return filepath.Join(e.root, "data", id)
}

// setLaunchMode rewrites the plugin's installed.json the way the game page's
// retry flow needs after a quick-exit Steamworks failure: mode "client".
func (e *env) setLaunchMode(appID, mode string) {
	e.t.Helper()
	var installed map[string]map[string]any
	data, err := os.ReadFile(filepath.Join(e.pluginData(), "installed.json"))
	if err == nil {
		json.Unmarshal(data, &installed)
	}
	if installed == nil || installed[appID] == nil {
		e.t.Fatalf("no installed entry for %s: %s", appID, string(data))
	}
	installed[appID]["mode"] = mode
	data, _ = json.Marshal(installed)
	os.WriteFile(filepath.Join(e.pluginData(), "installed.json"), data, 0o644)
}

// The email code from attempt 1 works on attempt 2 because both share ONE
// auth session: a retry must not start a new Begin (which would send a new
// email and invalidate the typed code). Exactly one Begin and one Update
// hit the wire.
func TestSteamPasswordTwoStepEmail(t *testing.T) {
	e := setup(t)
	e.setCMState("auth_guard", "email")
	var st types.AuthStatus
	// Attempt 1 carries no code: the dialog must be told which kind.
	err := e.call("auth.complete", map[string]any{"fields": map[string]string{
		"username": "HydraUser", "password": "correct-horse"}}, &st)
	if err == nil || !strings.Contains(err.Error(), "emailed") {
		t.Fatalf("want email-code prompt, got %+v, %v", st, err)
	}
	// Attempt 2 feeds the attempt-1 code into the SAME session.
	if err := e.call("auth.complete", map[string]any{"fields": map[string]string{
		"username": "HydraUser", "password": "correct-horse", "guard": "ABC12"}}, &st); err != nil || !st.LoggedIn {
		t.Fatalf("second attempt: %+v, %v", st, err)
	}
	if n := strings.Count(e.cmLog(), "Authentication.BeginAuthSessionViaCredentials#1"); n != 1 {
		t.Fatalf("want exactly 1 Begin, got %d", n)
	}
	if n := strings.Count(e.cmLog(), "Authentication.UpdateAuthSessionWithSteamGuardCode#1"); n != 1 {
		t.Fatalf("want exactly 1 Update, got %d", n)
	}
	if err := e.call("auth.logout", nil, &st); err != nil {
		t.Fatalf("logout: %v", err)
	}
}

// Same shape for TOTP codes: the kind is named, one Begin, one Update.
func TestSteamPasswordTwoStepTOTP(t *testing.T) {
	e := setup(t)
	e.setCMState("auth_guard", "totp")
	var st types.AuthStatus
	err := e.call("auth.complete", map[string]any{"fields": map[string]string{
		"username": "HydraUser", "password": "correct-horse"}}, &st)
	if err == nil || !strings.Contains(err.Error(), "authenticator") {
		t.Fatalf("want authenticator prompt, got %+v, %v", st, err)
	}
	if err := e.call("auth.complete", map[string]any{"fields": map[string]string{
		"username": "HydraUser", "password": "correct-horse", "guard": "123456"}}, &st); err != nil || !st.LoggedIn {
		t.Fatalf("second attempt: %+v, %v", st, err)
	}
	if n := strings.Count(e.cmLog(), "Authentication.BeginAuthSessionViaCredentials#1"); n != 1 {
		t.Fatalf("want exactly 1 Begin, got %d", n)
	}
	if err := e.call("auth.logout", nil, &st); err != nil {
		t.Fatalf("logout: %v", err)
	}
}

// Mobile confirmation needs no code at all: one submit waits on the same
// session until the phone approves.
func TestSteamPasswordMobileConfirm(t *testing.T) {
	e := setup(t)
	e.setCMState("auth_guard", "confirm")
	e.setCMState("auth_mobile_approved", true)
	var st types.AuthStatus
	if err := e.call("auth.complete", map[string]any{"fields": map[string]string{
		"username": "HydraUser", "password": "correct-horse"}}, &st); err != nil || !st.LoggedIn {
		t.Fatalf("confirm login: %+v, %v", st, err)
	}
	if n := strings.Count(e.cmLog(), "Authentication.BeginAuthSessionViaCredentials#1"); n != 1 {
		t.Fatalf("want exactly 1 Begin, got %d", n)
	}
	if err := e.call("auth.logout", nil, &st); err != nil {
		t.Fatalf("logout: %v", err)
	}
}

// The login uses hydra-steam-dl auth against the fake CM: the CM log must
// show the Authentication RPCs, and no depotdownloader call file may exist.
func (e *env) authLog() string {
	e.t.Helper()
	return e.cmLog()
}

func TestSteamAuth(t *testing.T) {
	e := setup(t)
	// Password logins need an email code from the fake CM.
	e.setCMState("auth_guard", "email")
	var st types.AuthStatus
	if err := e.call("auth.status", nil, &st); err != nil || st.LoggedIn {
		t.Fatalf("status: %+v, %v", st, err)
	}
	var pr types.AuthPrompt
	if err := e.call("auth.begin", nil, &pr); err != nil {
		t.Fatalf("begin: %v", err)
	}
	if pr.URL != "" || len(pr.Fields) != 3 {
		t.Fatalf("begin: %+v", pr)
	}
	if !strings.Contains(pr.QR, "█") {
		t.Fatalf("begin QR art missing block chars")
	}
	// Wrong password is rejected, and nothing is persisted.
	if err := e.call("auth.complete", map[string]any{"fields": map[string]string{
		"username": "HydraUser", "password": "wrong"}}, &st); err == nil {
		t.Fatal("wrong password accepted")
	} else if !strings.Contains(err.Error(), "wrong account name or password") {
		t.Fatalf("wrong-password message: %v", err)
	}
	if _, err := os.Stat(filepath.Join(e.pluginData(), "refresh.token")); !os.IsNotExist(err) {
		t.Fatal("refresh token written for a failed login")
	}
	// Bad guard code is rejected too.
	if err := e.call("auth.complete", map[string]any{"fields": map[string]string{
		"username": "HydraUser", "password": "correct-horse", "guard": "WRONG"}}, &st); err == nil {
		t.Fatal("bad guard code accepted")
	} else if !strings.Contains(err.Error(), "wrong Steam Guard code") {
		t.Fatalf("bad-code message: %v", err)
	}
	e.login()
	// The login went through the Go helper's Authentication RPCs, not a
	// depotdownloader CLI: no .calls file from the old fake may exist.
	if _, err := os.Stat(os.Getenv("FAKE_DD_STATE") + ".calls"); !os.IsNotExist(err) {
		t.Fatal("depotdownloader fake was called")
	}
	for _, want := range []string{"Authentication.GetPasswordRSAPublicKey#1",
		"Authentication.BeginAuthSessionViaCredentials#1",
		"Authentication.UpdateAuthSessionWithSteamGuardCode#1",
		"Authentication.PollAuthSessionStatus#1"} {
		if !strings.Contains(e.authLog(), want) {
			t.Fatalf("CM log missing %q: %s", want, e.authLog())
		}
	}
	// The refresh token lives inside the plugin's data folder (0600), not
	// the user's real home, with the account alongside it.
	fi, err := os.Stat(filepath.Join(e.pluginData(), "refresh.token"))
	if err != nil {
		t.Fatalf("refresh token not kept inside HYDRA_PLUGIN_DATA: %v", err)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		t.Fatalf("refresh.token too open: %v", fi.Mode())
	}
	var tok struct {
		Account string `json:"account"`
	}
	raw, _ := os.ReadFile(filepath.Join(e.pluginData(), "token.json"))
	json.Unmarshal(raw, &tok)
	if tok.Account != "HydraUser" {
		t.Fatalf("token account: %q", tok.Account)
	}
	if got := os.Getenv("HOME"); got != "" {
		if _, err := os.Stat(filepath.Join(got, ".local/share/IsolatedStorage/a1b2c3d4.9z8y")); err == nil {
			t.Fatal("isolated storage leaked into the real home")
		}
	}
	if err := e.call("auth.logout", nil, &st); err != nil || st.LoggedIn {
		t.Fatalf("logout: %+v, %v", st, err)
	}
	if _, err := os.Stat(filepath.Join(e.pluginData(), "refresh.token")); !os.IsNotExist(err) {
		t.Fatal("login key survived logout")
	}
}

// Token-bearing files must end up 0600 even when they already exist as
// 0644 (pre-fix saves): the login overwrites refresh.token/token.json
// through the Go helper (chmod after write) and library.list writes and
// re-touches steam-access.json through the plugin (save_secret/tighten).
// session.json carries no secret and stays 0644.
func TestSteamTokenFiles0600(t *testing.T) {
	e := setup(t)
	e.setCMState("auth_guard", "email")
	// Pre-create the token files world-readable; the flows below must
	// tighten them.
	os.WriteFile(filepath.Join(e.pluginData(), "refresh.token"), []byte("stale\n"), 0o644)
	os.WriteFile(filepath.Join(e.pluginData(), "token.json"), []byte(`{"refresh_token":"stale"}`), 0o644)
	e.login()
	for _, name := range []string{"refresh.token", "token.json"} {
		fi, err := os.Stat(filepath.Join(e.pluginData(), name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if fi.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s mode %v, want 0600", name, fi.Mode())
		}
	}
	if lib := e.library(); len(lib) == 0 {
		t.Fatal("empty library")
	}
	fi, err := os.Stat(filepath.Join(e.pluginData(), "steam-access.json"))
	if err != nil {
		t.Fatalf("steam-access.json: %v", err)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		t.Fatalf("steam-access.json mode %v, want 0600", fi.Mode())
	}
	// A cached access token is tightened on read, without re-exchange.
	os.Chmod(filepath.Join(e.pluginData(), "steam-access.json"), 0o644)
	if lib := e.library(); len(lib) == 0 {
		t.Fatal("empty library from cache")
	}
	if fi, _ := os.Stat(filepath.Join(e.pluginData(), "steam-access.json")); fi.Mode().Perm()&0o077 != 0 {
		t.Fatalf("steam-access.json mode after cached read %v, want 0600", fi.Mode())
	}
	if _, err := os.Stat(filepath.Join(e.pluginData(), "session.json")); err != nil {
		t.Fatalf("session.json: %v", err)
	}
}

// The first auth.status right after a login already shows the display
// (persona) name, not the account name: the login path resolves and
// stores it before reporting loggedIn (the fake CM answers TestPersona).
// No library.list happens first.
func TestSteamPersonaAtLogin(t *testing.T) {
	e := setup(t)
	e.login()
	var st types.AuthStatus
	if err := e.call("auth.status", nil, &st); err != nil || !st.LoggedIn {
		t.Fatalf("status: %+v, %v", st, err)
	}
	if st.User != "TestPersona" {
		t.Fatalf("want persona TestPersona right after login, got %q", st.User)
	}
}

func TestSteamLibraryInstallLaunchUninstall(t *testing.T) {
	e := setup(t)
	e.login()

	// Two entries: the game and a tool. The tool is owned but not
	// installable; ids are p.steam-standalone.<appid> with source
	// steam-standalone (never "steam", so built-in Steam can't clash).
	lib := e.library()
	var game *types.Game
	for i := range lib {
		g := &lib[i]
		if g.Source != id || g.Store != types.StoreSteam {
			t.Fatalf("wrong source/store: %+v", g)
		}
		if g.StoreAppID == "4321" {
			game = g
		} else if g.StoreAppID == "4322" && g.Installable {
			t.Fatalf("tool marked installable: %+v", g)
		}
	}
	if game == nil || game.IsInstalled || !game.Installable {
		t.Fatalf("game entry: %+v", game)
	}
	if game.ID != stores.GameID(id, "4321") {
		t.Fatalf("id: %s", game.ID)
	}

	var size plugins.InstallSize
	if err := e.call("game.installSize", map[string]any{"id": "4321"}, &size); err != nil {
		t.Fatalf("installSize: %v", err)
	}
	if size.DiskBytes != int64(e.fix.DiskBytes) || size.DownloadBytes != int64(e.fix.CompressedBytes) {
		t.Fatalf("size: %+v, want disk=%d compressed=%d", size, e.fix.DiskBytes, e.fix.CompressedBytes)
	}

	// Install through the job queue, with full progress.
	j, err := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if err != nil {
		t.Fatal(err)
	}
	done := e.waitJob(j.ID)
	if done.State != jobs.StateDone || done.Percent != 100 || done.Total <= 0 || done.Bytes != done.Total {
		t.Fatalf("install: %+v", done)
	}

	lib = e.library()
	game = e.find(lib, "4321")
	if !game.IsInstalled || game.OutsideFolder {
		t.Fatalf("after install: %+v", game)
	}

	// Mode (a): the fake planted no Steamworks, so the game launches
	// directly and the game page says so.
	l, err := e.store.Launch(context.Background(), *game, true)
	if err != nil || l.Direct == nil {
		t.Fatalf("launch preview: %+v, %v", l, err)
	}
	l, err = e.store.Launch(context.Background(), *game, false)
	if err != nil || l.Direct == nil {
		t.Fatalf("launch: %+v, %v", l, err)
	}
	if l.Direct.NeedsWine {
		t.Fatalf("linux build marked needsWine: %+v", l.Direct)
	}
	if l.Direct.Env["SteamAppId"] != "" {
		t.Fatalf("mode (a) set SteamAppId: %+v", l.Direct.Env)
	}
	// The game itself runs: the fake's game.sh echoes.
	out, err := exec.Command(l.Direct.Exe, l.Direct.Args...).Output()
	if err != nil || !strings.Contains(string(out), "steam standalone game") {
		t.Fatalf("game script: %s, %v", out, err)
	}

	// Details come from the store API.
	d, err := e.store.Details(context.Background(), *game)
	if err != nil || d.Summary != "A standalone game." || len(d.Screenshots) != 1 || len(d.Genres) != 1 {
		t.Fatalf("details: %+v, %v", d, err)
	}

	// Uninstall removes the folder and the installed flag.
	u, err := e.q.Add(jobs.KindUninstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if err != nil {
		t.Fatal(err)
	}
	if got := e.waitJob(u.ID); got.State != jobs.StateDone {
		t.Fatalf("uninstall: %+v", got)
	}
	lib = e.library()
	if g := e.find(lib, "4321"); g.IsInstalled {
		t.Fatal("still installed")
	}
	entries, _ := os.ReadDir(e.folder)
	if len(entries) != 0 {
		t.Fatalf("install folder not empty: %v", entries)
	}
}

// Mode (b): with a Steamworks library planted, the spec still launches
// directly but carries SteamAppId/SteamGameId, and the game page says
// "optional".
// Store lookups are cached on disk: a rescan with the Store unreachable (or
// slow, or rate limiting) keeps names and the tool/game split.
func TestSteamStoreDetailsCached(t *testing.T) {
	e := setup(t)
	e.login()
	e.library()
	if _, err := os.Stat(filepath.Join(e.pluginData(), "store-apps.json")); err != nil {
		t.Fatalf("no store cache: %v", err)
	}
	// Now the Store knows nothing; the owned list still answers.
	api := os.Getenv("HYDRA_STEAM_API_FILE")
	raw, err := os.ReadFile(api)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	json.Unmarshal(raw, &doc)
	doc["_test"] = map[string]any{}
	raw, _ = json.Marshal(doc)
	os.WriteFile(api, raw, 0o644)

	lib := e.library()
	tool := e.find(lib, "4322")
	if tool == nil || tool.Installable {
		t.Fatalf("tool after cache-only scan: %+v", tool)
	}
	if game := e.find(lib, "4321"); game == nil || game.Title != "Standalone Game" || !game.Installable {
		t.Fatalf("game after cache-only scan: %+v", game)
	}
}

// Downloads use the Steam client's download region (or HYDRA_STEAM_CELLID)
// as the server-list cell id hydra-steam-dl requests.
func TestSteamDownloadCellID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	os.MkdirAll(filepath.Join(home, ".local/share/Steam/config"), 0o755)
	os.WriteFile(filepath.Join(home, ".local/share/Steam/config/config.vdf"),
		[]byte("\"InstallConfigStore\"\n{\n\t\"Steam\"\n\t{\n\t\t\"CellIDServerOverride\"\t\t\"14\"\n\t}\n}\n"), 0o644)
	e := setup(t)
	e.login()
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	if log := e.cmLog(); !strings.Contains(log, "servers cell=14") {
		t.Fatalf("download without the Steam client's region: %s", log)
	}
}

func TestSteamDownloadCellIDEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no Steam client config
	t.Setenv("HYDRA_STEAM_CELLID", "96")
	e := setup(t)
	e.login()
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	if log := e.cmLog(); !strings.Contains(log, "servers cell=96") {
		t.Fatalf("env cell id ignored: %s", log)
	}
}

// Some depots ship the Linux binary without an extension or exec bit.
func TestSteamNativeBinaryWithoutExecBit(t *testing.T) {
	e := setup(t)
	e.login()
	e.setDDState("bare_elf", true)
	e.refreshCDN()
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	game = e.find(e.library(), "4321")
	if !game.Native {
		t.Fatalf("not native: %+v", game)
	}
	l, err := e.store.Launch(context.Background(), *game, false)
	if err != nil || l.Direct == nil {
		t.Fatalf("launch: %+v %v", l, err)
	}
	if filepath.Base(l.Direct.Exe) != "BareGame" {
		t.Fatalf("exe: %+v", l.Direct)
	}
	if fi, err := os.Stat(l.Direct.Exe); err != nil || fi.Mode()&0o100 == 0 {
		t.Fatalf("exec bit not set: %v %v", fi, err)
	}
}

// A Unity build ships a bigger launcher next to <name>.x86_64 + <name>_Data;
// the player is the game.
func TestSteamNativeUnityPlayerPreferred(t *testing.T) {
	e := setup(t)
	e.login()
	e.setDDState("bare_elf", true)
	e.setDDState("unity_decoy", true)
	e.refreshCDN()
	game := e.find(e.library(), "4321")
	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	game = e.find(e.library(), "4321")
	l, err := e.store.Launch(context.Background(), *game, false)
	if err != nil || l.Direct == nil || filepath.Base(l.Direct.Exe) != "Game.x86_64" {
		t.Fatalf("launch: %+v %v", l, err)
	}
}

func TestSteamModeOptional(t *testing.T) {
	t.Setenv("FAKE_DD_STEAMWORKS", "steam_api64.dll")
	e := setup(t)
	e.login()
	e.setDDState("windows_only", true)
	e.refreshCDN()
	lib := e.library()
	game := e.find(lib, "4321")

	j, _ := e.q.Add(jobs.KindInstall, id, game.ID, game.StoreAppID, game.Title, e.folder)
	if got := e.waitJob(j.ID); got.State != jobs.StateDone {
		t.Fatalf("install: %+v", got)
	}
	game = e.find(e.library(), "4321")
	if game.Native {
		t.Fatalf("windows build marked native: %+v", game)
	}
	if !strings.Contains(game.Note, "optional") {
		t.Fatalf("mode note missing from game page: %+v", game.Note)
	}

	l, err := e.store.Launch(context.Background(), *game, false)
	if err != nil || l.Direct == nil {
		t.Fatalf("launch: %+v, %v", l, err)
	}
	if !l.Direct.NeedsWine {
		t.Fatalf("windows build must need wine: %+v", l.Direct)
	}
	if l.Direct.Env["SteamAppId"] != "4321" || l.Direct.Env["SteamGameId"] != "4321" {
		t.Fatalf("mode (b) env: %+v", l.Direct.Env)
	}

	// A direct launch that dies with a Steamworks init error flips the game
	// to mode (c) — the game page's retry calls the plugin, which rewrites
	// the mode. Here the state change is driven directly, then the launch
	// spec must become a client delegation.
	e.setLaunchMode("4321", "client")
	l, err = e.store.Launch(context.Background(), *game, true)
	if err != nil {
		t.Fatalf("launch after client mode: %v", err)
	}
	if l.Client == nil {
		t.Fatalf("expected client launch: %+v", l)
	}
	if l.Client.Command[0] != "steam" || !contains(l.Client.Command, "-silent") || !contains(l.Client.Command, "4321") {
		t.Fatalf("client command: %+v", l.Client.Command)
	}
}

// Mode (c) validation: the delegate command must use a tool from `requires`
// (steam is on it) and identify the game's process for session tracking.
func TestSteamModeClientRequiresSteamTool(t *testing.T) {
	// `steam` IS in the manifest's requires list, so the delegate validates.
	man := struct {
		Requires []struct {
			Bin string `json:"bin"`
		} `json:"requires"`
	}{}
	data, err := os.ReadFile("plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(data, &man) != nil {
		t.Fatal("plugin.json unreadable")
	}
	found := false
	for _, r := range man.Requires {
		if r.Bin == "steam" {
			found = true
		}
	}
	if !found {
		t.Fatal("manifest must require steam for mode (c) client launches")
	}
}

// Coexistence: a game owned through built-in Steam (steam_4321) and through
// the plugin (p.steam-standalone.4321) must key differently, and the plugin's
// xml/games folder never mixes with Steam's.
func TestCoexistenceWithBuiltinSteam(t *testing.T) {
	if got := stores.GameID("steam-standalone", "4321"); got != "p.steam-standalone.4321" {
		t.Fatalf("plugin game id: %s", got)
	}
	if stores.GameID("steam", "4321") == stores.GameID("steam-standalone", "4321") {
		t.Fatal("plugin and built-in Steam ids collide")
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// The QR login path: auth.begin starts hydra-steam-dl auth in the
// background and returns the challenge as block-art; approving it (the
// test flips the fake CM's auth_qr_approved knob, like the Steam mobile
// app would) makes an empty auth.complete log in.
func TestSteamQRAuth(t *testing.T) {
	e := setup(t)
	var st types.AuthStatus
	if err := e.call("auth.status", nil, &st); err != nil || st.LoggedIn {
		t.Fatalf("status: %+v, %v", st, err)
	}
	var pr types.AuthPrompt
	if err := e.call("auth.begin", nil, &pr); err != nil {
		t.Fatalf("begin: %v", err)
	}
	checkQRArt(t, pr.QR)

	// Approve from the "phone" and confirm the empty-form flow logs in.
	e.setCMState("auth_qr_approved", true)
	if err := e.call("auth.complete", map[string]any{"fields": map[string]string{}}, &st); err != nil || !st.LoggedIn {
		t.Fatalf("qr complete: %+v, %v", st, err)
	}
	// The persona is already resolved: no library.list happened yet.
	if st.User != "TestPersona" {
		t.Fatalf("want persona TestPersona right after QR login, got %q", st.User)
	}
	// The refresh token lives inside the plugin's private data folder.
	if _, err := os.Stat(filepath.Join(e.pluginData(), "refresh.token")); err != nil {
		t.Fatalf("login key not in plugin data: %v", err)
	}
	// A library call works with the QR-issued token.
	if lib := e.library(); len(lib) == 0 {
		t.Fatal("empty library after QR login")
	}
	// Logging out wipes everything, including any running QR process state.
	if err := e.call("auth.logout", nil, &st); err != nil || st.LoggedIn {
		t.Fatalf("logout: %+v, %v", st, err)
	}

	// A fresh QR login after logout works too.
	var pr2 types.AuthPrompt
	if err := e.call("auth.begin", nil, &pr2); err != nil {
		t.Fatalf("begin: %v", err)
	}
	checkQRArt(t, pr2.QR)
	e.setCMState("auth_qr_approved", true)
	if err := e.call("auth.complete", map[string]any{"fields": map[string]string{}}, &st); err != nil || !st.LoggedIn {
		t.Fatalf("second qr complete: %+v, %v", st, err)
	}
}

// A duplicate auth.begin (React StrictMode sends two back to back) must be
// harmless: both calls return the same QR, no error, exactly one helper
// stays alive and none is left a zombie. On the old code the second call
// killed the first helper, whose watcher failed the shared state with
// "the login helper ended without logging in".
func TestSteamQRConcurrentBegin(t *testing.T) {
	e := setup(t)
	const n = 2
	prs := make([]types.AuthPrompt, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = e.call("auth.begin", nil, &prs[i])
		}(i)
	}
	wg.Wait()
	for i := range errs {
		if errs[i] != nil {
			t.Fatalf("begin %d: %v", i, errs[i])
		}
		checkQRArt(t, prs[i].QR)
	}
	if prs[0].QR != prs[1].QR {
		t.Fatal("concurrent begins served different QR codes")
	}
	alive, zombie := e.countAuthHelpers()
	if alive != 1 {
		t.Fatalf("want exactly 1 live auth helper, got %d", alive)
	}
	if zombie != 0 {
		t.Fatalf("want no defunct auth helpers, got %d", zombie)
	}
	if err := e.call("auth.logout", nil, &types.AuthStatus{}); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if alive, _ := e.countAuthHelpers(); alive != 0 {
		t.Fatalf("want no auth helpers after logout, got %d", alive)
	}
}

// countAuthHelpers counts this test's live vs zombie hydra-steam-dl auth
// processes: direct children of the harness's plugin process only, so
// helpers from parallel packages, other tests, or the user's dev launcher
// never leak into the count. (An earlier version scraped `ps` output for
// an args substring and even matched the test runner itself.)
func (e *env) countAuthHelpers() (alive, zombie int) {
	e.t.Helper()
	mine := e.m.Running(id)
	if mine == 0 {
		e.t.Fatal("plugin process is not running")
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		e.t.Fatalf("no /proc: %v", err)
	}
	for _, de := range entries {
		pid, err := strconv.Atoi(de.Name())
		if err != nil {
			continue
		}
		ppid, comm, state, cmdline, ok := procInfo(pid)
		if !ok || ppid != mine {
			continue
		}
		if state == "Z" {
			// Zombies have no cmdline; comm (15 chars) still names
			// the binary. Only auth helpers spawn in this test.
			if comm == "hydra-steam-dl" {
				zombie++
			}
			continue
		}
		if procExe(cmdline) != "hydra-steam-dl" || !slices.Contains(cmdline, "auth") {
			continue
		}
		alive++
	}
	return alive, zombie
}

// procInfo reads one /proc entry: parent pid, comm, state and argv.
func procInfo(pid int) (ppid int, comm, state string, cmdline []string, ok bool) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, "", "", nil, false
	}
	s := string(raw)
	open, close := strings.IndexByte(s, '('), strings.LastIndexByte(s, ')')
	if open < 0 || close < 0 || close < open {
		return 0, "", "", nil, false
	}
	comm = s[open+1 : close]
	rest := strings.Fields(s[close+2:])
	if len(rest) < 2 {
		return 0, "", "", nil, false
	}
	state = rest[0]
	if n, err := strconv.Atoi(rest[1]); err == nil {
		ppid = n
	} else {
		return 0, "", "", nil, false
	}
	argv, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return 0, "", "", nil, false
	}
	for _, a := range strings.Split(string(argv), "\x00") {
		if a != "" {
			cmdline = append(cmdline, a)
		}
	}
	return ppid, comm, state, cmdline, true
}

// procExe basenames argv[0], "" when absent (zombies).
func procExe(cmdline []string) string {
	if len(cmdline) == 0 {
		return ""
	}
	exe := cmdline[0]
	if i := strings.LastIndexByte(exe, '/'); i >= 0 {
		exe = exe[i+1:]
	}
	return exe
}

// checkQRArt asserts the block-art is a square QR with the finder
// pattern intact (module = two "█" chars; rows without dark modules are
// quiet-zone padding the renderer drops, so only dark-bearing rows count).
func checkQRArt(t *testing.T, art string) {
	t.Helper()
	if !strings.Contains(art, "█") {
		t.Fatal("QR art has no block chars")
	}
	var rows []string
	for _, r := range strings.Split(art, "\n") {
		if strings.Contains(r, "█") {
			rows = append(rows, r)
		}
	}
	if len(rows) < 21 {
		t.Fatalf("QR art: %d content rows, want at least 21", len(rows))
	}
	runes := func(s string) []rune { return []rune(s) }
	// Width in modules from the widest row (two runes per module); the
	// block must be square.
	widest := 0
	for _, r := range rows {
		if n := len(runes(strings.TrimRight(r, " "))) / 2; n > widest {
			widest = n
		}
	}
	if len(rows) != widest {
		t.Fatalf("QR art not square: %d rows, %d modules wide", len(rows), widest)
	}
	// Finder corners: (0,0), (6,0), (0,6), (6,6) dark, (1,1) light, (3,3) dark.
	at := func(x, y int) bool {
		r := runes(rows[y])
		return len(r) > x*2+1 && r[x*2] == '█'
	}
	for _, m := range [][2]int{{0, 0}, {6, 0}, {0, 6}, {6, 6}, {3, 3}} {
		if !at(m[0], m[1]) {
			t.Fatalf("finder module (%d,%d) not dark", m[0], m[1])
		}
	}
	if at(1, 1) {
		t.Fatal("module (1,1) should be light")
	}
}

// Steam expires the login QR and hands out a new challenge URL; the Go
// helper emits a fresh qr event and the plugin serves the latest art
// through auth.status so the dialog can redraw it.
func TestSteamQRRefresh(t *testing.T) {
	e := setup(t)
	e.setCMState("auth_qr_refresh", true)
	var pr types.AuthPrompt
	if err := e.call("auth.begin", nil, &pr); err != nil {
		t.Fatalf("begin: %v", err)
	}
	checkQRArt(t, pr.QR)
	first := pr.QR
	// The fake refreshes on the first poll; auth.status must pick the new
	// art up.
	var st types.AuthStatus
	deadline := time.Now().Add(15 * time.Second)
	for {
		if err := e.call("auth.status", nil, &st); err != nil {
			t.Fatalf("status: %v", err)
		}
		if st.LoggedIn {
			t.Fatal("status claims logged in before any approval")
		}
		if st.QR != "" && st.QR != first {
			break // refreshed
		}
		if time.Now().After(deadline) {
			t.Fatal("auth.status never served the refreshed QR code")
		}
		time.Sleep(300 * time.Millisecond)
	}
	checkQRArt(t, st.QR)
	// And the refreshed code still logs in when approved.
	e.setCMState("auth_qr_approved", true)
	if err := e.call("auth.complete", map[string]any{"fields": map[string]string{}}, &st); err != nil || !st.LoggedIn {
		t.Fatalf("qr complete after refresh: %+v, %v", st, err)
	}
}
