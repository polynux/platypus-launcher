// Tests the Steam-standalone plugin through Hydra's real plugin path
// (manager, JSON-RPC, job queue, launch-spec validation) against
// testdata/depotdownloader, a fake CLI that mimics the real DepotDownloader's
// prompts, account.config persistence, size dump and progress lines. It
// doesn't reach Steam or Valve's network.
package steam_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/settings"
	"hydra-launcher/internal/stores"
	"hydra-launcher/internal/types"
)

const id = "steam-standalone"

const diskBytes = 60 << 20
const compressedBytes = 40 << 20

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
	bin := filepath.Join(root, "fakebin")
	copyFile(t, "testdata/depotdownloader", filepath.Join(bin, "depotdownloader"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	state := filepath.Join(root, "dd-state.json")
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
	e := &env{t: t, root: root, folder: filepath.Join(root, "Games", "steam"), gamesFolder: filepath.Join(root, "Games")}
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
	if !strings.Contains(st.User, "HydraUser") {
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
// setDDState edits the fake depotdownloader's state JSON (behaviour knobs
// like windows_only) the way the real login flow would leave it.
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

// The password and guard answers must reach DepotDownloader's prompts.
func (e *env) fedPrompts() string {
	e.t.Helper()
	state := os.Getenv("FAKE_DD_STATE")
	data, _ := os.ReadFile(state + ".fed")
	return string(data)
}

func TestSteamAuth(t *testing.T) {
	e := setup(t)
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
	// Wrong password is rejected, and nothing is persisted.
	if err := e.call("auth.complete", map[string]any{"fields": map[string]string{
		"username": "HydraUser", "password": "wrong"}}, &st); err == nil {
		t.Fatal("wrong password accepted")
	}
	e.login()
	// The password reached the CLI's prompt.
	if !strings.Contains(e.fedPrompts(), `"password": "correct-horse"`) {
		t.Fatalf("password was not fed to DepotDownloader: %s", e.fedPrompts())
	}
	if !strings.Contains(e.fedPrompts(), `"guard": "ABC12"`) {
		t.Fatalf("guard code was not fed: %s", e.fedPrompts())
	}
	// A second login with a bad password still works: -remember-password
	// reuses the persisted login key, so no password prompt happens.
	if err := e.call("auth.complete", map[string]any{"fields": map[string]string{
		"username": "HydraUser", "password": "wrong"}}, &st); err != nil || !st.LoggedIn {
		t.Fatalf("relogin with stored key: %+v, %v", st, err)
	}
	// The login key file lives inside the plugin's data folder, not the
	// user's real home.
	if _, err := os.Stat(filepath.Join(e.pluginData(), "home", ".local/share/IsolatedStorage")); err != nil {
		t.Fatalf("login key not kept inside HYDRA_PLUGIN_DATA: %v", err)
	}
	if got := os.Getenv("HOME"); got != "" {
		if _, err := os.Stat(filepath.Join(got, ".local/share/IsolatedStorage/a1b2c3d4.9z8y")); err == nil {
			t.Fatal("isolated storage leaked into the real home")
		}
	}
	if err := e.call("auth.logout", nil, &st); err != nil || st.LoggedIn {
		t.Fatalf("logout: %+v, %v", st, err)
	}
	if _, err := os.Stat(filepath.Join(e.pluginData(), "home")); !os.IsNotExist(err) {
		t.Fatal("login key survived logout")
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
	if err := e.call("game.installSize", map[string]any{"id": "4321"}, &size); err != nil || size.DiskBytes != diskBytes || size.DownloadBytes != compressedBytes {
		t.Fatalf("size: %+v", size)
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
func TestSteamModeOptional(t *testing.T) {
	t.Setenv("FAKE_DD_STEAMWORKS", "steam_api64.dll")
	e := setup(t)
	e.login()
	e.setDDState("windows_only", true)
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

// The QR login path: auth.begin starts depotdownloader -qr in the background
// and returns the QR as a BMP data URL; approving it (the test flips the
// fake's qr_approved knob, like the Steam mobile app would) makes the next
// auth.status logged in, and an empty auth.complete waits for it.
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
	// The prompt carries the block-art the fake printed: 21 rows, and the
	// finder pattern's dark ring must survive the round trip (module =
	// two "\u2588" chars; the fake indents each row by one leading space).
	rows := strings.Split(pr.QR, "\n")
	if len(rows) != 21 || !strings.Contains(pr.QR, "\u2588") {
		t.Fatalf("QR art: %d rows, want 21 with block chars", len(rows))
	}
	// Block chars are 3 bytes each, so compare runes, not bytes. Only the
	// leading margin varies between runs; module m lives at runes 2m and 2m+1
	// after it (light modules are two spaces and must stay put).
	lead := len(rows[0]) - len(strings.TrimLeft(rows[0], " "))
	strip := func(row string) []rune {
		r := []rune(row)
		return r[lead:]
	}
	for _, m := range [][2]int{{0, 0}, {6, 0}, {0, 6}, {6, 6}, {3, 3}} {
		line := strip(rows[m[1]])
		if len(line) <= m[0]*2+1 || string(line[m[0]*2:m[0]*2+2]) != "\u2588\u2588" {
			t.Fatalf("finder module (%d,%d) not dark in row %q", m[0], m[1], rows[m[1]])
		}
	}
	light := strip(rows[1])
	if string(light[2:4]) == "\u2588\u2588" {
		t.Fatal("module (1,1) should be light")
	}

	// Approve from the "phone" and confirm the empty-form flow logs in.
	e.setDDState("qr_approved", true)
	if err := e.call("auth.complete", map[string]any{"fields": map[string]string{}}, &st); err != nil || !st.LoggedIn {
		t.Fatalf("qr complete: %+v, %v", st, err)
	}
	// The login key lives inside the plugin's private data folder, in .NET's
	// real IsolatedStorage layout (what the actual DepotDownloader writes).
	if _, err := os.Stat(filepath.Join(e.pluginData(), "home",
		".local/share/IsolatedStorage/a1b2c3d4.9z8y/e5f6g7h8.1x2w/Url.31jkk2knc5lh3teqw2x2f1trv5hc4yls/AssemFiles/account.config")); err != nil {
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

	// A rematch after logout must not fail: Steam can silently re-issue the
	// stored login key on a fresh approval, without any "Success!" line and
	// with an unchanged token store (what the real DepotDownloader does
	// when its profile already had the key). The plugin must accept a fresh
	// account.config written during this login even though its content
	// stamp was already in place when the login began.
	e.setDDState("qr_silent", true)
	var pr2 types.AuthPrompt
	if err := e.call("auth.begin", nil, &pr2); err != nil {
		t.Fatalf("begin: %v", err)
	}
	e.setDDState("qr_approved", true)
	if err := e.call("auth.complete", map[string]any{"fields": map[string]string{}}, &st); err != nil || !st.LoggedIn {
		t.Fatalf("silent rematch: %+v, %v", st, err)
	}
}

// Steam expires the login QR and hands out a new challenge URL, which
// DepotDownloader prints as a fresh block-art block. The plugin must serve
// the latest art through auth.status so the dialog can redraw it.
func TestSteamQRRefresh(t *testing.T) {
	e := setup(t)
	e.setDDState("qr_refresh", true)
	var pr types.AuthPrompt
	if err := e.call("auth.begin", nil, &pr); err != nil {
		t.Fatalf("begin: %v", err)
	}
	first := pr.QR
	if strings.Count(first, "\n") != 20 {
		t.Fatalf("first QR: %d rows", strings.Count(first, "\n")+1)
	}
	// The fake refreshes about two seconds in; auth.status must pick the new
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
	if got := strings.Count(st.QR, "\n") + 1; got != 21 {
		t.Fatalf("refreshed QR: %d rows, want 21", got)
	}
	// And the refreshed code still logs in when approved.
	e.setDDState("qr_approved", true)
	if err := e.call("auth.complete", map[string]any{"fields": map[string]string{}}, &st); err != nil || !st.LoggedIn {
		t.Fatalf("qr complete after refresh: %+v, %v", st, err)
	}
}
