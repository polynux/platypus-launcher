package stores

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
	"hydra-launcher/internal/types"
)

// echoEnv is the whole plugin path wired the way app.go wires it, around the
// real echo plugin binary.
type echoEnv struct {
	m      *plugins.Manager
	q      *jobs.Queue
	store  *Plugin
	games  string // Games folder
	folder string // echo's install folder
}

type testBackend struct{ m *plugins.Manager }

func (b testBackend) Size(ctx context.Context, j types.Job) (plugins.InstallSize, error) {
	var s plugins.InstallSize
	err := b.m.Call(ctx, j.Source, "game.installSize", map[string]any{"id": j.StoreGameID}, &s)
	return s, err
}

func (b testBackend) Start(ctx context.Context, j types.Job) error {
	p := map[string]any{"id": j.StoreGameID, "jobId": j.ID}
	if j.Kind != jobs.KindUninstall {
		p["installDir"] = j.InstallDir
	}
	return b.m.Call(ctx, j.Source, "game."+j.Kind, p, nil)
}

func (b testBackend) Cancel(ctx context.Context, j types.Job) error {
	return b.m.Call(ctx, j.Source, "job.cancel", map[string]any{"jobId": j.ID}, nil)
}

func newEchoEnv(t *testing.T) *echoEnv {
	t.Helper()
	root := t.TempDir()
	pluginDir := filepath.Join(root, "plugins", "echo")
	if err := os.MkdirAll(filepath.Join(pluginDir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("go", "build", "-o", filepath.Join(pluginDir, "bin/hydra-echo"), "hydra-launcher/plugins/echo").CombinedOutput()
	if err != nil {
		t.Fatalf("build echo: %s", out)
	}
	man, _ := os.ReadFile("../../plugins/echo/plugin.json")
	os.WriteFile(filepath.Join(pluginDir, "plugin.json"), man, 0o644)

	st, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	e := &echoEnv{games: filepath.Join(root, "Games")}
	var q *jobs.Queue
	e.m = plugins.NewManager(plugins.Options{
		Dir: filepath.Join(root, "plugins"), LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"), Settings: st,
		OnNotify: func(id, method string, params json.RawMessage) {
			var u plugins.JobUpdate
			if json.Unmarshal(params, &u) == nil {
				q.Notify(id, method, u)
			}
		},
		OnExit: func(id string, err error) { q.PluginExited(id, err) },
	})
	q, err = jobs.Open(filepath.Join(root, "jobs.json"), testBackend{e.m}, nil)
	if err != nil {
		t.Fatal(err)
	}
	e.q = q
	ctx, cancel := context.WithCancel(context.Background())
	go q.Run(ctx)
	t.Cleanup(func() { cancel(); e.m.Shutdown() })

	e.m.Discover()
	if err := e.m.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	p, _ := e.m.Plugin("echo")
	e.folder, _ = e.m.InstallFolder("echo", e.games)
	e.store = NewPlugin(e.m, p, func() []string { return e.m.Folders("echo", e.games) })
	return e
}

func (e *echoEnv) ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return c
}

func (e *echoEnv) login(t *testing.T) {
	t.Helper()
	var st types.AuthStatus
	err := e.m.Call(e.ctx(t), "echo", "auth.complete", map[string]any{"fields": map[string]string{"code": "nope"}}, &st)
	if err == nil {
		t.Fatal("wrong code accepted")
	}
	if err := e.m.Call(e.ctx(t), "echo", "auth.complete", map[string]any{"fields": map[string]string{"code": "letmein"}}, &st); err != nil || !st.LoggedIn {
		t.Fatalf("login: %+v, %v", st, err)
	}
}

func (e *echoEnv) game(t *testing.T, id string) types.Game {
	t.Helper()
	lib, err := e.store.Library(e.ctx(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range lib {
		if g.StoreAppID == id {
			return g
		}
	}
	t.Fatalf("%s not in library", id)
	return types.Game{}
}

func (e *echoEnv) install(t *testing.T, kind, id string) types.Job {
	t.Helper()
	g := e.game(t, id)
	j, err := e.q.Add(kind, "echo", g.ID, g.StoreAppID, g.Title, e.folder)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func waitJob(t *testing.T, q *jobs.Queue, id string, states ...string) types.Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		for _, j := range q.List() {
			if j.ID == id {
				for _, s := range states {
					if j.State == s {
						return j
					}
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("job never reached %v: %+v", states, q.List())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestEchoLibraryNeedsLogin(t *testing.T) {
	e := newEchoEnv(t)
	_, err := e.store.Library(e.ctx(t))
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != plugins.CodeAuthRequired {
		t.Fatalf("want auth-required, got %v", err)
	}
	e.login(t)
	lib, err := e.store.Library(e.ctx(t))
	if err != nil || len(lib) != 9 {
		t.Fatalf("library: %d games, %v", len(lib), err)
	}
	tiny := e.game(t, "tiny")
	if tiny.ID != "p.echo.tiny" || tiny.Source != "echo" || tiny.Store != "echo" || tiny.IsInstalled || !tiny.Installable || !tiny.Native {
		t.Fatalf("tiny: %+v", tiny)
	}
	if e.game(t, "client").Native {
		t.Error("client game wrongly marked native")
	}
}

func TestEchoDetails(t *testing.T) {
	e := newEchoEnv(t)
	e.login(t)
	d, err := e.store.Details(e.ctx(t), e.game(t, "tiny"))
	if err != nil {
		t.Fatal(err)
	}
	if d.Summary == "" || len(d.Genres) != 1 || d.ReleaseDate != "2024-01-01" ||
		d.Achievements == nil || d.Achievements.Total != 2 || d.Achievements.Unlocked != -1 || len(d.Achievements.List) != 2 {
		t.Fatalf("details: %+v", d)
	}
	// A game the plugin has no metadata for shows the library entry only.
	d, err = e.store.Details(e.ctx(t), e.game(t, "slow"))
	if err != nil || d.Summary != "" || d.Achievements != nil {
		t.Fatalf("empty details: %+v, %v", d, err)
	}
	if esc := e.game(t, "escape"); !esc.IsInstalled || !esc.OutsideFolder || esc.Installable {
		t.Fatalf("escape: %+v", esc)
	}
}

func TestEchoInstallLaunchUninstall(t *testing.T) {
	e := newEchoEnv(t)
	e.login(t)
	j := e.install(t, jobs.KindInstall, "tiny")
	waitJob(t, e.q, j.ID, jobs.StateRunning)
	done := waitJob(t, e.q, j.ID, jobs.StateDone, jobs.StateFailed)
	if done.State != jobs.StateDone || done.Bytes != 3<<20 || done.Total != 3<<20 {
		t.Fatalf("install: %+v", done)
	}

	tiny := e.game(t, "tiny")
	if !tiny.IsInstalled || tiny.OutsideFolder || !strings.HasPrefix(tiny.InstallDir, e.folder) {
		t.Fatalf("after install: %+v", tiny)
	}

	// Real launch: a validated direct spec, the secret only in the real one.
	l, err := e.store.Launch(e.ctx(t), tiny, false)
	if err != nil || l.Direct == nil {
		t.Fatalf("launch: %+v, %v", l, err)
	}
	if l.Direct.NeedsWine || l.Direct.Env["ECHO_GAME"] != "1" || !strings.HasPrefix(l.Direct.Exe, e.folder) {
		t.Fatalf("direct: %+v", l.Direct)
	}
	shown := strings.Join(RedactArgs(l.Direct.Args, l.Direct.Redact), " ")
	if strings.Contains(shown, "s3cret") || !strings.Contains(shown, "-AUTH_PASSWORD=***") {
		t.Fatalf("not redacted: %s", shown)
	}
	out, err := exec.Command(l.Direct.Exe, l.Direct.Args...).Output()
	if err != nil && !strings.Contains(string(out), "echo game") {
		t.Fatalf("game script: %s, %v", out, err)
	}

	u := e.install(t, jobs.KindUninstall, "tiny")
	if got := waitJob(t, e.q, u.ID, jobs.StateDone, jobs.StateFailed); got.State != jobs.StateDone {
		t.Fatalf("uninstall: %+v", got)
	}
	if e.game(t, "tiny").IsInstalled {
		t.Fatal("still installed")
	}
}

func TestEchoLaunchPaths(t *testing.T) {
	e := newEchoEnv(t)
	e.login(t)

	client, err := e.store.Launch(e.ctx(t), e.game(t, "client"), false)
	if err != nil || client.Client == nil || client.Client.Client != "Echo client" || client.Client.Command[0] != "sleep" {
		t.Fatalf("client launch: %+v, %v", client, err)
	}
	if client.Client.Find() != 0 {
		t.Fatal("found a game that isn't running")
	}
	cmd := exec.Command(client.Client.Command[0], client.Client.Command[1:]...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	time.Sleep(100 * time.Millisecond)
	if pid := client.Client.Find(); pid != cmd.Process.Pid {
		t.Fatalf("find = %d, want %d", pid, cmd.Process.Pid)
	}

	if _, err := e.store.Launch(e.ctx(t), e.game(t, "escape"), false); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("escape launched: %v", err)
	}
}

func TestEchoCrashResumes(t *testing.T) {
	e := newEchoEnv(t)
	e.login(t)
	j := e.install(t, jobs.KindInstall, "flaky")
	got := waitJob(t, e.q, j.ID, jobs.StateDone, jobs.StateFailed)
	if got.State != jobs.StateDone || got.Attempts != 2 {
		t.Fatalf("flaky: %+v", got)
	}
	if !e.game(t, "flaky").IsInstalled {
		t.Fatal("flaky not installed after resume")
	}
}

func TestEchoCancelAndSpace(t *testing.T) {
	e := newEchoEnv(t)
	e.login(t)
	j := e.install(t, jobs.KindInstall, "slow")
	waitJob(t, e.q, j.ID, jobs.StateRunning)
	time.Sleep(500 * time.Millisecond)
	if err := e.q.Cancel(context.Background(), j.ID); err != nil {
		t.Fatal(err)
	}
	waitJob(t, e.q, j.ID, jobs.StateCanceled)
	time.Sleep(500 * time.Millisecond)
	if e.game(t, "slow").IsInstalled {
		t.Fatal("canceled install finished anyway")
	}

	h := e.install(t, jobs.KindInstall, "huge")
	if got := waitJob(t, e.q, h.ID, jobs.StateFailed, jobs.StateDone); got.State != jobs.StateFailed || !strings.Contains(got.Error, "not enough space") {
		t.Fatalf("huge: %+v", got)
	}
}
