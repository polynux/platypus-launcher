package plugins

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"platypus/internal/jsonrpc"
)

// capsEnv installs echo declaring caps in its manifest.
func capsEnv(t *testing.T, caps ...string) *env {
	t.Helper()
	e := newEnv(t)
	install(t, e.plugins, func(m map[string]any) { m["capabilities"] = caps })
	e.discover(t)
	if err := e.m.Approve("echo"); err != nil {
		t.Fatal(err)
	}
	return e
}

func echoCalls(t *testing.T, e *env) map[string]int {
	t.Helper()
	var got map[string]int
	if err := e.m.Call(ctx(t), "echo", "calls", nil, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestRuntimeBeforeInitializeIsTheManifest(t *testing.T) {
	e := capsEnv(t, "library", "install", "auth")
	rt := e.m.Runtime("echo")
	if rt.Initialized || !slices.Equal(rt.Capabilities, []string{"library", "install", "auth"}) {
		t.Fatalf("before the first initialize: %+v", rt)
	}
	if !e.m.Has("echo", "install") || e.m.Has("echo", "versions") {
		t.Fatal("Has before initialize should follow the manifest")
	}
	if e.m.Has("no-such-plugin", "library") {
		t.Fatal("an unknown plugin offers nothing")
	}
}

func TestInitializeReturnsASubsetOfTheManifest(t *testing.T) {
	t.Setenv("ECHO_OFF", "auth,versions")
	e := capsEnv(t, "library", "install", "auth", "versions")
	info, err := e.m.Initialize(ctx(t), "echo")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"library", "install"}
	if !slices.Equal(info.Capabilities, want) {
		t.Fatalf("initialize capabilities = %v, want %v", info.Capabilities, want)
	}
	rt := e.m.Runtime("echo")
	if !rt.Initialized || !slices.Equal(rt.Capabilities, want) {
		t.Fatalf("runtime = %+v", rt)
	}
	if e.m.Has("echo", "auth") || !e.m.Has("echo", "library") {
		t.Fatal("Has should follow the runtime set once initialized")
	}
	// The manifest, which the user approved and which is hashed, is untouched.
	if p, _ := e.m.Plugin("echo"); !slices.Equal(p.Manifest.Capabilities, []string{"library", "install", "auth", "versions"}) {
		t.Fatalf("manifest changed: %v", p.Manifest.Capabilities)
	}
}

func TestInitializeReportsFeaturesOfRuntimeCapabilitiesOnly(t *testing.T) {
	t.Setenv("ECHO_OFF", "verify")
	t.Setenv("ECHO_FEATURES", `{"versions":{"channels":true,"builds":true,"password":false},"verify":{"repair":true},"made-up":{"x":true}}`)
	e := capsEnv(t, "library", "versions", "verify")
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	f := e.m.Runtime("echo").Features
	if v := f["versions"]; !v["channels"] || !v["builds"] || v["password"] {
		t.Fatalf("versions features = %v", v)
	}
	if _, ok := f["verify"]; ok {
		t.Fatalf("features of a switched-off capability are reported: %v", f)
	}
	if _, ok := f["made-up"]; ok {
		t.Fatalf("features of an undeclared capability are reported: %v", f)
	}
}

func TestInitializeClaimingMoreThanTheManifestIsClamped(t *testing.T) {
	t.Setenv("ECHO_EXTRA", "versions")
	e := capsEnv(t, "library", "install")
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	if rt := e.m.Runtime("echo"); !slices.Equal(rt.Capabilities, []string{"library", "install"}) {
		t.Fatalf("capabilities beyond the manifest were accepted: %+v", rt)
	}
	if e.m.Has("echo", "versions") {
		t.Fatal("an undeclared capability is on")
	}
	data, _ := os.ReadFile(filepath.Join(e.logs, "plugin-echo.log"))
	if !strings.Contains(string(data), "versions") {
		t.Fatalf("the plugin log doesn't mention the ignored claim:\n%s", data)
	}
}

func TestInitializeWithoutCapabilitiesMeansAllDeclared(t *testing.T) {
	t.Setenv("ECHO_INIT_OMIT", "1")
	e := capsEnv(t, "library", "install")
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	if rt := e.m.Runtime("echo"); !rt.Initialized || !slices.Equal(rt.Capabilities, []string{"library", "install"}) {
		t.Fatalf("runtime = %+v", rt)
	}
}

func TestRuntimeSurvivesACrashUntilTheNextInitialize(t *testing.T) {
	t.Setenv("ECHO_OFF", "install")
	e := capsEnv(t, "library", "install")
	if _, err := e.m.Initialize(ctx(t), "echo"); err != nil {
		t.Fatal(err)
	}
	pid := e.m.Running("echo")
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); e.m.Running("echo") == pid && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	if e.m.Has("echo", "install") {
		t.Fatal("a crash must not switch the plugin's features back on")
	}
}

func TestCallsToASwitchedOffCapabilityNeverReachThePlugin(t *testing.T) {
	t.Setenv("ECHO_OFF", "install")
	e := capsEnv(t, "library", "install", "auth")
	err := e.m.Call(ctx(t), "echo", "game.installSize", map[string]any{"id": "tiny", "channel": "beta"}, nil)
	if rpcCode(err) != jsonrpc.CodeMethodNotFound || !strings.Contains(err.Error(), "install") {
		t.Fatalf("game.installSize with install off: %v", err)
	}
	if n := echoCalls(t, e)["game.installSize"]; n != 0 {
		t.Fatalf("the plugin received the call %d times", n)
	}
	// Methods of capabilities that are on, and methods outside any capability, go through.
	if err := e.m.Call(ctx(t), "echo", "auth.status", nil, nil); err != nil {
		t.Fatalf("auth.status: %v", err)
	}
	if err := e.m.Call(ctx(t), "echo", "echo", map[string]any{"x": 1}, nil); err != nil {
		t.Fatalf("echo: %v", err)
	}
}

func TestOptionalMethodIsProbedOncePerRun(t *testing.T) {
	t.Setenv("ECHO_MISSING", "versions.unlock")
	e := capsEnv(t, "library", "versions")
	for i := 0; i < 3; i++ {
		err := e.m.Call(ctx(t), "echo", "versions.unlock", map[string]any{"id": "tiny", "channel": "beta"}, nil)
		if rpcCode(err) != jsonrpc.CodeMethodNotFound {
			t.Fatalf("call %d: %v, want -32601", i, err)
		}
	}
	if n := echoCalls(t, e)["versions.unlock"]; n != 1 {
		t.Fatalf("the plugin received an optional method it had already refused %d times, want 1", n)
	}
	// The plugin starts answering, but the answer is cached for this run.
	if err := e.m.Call(ctx(t), "echo", "config", map[string]any{"missing": []string{}}, nil); err != nil {
		t.Fatal(err)
	}
	err := e.m.Call(ctx(t), "echo", "versions.unlock", map[string]any{"id": "tiny", "channel": "beta"}, nil)
	if rpcCode(err) != jsonrpc.CodeMethodNotFound {
		t.Fatalf("cached refusal was dropped within one run: %v", err)
	}
}

func TestOptionalMethodProbeIsForgottenWhenThePluginRestarts(t *testing.T) {
	t.Setenv("ECHO_MISSING", "versions.unlock")
	e := capsEnv(t, "library", "versions")
	if err := e.m.Call(ctx(t), "echo", "versions.unlock", map[string]any{"id": "tiny", "channel": "beta"}, nil); rpcCode(err) != jsonrpc.CodeMethodNotFound {
		t.Fatalf("first run: %v", err)
	}
	pid := e.m.Running("echo")
	_ = syscall.Kill(pid, syscall.SIGKILL)
	for deadline := time.Now().Add(5 * time.Second); e.m.Running("echo") == pid && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	t.Setenv("ECHO_MISSING", "") // the new run answers it
	if err := e.m.Call(ctx(t), "echo", "versions.unlock", map[string]any{"id": "tiny", "channel": "beta"}, nil); err != nil {
		t.Fatalf("after a restart the method must be asked again: %v", err)
	}
}

func TestRequiredMethodRefusalIsNotCached(t *testing.T) {
	t.Setenv("ECHO_MISSING", "versions.list")
	e := capsEnv(t, "library", "versions")
	for i := 0; i < 2; i++ {
		if err := e.m.Call(ctx(t), "echo", "versions.list", map[string]any{"id": "tiny", "channel": "beta"}, nil); rpcCode(err) != jsonrpc.CodeMethodNotFound {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if n := echoCalls(t, e)["versions.list"]; n != 2 {
		t.Fatalf("a required method was asked %d times, want 2 (only optional ones are cached)", n)
	}
}

func TestUnavailableIsDistinctFromLoginAndNotCached(t *testing.T) {
	t.Setenv("ECHO_UNAVAILABLE", "versions.unlock")
	e := capsEnv(t, "library", "versions")
	for i := 0; i < 2; i++ {
		err := e.m.Call(ctx(t), "echo", "versions.unlock", map[string]any{"id": "tiny", "channel": "beta"}, nil)
		if rpcCode(err) != CodeUnavailable || !IsUnavailable(err) || IsAuthRequired(err) {
			t.Fatalf("call %d: %v", i, err)
		}
		if !strings.Contains(err.Error(), "right now") {
			t.Fatalf("the plugin's message is lost: %q", err)
		}
	}
	if n := echoCalls(t, e)["versions.unlock"]; n != 2 {
		t.Fatalf("-32002 is temporary and must be asked again: received %d times", n)
	}
}

func TestErrorClassifiers(t *testing.T) {
	auth := &jsonrpc.Error{Code: CodeAuthRequired, Message: "log in"}
	busy := &jsonrpc.Error{Code: CodeUnavailable, Message: "later"}
	if !IsAuthRequired(auth) || IsUnavailable(auth) || IsAuthRequired(busy) || !IsUnavailable(busy) {
		t.Fatal("-32001 and -32002 are mixed up")
	}
	if IsAuthRequired(errors.New("x")) || IsUnavailable(nil) {
		t.Fatal("non-RPC errors matched")
	}
	if CodeUnavailable != -32002 {
		t.Fatalf("CodeUnavailable = %d", CodeUnavailable)
	}
}
