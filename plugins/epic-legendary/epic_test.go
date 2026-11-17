// Tests the Epic plugin through Platypus's real plugin path (manager, JSON-RPC,
// job queue, launch-spec validation) against testdata/legendary, a fake CLI
// that prints the real legendary's output formats. They don't reach Epic.
package epic_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/jobs"
	"platypus/internal/jsonrpc"
	"platypus/internal/plugins"
	"platypus/internal/settings"
	"platypus/internal/stats"
	"platypus/internal/stores"
	"platypus/internal/types"
)

const (
	id        = "epic-legendary"
	accountID = "0123456789abcdef0123456789abcdef"
)

type playtimeStub struct {
	srv  *httptest.Server
	deny atomic.Bool
}

// playtimeServer answers Epic's library playtime endpoint on loopback. deny
// makes it answer 401 instead, like an expired token.
func playtimeServer(t *testing.T) *playtimeStub {
	t.Helper()
	pt := &playtimeStub{}
	pt.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if pt.deny.Load() {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/library/api/public/playtime/account/"+accountID+"/all" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "bearer eg1~faketoken" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"accountId":"`+accountID+`","artifactId":"Goose","totalTime":5400},
			{"accountId":"`+accountID+`","artifactId":"Pre","totalTime":120}]`)
	}))
	t.Cleanup(pt.srv.Close)
	return pt
}

type env struct {
	m      *plugins.Manager
	q      *jobs.Queue
	store  *stores.Plugin
	state  string
	folder string
	games  string
	pt     *playtimeStub
}

type backend struct{ m *plugins.Manager }

func (b backend) Size(ctx context.Context, j types.Job) (plugins.InstallSize, error) {
	var s plugins.InstallSize
	params := map[string]any{"id": j.StoreGameID}
	if j.Target != nil {
		params["target"] = j.Target
	}
	err := b.m.Call(ctx, j.Source, "game.installSize", params, &s)
	return s, err
}

func (b backend) Start(ctx context.Context, j types.Job) error {
	p := map[string]any{"id": j.StoreGameID, "jobId": j.ID}
	if j.Kind != jobs.KindUninstall {
		p["installDir"] = j.InstallDir
	}
	if j.Target != nil {
		p["target"] = j.Target
	}
	if j.Kind == jobs.KindVerify {
		p["repair"] = j.Repair
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
	copyFile(t, "bin/platypus-epic", filepath.Join(plug, "bin/platypus-epic"), 0o755)
	bin := filepath.Join(root, "fakebin")
	copyFile(t, "testdata/legendary", filepath.Join(bin, "legendary"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	e := &env{state: filepath.Join(root, "legendary-state.json"), games: filepath.Join(root, "Games")}
	t.Setenv("FAKE_LEGENDARY_STATE", e.state)
	// legendary's config folder holds the manifests this plugin archives.
	if os.Getenv("LEGENDARY_CONFIG_PATH") == "" {
		t.Setenv("LEGENDARY_CONFIG_PATH", filepath.Join(root, "legendary-config"))
	}
	e.pt = playtimeServer(t)
	t.Setenv("PLATYPUS_EPIC_LIBRARY_URL", e.pt.srv.URL)
	// A game legendary installed before Platypus, outside Platypus's folder.
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

func (e *env) wait(t *testing.T, jobID string) types.Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, x := range e.q.List() {
			if x.ID == jobID && (x.State == jobs.StateDone || x.State == jobs.StateFailed) {
				return x
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job never finished: %+v", e.q.List())
	return types.Job{}
}

func (e *env) run(t *testing.T, kind string, g types.Game) types.Job {
	t.Helper()
	j, err := e.q.Add(kind, id, g.ID, g.StoreAppID, g.Title, e.folder)
	if err != nil {
		t.Fatal(err)
	}
	return e.wait(t, j.ID)
}

func (e *env) runTarget(t *testing.T, kind string, g types.Game, target *types.VersionTarget) types.Job {
	t.Helper()
	j, err := e.q.AddTargeted(kind, id, g.ID, g.StoreAppID, g.Title, e.folder, target)
	if err != nil {
		t.Fatal(err)
	}
	return e.wait(t, j.ID)
}

// setLatest moves the fake store's current build, like Epic publishing an update.
func (e *env) setLatest(t *testing.T, app, version string) {
	t.Helper()
	data, err := os.ReadFile(e.state)
	if err != nil {
		t.Fatal(err)
	}
	var st map[string]any
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatal(err)
	}
	latest, _ := st["latest"].(map[string]any)
	if latest == nil {
		latest = map[string]any{}
		st["latest"] = latest
	}
	latest[app] = version
	out, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.state, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func (e *env) verify(t *testing.T, g types.Game, repair bool) types.Job {
	t.Helper()
	j, err := e.q.AddVerify(id, g.ID, g.StoreAppID, g.Title, g.InstallDir, repair)
	if err != nil {
		t.Fatal(err)
	}
	return e.wait(t, j.ID)
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

func TestEpicVerifyAndRepair(t *testing.T) {
	e := setup(t)
	login(t, e)
	goose := e.library(t)["Goose"]
	if j := e.run(t, jobs.KindInstall, goose); j.State != jobs.StateDone {
		t.Fatalf("install: %+v", j)
	}
	goose = e.library(t)["Goose"]

	j := e.verify(t, goose, false)
	if j.State != jobs.StateDone || j.Verify == nil || j.Verify.Checked != 3 ||
		j.Verify.Damaged != 0 || j.Verify.Missing != 0 || j.Verify.Repaired != 0 {
		t.Fatalf("clean check: %+v", j)
	}
	marker := filepath.Join(goose.InstallDir, ".damaged")
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	j = e.verify(t, goose, false)
	if j.State != jobs.StateDone || j.Verify == nil || j.Verify.Damaged != 1 || j.Verify.Missing != 0 {
		t.Fatalf("damaged check: %+v", j)
	}
	j = e.verify(t, goose, true)
	if j.State != jobs.StateDone || j.Verify == nil || j.Verify.Repaired != 1 ||
		j.Verify.Damaged != 0 || j.Verify.Missing != 0 {
		t.Fatalf("repair: %+v", j)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("repair left the damaged file: %v", err)
	}
	if j := e.verify(t, goose, false); j.Verify == nil || j.Verify.Damaged != 0 {
		t.Fatalf("check after repair: %+v", j)
	}
}

func TestEpicRelocated(t *testing.T) {
	e := setup(t)
	login(t, e)
	goose := e.library(t)["Goose"]
	if j := e.run(t, jobs.KindInstall, goose); j.State != jobs.StateDone {
		t.Fatalf("install: %+v", j)
	}
	goose = e.library(t)["Goose"]
	from := goose.InstallDir
	to := filepath.Join(t.TempDir(), "Moved", filepath.Base(from))
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
	bad := map[string]any{"id": "Goose", "from": from + "-elsewhere", "to": to}
	var rpcErr *jsonrpc.Error
	if err := e.call(t, "game.relocated", bad, nil); !errors.As(err, &rpcErr) || rpcErr.Code != -32602 {
		t.Fatalf("relocated from a folder the game isn't in: %v", err)
	}
	if err := e.call(t, "game.relocated", map[string]any{"id": "Goose", "from": from, "to": to}, nil); err != nil {
		t.Fatal(err)
	}
	if goose = e.library(t)["Goose"]; !goose.IsInstalled || goose.InstallDir != to {
		t.Fatalf("after relocated: %+v", goose)
	}
	if err := e.m.AllowFolder(id, filepath.Dir(to)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := e.store.Launch(ctx, goose, true); err != nil {
		t.Fatalf("launch after relocated: %v", err)
	}
}

func TestEpicAchievementsAndPlaytime(t *testing.T) {
	e := setup(t)
	login(t, e)

	var ach stats.AchievementsReply
	if err := e.call(t, "achievements.list", map[string]any{"id": "Goose"}, &ach); err != nil {
		t.Fatal(err)
	}
	if ach.Total != 2 || ach.Unlocked != 1 || len(ach.List) != 2 {
		t.Fatalf("achievements: %+v", ach)
	}
	byID := map[string]stats.Achievement{}
	for _, a := range ach.List {
		byID[a.ID] = a
	}
	if prank := byID["prank"]; prank.Name != "First prank" || prank.UnlockedAt != 1588334400 ||
		prank.Icon != "https://cdn1.epicgames.com/prank.png" {
		t.Fatalf("prank: %+v", prank)
	}
	if honk := byID["honk"]; !honk.Hidden || honk.UnlockedAt != 0 ||
		honk.GlobalPercent == nil || *honk.GlobalPercent != 42.5 {
		t.Fatalf("honk: %+v", honk)
	}
	var junk json.RawMessage
	if err := e.call(t, "achievements.list", map[string]any{"id": "Nope"}, &junk); err == nil {
		t.Fatal("achievements of an unknown game answered data")
	}
	var none stats.AchievementsReply
	if err := e.call(t, "achievements.list", map[string]any{"id": "Pre"}, &none); err != nil || none.Total != 0 || len(none.List) != 0 {
		t.Fatalf("game without achievements: %+v, %v", none, err)
	}

	var pt stats.PlaytimeReply
	if err := e.call(t, "playtime.list", nil, &pt); err != nil {
		t.Fatal(err)
	}
	minutes := map[string]int{}
	for _, g := range pt.Games {
		minutes[g.ID] = g.Minutes
	}
	if minutes["Goose"] != 90 || minutes["Pre"] != 2 {
		t.Fatalf("playtime: %v", pt)
	}

	var rpcErr *jsonrpc.Error
	e.pt.deny.Store(true)
	if err := e.call(t, "playtime.list", nil, &junk); !errors.As(err, &rpcErr) || rpcErr.Code != plugins.CodeAuthRequired {
		t.Fatalf("playtime with an expired token: %v", err)
	}
	e.pt.deny.Store(false)

	var st types.AuthStatus
	if err := e.call(t, "auth.logout", nil, &st); err != nil {
		t.Fatal(err)
	}
	if err := e.call(t, "playtime.list", nil, &junk); !errors.As(err, &rpcErr) || rpcErr.Code != plugins.CodeAuthRequired {
		t.Fatalf("playtime while logged out: %v", err)
	}
	if err := e.call(t, "achievements.list", map[string]any{"id": "Goose"}, &junk); !errors.As(err, &rpcErr) || rpcErr.Code != plugins.CodeAuthRequired {
		t.Fatalf("achievements while logged out: %v", err)
	}
}

func TestEpicVerifyMissingManifestAndFolderMismatch(t *testing.T) {
	e := setup(t)
	login(t, e)
	goose := e.library(t)["Goose"]
	if j := e.run(t, jobs.KindInstall, goose); j.State != jobs.StateDone {
		t.Fatalf("install: %+v", j)
	}
	goose = e.library(t)["Goose"]

	// A folder the legendary record doesn't name is refused before any work.
	wrong := filepath.Join(t.TempDir(), filepath.Base(goose.InstallDir))
	err := e.call(t, "game.verify", map[string]any{"id": "Goose", "installDir": wrong,
		"jobId": "verify-wrong-dir", "repair": false}, nil)
	if err == nil || !strings.Contains(err.Error(), wrong) || !strings.Contains(err.Error(), goose.InstallDir) {
		t.Fatalf("verify in another folder: %v", err)
	}

	if err := os.WriteFile(filepath.Join(goose.InstallDir, ".no-manifest"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	j := e.verify(t, goose, false)
	if j.State != jobs.StateFailed || !strings.Contains(j.Error, "Manifest") {
		t.Fatalf("verify without a manifest: %+v", j)
	}
}

func TestEpicRepairFailure(t *testing.T) {
	e := setup(t)
	login(t, e)
	goose := e.library(t)["Goose"]
	if j := e.run(t, jobs.KindInstall, goose); j.State != jobs.StateDone {
		t.Fatalf("install: %+v", j)
	}
	goose = e.library(t)["Goose"]
	for _, name := range []string{".damaged", ".repair-fail"} {
		if err := os.WriteFile(filepath.Join(goose.InstallDir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if j := e.verify(t, goose, false); j.Verify == nil || j.Verify.Damaged != 1 {
		t.Fatalf("check: %+v", j)
	}
	j := e.verify(t, goose, true)
	if j.State != jobs.StateFailed || !strings.Contains(j.Error, "Installation failed") {
		t.Fatalf("failed repair reported as done: %+v", j)
	}
}

func TestEpicRelocatedRefusals(t *testing.T) {
	e := setup(t)
	login(t, e)
	goose := e.library(t)["Goose"]
	if j := e.run(t, jobs.KindInstall, goose); j.State != jobs.StateDone {
		t.Fatalf("install: %+v", j)
	}
	goose = e.library(t)["Goose"]
	renamed := filepath.Join(t.TempDir(), "Moved", "DifferentName")
	var rpcErr *jsonrpc.Error
	err := e.call(t, "game.relocated", map[string]any{"id": "Goose", "from": goose.InstallDir, "to": renamed}, nil)
	if !errors.As(err, &rpcErr) || rpcErr.Code != -32000 {
		t.Fatalf("relocated with a renamed folder: %v", err)
	}
}

func TestEpicRelocatedMoveLockFailure(t *testing.T) {
	t.Setenv("FAKE_LEGENDARY_MOVE_FAIL", "1")
	e := setup(t)
	login(t, e)
	goose := e.library(t)["Goose"]
	if j := e.run(t, jobs.KindInstall, goose); j.State != jobs.StateDone {
		t.Fatalf("install: %+v", j)
	}
	goose = e.library(t)["Goose"]
	to := filepath.Join(t.TempDir(), "Moved", filepath.Base(goose.InstallDir))
	err := e.call(t, "game.relocated", map[string]any{"id": "Goose", "from": goose.InstallDir, "to": to}, nil)
	if err == nil || !strings.Contains(err.Error(), "lock") {
		t.Fatalf("relocated despite the lock: %v", err)
	}
	if goose = e.library(t)["Goose"]; goose.InstallDir == to {
		t.Fatalf("record changed without a move: %+v", goose)
	}
}

func TestEpicAchievementWithoutUnlockTime(t *testing.T) {
	e := setup(t)
	login(t, e)
	if err := os.WriteFile(e.state+".na-unlock", nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var ach stats.AchievementsReply
	if err := e.call(t, "achievements.list", map[string]any{"id": "Goose"}, &ach); err != nil {
		t.Fatal(err)
	}
	for _, a := range ach.List {
		if a.ID == "prank" && a.UnlockedAt != 1 {
			t.Fatalf("N/A unlock date: %+v", a)
		}
	}
}

func TestEpicOldLegendaryHidesAchievements(t *testing.T) {
	t.Setenv("FAKE_LEGENDARY_VERSION", "0.20.3")
	e := setup(t)
	var init struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := e.call(t, "initialize", nil, &init); err != nil {
		t.Fatal(err)
	}
	has := func(name string) bool {
		return slices.Contains(init.Capabilities, name)
	}
	if has("achievements") || !has("playtime") || !has("verify") {
		t.Fatalf("capabilities on legendary 0.20: %v", init.Capabilities)
	}
}

// Epic lists no old builds: the plugin archives the manifest of what is
// installed, before every update and after every install, and installs one of
// those back through `legendary install --manifest` (its documented downgrade
// path). The fake legendary writes a manifest per install, like the real one.
func TestEpicVersionsArchiveAndDowngrade(t *testing.T) {
	e := setup(t)
	e.login(t)
	var installedMu sync.Mutex
	installed := map[string]types.Installed{}
	e.q.SetOnInstalled(func(j types.Job, in types.Installed) {
		installedMu.Lock()
		installed[j.ID] = in
		installedMu.Unlock()
	})
	archive := filepath.Join(e.m.DataDirOf(id), "archive", "Goose")
	versions := func(pinned map[string]any, ledger []any) types.VersionInfo {
		t.Helper()
		params := map[string]any{"id": "Goose", "ledger": ledger}
		if pinned != nil {
			params["pinned"] = pinned
		}
		var info types.VersionInfo
		if err := e.call(t, "versions.list", params, &info); err != nil {
			t.Fatal(err)
		}
		if len(info.Channels) != 0 || info.Manual != nil {
			t.Fatalf("Epic listed channels or manual values: %+v", info)
		}
		return info
	}
	builds := func(info types.VersionInfo) map[string]types.VersionBuild {
		out := map[string]types.VersionBuild{}
		for _, b := range info.Builds {
			out[b.ID] = b
		}
		return out
	}
	target := func() json.RawMessage {
		return json.RawMessage(`"1.0.manifest"`)
	}

	goose := e.library(t)["Goose"]
	j := e.run(t, jobs.KindInstall, goose)
	if j.State != jobs.StateDone {
		t.Fatalf("install: %+v", j)
	}
	if _, err := os.Stat(filepath.Join(archive, "1.0.manifest")); err != nil {
		t.Fatalf("install wasn't archived: %v", err)
	}
	if in := installed[j.ID]; in.Build != "1.0" || string(in.ArchiveRef) != `"1.0.manifest"` {
		t.Fatalf("job.done installed: %+v", in)
	}
	info := versions(nil, nil)
	if b := builds(info); len(b) != 1 || !b["1.0"].Current || info.UpdateAvailable {
		t.Fatalf("after install: %+v", info)
	}

	// A target's size comes from the archived manifest, not the current store build.
	archivedSize := map[string]any{"id": "Goose", "target": map[string]any{"build": "1.0", "archiveRef": target()}}
	var size plugins.InstallSize
	if err := e.call(t, "game.installSize", archivedSize, &size); err != nil || size.DownloadBytes != 7<<20 || size.DiskBytes != 9<<20 {
		t.Fatalf("archived size: %+v, %v", size, err)
	}

	// The store moves on; the update archives 1.0 before replacing it.
	e.setLatest(t, "Goose", "1.1")
	if info = versions(nil, nil); !info.UpdateAvailable {
		t.Fatalf("no update offered: %+v", info)
	}
	if u := e.run(t, jobs.KindUpdate, goose); u.State != jobs.StateDone {
		t.Fatalf("update: %+v", u)
	}
	info = versions(nil, nil)
	if b := builds(info); len(b) != 2 || !b["1.1"].Current || b["1.0"].Source != "archive" ||
		info.UpdateAvailable {
		t.Fatalf("after update: %+v", info)
	}

	// Downgrade to the archived build through --manifest.
	j = e.runTarget(t, jobs.KindUpdate, goose, &types.VersionTarget{Build: "1.0", ArchiveRef: target()})
	if j.State != jobs.StateDone {
		t.Fatalf("downgrade: %+v", j)
	}
	if !strings.Contains(e.calls(t), "install Goose --update-only --skip-sdl --manifest "+filepath.Join(archive, "1.0.manifest")) {
		t.Fatalf("downgrade call: %s", e.calls(t))
	}
	if in := installed[j.ID]; in.Build != "1.0" {
		t.Fatalf("downgrade installed: %+v", in)
	}
	info = versions(nil, nil)
	if b := builds(info); !b["1.0"].Current || !info.UpdateAvailable {
		t.Fatalf("after downgrade: %+v", info)
	}
	if pinned := versions(map[string]any{"kind": "build", "id": "1.0"}, nil); pinned.UpdateAvailable {
		t.Fatalf("update offered while pinned: %+v", pinned)
	}

	// Bounded: three saved builds per game, oldest pruned.
	for _, v := range []string{"1.2", "1.3", "1.4"} {
		e.setLatest(t, "Goose", v)
		if u := e.run(t, jobs.KindUpdate, goose); u.State != jobs.StateDone {
			t.Fatalf("update to %s: %+v", v, u)
		}
	}
	manifests, _ := filepath.Glob(filepath.Join(archive, "*.manifest"))
	if len(manifests) != 3 {
		t.Fatalf("archives not bounded: %v", manifests)
	}
	if _, err := os.Stat(filepath.Join(archive, "1.0.manifest")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the oldest archive survived pruning: %v", err)
	}

	// A pruned build is listed from the install history, but not installable.
	gone := versions(nil, []any{map[string]any{"build": "1.0", "versionName": "1.0", "date": "2025-01-01"}})
	if b := builds(gone); b["1.0"].Available == nil || *b["1.0"].Available || b["1.0"].Source != "archive" {
		t.Fatalf("pruned build listed as installable: %+v", gone)
	}
	err := e.call(t, "game.update", map[string]any{"id": "Goose", "jobId": "gone-1", "installDir": e.folder,
		"target": map[string]any{"build": "1.0", "archiveRef": target()}}, nil)
	if err == nil || !strings.Contains(err.Error(), "no longer saved") {
		t.Fatalf("a pruned build was installable: %v", err)
	}

	// A fresh install of an archived build works too (base path, not update-only).
	if u := e.run(t, jobs.KindUninstall, goose); u.State != jobs.StateDone {
		t.Fatalf("uninstall: %+v", u)
	}
	j = e.runTarget(t, jobs.KindInstall, goose, &types.VersionTarget{Build: "1.2", ArchiveRef: json.RawMessage(`"1.2.manifest"`)})
	if j.State != jobs.StateDone {
		t.Fatalf("install archived build: %+v", j)
	}
	if !strings.Contains(e.calls(t), "install Goose --base-path "+e.folder+" --skip-sdl --platform Windows --manifest "+
		filepath.Join(archive, "1.2.manifest")) {
		t.Fatalf("archived install call: %s", e.calls(t))
	}
	if g := e.library(t)["Goose"]; !g.IsInstalled || g.Version != "1.2" {
		t.Fatalf("after archived install: %+v", g)
	}

	var junk json.RawMessage
	if err := e.call(t, "versions.list", map[string]any{"id": "Nope"}, &junk); err == nil {
		t.Fatal("versions listed for an app the account doesn't own")
	}
}
