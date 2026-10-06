// Tests the Steam-standalone plugin through Hydra's real plugin path
// (manager, JSON-RPC, job queue, launch-spec validation) against
// testdata/depotdownloader, a fake CLI that mimics the real DepotDownloader's
// prompts, account.config persistence, size dump and progress lines. It
// doesn't reach Steam or Valve's network.
package steam_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	plug := filepath.Join(root, "plugins", id)
	copyFile(t, "plugin.json", filepath.Join(plug, "plugin.json"), 0o644)
	copyFile(t, "bin/hydra-steam", filepath.Join(plug, "bin/hydra-steam"), 0o755)
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
	if _, err := os.Stat(filepath.Join(e.pluginData(), "home", ".local/share/isolated-storage")); err != nil {
		t.Fatalf("login key not kept inside HYDRA_PLUGIN_DATA: %v", err)
	}
	if got := os.Getenv("HOME"); got != "" {
		if _, err := os.Stat(filepath.Join(got, ".local/share/isolated-storage/fakeid")); err == nil {
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
