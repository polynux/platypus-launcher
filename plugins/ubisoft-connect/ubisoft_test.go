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
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"platypus/internal/jsonrpc"
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
	// The manifest speaks exactly 1.20 (wine-client + client.setup); the
	// core offering 1.21 negotiates exactly that.
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
	if !slices.Equal(man.Protocol.Versions, []string{"1.20"}) {
		t.Fatalf("manifest protocol versions = %v, want [1.20]", man.Protocol.Versions)
	}
	var init struct {
		Capabilities []string `json:"capabilities"`
	}
	call(t, s, "initialize", map[string]any{"api": 1}, &init)
	if !slices.Equal(init.Capabilities, []string{"library", "launch"}) {
		t.Fatalf("capabilities = %v, want [library launch]", init.Capabilities)
	}
	// Install is gone (the core-run client-commands issue): the gate refuses
	// its methods before the plugin.
	wantCode(t, s, "game.install", map[string]any{}, jsonrpc.CodeMethodNotFound)
	wantCode(t, s, "game.installSize", map[string]any{}, jsonrpc.CodeMethodNotFound)
	wantCode(t, s, "job.cancel", map[string]any{}, jsonrpc.CodeMethodNotFound)
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
		Delegate     string `json:"delegate"`
		Client       string `json:"client"`
		Exe          string `json:"exe"`
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
