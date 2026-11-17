// Tests the Ubisoft Connect plugin (issues #193/#194: setup + launch only)
// through Platypus's real plugin path (manager, JSON-RPC). The installer
// fetch drives the real download code against a loopback server with a
// patched pin (testenv allows loopback); client state is planted in a temp
// prefix. No network, no Wine, no Ubisoft account.
package ubisoft_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"platypus/internal/jsonrpc"
	"platypus/internal/plugins"
	"platypus/internal/plugtest"
)

const id = "ubisoft-connect"

// pinServer serves bytes as the installer download and counts its hits.
func pinServer(t *testing.T, data []byte) (*httptest.Server, *int64) {
	t.Helper()
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func pinEnv(srv *httptest.Server, data []byte) map[string]string {
	sum := sha256.Sum256(data)
	return map[string]string{
		"PLATYPUS_UBISOFT_INSTALLER_URL":    srv.URL + "/UbisoftConnectInstaller.exe",
		"PLATYPUS_UBISOFT_INSTALLER_SHA256": hex.EncodeToString(sum[:]),
		"PLATYPUS_UBISOFT_INSTALLER_SIZE":   fmt.Sprint(len(data)),
	}
}

func suite(extra map[string]string) plugtest.Suite {
	return plugtest.Suite{ID: id, CopyDir: ".", Env: extra}
}

// fakeInstaller is the loopback "Connect installer": 64 KiB of pattern bytes.
func fakeInstaller() []byte {
	data := make([]byte, 64<<10)
	for i := range data {
		data[i] = byte(i * 31)
	}
	return data
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

// plantRecord writes a client-owned install record (the interim stand-in the
// plugin reads; see the plugin docstring).
func plantRecord(t *testing.T, prefix, app, exe string) {
	t.Helper()
	dir := filepath.Join(prefix, "drive_c", "platypus-ubisoft", "records")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, app+".json"),
		[]byte(`{"exe": "`+exe+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

// plantClient writes a client executable into a drive layout.
func plantClient(t *testing.T, prefix, layout, name string) {
	t.Helper()
	exe := filepath.Join(prefix, layout, "Program Files (x86)", "Ubisoft",
		"Ubisoft Game Launcher", name)
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("fake "+name), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestUbisoftHandshake(t *testing.T) {
	s := suite(nil).Handshake(t)
	// The manifest speaks 1.20 (setup + launch), 1.23 (plus the core-run
	// client commands) and 1.25 (plus client.openSpec for the plugin card);
	// the core offering up to 1.25 negotiates exactly that.
	data, err := os.ReadFile("plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	var man struct {
		Protocol struct {
			Versions []string `json:"versions"`
		} `json:"protocol"`
		Capabilities []string `json:"capabilities"`
		Version      string   `json:"version"`
	}
	if err := json.Unmarshal(data, &man); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(man.Protocol.Versions, []string{"1.20", "1.23", "1.25"}) {
		t.Fatalf("manifest protocol versions = %v, want [1.20 1.23 1.25]", man.Protocol.Versions)
	}
	if man.Version != "0.2.0" {
		t.Fatalf("manifest version = %q, want 0.2.0", man.Version)
	}
	if !slices.Equal(man.Capabilities, []string{"library", "install", "launch"}) {
		t.Fatalf("manifest capabilities = %v, want [library install launch]", man.Capabilities)
	}
	var init struct {
		Protocol     string   `json:"protocol"`
		Capabilities []string `json:"capabilities"`
	}
	call(t, s, "initialize",
		map[string]any{"api": 1, "protocols": []string{"1.20", "1.23", "1.25"}}, &init)
	if init.Protocol != "1.25" {
		t.Fatalf("protocol = %q, want the 1.25 pick", init.Protocol)
	}
	if !slices.Equal(init.Capabilities, []string{"library", "install", "launch"}) {
		t.Fatalf("capabilities = %v, want [library install launch] over 1.25", init.Capabilities)
	}
	call(t, s, "initialize",
		map[string]any{"api": 1, "protocols": []string{"1.20", "1.23"}}, &init)
	if init.Protocol != "1.23" {
		t.Fatalf("protocol = %q, want the 1.23 pick", init.Protocol)
	}
	call(t, s, "initialize",
		map[string]any{"api": 1, "protocols": []string{"1.20"}}, &init)
	if init.Protocol != "1.20" {
		t.Fatalf("protocol = %q, want the 1.20 fallback", init.Protocol)
	}
	if !slices.Equal(init.Capabilities, []string{"library", "launch"}) {
		t.Fatalf("capabilities = %v, want [library launch] over 1.20 (no install)", init.Capabilities)
	}
	// Over 1.20 the install methods read as unimplemented; job.cancel stays
	// harmless (stopping nothing).
	wantCode(t, s, "game.install", map[string]any{}, jsonrpc.CodeMethodNotFound)
	wantCode(t, s, "game.installSize", map[string]any{}, jsonrpc.CodeMethodNotFound)
	var discarded json.RawMessage
	call(t, s, "job.cancel", map[string]any{"jobId": "nothing-running"}, &discarded)
}

func TestUbisoftClientSetupFetch(t *testing.T) {
	data := fakeInstaller()
	srv, hits := pinServer(t, data)
	sess := suite(pinEnv(srv, data)).Start(t)
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
	wantExe := filepath.Join(prefix, "drive_c", "platypus-ubisoft", "UbisoftConnectInstaller.exe")
	if decoded.Exe != wantExe {
		t.Errorf("exe = %q, want the prefix copy %q", decoded.Exe, wantExe)
	}
	if got, err := os.ReadFile(decoded.Exe); err != nil || string(got) != string(data) {
		t.Errorf("prefix copy isn't the downloaded bytes: %v", err)
	}
	if !slices.Equal(decoded.Args, []string{"/S"}) {
		t.Errorf("args = %v, want the silent-install [/S]", decoded.Args)
	}
	if decoded.Size != int64(len(data)) {
		t.Errorf("size = %d, want %d", decoded.Size, len(data))
	}
	if !strings.Contains(decoded.Confirm, "downloads") || !strings.Contains(decoded.Confirm, "127.0.0.1") {
		t.Errorf("confirm = %q, want the download approval text", decoded.Confirm)
	}
	// The empty-jobId inspection answers the same spec, reusing the cached
	// download instead of fetching again.
	var again json.RawMessage
	call(t, sess, "client.setup", map[string]any{"jobId": "", "prefix": prefix}, &again)
	if string(again) != string(raw) {
		t.Errorf("inspection spec = %s, want %s", again, raw)
	}
	if n := atomic.LoadInt64(hits); n != 1 {
		t.Errorf("server saw %d downloads, want 1 (cache reuse)", n)
	}
	// The Proton-only layout takes the copy under pfx/drive_c.
	proton := filepath.Join(t.TempDir(), "proton")
	os.MkdirAll(filepath.Join(proton, "pfx", "drive_c"), 0o755)
	call(t, sess, "client.setup", map[string]any{"jobId": "setup-pfx", "prefix": proton}, &raw)
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(proton, "pfx", "drive_c", "platypus-ubisoft",
		"UbisoftConnectInstaller.exe"); decoded.Exe != want {
		t.Errorf("exe = %q, want the Proton-layout copy %q", decoded.Exe, want)
	}
	wantCode(t, sess, "client.setup", map[string]any{"jobId": "x"}, jsonrpc.CodeInvalidParams)
}

func TestUbisoftClientSetupPinMismatch(t *testing.T) {
	data := fakeInstaller()
	srv, _ := pinServer(t, data)

	t.Run("sha", func(t *testing.T) {
		env := pinEnv(srv, data)
		env["PLATYPUS_UBISOFT_INSTALLER_SHA256"] = strings.Repeat("0", 64)
		sess := suite(env).Start(t)
		wantCode(t, sess, "client.setup",
			map[string]any{"jobId": "s", "prefix": t.TempDir()}, -32000)
	})
	t.Run("size", func(t *testing.T) {
		env := pinEnv(srv, data)
		env["PLATYPUS_UBISOFT_INSTALLER_SIZE"] = fmt.Sprint(len(data) + 1)
		sess := suite(env).Start(t)
		wantCode(t, sess, "client.setup",
			map[string]any{"jobId": "s", "prefix": t.TempDir()}, -32000)
	})
}

func TestUbisoftLibraryEmpty(t *testing.T) {
	sess := suite(nil).Start(t)
	var lib struct {
		Games []any `json:"games"`
	}
	call(t, sess, "library.list", nil, &lib)
	if len(lib.Games) != 0 {
		t.Fatalf("library.list = %+v, want no games until #192", lib.Games)
	}
	wantCode(t, sess, "game.details", map[string]any{"id": "635"}, -32000)
}

func TestUbisoftLaunchSpec(t *testing.T) {
	sess := suite(nil).Start(t)
	prefix := filepath.Join(t.TempDir(), "client")
	// No client executable planted: the spec is still answered (Windows
	// paths), so the core reports client.notSetUp and offers the setup.
	plantRecord(t, prefix, "635", "Game.exe")

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
		Delegate string   `json:"delegate"`
		Client   string   `json:"client"`
		Exe      string   `json:"exe"`
		Args     []string `json:"args"`
		Prefix   string   `json:"prefix"`
		Find     struct {
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
	if want := `C:\Program Files (x86)\Ubisoft\Ubisoft Game Launcher\UbisoftConnect.exe`; launch.Exe != want {
		t.Errorf("exe = %q, want the preferred client path %q", launch.Exe, want)
	}
	if !slices.Equal(launch.Args, []string{"uplay://launch/635/0"}) {
		t.Errorf("args = %v, want the launch URL", launch.Args)
	}
	if launch.Prefix != "client" {
		t.Errorf("prefix = %q, want client", launch.Prefix)
	}
	if !slices.Equal(launch.Find.Cmdline, []string{"Game.exe"}) {
		t.Errorf("find = %v, want the game's exe from its record", launch.Find.Cmdline)
	}
	if launch.StartTimeout != 900 {
		t.Errorf("startTimeout = %d, want the 900 s client window", launch.StartTimeout)
	}
	// No record, no spec: the game isn't installed.
	wantCode(t, sess, "game.launchSpec",
		map[string]any{"id": "636", "prefix": prefix}, -32000)
	// A record naming the client itself is refused (find must track the game).
	plantRecord(t, prefix, "637", "upc.exe")
	wantCode(t, sess, "game.launchSpec",
		map[string]any{"id": "637", "prefix": prefix}, -32000)
}

func TestUbisoftClientOpenSpec(t *testing.T) {
	sess := suite(nil).Start(t)
	prefix := filepath.Join(t.TempDir(), "client")
	var raw json.RawMessage
	call(t, sess, "client.openSpec", map[string]any{"prefix": prefix}, &raw)
	var spec map[string]json.RawMessage
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("client.openSpec isn't an object: %s", raw)
	}
	for k := range spec {
		if !slices.Contains(plugtest.OpenSpecKeys, k) {
			t.Errorf("client.openSpec has the key %q, which the protocol doesn't define", k)
		}
	}
	var open struct {
		Exe  string   `json:"exe"`
		Args []string `json:"args"`
	}
	if err := json.Unmarshal(raw, &open); err != nil {
		t.Fatalf("client.openSpec isn't a client spec: %s", raw)
	}
	// The standalone client is the same preferred exe the launches use,
	// with no game args: opening signs in, it launches nothing.
	if want := `C:\Program Files (x86)\Ubisoft\Ubisoft Game Launcher\UbisoftConnect.exe`; open.Exe != want {
		t.Errorf("exe = %q, want the preferred client path %q", open.Exe, want)
	}
	if len(open.Args) != 0 {
		t.Errorf("args = %v, want no game args", open.Args)
	}
	// No prefix, no spec.
	wantCode(t, sess, "client.openSpec", map[string]any{}, jsonrpc.CodeInvalidParams)
}

func TestUbisoftClientStatus(t *testing.T) {
	data := fakeInstaller()
	srv, _ := pinServer(t, data)
	sess := suite(pinEnv(srv, data)).Start(t)

	// No setup ran in this session: the client isn't set up.
	var status struct {
		State string `json:"state"`
		Label string `json:"label"`
	}
	call(t, sess, "game.clientStatus",
		map[string]any{"id": "635", "since": 0}, &status)
	if status.State != "idle" || status.Label == "" {
		t.Errorf("clientStatus = %+v, want the idle not-set-up state", status)
	}

	prefix := filepath.Join(t.TempDir(), "client")
	call(t, sess, "client.setup", map[string]any{"jobId": "", "prefix": prefix}, nil)
	// Client present but the game unknown: waiting (sign in if it asks).
	plantClient(t, prefix, "drive_c", "UbisoftConnect.exe")
	call(t, sess, "game.clientStatus",
		map[string]any{"id": "635", "since": 0}, &status)
	if status.State != "updating" || status.Label == "" {
		t.Errorf("clientStatus = %+v, want the waiting state", status)
	}
	plantRecord(t, prefix, "635", "Game.exe")
	call(t, sess, "game.clientStatus",
		map[string]any{"id": "635", "since": 0}, &status)
	if status.State != "launching" || status.Label == "" {
		t.Errorf("clientStatus = %+v, want the launching state", status)
	}

	// Proton layout plus the older exe name: still found (S2, S3).
	legacy := filepath.Join(t.TempDir(), "legacy")
	call(t, sess, "client.setup", map[string]any{"jobId": "l", "prefix": legacy}, nil)
	plantClient(t, legacy, filepath.Join("pfx", "drive_c"), "upc.exe")
	plantRecord(t, legacy, "635", "Game.exe")
	call(t, sess, "game.clientStatus",
		map[string]any{"id": "635", "since": 0}, &status)
	if status.State != "launching" {
		t.Errorf("clientStatus = %+v, want launching via the Proton/upc.exe fallback", status)
	}
}

// waitJobEvent polls the plugin's job notifications for one of the named
// methods on a job, failing when it doesn't arrive in time.
func waitJobEvent(t *testing.T, s *plugtest.Session, jobID string, methods []string, within time.Duration) (string, plugins.JobUpdate) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		for _, e := range s.Events(jobID) {
			for _, m := range methods {
				if e.Method == m {
					return e.Method, e.Update
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	var got []string
	for _, e := range s.Events(jobID) {
		got = append(got, e.Method+":"+e.Update.JobID)
	}
	t.Fatalf("no %v for job %s within %v (saw %v)", methods, jobID, within, got)
	return "", plugins.JobUpdate{}
}

// noJobEvent fails when one of the named methods arrives for a job within
// the window (a canceled watch must stay silent).
func noJobEvent(t *testing.T, s *plugtest.Session, jobID string, methods []string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		for _, e := range s.Events(jobID) {
			for _, m := range methods {
				if e.Method == m {
					t.Fatalf("unexpected %s for job %s", m, jobID)
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
}

type clientRunAnswer struct {
	ClientRun struct {
		Exe  string   `json:"exe"`
		Args []string `json:"args"`
	} `json:"clientRun"`
	ClientOwnsLocation bool `json:"clientOwnsLocation"`
}

func setupPrefix(t *testing.T, sess *plugtest.Session, prefix string) {
	t.Helper()
	call(t, sess, "client.setup", map[string]any{"jobId": "", "prefix": prefix}, nil)
	plantClient(t, prefix, "drive_c", "UbisoftConnect.exe")
}

func TestUbisoftInstallAnswersClientRun(t *testing.T) {
	data := fakeInstaller()
	srv, _ := pinServer(t, data)
	env := pinEnv(srv, data)
	env["PLATYPUS_UBISOFT_WATCH_TIMEOUT"] = "10"
	env["PLATYPUS_UBISOFT_WATCH_POLL"] = "0.05"
	sess := suite(env).Start(t)
	prefix := filepath.Join(t.TempDir(), "client")
	setupPrefix(t, sess, prefix)

	var ans clientRunAnswer
	call(t, sess, "game.install",
		map[string]any{"id": "635", "installDir": t.TempDir(), "jobId": "install-1"}, &ans)
	if ans.ClientRun.Exe != `C:\Program Files (x86)\Ubisoft\Ubisoft Game Launcher\UbisoftConnect.exe` {
		t.Errorf("clientRun exe = %q, want the client path", ans.ClientRun.Exe)
	}
	if !slices.Equal(ans.ClientRun.Args, []string{"uplay://install/635"}) {
		t.Errorf("clientRun args = %v, want the install URL", ans.ClientRun.Args)
	}
	if !ans.ClientOwnsLocation {
		t.Error("clientOwnsLocation = false, want true (the client owns installs)")
	}
	// The plugin ends the job from the client's records: plant the game's
	// record and the watch reports done.
	plantRecord(t, prefix, "635", "Game.exe")
	waitJobEvent(t, sess, "install-1", []string{"job.done"}, 5*time.Second)

	var upd clientRunAnswer
	call(t, sess, "game.update",
		map[string]any{"id": "635", "installDir": t.TempDir(), "jobId": "update-1"}, &upd)
	if !slices.Equal(upd.ClientRun.Args, []string{"uplay://install/635"}) {
		t.Errorf("update args = %v, want the install URL", upd.ClientRun.Args)
	}
	// A pre-existing record is not completion: the update must not finish
	// until the record changes.
	noJobEvent(t, sess, "update-1", []string{"job.done", "job.failed"}, time.Second)
	plantRecord(t, prefix, "635", "Game2.exe")
	waitJobEvent(t, sess, "update-1", []string{"job.done"}, 5*time.Second)

	var un clientRunAnswer
	call(t, sess, "game.uninstall",
		map[string]any{"id": "635", "jobId": "uninstall-1"}, &un)
	if !slices.Equal(un.ClientRun.Args, []string{"uplay://uninstall/635"}) {
		t.Errorf("uninstall args = %v, want the uninstall URL", un.ClientRun.Args)
	}
	rec := filepath.Join(prefix, "drive_c", "platypus-ubisoft", "records", "635.json")
	if err := os.Remove(rec); err != nil {
		t.Fatal(err)
	}
	waitJobEvent(t, sess, "uninstall-1", []string{"job.done"}, 5*time.Second)

	// An uninstall for a game the client doesn't have fails readably.
	wantCode(t, sess, "game.uninstall",
		map[string]any{"id": "636", "jobId": "uninstall-x"}, -32000)

	// A canceled watch stays silent: cancel, then complete the install.
	var cancelAns clientRunAnswer
	call(t, sess, "game.install",
		map[string]any{"id": "636", "installDir": t.TempDir(), "jobId": "install-cancel"}, &cancelAns)
	call(t, sess, "job.cancel", map[string]any{"jobId": "install-cancel"}, nil)
	plantRecord(t, prefix, "636", "Other.exe")
	noJobEvent(t, sess, "install-cancel", []string{"job.done", "job.failed"}, time.Second)
}

func TestUbisoftInstallWatchTimeout(t *testing.T) {
	data := fakeInstaller()
	srv, _ := pinServer(t, data)
	env := pinEnv(srv, data)
	env["PLATYPUS_UBISOFT_WATCH_TIMEOUT"] = "0.3"
	env["PLATYPUS_UBISOFT_WATCH_POLL"] = "0.05"
	sess := suite(env).Start(t)
	prefix := filepath.Join(t.TempDir(), "client")
	setupPrefix(t, sess, prefix)

	var ans clientRunAnswer
	call(t, sess, "game.install",
		map[string]any{"id": "635", "installDir": t.TempDir(), "jobId": "install-timeout"}, &ans)
	if ans.ClientRun.Exe == "" {
		t.Fatal("install didn't answer a clientRun command")
	}
	// No record ever appears: the watch fails with a readable message that
	// promises nothing (no library reader picks the install up, #192).
	method, update := waitJobEvent(t, sess, "install-timeout", []string{"job.failed"}, 5*time.Second)
	if method != "job.failed" || update.Error == "" ||
		!strings.Contains(update.Error, "didn't report the install of 635 in time") {
		t.Fatalf("job end = %s %+v, want the honest timeout failure", method, update)
	}
}

func TestUbisoftClientStatusSignin(t *testing.T) {
	data := fakeInstaller()
	srv, _ := pinServer(t, data)
	env := pinEnv(srv, data)
	env["PLATYPUS_UBISOFT_SIGNIN_GRACE"] = "0"
	sess := suite(env).Start(t)
	prefix := filepath.Join(t.TempDir(), "client")
	setupPrefix(t, sess, prefix)

	var status struct {
		State string `json:"state"`
		Label string `json:"label"`
	}
	// No install record: not launchable (updating), even with no grace.
	call(t, sess, "game.clientStatus",
		map[string]any{"id": "635", "since": 0}, &status)
	if status.State != "updating" {
		t.Fatalf("clientStatus = %+v, want updating with no record", status)
	}
	plantRecord(t, prefix, "635", "Game.exe")
	// Record present but nothing running: the client starts it (launching).
	call(t, sess, "game.clientStatus",
		map[string]any{"id": "635", "since": 0}, &status)
	if status.State != "launching" {
		t.Fatalf("clientStatus = %+v, want launching with nothing running", status)
	}
	// A client process in this prefix without the game: waiting for the
	// user in the client. (bash's exec -a sets argv[0], which the /proc
	// scan reads; dash's exec has no -a. WINEPREFIX scopes the match to
	// this prefix.)
	spawn := func(name string) {
		t.Helper()
		cmd := exec.Command("bash", "-c", "exec -a "+name+" sleep 30")
		cmd.Env = append(os.Environ(), "WINEPREFIX="+prefix)
		if err := cmd.Start(); err != nil {
			t.Skipf("can't spawn the fake %s process: %v", name, err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	}
	spawn("UbisoftConnect.exe")
	waitStatus := func(want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			call(t, sess, "game.clientStatus",
				map[string]any{"id": "635", "since": 0}, &status)
			if status.State == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("clientStatus = %+v, want %s", status, want)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	waitStatus("signin")
	if status.Label == "" {
		t.Error("signin has no label for the UI")
	}
	// The game itself running reads as launching, as before.
	spawn("Game.exe")
	waitStatus("launching")
}
