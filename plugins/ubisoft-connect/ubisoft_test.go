// Tests the Ubisoft Connect plugin (issues #193/#194) through Platypus's real
// plugin path (manager, JSON-RPC) against testdata/wine, a fake `wine` that
// applies uplay:// URLs to the plugin's interim install records. No network,
// no Wine, no Ubisoft account.
package ubisoft_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"platypus/internal/jobs"
	"platypus/internal/jsonrpc"
	"platypus/internal/plugtest"
	"platypus/internal/types"
)

const id = "ubisoft-connect"

// suite wires the plugin with its test doubles: the fake wine on PATH, the
// fake installer instead of a download, and a short record-watch bound.
func suite(t *testing.T, extra map[string]string) plugtest.Suite {
	t.Helper()
	hook, err := filepath.Abs(filepath.Join("testdata", "fake-installer.exe"))
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"PLATYPUS_UBISOFT_INSTALLER_FILE": hook,
		"PLATYPUS_UBISOFT_WATCH_TIMEOUT": "20",
		"FAKE_WINE_DELAY":                 "0.2",
	}
	for k, v := range extra {
		env[k] = v
	}
	return plugtest.Suite{
		ID:      id,
		CopyDir: ".",
		FakeCLI: map[string]string{"wine": filepath.Join("testdata", "wine")},
		Env:     env,
	}
}

func call(t *testing.T, s *plugtest.Session, method string, params, result any) {
	t.Helper()
	if err := s.Call(method, params, result); err != nil {
		t.Fatalf("%s: %v", method, err)
	}
}

func wantCode(t *testing.T, s *plugtest.Session, method string, params any, code int) {
	t.Helper()
	var raw json.RawMessage
	err := s.Call(method, params, &raw)
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("%s: got %s (%v), want RPC error %d", method, raw, err, code)
	}
	if rpcErr.Code != code {
		t.Fatalf("%s: error code %d (%s), want %d", method, rpcErr.Code, rpcErr.Message, code)
	}
}

// waitMethod polls the session's notifications for a job event.
func waitMethod(t *testing.T, s *plugtest.Session, jobID, method string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		for _, e := range s.Events(jobID) {
			if e.Method == method {
				if e.Method == "job.failed" {
					t.Fatalf("job %s failed: %s", jobID, e.Update.Error)
				}
				return
			}
			if e.Method == "job.failed" {
				t.Fatalf("job %s failed: %s", jobID, e.Update.Error)
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("job %s saw no %s within %s", jobID, method, within)
}

func countMethod(s *plugtest.Session, jobID, method string) int {
	n := 0
	for _, e := range s.Events(jobID) {
		if e.Method == method {
			n++
		}
	}
	return n
}

// clientSetup inspects the installer spec for a fresh prefix and plants the
// fake client executable the plugin's fail-fast checks look for.
func clientSetup(t *testing.T, s *plugtest.Session) string {
	t.Helper()
	prefix := filepath.Join(t.TempDir(), "client")
	var raw json.RawMessage
	call(t, s, "client.setup", map[string]any{"jobId": "", "prefix": prefix}, &raw)
	exe := filepath.Join(prefix, "drive_c", "Program Files (x86)", "Ubisoft",
		"Ubisoft Game Launcher", "upc.exe")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("fake upc.exe"), 0o755); err != nil {
		t.Fatal(err)
	}
	return prefix
}

func TestUbisoftHandshake(t *testing.T) {
	s := suite(t, nil).Handshake(t)
	// The manifest declares up to 1.20 (wine-client + client.setup); with the
	// core offering 1.21 that negotiates exactly 1.20.
	data, err := os.ReadFile("plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	var man struct {
		Protocol struct {
			Versions []string `json:"versions"`
		} `json:"protocol"`
	}
	if err := json.Unmarshal(data, &man); err != nil {
		t.Fatal(err)
	}
	if got := man.Protocol.Versions; !slices.Contains(got, "1.20") || got[len(got)-1] != "1.20" {
		t.Fatalf("manifest protocol versions = %v, want up to 1.20", got)
	}
	var init struct {
		Capabilities []string `json:"capabilities"`
	}
	call(t, s, "initialize", map[string]any{"api": 1}, &init)
	if !slices.Equal(init.Capabilities, []string{"library", "install", "launch"}) {
		t.Fatalf("capabilities = %v, want [library install launch]", init.Capabilities)
	}
}

func TestUbisoftClientSetup(t *testing.T) {
	sess := suite(t, nil).Start(t)
	prefix := filepath.Join(t.TempDir(), "client")
	var raw json.RawMessage
	call(t, sess, "client.setup",
		map[string]any{"jobId": "setup-1", "prefix": prefix}, &raw)
	var spec map[string]json.RawMessage
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("client.setup isn't an object: %s", raw)
	}
	for k := range spec {
		if !slices.Contains(plugtest.InstallerKeys, k) {
			t.Errorf("client.setup has the key %q, which the protocol doesn't define", k)
		}
	}
	var decoded struct {
		Exe     string   `json:"exe"`
		Args    []string `json:"args"`
		Size    int64    `json:"size"`
		Confirm string   `json:"confirm"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("client.setup isn't an installer spec: %s", raw)
	}
	hook, _ := os.ReadFile(filepath.Join("testdata", "fake-installer.exe"))
	if !filepath.IsAbs(decoded.Exe) || !strings.HasPrefix(decoded.Exe, prefix) {
		t.Errorf("exe = %q, want an absolute path inside the prefix", decoded.Exe)
	}
	if st, err := os.Stat(decoded.Exe); err != nil || st.Size() != int64(len(hook)) {
		t.Errorf("installer at %q is %v, want the %d-byte test hook", decoded.Exe, st, len(hook))
	}
	if !slices.Equal(decoded.Args, []string{"/S"}) {
		t.Errorf("args = %v, want the silent-install [/S]", decoded.Args)
	}
	if decoded.Size != int64(len(hook)) {
		t.Errorf("size = %d, want %d", decoded.Size, len(hook))
	}
	if !strings.Contains(decoded.Confirm, "247 MB") {
		t.Errorf("confirm = %q, want the download approval text", decoded.Confirm)
	}
	// The empty-jobId inspection answers the same spec.
	var again json.RawMessage
	call(t, sess, "client.setup", map[string]any{"jobId": "", "prefix": prefix}, &again)
	if string(again) != string(raw) {
		t.Errorf("inspection spec = %s, want %s", again, raw)
	}
	wantCode(t, sess, "client.setup", map[string]any{"jobId": "x"}, jsonrpc.CodeInvalidParams)
}

func TestUbisoftLibraryEmpty(t *testing.T) {
	sess := suite(t, nil).Start(t)
	var lib struct {
		Games []any `json:"games"`
	}
	call(t, sess, "library.list", nil, &lib)
	if len(lib.Games) != 0 {
		t.Fatalf("library.list = %+v, want no games until #192", lib.Games)
	}
	// Unknown ids error; install sizes stay unknown (indeterminate installs).
	wantCode(t, sess, "game.details", map[string]any{"id": "635"}, -32000)
	var size struct {
		Download int64 `json:"downloadBytes"`
		Disk     int64 `json:"diskBytes"`
	}
	call(t, sess, "game.installSize", map[string]any{"id": "635"}, &size)
	if size != (struct {
		Download int64 `json:"downloadBytes"`
		Disk     int64 `json:"diskBytes"`
	}{}) {
		t.Fatalf("installSize = %+v, want zeros", size)
	}
}

func TestUbisoftInstallLaunchUninstall(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "wine.calls")
	sess := suite(t, map[string]string{"FAKE_WINE_CALLS": calls}).Start(t)
	prefix := clientSetup(t, sess)
	game := types.Game{ID: sess.Store.WireID("635"), StoreAppID: "635", Title: "Fake Game 635"}

	sess.RunJob(jobs.KindInstall, game)
	record := filepath.Join(prefix, "drive_c", "platypus-ubisoft", "records", "635.json")
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatalf("no install record after the install job: %v", err)
	}
	var rec struct {
		Exe string `json:"exe"`
	}
	if err := json.Unmarshal(data, &rec); err != nil || rec.Exe == "" {
		t.Fatalf("install record = %s, want the game exe name", data)
	}

	var raw json.RawMessage
	call(t, sess, "game.launchSpec",
		map[string]any{"id": "635", "prefix": prefix, "preview": true}, &raw)
	var spec map[string]json.RawMessage
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("launchSpec isn't an object: %s", raw)
	}
	for k := range spec {
		if !slices.Contains(plugtest.LaunchKeys, k) {
			t.Errorf("launchSpec has the key %q, which the protocol doesn't define", k)
		}
	}
	var launch struct {
		Delegate     string   `json:"delegate"`
		Client       string   `json:"client"`
		Exe          string   `json:"exe"`
		Args         []string `json:"args"`
		Prefix       string   `json:"prefix"`
		Find         struct {
			Cmdline []string `json:"cmdline"`
		} `json:"find"`
		StartTimeout int `json:"startTimeout"`
	}
	if err := json.Unmarshal(raw, &launch); err != nil {
		t.Fatalf("launchSpec isn't a wine-client spec: %s", raw)
	}
	if launch.Delegate != "wine-client" || launch.Client == "" {
		t.Errorf("launch = %+v, want the wine-client delegate", launch)
	}
	if launch.Exe != `C:\Program Files (x86)\Ubisoft\Ubisoft Game Launcher\upc.exe` {
		t.Errorf("exe = %q, want the client path", launch.Exe)
	}
	if !slices.Equal(launch.Args, []string{"uplay://launch/635/0"}) {
		t.Errorf("args = %v, want the launch URL", launch.Args)
	}
	if launch.Prefix != "client" {
		t.Errorf("prefix = %q, want client", launch.Prefix)
	}
	if !slices.Equal(launch.Find.Cmdline, []string{rec.Exe}) {
		t.Errorf("find = %v, want the game's exe from its record", launch.Find.Cmdline)
	}
	if launch.StartTimeout < 60 {
		t.Errorf("startTimeout = %d, want a generous client window", launch.StartTimeout)
	}

	var status struct {
		State string `json:"state"`
		Label string `json:"label"`
	}
	call(t, sess, "game.clientStatus",
		map[string]any{"id": "635", "since": time.Now().Unix()}, &status)
	if status.State != "launching" || status.Label == "" {
		t.Errorf("clientStatus = %+v, want the launching state with a label", status)
	}

	// The fake wine saw direct client-exe invocations with the uplay URLs.
	body, _ := os.ReadFile(calls)
	var sawInstall, sawUninstall bool
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		if line == "" {
			continue
		}
		var argv []string
		if err := json.Unmarshal([]byte(line), &argv); err != nil || len(argv) != 3 {
			t.Fatalf("wine call = %q, want [wine client-exe url]", line)
		}
		if !strings.HasSuffix(argv[1], "upc.exe") {
			t.Errorf("wine ran %q, want the client executable", argv[1])
		}
		sawInstall = sawInstall || argv[2] == "uplay://install/635"
		sawUninstall = sawUninstall || argv[2] == "uplay://uninstall/635"
	}
	if !sawInstall {
		t.Error("wine never got uplay://install/635")
	}

	sess.RunJob(jobs.KindUninstall, game)
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("install record still present after the uninstall job")
	}
	if !sawUninstall {
		// Re-read: the uninstall ran after the first assertion pass.
		body, _ = os.ReadFile(calls)
		if !strings.Contains(string(body), "uplay://uninstall/635") {
			t.Error("wine never got uplay://uninstall/635")
		}
	}
	wantCode(t, sess, "game.launchSpec",
		map[string]any{"id": "635", "prefix": prefix}, -32000)
	call(t, sess, "game.clientStatus",
		map[string]any{"id": "635", "since": time.Now().Unix()}, &status)
	if status.State == "launching" {
		t.Errorf("clientStatus = %+v after uninstall, want the waiting state", status)
	}
}

func TestUbisoftCancelInstall(t *testing.T) {
	sess := suite(t, map[string]string{"FAKE_WINE_DELAY": "30"}).Start(t)
	prefix := clientSetup(t, sess)
	installDir := filepath.Join(t.TempDir(), "games")
	if err := sess.Call("game.install",
		map[string]any{"id": "635", "installDir": installDir, "jobId": "cancel-1"}, nil); err != nil {
		t.Fatalf("game.install: %v", err)
	}
	waitMethod(t, sess, "cancel-1", "job.progress", 30*time.Second)
	if err := sess.Call("job.cancel", map[string]any{"jobId": "cancel-1"}, nil); err != nil {
		t.Fatalf("job.cancel: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	for _, e := range sess.Events("cancel-1") {
		if e.Method == "job.done" {
			t.Fatal("plugin reported job.done for a canceled install")
		}
	}
	record := filepath.Join(prefix, "drive_c", "platypus-ubisoft", "records", "635.json")
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("canceled install left the install record")
	}
}

func TestUbisoftUninstallCancel(t *testing.T) {
	sess := suite(t, map[string]string{"FAKE_WINE_DELAY": "30"}).Start(t)
	prefix := clientSetup(t, sess)
	record := filepath.Join(prefix, "drive_c", "platypus-ubisoft", "records", "635.json")
	if err := os.MkdirAll(filepath.Dir(record), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(record, []byte(`{"exe": "Game.exe"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := sess.Call("game.uninstall",
		map[string]any{"id": "635", "jobId": "u-cancel"}, nil); err != nil {
		t.Fatalf("game.uninstall: %v", err)
	}
	waitMethod(t, sess, "u-cancel", "job.progress", 30*time.Second)
	if err := sess.Call("job.cancel", map[string]any{"jobId": "u-cancel"}, nil); err != nil {
		t.Fatalf("job.cancel: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	for _, e := range sess.Events("u-cancel") {
		if e.Method == "job.done" {
			t.Fatal("plugin reported job.done for a canceled uninstall")
		}
	}
	if _, err := os.Stat(record); err != nil {
		t.Fatal("canceled uninstall deleted the install record")
	}
}

func TestUbisoftRepeatedJobID(t *testing.T) {
	sess := suite(t, nil).Start(t)
	clientSetup(t, sess)
	installDir := filepath.Join(t.TempDir(), "games")
	dup := map[string]any{"id": "635", "installDir": installDir, "jobId": "dup-1"}
	if err := sess.Call("game.install", dup, nil); err != nil {
		t.Fatalf("game.install: %v", err)
	}
	if err := sess.Call("game.install", dup, nil); err != nil {
		t.Fatalf("game.install with a repeated jobId: %v", err)
	}
	waitMethod(t, sess, "dup-1", "job.done", 30*time.Second)
	time.Sleep(500 * time.Millisecond)
	if n := countMethod(sess, "dup-1", "job.done"); n != 1 {
		t.Fatalf("repeated jobId reported job.done %d times, want 1", n)
	}
}
