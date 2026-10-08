// Package plugtest drives any store plugin through every protocol method it
// declares, the way Hydra drives it (see docs/plugins/protocol.md). The
// conformance suite lives here, not in one plugin's tests, so each plugin
// package only wires up its own plugin-specific bits: where its fake CLI is
// and how to log in and pick a game from its library.
package plugtest

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"hydra-launcher/internal/jobs"
	"hydra-launcher/internal/jsonrpc"
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

	// Escape, when set, returns a library game the plugin reports as
	// installed outside its folder; Hydra must refuse to launch it.
	Escape func(t *testing.T, games []types.Game) *types.Game
	// NoSpeed excludes the check that job.progress carries a speed (a
	// plugin whose tool reports none).
	NoSpeed bool
	// NoDuplicateJobID excludes the same-jobId idempotency check.
	NoDuplicateJobID bool
	// NoUninstallCancel excludes the check that job.cancel stops an uninstall
	// before it deletes anything (a plugin whose uninstall is instant).
	NoUninstallCancel bool
	// NoUnknownDetails excludes the check that game.details on an unknown id
	// errors (a plugin whose test double answers every id).
	NoUnknownDetails bool
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

	evMu   sync.Mutex
	events []event
}

// event is one job notification the plugin sent, as the host saw it.
type event struct {
	Method string
	Update plugins.JobUpdate
}

// Events returns the notifications seen for a job, in order.
func (s *Session) Events(jobID string) []event {
	s.evMu.Lock()
	defer s.evMu.Unlock()
	var out []event
	for _, e := range s.events {
		if e.Update.JobID == jobID {
			out = append(out, e)
		}
	}
	return out
}

func (s *Session) record(method string, u plugins.JobUpdate) {
	s.evMu.Lock()
	s.events = append(s.events, event{method, u})
	s.evMu.Unlock()
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
				sess.record(method, u)
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
// initialize (against the manifest), error codes, auth with a logout and
// -32001 round trip, library.list, game.details (tolerating a plugin that
// doesn't implement it, but not an unknown id answered with data),
// installSize, install with progress through the real job queue, launchSpec
// (preview, real and negative), update, cancel of an install and of an
// uninstall, a repeated jobId, concurrent requests while a job runs, and
// uninstall. Plugin-specific expectations stay in the plugin's own tests;
// this one checks the protocol and the behaviours every plugin must share.
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

	// initialize must agree with the manifest the user approved.
	var init struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := sess.call("initialize", map[string]any{"api": 1}, &init); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	gotCaps, wantCaps := append([]string{}, init.Capabilities...), append([]string{}, caps()...)
	sort.Strings(gotCaps)
	sort.Strings(wantCaps)
	if !slices.Equal(gotCaps, wantCaps) {
		t.Fatalf("initialize capabilities %v differ from the manifest's %v", gotCaps, wantCaps)
	}

	// Error codes: an unknown method is -32601 (missing params, -32602, is
	// checked once logged in).
	sess.wantCode("no.such.method", nil, jsonrpc.CodeMethodNotFound)

	if has("auth") {
		if s.Login == nil {
			t.Fatal("plugin declares auth but the Suite has no Login")
		}
		var st types.AuthStatus
		if err := sess.call("auth.status", nil, &st); err != nil {
			t.Fatalf("auth.status: %v", err)
		}
		if st.LoggedIn {
			t.Fatalf("auth.status reports logged in before any login: %+v", st)
		}
		if has("library") {
			sess.wantCode("library.list", nil, CodeAuthRequired)
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
		// Logging out takes the library away again (-32001), and a second
		// login works.
		var out types.AuthStatus
		if err := sess.call("auth.logout", nil, &out); err != nil || out.LoggedIn {
			t.Fatalf("auth.logout: %+v, %v", out, err)
		}
		if has("library") {
			sess.wantCode("library.list", nil, CodeAuthRequired)
		}
		if err := sess.call("auth.begin", nil, &pr); err != nil {
			t.Fatalf("auth.begin after logout: %v", err)
		}
		s.Login(t, sess)
	}

	if has("install") {
		sess.wantCode("game.installSize", map[string]any{}, jsonrpc.CodeInvalidParams)
	}

	if !has("library") {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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
	if !s.NoUnknownDetails {
		// An unknown id is an error (or the method is absent), never data or
		// an empty page: all plugins answer the same way.
		var raw json.RawMessage
		err := sess.call("game.details", map[string]any{"id": "no-such-game-0xdead"}, &raw)
		var rpcErr *jsonrpc.Error
		if err == nil || !errors.As(err, &rpcErr) {
			t.Fatalf("game.details on an unknown id answered %s (err %v), want an RPC error", raw, err)
		}
	}

	install := func() types.Job {
		t.Helper()
		j, err := sess.q.Add(jobs.KindInstall, s.ID, game.ID, game.StoreAppID, game.Title, sess.Folder)
		if err != nil {
			t.Fatalf("queue install: %v", err)
		}
		return sess.waitJob(j.ID, jobs.StateDone, 60*time.Second)
	}
	done := install()
	if done.Total <= 0 || done.Bytes != done.Total {
		t.Fatalf("install job didn't report full progress: %+v", done)
	}
	sess.checkProgress(done.ID, s.NoSpeed)

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
		// Negative cases: an id the plugin never installed has no spec, and a
		// game installed outside the plugin's folder is refused.
		if _, err := sess.Store.Launch(ctx, types.Game{StoreAppID: "no-such-game-0xdead"}, false); err == nil {
			t.Fatal("launchSpec for an unknown game succeeded")
		}
		if s.Escape != nil {
			if esc := s.Escape(t, lib); esc != nil {
				if _, err := sess.Store.Launch(ctx, *esc, false); err == nil {
					t.Fatalf("launchSpec for a game outside the install folder was not refused: %+v", esc)
				}
			}
		}
	}

	// update: a plugin answers game.update with its own update path, and the
	// game stays installed. 5 library.list run in parallel while it works
	// (the doc requires answering requests concurrently).
	up, err := sess.q.Add(jobs.KindUpdate, s.ID, game.ID, game.StoreAppID, game.Title, sess.Folder)
	if err != nil {
		t.Fatalf("queue update: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := sess.Store.Library(ctx); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("library.list while a job runs: %v", err)
	}
	sess.waitJob(up.ID, jobs.StateDone, 60*time.Second)
	lib, _ = sess.Store.Library(ctx)
	if g := findGame(lib, game.ID); g == nil || !g.IsInstalled {
		t.Fatalf("game not installed after update: %+v", g)
	}

	// job.cancel on an uninstall must stop it before anything is deleted:
	// the queue already treats the job as canceled, so a plugin that carries
	// on would delete the game behind the user's back.
	if !s.NoUninstallCancel {
		cu, err := sess.q.Add(jobs.KindUninstall, s.ID, game.ID, game.StoreAppID, game.Title, sess.Folder)
		if err != nil {
			t.Fatalf("queue uninstall: %v", err)
		}
		sess.waitState(cu.ID, jobs.StateRunning, 10*time.Second)
		if err := sess.q.Cancel(ctx, cu.ID); err != nil {
			t.Fatalf("cancel uninstall: %v", err)
		}
		time.Sleep(1500 * time.Millisecond)
		lib, _ = sess.Store.Library(ctx)
		if g := findGame(lib, game.ID); g == nil || !g.IsInstalled {
			t.Fatalf("canceled uninstall still deleted the game: %+v", g)
		}
	}

	// Real uninstall.
	u, err := sess.q.Add(jobs.KindUninstall, s.ID, game.ID, game.StoreAppID, game.Title, sess.Folder)
	if err != nil {
		t.Fatalf("queue uninstall: %v", err)
	}
	sess.waitJob(u.ID, jobs.StateDone, 60*time.Second)
	lib, err = sess.Store.Library(ctx)
	if err != nil {
		t.Fatalf("library after uninstall: %v", err)
	}
	if left := findGame(lib, game.ID); left != nil && left.IsInstalled {
		t.Fatalf("game still installed after uninstall: %+v", left)
	}

	// job.cancel on an install: the queue marks it canceled, the plugin stops
	// and neither reports job.done nor leaves the game installed.
	ci, err := sess.q.Add(jobs.KindInstall, s.ID, game.ID, game.StoreAppID, game.Title, sess.Folder)
	if err != nil {
		t.Fatalf("queue install: %v", err)
	}
	sess.waitProgress(ci.ID, 30*time.Second)
	if err := sess.q.Cancel(ctx, ci.ID); err != nil {
		t.Fatalf("cancel install: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	for _, e := range sess.Events(ci.ID) {
		if e.Method == "job.done" {
			t.Fatalf("plugin reported job.done for a canceled install")
		}
	}
	for _, j := range sess.q.List() {
		if j.ID == ci.ID && j.State != jobs.StateCanceled {
			t.Fatalf("canceled install ended %s", j.State)
		}
	}
	lib, _ = sess.Store.Library(ctx)
	if g := findGame(lib, game.ID); g != nil && g.IsInstalled {
		t.Fatalf("canceled install left the game installed: %+v", g)
	}

	// The same jobId sent twice (Hydra resends after a crash) runs once.
	if !s.NoDuplicateJobID {
		dup := map[string]any{"id": game.StoreAppID, "installDir": sess.Folder, "jobId": "dup-job-1"}
		if err := sess.call("game.install", dup, nil); err != nil {
			t.Fatalf("game.install: %v", err)
		}
		if err := sess.call("game.install", dup, nil); err != nil {
			t.Fatalf("game.install with a repeated jobId: %v", err)
		}
		sess.waitEvent("dup-job-1", "job.done", 60*time.Second)
		time.Sleep(500 * time.Millisecond)
		n := 0
		for _, e := range sess.Events("dup-job-1") {
			if e.Method == "job.done" {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("repeated jobId reported job.done %d times, want 1", n)
		}
		u, err := sess.q.Add(jobs.KindUninstall, s.ID, game.ID, game.StoreAppID, game.Title, sess.Folder)
		if err != nil {
			t.Fatalf("queue uninstall: %v", err)
		}
		sess.waitJob(u.ID, jobs.StateDone, 60*time.Second)
	}
}

// CodeAuthRequired is the code a plugin answers with when the user has to
// log in first (docs/plugins/protocol.md).
const CodeAuthRequired = -32001

// wantCode checks that a call fails with a JSON-RPC error of that code.
func (s *Session) wantCode(method string, params any, code int) {
	s.t.Helper()
	var raw json.RawMessage
	err := s.call(method, params, &raw)
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) {
		s.t.Fatalf("%s: got %s, %v; want an RPC error %d", method, raw, err, code)
	}
	if rpcErr.Code != code {
		s.t.Fatalf("%s: error code %d (%s), want %d", method, rpcErr.Code, rpcErr.Message, code)
	}
}

// checkProgress asserts the install's job.progress notifications carried
// bytes and total, and a speed when the plugin is expected to report one.
func (s *Session) checkProgress(jobID string, noSpeed bool) {
	s.t.Helper()
	var withBytes, withSpeed bool
	for _, e := range s.Events(jobID) {
		if e.Method != "job.progress" {
			continue
		}
		if e.Update.Bytes > 0 && e.Update.Total > 0 {
			withBytes = true
		}
		if e.Update.Speed > 0 {
			withSpeed = true
		}
	}
	if !withBytes {
		s.t.Fatalf("no job.progress carried bytes and total")
	}
	if !noSpeed && !withSpeed {
		s.t.Fatalf("no job.progress carried a speed (bytes/s)")
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

// waitState waits for a job to be in a state, failing if it already ended.
func (s *Session) waitState(id, state string, within time.Duration) {
	s.t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		for _, j := range s.q.List() {
			if j.ID != id {
				continue
			}
			if j.State == state {
				return
			}
			if j.State == jobs.StateDone || j.State == jobs.StateFailed || j.State == jobs.StateCanceled {
				s.t.Fatalf("job %s ended %s (%s) before reaching %s", id, j.State, j.Error, state)
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	s.t.Fatalf("job %s never reached %s within %s", id, state, within)
}

// waitProgress waits until the plugin has reported bytes for a job.
func (s *Session) waitProgress(id string, within time.Duration) {
	s.t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		for _, e := range s.Events(id) {
			if e.Method == "job.progress" && e.Update.Bytes > 0 {
				return
			}
			if e.Method == "job.done" || e.Method == "job.failed" {
				s.t.Fatalf("job %s ended (%s) before it could be canceled", id, e.Method)
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	s.t.Fatalf("job %s reported no progress within %s", id, within)
}

// waitEvent waits for one notification method on a job.
func (s *Session) waitEvent(id, method string, within time.Duration) {
	s.t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		for _, e := range s.Events(id) {
			if e.Method == method {
				return
			}
			if e.Method == "job.failed" {
				s.t.Fatalf("job %s failed: %s", id, e.Update.Error)
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("job %s never sent %s within %s", id, method, within)
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
