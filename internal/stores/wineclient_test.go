package stores

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/apperr"
	"platypus/internal/plugins"
)

// A wine-client launch (protocol 1.20) runs the store client itself under the
// runner in its own Platypus-managed prefix. Validation is the direct
// launch's containment (the prefix rides in the folders) with the same env
// denylist, plus the prefix key and the game's find strings.
func TestResolveWineExe(t *testing.T) {
	root := t.TempDir()
	prefix := ClientPrefixDir("echo", filepath.Join(root, "prefixes"))
	wineExe := filepath.Join(prefix, "drive_c", "client", "client.exe")
	os.MkdirAll(filepath.Dir(wineExe), 0o755)
	os.WriteFile(wineExe, []byte("MZ"), 0o644)
	protonExe := filepath.Join(prefix, "pfx", "drive_c", "client", "proton.exe")
	os.MkdirAll(filepath.Dir(protonExe), 0o755)
	os.WriteFile(protonExe, []byte("MZ"), 0o644)

	for name, c := range map[string]struct {
		exe  string
		want string
	}{
		"absolute linux path":   {wineExe, wineExe},
		"windows backslashes":   {`C:\client\client.exe`, wineExe},
		"windows slashes":       {"C:/client/client.exe", wineExe},
		"lowercase drive":       {`c:\client\client.exe`, wineExe},
		"proton layout":         {`C:\client\proton.exe`, protonExe},
		"missing client exe":    {`C:\client\missing.exe`, filepath.Join(prefix, "drive_c", "client", "missing.exe")},
		"missing absolute path": {filepath.Join(prefix, "drive_c", "gone.exe"), filepath.Join(prefix, "drive_c", "gone.exe")},
	} {
		got, err := resolveWineExe(c.exe, prefix)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", name, got, c.want)
		}
	}

	for name, exe := range map[string]string{
		"other drive": `D:\client\client.exe`,
		"escape":      `C:\..\client.exe`,
		"bare drive":  `C:`,
		"relative":    `drive_c/client.exe`,
		"url":         `uplay://launch/1/0`,
		"empty":       ``,
	} {
		if _, err := resolveWineExe(exe, prefix); err == nil {
			t.Errorf("%s (%q): accepted", name, exe)
		}
	}
}

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
	// A symlink inside the prefix pointing outside it must not validate.
	link := filepath.Join(clientDir, "escape.exe")
	os.Symlink(outside, link)

	man := plugins.Manifest{ID: "echo", Name: "Echo"}
	folders := []string{install, prefix}
	find := &plugins.FindSpec{Cmdline: []string{"Game.exe"}}

	valid := []plugins.LaunchSpec{
		{Delegate: "wine-client", Client: "Echo client",
			Exe: exe, Args: []string{"uplay://launch/1/0"}, Prefix: "client",
			Find: find, StartTimeout: 60},
		{Delegate: "wine-client", // a Windows path naming the same exe
			Exe: `C:\client\client.exe`, Prefix: "client", Find: find},
		{Delegate: "wine-client", Client: "", // the client name defaults to the manifest's
			Exe: exe, Prefix: "client", Find: find},
		{Delegate: "wine-client", // an absurd timeout clamps to the session default
			Exe: exe, Prefix: "client", Find: find, StartTimeout: 1 << 30},
	}
	for i, spec := range valid {
		l, err := ValidateSpec(spec, man, folders, prefix)
		if err != nil {
			t.Errorf("valid[%d]: %v", i, err)
			continue
		}
		if l.Client == nil || l.Client.Wine == nil {
			t.Errorf("valid[%d]: came back as %+v, want a wine client launch", i, l)
		}
	}
	l, err := ValidateSpec(valid[0], man, folders, prefix)
	if err != nil {
		t.Fatal(err)
	}
	if l.Client.Wine.Exe != exe || len(l.Client.Wine.Args) != 1 || l.Client.StartTimeout.Seconds() != 60 {
		t.Fatalf("wine launch = %+v, want the validated client spec", l.Client.Wine)
	}
	if l.Client.Client != "Echo client" {
		t.Fatalf("client = %q", l.Client.Client)
	}

	bad := map[string]plugins.LaunchSpec{
		"other prefix":   {Delegate: "wine-client", Exe: exe, Prefix: "games", Find: find},
		"missing prefix": {Delegate: "wine-client", Exe: exe, Find: find},
		"no exe":         {Delegate: "wine-client", Prefix: "client", Find: find},
		"relative exe":   {Delegate: "wine-client", Exe: "drive_c/client.exe", Prefix: "client", Find: find},
		"exe outside":    {Delegate: "wine-client", Exe: outside, Prefix: "client", Find: find},
		"symlink escape": {Delegate: "wine-client", Exe: link, Prefix: "client", Find: find},
		"no find":        {Delegate: "wine-client", Exe: exe, Prefix: "client"},
		"weak find":      {Delegate: "wine-client", Exe: exe, Prefix: "client", Find: &plugins.FindSpec{Cmdline: []string{"a"}}},
		"denied env":     {Delegate: "wine-client", Exe: exe, Prefix: "client", Find: find, Env: map[string]string{"WINEPREFIX": "/tmp"}},
	}
	for name, spec := range bad {
		if _, err := ValidateSpec(spec, man, folders, prefix); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	// A missing executable inside the client's prefix means the client isn't
	// set up yet (client.notSetUp), not a broken spec.
	missing := filepath.Join(clientDir, "not-installed.exe")
	_, err = ValidateSpec(plugins.LaunchSpec{Delegate: "wine-client", Exe: missing, Prefix: "client", Find: find}, man, folders, prefix)
	if apperr.CodeOf(err) != "client.notSetUp" || !strings.Contains(err.Error(), "isn't set up yet") {
		t.Fatalf("missing client exe: got %v, want the client.notSetUp prompt", err)
	}
}

func TestValidateClientSetup(t *testing.T) {
	root := t.TempDir()
	prefix := ClientPrefixDir("echo", filepath.Join(root, "prefixes"))
	binDir := filepath.Join(prefix, "drive_c", "installer")
	os.MkdirAll(binDir, 0o755)
	exe := filepath.Join(binDir, "setup.exe")
	os.WriteFile(exe, []byte("MZ"), 0o755)
	outside := filepath.Join(root, "setup.exe")
	os.WriteFile(outside, []byte("MZ"), 0o755)

	man := plugins.Manifest{ID: "echo", Name: "Echo"}
	folders := []string{filepath.Join(root, "Games", "echo"), prefix}

	got, err := ValidateClientSetup(plugins.ClientSetupSpec{
		Exe: exe, Args: []string{"/S"}, Env: map[string]string{"LOG": "1"},
		Redact: []string{"--token="}, Size: 1 << 20, Confirm: "This downloads 800 MB.",
	}, man, folders, prefix)
	if err != nil {
		t.Fatalf("valid installer spec: %v", err)
	}
	if got.Exe != exe || len(got.Args) != 1 || got.Env["LOG"] != "1" ||
		len(got.Redact) != 1 || got.Size != 1<<20 || got.Confirm == "" {
		t.Fatalf("installer spec = %+v, want the validated fields", got)
	}

	win, err := ValidateClientSetup(plugins.ClientSetupSpec{Exe: `C:\installer\setup.exe`}, man, folders, prefix)
	if err != nil || win.Exe != exe {
		t.Fatalf("windows installer path: got %+v, %v; want %s", win, err, exe)
	}

	for name, spec := range map[string]plugins.ClientSetupSpec{
		"no exe":        {},
		"missing exe":   {Exe: filepath.Join(binDir, "gone.exe")},
		"exe outside":   {Exe: outside},
		"denied env":    {Exe: exe, Env: map[string]string{"WINEPREFIX": "/tmp"}},
		"negative size": {Exe: exe, Size: -1},
		"other drive":   {Exe: `D:\setup.exe`},
		"relative exe":  {Exe: "setup.exe"},
	} {
		if _, err := ValidateClientSetup(spec, man, folders, prefix); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// A clientRun answer (protocol 1.23) validates like a client.setup exe: it
// must exist now, inside the prefix or an allowed folder, with the same env
// denylist.
func TestValidateClientRun(t *testing.T) {
	root := t.TempDir()
	prefix := ClientPrefixDir("echo", filepath.Join(root, "prefixes"))
	binDir := filepath.Join(prefix, "drive_c", "client")
	os.MkdirAll(binDir, 0o755)
	exe := filepath.Join(binDir, "client.exe")
	os.WriteFile(exe, []byte("MZ"), 0o755)
	outside := filepath.Join(root, "client.exe")
	os.WriteFile(outside, []byte("MZ"), 0o755)

	man := plugins.Manifest{ID: "echo", Name: "Echo"}
	folders := []string{filepath.Join(root, "Games", "echo"), prefix}

	got, err := ValidateClientRun(plugins.ClientRunSpec{
		Exe: exe, Args: []string{"uplay://install/1"}, Env: map[string]string{"LOG": "1"},
		Redact: []string{"--token="},
	}, man, folders, prefix)
	if err != nil {
		t.Fatalf("valid clientRun spec: %v", err)
	}
	if got.Exe != exe || len(got.Args) != 1 || got.Env["LOG"] != "1" || len(got.Redact) != 1 {
		t.Fatalf("clientRun spec = %+v, want the validated fields", got)
	}

	win, err := ValidateClientRun(plugins.ClientRunSpec{Exe: `C:\client\client.exe`}, man, folders, prefix)
	if err != nil || win.Exe != exe {
		t.Fatalf("windows client path: got %+v, %v; want %s", win, err, exe)
	}

	for name, spec := range map[string]plugins.ClientRunSpec{
		"no exe":       {},
		"missing exe":  {Exe: filepath.Join(binDir, "gone.exe")},
		"exe outside":  {Exe: outside},
		"denied env":   {Exe: exe, Env: map[string]string{"WINEPREFIX": "/tmp"}},
		"other drive":  {Exe: `D:\client.exe`},
		"relative exe": {Exe: "client.exe"},
	} {
		if _, err := ValidateClientRun(spec, man, folders, prefix); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// A client.openSpec answer (protocol 1.25) validates like a clientRun exe
// and defaults its working dir to the executable's folder. A missing
// executable inside the prefix reads as client.notSetUp, not a broken spec.
func TestValidateClientOpen(t *testing.T) {
	root := t.TempDir()
	prefix := ClientPrefixDir("echo", filepath.Join(root, "prefixes"))
	binDir := filepath.Join(prefix, "drive_c", "client")
	os.MkdirAll(binDir, 0o755)
	exe := filepath.Join(binDir, "client.exe")
	os.WriteFile(exe, []byte("MZ"), 0o755)
	outside := filepath.Join(root, "client.exe")
	os.WriteFile(outside, []byte("MZ"), 0o755)
	link := filepath.Join(binDir, "escape.exe")
	os.Symlink(outside, link)

	man := plugins.Manifest{ID: "echo", Name: "Echo"}
	folders := []string{filepath.Join(root, "Games", "echo"), prefix}

	got, err := ValidateClientOpen(plugins.ClientOpenSpec{Exe: exe}, man, folders, prefix)
	if err != nil {
		t.Fatalf("valid open spec: %v", err)
	}
	if got.Exe != exe || len(got.Args) != 0 || got.Dir != binDir {
		t.Fatalf("open spec = %+v, want the exe with its folder", got)
	}
	win, err := ValidateClientOpen(plugins.ClientOpenSpec{Exe: `C:\client\client.exe`}, man, folders, prefix)
	if err != nil || win.Exe != exe {
		t.Fatalf("windows client path: got %+v, %v; want %s", win, err, exe)
	}

	for name, spec := range map[string]plugins.ClientOpenSpec{
		"no exe":         {},
		"exe outside":    {Exe: outside},
		"symlink escape": {Exe: link},
		"other drive":    {Exe: `D:\client.exe`},
		"relative exe":   {Exe: "client.exe"},
		"escape drive":   {Exe: `C:\..\client.exe`},
		"bad cwd":        {Exe: exe, Cwd: `D:\client`},
		"missing cwd":    {Exe: exe, Cwd: filepath.Join(prefix, "drive_c", "gone")},
	} {
		if _, err := ValidateClientOpen(spec, man, folders, prefix); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	// A missing executable inside the client's prefix means the client isn't
	// set up yet (client.notSetUp), not a broken spec.
	_, err = ValidateClientOpen(plugins.ClientOpenSpec{
		Exe: filepath.Join(binDir, "not-installed.exe"),
	}, man, folders, prefix)
	if apperr.CodeOf(err) != "client.notSetUp" || !strings.Contains(err.Error(), "set it up first") {
		t.Fatalf("missing client exe: got %v, want the client.notSetUp prompt", err)
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
// client, even when the client is installed: the delegate is 1.20.
func TestWineClientSpecRefusedBelow120(t *testing.T) {
	t.Setenv("ECHO_PROTOCOLS", "1.0,1.18")
	e := newEchoEnv(t)
	e.login(t)
	if got := e.m.Runtime("echo").Protocol; got != "1.18" {
		t.Fatalf("negotiated %q, want 1.18", got)
	}
	prefix := t.TempDir()
	var installer plugins.ClientSetupSpec
	if err := e.m.Call(e.ctx(t), "echo", "client.setup",
		map[string]any{"jobId": "setup-1", "prefix": prefix}, &installer); err != nil {
		t.Fatalf("client.setup: %v", err)
	}
	if installer.Exe == "" || len(installer.Args) != 1 || installer.Args[0] != prefix {
		t.Fatalf("client.setup answered %+v, want the installer spec", installer)
	}
	// Core runs the installer (natively here: the fake runner passes the
	// command through); the client executable appears in the prefix.
	run := exec.Command(installer.Exe, installer.Args...)
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("installer: %v\n%s", err, out)
	}
	exe := filepath.Join(prefix, "drive_c", "echo-client", "echo-client.sh")
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("installer didn't install the fake client: %v", err)
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
