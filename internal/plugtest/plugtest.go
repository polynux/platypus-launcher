// Package plugtest drives any store plugin through every protocol method it
// declares, the way Hydra drives it (see docs/plugins/protocol.md). The
// conformance suite lives here, not in one plugin's tests, so each plugin
// package only wires up its own plugin-specific bits: where its fake CLI is
// and how to log in and pick a game from its library.
package plugtest

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/settings"
	"hydra-launcher/internal/stores"
	"hydra-launcher/internal/types"
)

// Suite describes one plugin under test. CopyDir is the plugin's source folder
// (a plugin whose exec needs building may set BuildCmd, run from the repo root
// after the folder is copied). FakeCLI adds a command to PATH (name -> path in
// the testdata folder); Env sets extra environment for the plugin process.
// Login runs the plugin's auth flow; PickGame returns a library game to drive
// through install/launch, plus whether it launches natively (no Wine).
type Suite struct {
	ID      string
	CopyDir string // plugin source folder, relative to the repo root
	// BuildCmd builds a Go plugin from the repo root; BuildOut is where the
	// binary lands (repo-relative). The harness copies it over the plugin's
	// exec file in the copied tree.
	BuildCmd []string
	BuildOut string

	FakeCLI map[string]string
	Env     map[string]string

	Login    func(t *testing.T, s *Session)
	PickGame func(t *testing.T, games []types.Game) types.Game
}

// Session is the running plugin wired the way app.go wires it.
type Session struct {
	t      *testing.T
	id     string
	m      *plugins.Manager
	q      *jobs.Queue
	Store  *stores.Plugin
	Games  string // the Games folder
	Folder string // this plugin's install folder
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

// Start copies the plugin, wires the manager/queue/store and approves it.
func (s Suite) Start(t *testing.T) *Session {
	t.Helper()
	root := t.TempDir()
	pluginDir := filepath.Join(root, "plugins", s.ID)
	if out, err := copyDir(s.CopyDir, pluginDir); err != nil {
		t.Fatalf("copy plugin: %v: %s", err, out)
	}
	if len(s.BuildCmd) > 0 {
		// BuildCmd/BuildOut are repo-relative; the test's cwd is this package's
		// dir, so anchor them two levels up.
		repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		cmd := append([]string{}, s.BuildCmd...)
		for i, a := range cmd {
			if a == "-o" && i+1 < len(cmd) {
				cmd[i+1] = filepath.Join(repoRoot, cmd[i+1])
			}
		}
		if out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("build plugin: %s", out)
		}
		if s.BuildOut == "" {
			t.Fatal("BuildCmd set without BuildOut")
		}
		var man struct {
			Exec string `json:"exec"`
		}
		data, err := os.ReadFile(filepath.Join(pluginDir, "plugin.json"))
		if err != nil {
			t.Fatal(err)
		}
		if json.Unmarshal(data, &man) != nil || man.Exec == "" {
			t.Fatalf("plugin.json has no exec")
		}
		built, err := os.ReadFile(filepath.Join(repoRoot, s.BuildOut))
		if err != nil {
			t.Fatal(err)
		}
		execPath := filepath.Join(pluginDir, filepath.FromSlash(man.Exec))
		os.MkdirAll(filepath.Dir(execPath), 0o755)
		if err := os.WriteFile(execPath, built, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.FakeCLI) > 0 {
		bin := filepath.Join(root, "fakebin")
		os.MkdirAll(bin, 0o755)
		for name, src := range s.FakeCLI {
			data, err := os.ReadFile(src)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bin, name), data, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	for k, v := range s.Env {
		t.Setenv(k, v)
	}

	st, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	sess := &Session{t: t, id: s.ID, Games: filepath.Join(root, "Games")}
	var q *jobs.Queue
	var qMu sync.Mutex
	sess.m = plugins.NewManager(plugins.Options{
		Dir: filepath.Join(root, "plugins"), LogDir: filepath.Join(root, "logs"),
		DataDir: filepath.Join(root, "data"), Settings: st,
		OnNotify: func(id, method string, params json.RawMessage) {
			qMu.Lock()
			qq := q
			qMu.Unlock()
			if qq == nil {
				return // queue not open yet; early notifications carry no job state
			}
			var u plugins.JobUpdate
			if json.Unmarshal(params, &u) == nil {
				qq.Notify(id, method, u)
			}
		},
		OnExit: func(id string, err error) {
			qMu.Lock()
			qq := q
			qMu.Unlock()
			if qq != nil {
				qq.PluginExited(id, err)
			}
		},
	})
	sess.q, err = jobs.Open(filepath.Join(root, "jobs.json"), backend{sess.m}, nil)
	if err != nil {
		t.Fatal(err)
	}
	qMu.Lock()
	q = sess.q
	qMu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	go sess.q.Run(ctx)
	t.Cleanup(cancel)

	if _, err := sess.m.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := sess.m.Approve(s.ID); err != nil {
		t.Fatal(err)
	}
	p, ok := sess.m.Plugin(s.ID)
	if !ok || p.Status != plugins.StatusReady {
		t.Fatalf("plugin %s not ready: %+v", s.ID, p)
	}
	sess.Folder, _ = sess.m.InstallFolder(s.ID, sess.Games)
	sess.Store = stores.NewPlugin(sess.m, p, func() []string {
		return sess.m.Folders(s.ID, sess.Games)
	})
	return sess
}

func (s *Session) call(method string, params, result any) error {
	s.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return s.m.Call(ctx, s.id, method, params, result)
}

// Call sends one protocol request to the plugin, for plugin-specific login
// flows and assertions.
func (s *Session) Call(method string, params, result any) error {
	return s.call(method, params, result)
}

// Conformance drives every declared protocol method end to end:
// initialize, auth (when declared), library.list, game.details (tolerating a
// plugin that doesn't implement it), installSize, install with progress
// through the real job queue, launchSpec (preview and real), the job-queue
// update, and uninstall. Plugin-specific expectations stay in the plugin's
// own tests; this one checks the protocol shape.
func (s Suite) Conformance(t *testing.T) {
	t.Helper()
	sess := s.Start(t)
	caps := func() []string {
		p, _ := sess.m.Plugin(s.ID)
		return p.Manifest.Capabilities
	}
	has := func(c string) bool {
		for _, x := range caps() {
			if x == c {
				return true
			}
		}
		return false
	}

	if has("auth") {
		if s.Login == nil {
			t.Fatal("plugin declares auth but the Suite has no Login")
		}
		var st types.AuthStatus
		if err := sess.call("auth.status", nil, &st); err != nil {
			t.Fatalf("auth.status: %v", err)
		}
		var pr types.AuthPrompt
		if err := sess.call("auth.begin", nil, &pr); err != nil {
			t.Fatalf("auth.begin: %v", err)
		}
		s.Login(t, sess)
		var st2 types.AuthStatus
		if err := sess.call("auth.status", nil, &st2); err != nil || !st2.LoggedIn {
			t.Fatalf("auth after login: %+v, %v", st2, err)
		}
	}

	if !has("library") {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	games, err := sess.Store.Library(ctx)
	if err != nil {
		t.Fatalf("library.list: %v", err)
	}
	if len(games) == 0 {
		t.Fatal("library.list returned no games")
	}
	for _, g := range games {
		if g.ID == "" || g.Title == "" || g.StoreAppID == "" {
			t.Fatalf("library entry missing id/title/store id: %+v", g)
		}
		if g.ID != stores.GameID(s.ID, g.StoreAppID) {
			t.Fatalf("game id %q is not p.<plugin>.<store id> form", g.ID)
		}
	}
	game := s.PickGame(t, games)

	d, err := sess.Store.Details(ctx, game)
	if err != nil {
		t.Fatalf("game.details: %v", err)
	}
	_ = d // shape validated by stores.Plugin; a missing method comes back empty

	j, err := sess.q.Add(jobs.KindInstall, s.ID, game.ID, game.StoreAppID, game.Title, sess.Folder)
	if err != nil {
		t.Fatalf("queue install: %v", err)
	}
	done := sess.waitJob(j.ID, jobs.StateDone, 30*time.Second)
	if done.Total <= 0 || done.Bytes != done.Total {
		t.Fatalf("install job didn't report full progress: %+v", done)
	}

	lib, err := sess.Store.Library(ctx)
	if err != nil {
		t.Fatalf("library after install: %v", err)
	}
	installed := findGame(lib, game.ID)
	if installed == nil || !installed.IsInstalled || installed.OutsideFolder {
		t.Fatalf("game not installed inside the folder: %+v", installed)
	}

	if has("launch") {
		if _, err := sess.Store.Launch(ctx, *installed, true); err != nil {
			t.Fatalf("launchSpec preview: %v", err)
		}
		l, err := sess.Store.Launch(ctx, *installed, false)
		if err != nil {
			t.Fatalf("launchSpec: %v", err)
		}
		if l.Direct == nil && l.Client == nil {
			t.Fatalf("launchSpec returned neither direct nor client: %+v", l)
		}
	}

	u, err := sess.q.Add(jobs.KindUninstall, s.ID, game.ID, game.StoreAppID, game.Title, sess.Folder)
	if err != nil {
		t.Fatalf("queue uninstall: %v", err)
	}
	sess.waitJob(u.ID, jobs.StateDone, 30*time.Second)
	lib, err = sess.Store.Library(ctx)
	if err != nil {
		t.Fatalf("library after uninstall: %v", err)
	}
	if left := findGame(lib, game.ID); left != nil && left.IsInstalled {
		t.Fatalf("game still installed after uninstall: %+v", left)
	}
}

func (s *Session) waitJob(id, state string, within time.Duration) types.Job {
	s.t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		for _, j := range s.q.List() {
			if j.ID == id && (j.State == state || j.State == jobs.StateFailed) {
				if j.State != state {
					s.t.Fatalf("job %s ended %s: %s", id, j.State, j.Error)
				}
				return j
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("job %s never reached %s within %s", id, state, within)
	return types.Job{}
}

func findGame(games []types.Game, id string) *types.Game {
	for i := range games {
		if games[i].ID == id {
			return &games[i]
		}
	}
	return nil
}

func copyDir(src, dst string) (string, error) {
	// cp -r src dst puts <src's basename> inside dst when dst doesn't exist;
	// create dst first so the copy lands directly in it.
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return "", err
	}
	out, err := exec.Command("cp", "-r", src+"/.", dst+"/").CombinedOutput()
	return string(out), err
}
