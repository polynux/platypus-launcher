package stores

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"platypus/internal/plugins"
)

// A wine-client launch (protocol 1.19) runs the store client itself under the
// runner in its own Platypus-managed prefix. Validation is the direct
// launch's containment (the prefix rides in the folders) with the same env
// denylist, plus the prefix key and the game's find strings.
func TestValidateWineClientSpec(t *testing.T) {
	root := t.TempDir()
	prefix := ClientPrefixDir("echo", filepath.Join(root, "prefixes"))
	clientDir := filepath.Join(prefix, "drive_c", "client")
	os.MkdirAll(clientDir, 0o755)
	exe := filepath.Join(clientDir, "client.exe")
	os.WriteFile(exe, []byte("MZ"), 0o644)
	install := filepath.Join(root, "Games", "echo")
	os.MkdirAll(install, 0o755)
	outside := filepath.Join(root, "elsewhere.exe")
	os.WriteFile(outside, []byte("MZ"), 0o644)

	man := plugins.Manifest{ID: "echo", Name: "Echo"}
	folders := []string{install, prefix}
	find := &plugins.FindSpec{Cmdline: []string{"Game.exe"}}

	l, err := ValidateSpec(plugins.LaunchSpec{
		Delegate: "wine-client", Client: "Echo client",
		Exe: exe, Args: []string{"uplay://launch/1/0"}, Prefix: "client",
		Find: find, StartTimeout: 60,
	}, man, folders)
	if err != nil {
		t.Fatalf("valid wine-client spec: %v", err)
	}
	if l.Client == nil || l.Client.Wine == nil {
		t.Fatalf("valid wine-client spec came back as %+v, want a wine client launch", l)
	}
	if l.Client.Wine.Exe != exe || len(l.Client.Wine.Args) != 1 || l.Client.StartTimeout.Seconds() != 60 {
		t.Fatalf("wine launch = %+v, want the validated client spec", l.Client.Wine)
	}

	bad := map[string]plugins.LaunchSpec{
		"other prefix":   {Delegate: "wine-client", Exe: exe, Prefix: "games", Find: find},
		"missing prefix": {Delegate: "wine-client", Exe: exe, Find: find},
		"no exe":         {Delegate: "wine-client", Prefix: "client", Find: find},
		"relative exe":   {Delegate: "wine-client", Exe: "drive_c/client.exe", Prefix: "client", Find: find},
		"exe outside":    {Delegate: "wine-client", Exe: outside, Prefix: "client", Find: find},
		"no find":        {Delegate: "wine-client", Exe: exe, Prefix: "client"},
		"weak find":      {Delegate: "wine-client", Exe: exe, Prefix: "client", Find: &plugins.FindSpec{Cmdline: []string{"a"}}},
		"denied env":     {Delegate: "wine-client", Exe: exe, Prefix: "client", Find: find, Env: map[string]string{"WINEPREFIX": "/tmp"}},
		"unknown client": {Delegate: "wine-client", Exe: exe, Prefix: "client", Find: find, Client: ""},
		"huge timeout":   {Delegate: "wine-client", Exe: exe, Prefix: "client", Find: find, StartTimeout: 1 << 30},
	}
	for name, spec := range bad {
		switch name {
		case "unknown client", "huge timeout":
			// A missing client name defaults to the manifest's; an absurd
			// timeout clamps to the session default. Both validate.
			if _, err := ValidateSpec(spec, man, folders); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		default:
			if _, err := ValidateSpec(spec, man, folders); err == nil {
				t.Errorf("%s: accepted", name)
			}
		}
	}
	missing := filepath.Join(clientDir, "not-installed.exe")
	err = mustWineErr(plugins.LaunchSpec{Delegate: "wine-client", Exe: missing, Prefix: "client", Find: find}, man, folders)
	if err == nil || !strings.Contains(err.Error(), "isn't set up yet") {
		t.Fatalf("missing client exe: got %v, want the setup prompt", err)
	}
}

func TestClientPrefixDir(t *testing.T) {
	if got := ClientPrefixDir("echo", "/data/prefixes"); got != "/data/prefixes/p.echo.client" {
		t.Errorf("ClientPrefixDir = %q", got)
	}
	if ClientGameID("echo") != "p.echo.client" {
		t.Errorf("ClientGameID = %q", ClientGameID("echo"))
	}
}

// A plugin that settled on an older protocol can't launch through a wine
// client, even when the client is installed: the delegate is 1.19.
func TestWineClientSpecRefusedBelow119(t *testing.T) {
	t.Setenv("ECHO_PROTOCOLS", "1.0,1.18")
	e := newEchoEnv(t)
	e.login(t)
	if got := e.m.Runtime("echo").Protocol; got != "1.18" {
		t.Fatalf("negotiated %q, want 1.18", got)
	}
	prefix := t.TempDir()
	if err := e.m.Call(e.ctx(t), "echo", "client.setup",
		map[string]any{"jobId": "setup-1", "prefix": prefix}, nil); err != nil {
		t.Fatalf("client.setup: %v", err)
	}
	exe := filepath.Join(prefix, "drive_c", "echo-client", "echo-client.sh")
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(exe); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("client.setup never installed the fake client")
		}
		time.Sleep(20 * time.Millisecond)
	}
	p, _ := e.m.Plugin("echo")
	withPrefix := NewPlugin(e.m, p, func() []string {
		return append(e.m.Folders("echo", e.games), prefix)
	})
	if _, err := withPrefix.Launch(e.ctx(t), e.game(t, "wine"), false); err == nil ||
		!strings.Contains(err.Error(), "older protocol") {
		t.Fatalf("wine launch on 1.18: %v, want the protocol refusal", err)
	}
}

func mustWineErr(spec plugins.LaunchSpec, man plugins.Manifest, folders []string) error {
	_, err := ValidateSpec(spec, man, folders)
	return err
}
