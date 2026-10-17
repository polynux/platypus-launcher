package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"platypus/internal/apperr"
	"platypus/internal/dataplug"
	"platypus/internal/plugins"
	"platypus/internal/settings"
	"platypus/internal/stores"
	"platypus/internal/types"
)

func nativeTestApp(t *testing.T, home string) *App {
	t.Helper()
	root := t.TempDir()
	st, err := settings.OpenWithHome(filepath.Join(root, "settings.json"), home)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{settings: st, stores: stores.NewRegistry(nil), optHome: home}
	withTestRunnerPlugin(t, a, home)
	return a
}

// withTestRunnerPlugin installs the bundled runner plugin on a, so detection
// and launch wrapping run as they do in the app. The plugin reads HOME from its
// environment, so it gets the test's home.
func withTestRunnerPlugin(t *testing.T, a *App, home string) {
	t.Helper()
	root := t.TempDir()
	if a.data == nil {
		a.data = dataplug.NewRegistry()
	}
	a.plugins = plugins.NewManager(plugins.Options{
		Dir: filepath.Join(root, "plugins"), LogDir: filepath.Join(root, "logs"), DataDir: filepath.Join(root, "data"),
		Settings: a.settings, Home: home, Env: map[string]string{"HOME": home}, OnNotify: a.pluginNotify,
	})
	t.Cleanup(a.plugins.Shutdown)
	installTestRunnerPlugin(t, a)
}

func writeExe(t *testing.T, path string, data []byte, exec bool) string {
	t.Helper()
	perm := os.FileMode(0o644)
	if exec {
		perm = 0o755
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatal(err)
	}
	return path
}

var elfBytes = append([]byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0}, []byte("....")...)

// Issue #47, step 1: what works today — custom games now carry Native from
// content detection (ELF/shebang/AppImage), with the run_mode override.
func TestCustomGamesDetectNative(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := nativeTestApp(t, home)
	elf := writeExe(t, filepath.Join(home, "elf-game"), elfBytes, true)
	mz := writeExe(t, filepath.Join(home, "win-game.exe"), []byte("MZ\x90\x00"), true)
	sh := writeExe(t, filepath.Join(home, "start.sh"), []byte("#!/bin/sh\necho hi\n"), true)

	for _, tc := range []struct {
		title string
		exe   string
		nat   bool
	}{
		{"elf", elf, true},
		{"windows", mz, false},
		{"script", sh, true},
	} {
		if _, err := a.settings.AddCustomGame(tc.title, tc.exe); err != nil {
			t.Fatal(err)
		}
	}
	games := a.customGames()
	byTitle := map[string]types.Game{}
	for _, g := range games {
		byTitle[g.Title] = g
	}
	for _, tc := range []struct {
		title string
		nat   bool
	}{
		{"elf", true}, {"windows", false}, {"script", true},
	} {
		if byTitle[tc.title].Native != tc.nat {
			t.Errorf("%s: Native = %v, want %v", tc.title, byTitle[tc.title].Native, tc.nat)
		}
	}
}

// A native launch runs directly: no WINEPREFIX, no runner (even with none
// installed), wrappers kept, working dir defaulting to the binary's folder.
func TestCustomLaunchNativeNoPrefix(t *testing.T) {
	t.Parallel()
	home := t.TempDir() // no runners installed here
	a := nativeTestApp(t, home)
	exe := writeExe(t, filepath.Join(home, "game", "game"), elfBytes, true)
	cg, err := a.settings.AddCustomGame("Nat", exe)
	if err != nil {
		t.Fatal(err)
	}
	_ = cg
	g := a.customGames()[0]
	if !g.Native {
		t.Fatal("game should detect as native")
	}
	p, err := a.prepareLaunch(g, true)
	if err != nil {
		t.Fatalf("prepareLaunch native: %v", err)
	}
	if _, ok := p.env["WINEPREFIX"]; ok {
		t.Fatalf("native launch sets WINEPREFIX: %v", p.env)
	}
	if p.runner.Path != "" {
		t.Fatalf("native launch picked a runner: %q", p.runner.Path)
	}
	if p.dir != filepath.Dir(exe) {
		t.Fatalf("launch dir = %q, want the binary's folder %q", p.dir, filepath.Dir(exe))
	}
	found := false
	for _, arg := range p.args {
		if arg == exe {
			found = true
		}
	}
	if !found {
		t.Fatalf("args miss the exe: %q", p.args)
	}
}

// A Windows sideload still needs a runner. Without one the launch fails with
// the runner's message before any process starts; a host with one (system
// wine at /usr/bin/wine is found) launches through it.
func TestCustomLaunchWindowsNeedsRunner(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := nativeTestApp(t, home)
	exe := writeExe(t, filepath.Join(home, "game.exe"), []byte("MZ\x90\x00"), true)
	if _, err := a.settings.AddCustomGame("Win", exe); err != nil {
		t.Fatal(err)
	}
	g := a.customGames()[0]
	if g.Native {
		t.Fatal("MZ must detect as Windows")
	}
	p, err := a.prepareLaunch(g, true)
	if err != nil {
		var ae *apperr.Error
		if ok := errors.As(err, &ae); !ok || ae.Code != "runner.wrapFailed" || !strings.Contains(err.Error(), "no Proton or Wine runner is installed") {
			t.Fatalf("err = %v, want runner.wrapFailed when no runner is installed", err)
		}
		return
	}
	// A host with a runner installed launches through it: the Wine prefix is set.
	if p.env["WINEPREFIX"] == "" {
		t.Fatalf("windows launch sets no prefix: %v", p.env)
	}
	if p.runner.Path == "" {
		t.Fatalf("windows launch picked no runner")
	}
}

// .sh without the bit runs through /bin/sh; an ELF without it is a clear
// error offering the chmod fix, not a raw exec failure.
func TestCustomLaunchExecBit(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := nativeTestApp(t, home)
	sh := writeExe(t, filepath.Join(home, "start.sh"), []byte("#!/bin/sh\necho hi\n"), false)
	if _, err := a.settings.AddCustomGame("Sh", sh); err != nil {
		t.Fatal(err)
	}
	g := a.customGames()[0]
	p, err := a.prepareLaunch(g, true)
	if err != nil {
		t.Fatalf("sh fallback: %v", err)
	}
	// Wrappers (gamemoderun, …) may prefix /bin/sh: the pair must appear in order.
	at := -1
	for i := 0; i+1 < len(p.args); i++ {
		if p.args[i] == "/bin/sh" && p.args[i+1] == sh {
			at = i
		}
	}
	if at < 0 {
		t.Fatalf("sh fallback args = %q, want /bin/sh %s in order", p.args, sh)
	}

	a2 := nativeTestApp(t, home)
	elf := writeExe(t, filepath.Join(home, "game2", "game"), elfBytes, false)
	if _, err := a2.settings.AddCustomGame("Elf", elf); err != nil {
		t.Fatal(err)
	}
	g2 := a2.customGames()[0]
	if _, err := a2.prepareLaunch(g2, true); err == nil {
		t.Fatal("non-executable ELF should fail clearly")
	} else {
		var ae *apperr.Error
		if ok := errors.As(err, &ae); !ok || ae.Code != "sideload.notExecutable" {
			t.Fatalf("err = %v, want sideload.notExecutable", err)
		}
	}
}

// The chmod helper only touches the game's own file.
func TestMakeGameExecutable(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := nativeTestApp(t, home)
	exe := writeExe(t, filepath.Join(home, "game", "game"), elfBytes, false)
	sib := writeExe(t, filepath.Join(home, "game", "data"), []byte("x"), false)
	cg, err := a.settings.AddCustomGame("Nat", exe)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := a.IsGameExecutable(cg.ID); err != nil || ok {
		t.Fatalf("IsGameExecutable = %v, %v; want false", ok, err)
	}
	if _, err := a.MakeGameExecutable(cg.ID); err != nil {
		t.Fatalf("MakeGameExecutable: %v", err)
	}
	if ok, _ := a.IsGameExecutable(cg.ID); !ok {
		t.Fatal("still not executable afterwards")
	}
	if st, _ := os.Stat(sib); st.Mode().Perm()&0o111 != 0 {
		t.Fatal("the sibling must stay non-executable")
	}
	if _, err := a.DetectGameExecutable(exe); err != nil {
		t.Fatal(err)
	}
}

// Changing the executable re-detects (issue #47: decide on add, and again
// when the executable changes).
func TestUpdateCustomGameExecutableRedetects(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := nativeTestApp(t, home)
	mz := writeExe(t, filepath.Join(home, "game.exe"), []byte("MZ\x90\x00"), true)
	cg, err := a.settings.AddCustomGame("Game", mz)
	if err != nil {
		t.Fatal(err)
	}
	if got := a.customGames()[0].Native; got {
		t.Fatal("MZ should start as Windows")
	}
	elf := writeExe(t, filepath.Join(home, "game2"), elfBytes, true)
	updated, err := a.UpdateCustomGameExecutable(cg.ID, elf)
	if err != nil {
		t.Fatalf("UpdateCustomGameExecutable: %v", err)
	}
	if !updated.Native {
		t.Fatal("ELF should re-detect as native")
	}
	if updated.Executable != elf {
		t.Fatalf("executable = %q, want %q", updated.Executable, elf)
	}
}

// Removal never offers a prefix target for a native game (#43 + #47).
func TestRemoveTargetsNativeNoPrefix(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := nativeTestApp(t, home)
	exe := writeExe(t, filepath.Join(home, "game", "game"), elfBytes, true)
	if _, err := a.settings.AddCustomGame("Nat", exe); err != nil {
		t.Fatal(err)
	}
	g := a.customGames()[0]
	if !g.Native {
		t.Fatal("want a native game")
	}
	targets, err := a.removeTargets(g, types.RemoveOptions{DeletePrefix: true, DeleteFiles: true})
	if err != nil {
		t.Fatalf("removeTargets: %v", err)
	}
	for _, tg := range targets {
		if tg.kind == removeKindPrefix {
			t.Fatalf("native removal offers a prefix target: %+v", targets)
		}
	}
	if len(targets) != 1 || targets[0].kind != removeKindFiles {
		t.Fatalf("targets = %+v, want files only", targets)
	}
}

// The per-game override wins over detection, and rejects junk.
func TestSaveGameSettingsRunMode(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := nativeTestApp(t, home)
	exe := writeExe(t, filepath.Join(home, "game"), elfBytes, true)
	cg, err := a.settings.AddCustomGame("Nat", exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SaveGameSettings(cg.ID, map[string]any{"run_mode": "bogus"}); err == nil {
		t.Fatal("bogus run_mode was kept")
	}
	if err := a.SaveGameSettings(cg.ID, map[string]any{"run_mode": "wine"}); err != nil {
		t.Fatalf("run_mode wine: %v", err)
	}
	if got := a.customGames()[0].Native; got {
		t.Fatal("wine override must force Windows")
	}
	if err := a.SaveGameSettings(cg.ID, map[string]any{"run_mode": "native"}); err != nil {
		t.Fatalf("run_mode native: %v", err)
	}
	mz := writeExe(t, filepath.Join(home, "game.exe"), []byte("MZ\x90\x00"), true)
	cg2, err := a.settings.AddCustomGame("Win", mz)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SaveGameSettings(cg2.ID, map[string]any{"run_mode": "native"}); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range a.customGames() {
		if g.ID == cg2.ID && g.Native {
			found = true
		}
	}
	if !found {
		t.Fatal("native override must force a Windows exe native")
	}
}

// The picker accepts native and Windows types.
func TestPickGameExecutableFiltersNative(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	exe := writeExe(t, filepath.Join(home, "game"), elfBytes, true)
	h := &fakePickerHost{fileRet: exe}
	a := &App{host: h, settings: nil}
	if _, err := a.PickGameExecutable(exe); err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, f := range h.file.Filters {
		joined += f.Name + ":" + strings.Join(f.Extensions, ",") + " "
	}
	for _, want := range []string{"sh", "AppImage", "x86_64", "msi", "bat", "lnk"} {
		if !strings.Contains(joined, want) {
			t.Errorf("filters %q miss %q", joined, want)
		}
	}
}

func TestDetectGameExecutable(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	a := nativeTestApp(t, home)
	exe := writeExe(t, filepath.Join(home, "game"), elfBytes, true)
	res, err := a.DetectGameExecutable(exe)
	if err != nil || !res.Native || res.Kind != "elf" {
		t.Fatalf("DetectGameExecutable = %+v, %v", res, err)
	}
	if _, err := a.DetectGameExecutable(filepath.Join(home, "missing")); err == nil {
		t.Fatal("unreadable path should error")
	}
}
