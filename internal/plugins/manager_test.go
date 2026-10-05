package plugins

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"hydra-launcher/internal/jsonrpc"
	"hydra-launcher/internal/settings"
)

var (
	echoOnce sync.Once
	echoBin  string
	echoErr  error
)

// buildEcho compiles testdata/echo once per test run.
func buildEcho(t *testing.T) string {
	t.Helper()
	echoOnce.Do(func() {
		dir, err := os.MkdirTemp("", "hydra-echo-")
		if err != nil {
			echoErr = err
			return
		}
		echoBin = filepath.Join(dir, "hydra-echo")
		out, err := exec.Command("go", "build", "-o", echoBin, "./testdata/echo").CombinedOutput()
		if err != nil {
			echoErr = errors.New(string(out))
		}
	})
	if echoErr != nil {
		t.Fatalf("building echo plugin: %v", echoErr)
	}
	return echoBin
}

// install copies the echo plugin into pluginsDir/echo, optionally editing its
// manifest first.
func install(t *testing.T, pluginsDir string, edit func(m map[string]any)) string {
	t.Helper()
	dir := filepath.Join(pluginsDir, "echo")
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	bin, err := os.ReadFile(buildEcho(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin/hydra-echo"), bin, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/echo/plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(raw, &m)
	if edit != nil {
		edit(m)
	}
	raw, _ = json.Marshal(m)
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// tamper replaces the executable with the same binary plus a trailing byte,
// the way an update would: it still runs, but its hash differs.
func tamper(t *testing.T, bin string) {
	t.Helper()
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin+".new", append(data, 0), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(bin+".new", bin); err != nil {
		t.Fatal(err)
	}
}

type env struct {
	m       *Manager
	plugins string
	logs    string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	root := t.TempDir()
	trust, err := settings.Open(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	e := &env{plugins: filepath.Join(root, "plugins"), logs: filepath.Join(root, "logs")}
	e.m = NewManager(e.plugins, e.logs, trust, nil)
	e.m.t = timings{
		backoffBase: 20 * time.Millisecond,
		backoffMax:  100 * time.Millisecond,
		stableAfter: time.Minute,
		maxCrashes:  3,
		stopGrace:   time.Second,
		initTimeout: 5 * time.Second,
	}
	t.Cleanup(e.m.Shutdown)
	return e
}

func (e *env) discover(t *testing.T) map[string]Plugin {
	t.Helper()
	list, err := e.m.Discover()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]Plugin{}
	for _, p := range list {
		out[p.Manifest.ID] = p
	}
	return out
}

// ready installs and approves echo.
func (e *env) ready(t *testing.T) {
	t.Helper()
	install(t, e.plugins, nil)
	e.discover(t)
	if err := e.m.Approve("echo"); err != nil {
		t.Fatal(err)
	}
}

func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return c
}

func TestDiscoverNeedsApproval(t *testing.T) {
	e := newEnv(t)
	install(t, e.plugins, nil)
	p := e.discover(t)["echo"]
	if p.Status != StatusUnapproved || p.Hash == "" {
		t.Fatalf("new plugin: %+v", p)
	}
	if _, err := e.m.Initialize(ctx(t), "echo"); err == nil {
		t.Fatal("unapproved plugin was started")
	}
	if err := e.m.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	if p := e.discover(t)["echo"]; p.Status != StatusReady {
		t.Fatalf("approved plugin: %+v", p)
	}
}

func TestEchoInitialize(t *testing.T) {
	e := newEnv(t)
	e.ready(t)
	info, err := e.m.Initialize(ctx(t), "echo")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Echo (test plugin)" || info.API != APIVersion {
		t.Fatalf("initialize = %+v", info)
	}

	var got map[string]any
	if err := e.m.Call(ctx(t), "echo", "echo", map[string]any{"hello": "hydra"}, &got); err != nil {
		t.Fatal(err)
	}
	if got["hello"] != "hydra" {
		t.Fatalf("echo = %v", got)
	}

	err = e.m.Call(ctx(t), "echo", "game.install", nil, nil)
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) || rpcErr.Code != jsonrpc.CodeMethodNotFound {
		t.Fatalf("unknown method: %v", err)
	}

	// The `log` notification sent during initialize lands in the plugin log.
	deadline := time.Now().Add(2 * time.Second)
	for {
		data, _ := os.ReadFile(filepath.Join(e.logs, "plugin-echo.log"))
		if strings.Contains(string(data), "[info] echo initialized") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("log notification missing, log is %q", data)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestConcurrentCallsShareOneProcess(t *testing.T) {
	e := newEnv(t)
	e.ready(t)
	var wg sync.WaitGroup
	pids := make([]int, 20)
	for i := range pids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := e.m.Call(ctx(t), "echo", "pid", nil, &pids[i]); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	for _, pid := range pids {
		if pid != pids[0] || pid == 0 {
			t.Fatalf("calls hit different processes: %v", pids)
		}
	}
}

func TestRestartAfterCrash(t *testing.T) {
	e := newEnv(t)
	e.ready(t)
	var pid1, pid2 int
	if err := e.m.Call(ctx(t), "echo", "pid", nil, &pid1); err != nil {
		t.Fatal(err)
	}
	if err := e.m.Call(ctx(t), "echo", "crash", nil, nil); err == nil {
		t.Fatal("crash returned no error")
	}
	// The next call waits out the backoff and starts a fresh process.
	if err := e.m.Call(ctx(t), "echo", "pid", nil, &pid2); err != nil {
		t.Fatal(err)
	}
	if pid2 == 0 || pid2 == pid1 {
		t.Fatalf("not restarted: %d → %d", pid1, pid2)
	}
}

func TestGivesUpAfterRepeatedCrashes(t *testing.T) {
	e := newEnv(t)
	e.ready(t)
	for i := range e.m.t.maxCrashes {
		if err := e.m.Call(ctx(t), "echo", "crash", nil, nil); err == nil {
			t.Fatalf("crash %d returned no error", i)
		}
	}
	err := e.m.Call(ctx(t), "echo", "echo", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "crashed 3 times") {
		t.Fatalf("want give-up error, got %v", err)
	}
}

func TestChangedHashDisables(t *testing.T) {
	e := newEnv(t)
	e.ready(t)
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	pid := e.m.Running("echo")

	tamper(t, filepath.Join(e.plugins, "echo/bin/hydra-echo"))

	p := e.discover(t)["echo"]
	if p.Status != StatusChanged {
		t.Fatalf("status after change: %+v", p)
	}
	if e.m.Running("echo") != 0 || syscall.Kill(pid, 0) == nil {
		t.Fatal("changed plugin still running")
	}
	if err := e.m.Call(ctx(t), "echo", "echo", nil, nil); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("call on changed plugin: %v", err)
	}
	if err := e.m.SetEnabled("echo", true); err == nil {
		t.Fatal("enable bypassed re-approval")
	}
	if err := e.m.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatalf("after re-approval: %v", err)
	}
}

func TestHashCheckedAtStart(t *testing.T) {
	e := newEnv(t)
	e.ready(t)
	// Swapped after discovery but before the first start.
	tamper(t, filepath.Join(e.plugins, "echo/bin/hydra-echo"))
	if _, err := e.m.Initialize(ctx(t), "echo"); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("modified executable started: %v", err)
	}
}

func TestDisable(t *testing.T) {
	e := newEnv(t)
	e.ready(t)
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	if err := e.m.SetEnabled("echo", false); err != nil {
		t.Fatal(err)
	}
	if e.m.Running("echo") != 0 {
		t.Fatal("disabled plugin still running")
	}
	if p := e.discover(t)["echo"]; p.Status != StatusDisabled {
		t.Fatalf("status: %+v", p)
	}
	if err := e.m.SetEnabled("echo", true); err != nil {
		t.Fatal(err)
	}
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
}

func TestMissingRequirement(t *testing.T) {
	e := newEnv(t)
	install(t, e.plugins, func(m map[string]any) {
		m["requires"] = []map[string]string{
			{"bin": "sh", "hint": "always there"},
			{"bin": "hydra-no-such-tool", "hint": "pipx install hydra-no-such-tool"},
		}
	})
	e.discover(t)
	if err := e.m.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	p := e.discover(t)["echo"]
	if p.Status != StatusMissing || len(p.Missing) != 1 || p.Missing[0].Hint != "pipx install hydra-no-such-tool" {
		t.Fatalf("missing requirement: %+v", p)
	}
	if _, err := e.m.Initialize(ctx(t), "echo"); err == nil {
		t.Fatal("started without its required tool")
	}
}

func TestInvalidManifests(t *testing.T) {
	cases := map[string]func(m map[string]any){
		"wrong api":         func(m map[string]any) { m["api"] = 2 },
		"id mismatch":       func(m map[string]any) { m["id"] = "other" },
		"exec escapes":      func(m map[string]any) { m["exec"] = "../../bin/sh" },
		"exec absolute":     func(m map[string]any) { m["exec"] = "/bin/sh" },
		"exec missing":      func(m map[string]any) { m["exec"] = "bin/nope" },
		"no name":           func(m map[string]any) { delete(m, "name") },
		"bad capability":    func(m map[string]any) { m["capabilities"] = []string{"root"} },
		"requires path":     func(m map[string]any) { m["requires"] = []map[string]string{{"bin": "/usr/bin/legendary"}} },
		"install_dir abs":   func(m map[string]any) { m["install_dir"] = "/etc" },
		"relative perm":     func(m map[string]any) { m["permissions"] = map[string]any{"read": []string{"config"}} },
		"exec not runnable": nil, // handled below
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			dir := install(t, e.plugins, edit)
			if edit == nil {
				os.Chmod(filepath.Join(dir, "bin/hydra-echo"), 0o644)
			}
			p := e.discover(t)["echo"]
			if p.Status != StatusInvalid || p.Problem == "" {
				t.Fatalf("want invalid, got %+v", p)
			}
			if err := e.m.Approve("echo"); err == nil {
				t.Fatal("invalid plugin approved")
			}
		})
	}
}

func TestExecSymlinkOutsideFolder(t *testing.T) {
	e := newEnv(t)
	dir := install(t, e.plugins, nil)
	bin := filepath.Join(dir, "bin/hydra-echo")
	os.Remove(bin)
	if err := os.Symlink(buildEcho(t), bin); err != nil {
		t.Fatal(err)
	}
	if p := e.discover(t)["echo"]; p.Status != StatusInvalid {
		t.Fatalf("symlink out of the plugin folder accepted: %+v", p)
	}
}

func TestShutdownKills(t *testing.T) {
	e := newEnv(t)
	e.ready(t)
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	pid := e.m.Running("echo")
	e.m.Shutdown()
	if syscall.Kill(pid, 0) == nil {
		t.Fatalf("plugin %d survived shutdown", pid)
	}
	if err := e.m.Call(ctx(t), "echo", "echo", nil, nil); err == nil {
		t.Fatal("call after shutdown succeeded")
	}
}

func TestInstallPath(t *testing.T) {
	m := Manifest{ID: "epic-legendary"}
	if got := m.InstallPath("/home/u/Games"); got != "/home/u/Games/epic-legendary" {
		t.Errorf("default: %s", got)
	}
	m.InstallDir = "epic"
	if got := m.InstallPath("/home/u/Games"); got != "/home/u/Games/epic" {
		t.Errorf("install_dir: %s", got)
	}
}
