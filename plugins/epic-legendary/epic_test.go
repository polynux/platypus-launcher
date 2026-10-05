// Tests the Epic plugin through Hydra's real plugin path (manager, JSON-RPC,
// job queue, launch-spec validation) against testdata/legendary, a fake CLI
// that prints the real legendary's output formats. They don't reach Epic.
package epic_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/jsonrpc"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/settings"
	"hydra-launcher/internal/stores"
	"hydra-launcher/internal/types"
)

const id = "epic-legendary"

type env struct {
	m      *plugins.Manager
	q      *jobs.Queue
	store  *stores.Plugin
	state  string
	folder string
	games  string
}

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

func setup(t *testing.T) *env {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not installed")
	}
	root := t.TempDir()
	plug := filepath.Join(root, "plugins", id)
	copyFile(t, "plugin.json", filepath.Join(plug, "plugin.json"), 0o644)
	copyFile(t, "bin/hydra-epic", filepath.Join(plug, "bin/hydra-epic"), 0o755)
	bin := filepath.Join(root, "fakebin")
	copyFile(t, "testdata/legendary", filepath.Join(bin, "legendary"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	e := &env{state: filepath.Join(root, "legendary-state.json"), games: filepath.Join(root, "Games")}
	t.Setenv("FAKE_LEGENDARY_STATE", e.state)
	// A game legendary installed before Hydra, outside Hydra's folder.
	pre := filepath.Join(root, "Heroic", "Pre")
	os.MkdirAll(filepath.Join(pre, "Binaries"), 0o755)
	os.WriteFile(filepath.Join(pre, "Binaries", "Game.exe"), []byte("MZ"), 0o644)
	st, _ := json.Marshal(map[string]any{"account": nil, "installed": map[string]any{"Pre": map[string]any{
		"install_path": pre, "title": "Preinstalled", "version": "1.0", "platform": "Windows",
		"install_size": 1024, "is_dlc": false, "executable": "Binaries/Game.exe",
	}}})
	os.WriteFile(e.state, st, 0o644)

	sets, _ := settings.Open(filepath.Join(root, "settings.json"))
	var q *jobs.Queue
	e.m = plugins.NewManager(plugins.Options{
		Dir: filepath.Join(root, "plugins"), LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: sets,
		OnNotify: func(pid, method string, params json.RawMessage) {
			var u plugins.JobUpdate
			if json.Unmarshal(params, &u) == nil {
				q.Notify(pid, method, u)
			}
		},
		OnExit: func(pid string, err error) { q.PluginExited(pid, err) },
	})
	q, _ = jobs.Open(filepath.Join(root, "jobs.json"), backend{e.m}, nil)
	e.q = q
	ctx, cancel := context.WithCancel(context.Background())
	go q.Run(ctx)
	t.Cleanup(func() { cancel(); e.m.Shutdown() })

	list, _ := e.m.Discover()
	if len(list) != 1 || list[0].Status != plugins.StatusUnapproved {
		t.Fatalf("discovery: %+v", list)
	}
	if err := e.m.Approve(id); err != nil {
		t.Fatal(err)
	}
	p, _ := e.m.Plugin(id)
	if p.Status != plugins.StatusReady {
		t.Fatalf("not ready: %+v", p)
	}
	e.folder, _ = e.m.InstallFolder(id, e.games)
	e.store = stores.NewPlugin(e.m, p, func() []string { return e.m.Folders(id, e.games) })
	return e
}

func (e *env) call(t *testing.T, method string, params, result any) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return e.m.Call(ctx, id, method, params, result)
}

func (e *env) library(t *testing.T) map[string]types.Game {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	games, err := e.store.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]types.Game{}
	for _, g := range games {
		out[g.StoreAppID] = g
	}
	return out
}

func (e *env) calls(t *testing.T) string {
	data, _ := os.ReadFile(e.state + ".calls")
	return string(data)
}

func (e *env) run(t *testing.T, kind string, g types.Game) types.Job {
	t.Helper()
	j, err := e.q.Add(kind, id, g.ID, g.StoreAppID, g.Title, e.folder)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, x := range e.q.List() {
			if x.ID == j.ID && (x.State == jobs.StateDone || x.State == jobs.StateFailed) {
				return x
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job never finished: %+v", e.q.List())
	return j
}

func TestEpicAuth(t *testing.T) {
	e := setup(t)
	var st types.AuthStatus
	if err := e.call(t, "auth.status", nil, &st); err != nil || st.LoggedIn {
		t.Fatalf("status: %+v, %v", st, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := e.store.Library(ctx)
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != plugins.CodeAuthRequired {
		t.Fatalf("library while logged out: %v", err)
	}

	var pr types.AuthPrompt
	if err := e.call(t, "auth.begin", nil, &pr); err != nil || pr.URL != "https://legendary.gl/epiclogin" || len(pr.Fields) != 1 {
		t.Fatalf("begin: %+v, %v", pr, err)
	}
	bad := map[string]any{"fields": map[string]string{"code": "wrongwrongwrong1"}}
	if err := e.call(t, "auth.complete", bad, &st); err == nil {
		t.Fatal("wrong code accepted")
	}
	// The whole JSON response from the login page works too.
	good := map[string]any{"fields": map[string]string{"code": `{"redirectUrl":"https://x","authorizationCode":"abcdef0123456789abcdef0123456789","sid":null}`}}
	if err := e.call(t, "auth.complete", good, &st); err != nil || !st.LoggedIn || st.User != "EpicUser" {
		t.Fatalf("login: %+v, %v", st, err)
	}
	if !strings.Contains(e.calls(t), "auth --code abcdef0123456789abcdef0123456789 --disable-webview") {
		t.Fatalf("auth call: %s", e.calls(t))
	}
	if err := e.call(t, "auth.logout", nil, &st); err != nil || st.LoggedIn {
		t.Fatalf("logout: %+v, %v", st, err)
	}
}

func login(t *testing.T, e *env) {
	t.Helper()
	var st types.AuthStatus
	if err := e.call(t, "auth.complete", map[string]any{"fields": map[string]string{"code": "abcdef0123456789abcdef0123456789"}}, &st); err != nil {
		t.Fatal(err)
	}
}

func TestEpicLibrary(t *testing.T) {
	e := setup(t)
	login(t, e)
	lib := e.library(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, dlc := lib["GooseDLC"]; dlc || len(lib) != 3 {
		t.Fatalf("library: %+v", lib)
	}
	goose := lib["Goose"]
	if goose.ID != "p.epic-legendary.Goose" || goose.Store != "epic" || goose.IsInstalled || !goose.Installable ||
		goose.CoverURL != "https://cdn1.epicgames.com/goose-tall.jpg" || goose.HeroURL == "" {
		t.Fatalf("goose: %+v", goose)
	}
	d, err := e.store.Details(ctx, goose)
	if err != nil {
		t.Fatal(err)
	}
	if d.Summary != "A troublesome goose." || len(d.Screenshots) != 1 || d.ReleaseDate != "2019-09-20" || d.HeroURL == "" ||
		len(d.Genres) != 1 || d.Genres[0] != "Comedy" || len(d.Developers) != 1 || d.Developers[0] != "House House" {
		t.Fatalf("details: %+v", d)
	}
	if d.Achievements == nil || d.Achievements.Total != 2 || d.Achievements.Unlocked != -1 || len(d.Achievements.List) != 2 ||
		d.Achievements.List[0].Name != "First prank" || d.Achievements.List[0].Icon != "https://cdn1.epicgames.com/prank.png" {
		t.Fatalf("achievements: %+v", d.Achievements)
	}
	// A catalogue entry without achievements metadata shows none.
	pre := lib["Pre"]
	d, err = e.store.Details(ctx, pre)
	if err != nil || d.Achievements != nil || d.Summary != "" {
		t.Fatalf("pre details: %+v, %v", d, err)
	}
	if ea := lib["Origin"]; ea.Installable || !strings.Contains(ea.Note, "Origin") {
		t.Fatalf("EA game: %+v", ea)
	}
	if !pre.IsInstalled || !pre.UpdateAvailable || !pre.OutsideFolder || pre.Version != "1.0" {
		t.Fatalf("pre: %+v", pre)
	}
}

func TestEpicInstallLaunchUninstall(t *testing.T) {
	e := setup(t)
	login(t, e)
	goose := e.library(t)["Goose"]

	var size plugins.InstallSize
	if err := e.call(t, "game.installSize", map[string]any{"id": "Goose"}, &size); err != nil || size.DiskBytes != 12<<20 {
		t.Fatalf("size: %+v, %v", size, err)
	}

	j := e.run(t, jobs.KindInstall, goose)
	if j.State != jobs.StateDone || j.Total != 10<<20 || j.Bytes != 10<<20 {
		t.Fatalf("install: %+v", j)
	}
	if !strings.Contains(e.calls(t), "install Goose --base-path "+e.folder+" --skip-sdl --platform Windows") {
		t.Fatalf("install call: %s", e.calls(t))
	}
	goose = e.library(t)["Goose"]
	if !goose.IsInstalled || goose.OutsideFolder {
		t.Fatalf("after install: %+v", goose)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Preview must not fetch an exchange code.
	if _, err := e.store.Launch(ctx, goose, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.calls(t), "launch Goose --json --offline --skip-version-check") {
		t.Fatalf("preview call: %s", e.calls(t))
	}
	l, err := e.store.Launch(ctx, goose, false)
	if err != nil || l.Direct == nil {
		t.Fatalf("launch: %+v, %v", l, err)
	}
	d := l.Direct
	if !d.NeedsWine || !strings.HasSuffix(d.Exe, "Binaries/Game.exe") || !strings.HasSuffix(d.Dir, "Binaries") {
		t.Fatalf("direct: %+v", d)
	}
	if _, ok := d.Env["WINEPREFIX"]; ok || d.Env["DXVK_HUD"] != "fps" {
		t.Fatalf("env: %v", d.Env)
	}
	args := strings.Join(stores.RedactArgs(d.Args, d.Redact), " ")
	if !strings.Contains(args, "-AUTH_PASSWORD=***") || strings.Contains(args, "exchange0123") || !strings.HasPrefix(args, "-SaveToUserDir") {
		t.Fatalf("args: %s", args)
	}

	if u := e.run(t, jobs.KindUninstall, goose); u.State != jobs.StateDone {
		t.Fatalf("uninstall: %+v", u)
	}
	if e.library(t)["Goose"].IsInstalled {
		t.Fatal("still installed")
	}
}

func TestEpicOutsideFolderAndUpdate(t *testing.T) {
	e := setup(t)
	login(t, e)
	pre := e.library(t)["Pre"]
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Installed by legendary elsewhere: refused until the user allows the folder.
	if _, err := e.store.Launch(ctx, pre, true); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("outside folder launched: %v", err)
	}
	if err := e.m.AllowFolder(id, filepath.Dir(pre.InstallDir)); err != nil {
		t.Fatal(err)
	}
	if _, err := e.store.Launch(ctx, pre, true); err != nil {
		t.Fatalf("after allowing: %v", err)
	}
	// Out of date: a real launch asks for the update.
	if _, err := e.store.Launch(ctx, pre, false); err == nil || !strings.Contains(err.Error(), "update") {
		t.Fatalf("outdated launch: %v", err)
	}
	if u := e.run(t, jobs.KindUpdate, pre); u.State != jobs.StateDone {
		t.Fatalf("update: %+v", u)
	}
	if pre = e.library(t)["Pre"]; pre.UpdateAvailable {
		t.Fatalf("still outdated: %+v", pre)
	}
	if _, err := e.store.Launch(ctx, pre, false); err != nil {
		t.Fatalf("launch after update: %v", err)
	}
}
