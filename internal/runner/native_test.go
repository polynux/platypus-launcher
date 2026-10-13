package runner

import (
	"os"
	"path/filepath"
	"testing"

	"platypus/internal/types"
)

// Issue #47: detection is by content, not extension.
func TestDetectExecutable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	elf := append([]byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0}, []byte("....")...)
	appImg := append([]byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0, 'A', 'I', 0x02}, []byte("rest")...)

	cases := []struct {
		name   string
		path   string
		native bool
		kind   ExecKind
	}{
		{"elf no extension", write("game", elf), true, ExecELF},
		{"elf .x86_64", write("game.x86_64", elf), true, ExecELF},
		{"shebang", write("start.sh", []byte("#!/bin/sh\necho hi\n")), true, ExecScript},
		{"sh fallback without shebang", write("plain.sh", []byte("echo hi\n")), true, ExecScript},
		{"appimage magic", write("game.bin", appImg), true, ExecAppImage},
		{"appimage by name", write("game.AppImage", []byte("not an elf")), true, ExecAppImage},
		{"mz exe", write("game.exe", []byte("MZ\x90\x00garbage")), false, ExecWindowsExe},
		{"msi by name", write("setup.msi", []byte("not ole")), false, ExecWindowsInstaller},
		{"bat by name", write("run.bat", []byte("@echo off\n")), false, ExecWindowsScript},
		{"cmd by name", write("run.cmd", []byte("@echo off\n")), false, ExecWindowsScript},
		{"lnk by name", write("run.lnk", []byte("L\x00\x00\x00")), false, ExecWindowsShortcut},
		{"empty file", write("empty", nil), false, ExecUnknown},
		{"bin fallback", write("data.bin", []byte("hello")), true, ExecELF},
	}
	for _, tc := range cases {
		got, err := DetectExecutable(tc.path)
		if err != nil {
			t.Errorf("%s: DetectExecutable: %v", tc.name, err)
			continue
		}
		if got.Native != tc.native || got.Kind != tc.kind {
			t.Errorf("%s: got %+v, want native=%v kind=%s", tc.name, got, tc.native, tc.kind)
		}
	}

	// Unreadable: a directory reads as one, deterministically even as root.
	if _, err := DetectExecutable(dir); err == nil {
		t.Error("DetectExecutable(dir): want an error for the unreadable case")
	}
	// Missing file is unreadable too.
	if _, err := DetectExecutable(filepath.Join(dir, "missing")); err == nil {
		t.Error("DetectExecutable(missing): want an error")
	}
}

func TestResolveNative(t *testing.T) {
	t.Parallel()
	if !ResolveNative(true, "") || !ResolveNative(true, "auto") {
		t.Error("auto must keep the detected value")
	}
	if ResolveNative(false, "") || ResolveNative(false, "auto") {
		t.Error("auto must keep a windows detection")
	}
	if !ResolveNative(false, "native") {
		t.Error("native override must win")
	}
	if ResolveNative(true, "wine") {
		t.Error("wine override must win")
	}
	if !ValidRunMode("") || !ValidRunMode("auto") || !ValidRunMode("native") || !ValidRunMode("wine") {
		t.Error("valid run modes refused")
	}
	if ValidRunMode("proton") {
		t.Error("invalid run mode kept")
	}
}

// Issue #47: the native command has no WINEPREFIX, no runner, and keeps the
// wrappers (GameMode/MangoHud/Gamescope still apply).
func TestNativeCommandHasNoPrefixOrRunner(t *testing.T) {
	home := t.TempDir()
	// Fake wrappers on PATH so the pipeline wraps the native target too.
	bin := t.TempDir()
	for _, name := range []string{"gamemoderun", "mangohud", "gamescope"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	game := types.Game{ID: "nat", Title: "Nat", Settings: types.GameSettings{UseGameMode: true, UseMangoHud: true, UseGamescope: true}}
	target := Target{Exe: "/games/nat/game", Native: true}
	args, env, err := BuildCommandFor(game, types.ProtonVersion{Path: "/runners/proton"}, target, false, "", home)
	if err != nil {
		t.Fatalf("BuildCommandFor: %v", err)
	}
	for _, a := range args {
		if a == "/runners/proton" {
			t.Fatalf("native command runs the runner: %q", args)
		}
	}
	if _, ok := env["WINEPREFIX"]; ok {
		t.Fatalf("native command sets WINEPREFIX: %v", env)
	}
	if _, ok := env["STEAM_COMPAT_DATA_PATH"]; ok {
		t.Fatalf("native command sets STEAM_COMPAT_DATA_PATH: %v", env)
	}
	if _, err := os.Stat(filepath.Join(home, ".local/share/platypus/prefixes", "nat")); !os.IsNotExist(err) {
		t.Fatalf("native launch created a prefix: %v", err)
	}
	found := false
	for _, a := range args {
		if a == "/games/nat/game" {
			found = true
		}
	}
	if !found {
		t.Fatalf("native command misses the exe: %q", args)
	}
	for _, w := range []string{"gamemoderun", "mangohud", "gamescope"} {
		hit := false
		for _, a := range args {
			if a == w {
				hit = true
			}
		}
		if !hit {
			t.Fatalf("native command dropped the %s wrapper: %q", w, args)
		}
	}
}

// The chmod helper only touches that one file.
func TestMakeExecutableOnlyTouchesThatFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	exe := filepath.Join(dir, "game")
	sib := filepath.Join(dir, "sibling")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sib, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, err := IsExecutable(exe); err != nil || ok {
		t.Fatalf("IsExecutable = %v, %v; want false", ok, err)
	}
	if err := MakeExecutable(exe); err != nil {
		t.Fatalf("MakeExecutable: %v", err)
	}
	if ok, _ := IsExecutable(exe); !ok {
		t.Fatal("still not executable after MakeExecutable")
	}
	if st, _ := os.Stat(sib); st.Mode().Perm() != 0o644 {
		t.Fatalf("sibling mode = %o, want 644", st.Mode().Perm())
	}
	// Second call is a no-op.
	if err := MakeExecutable(exe); err != nil {
		t.Fatalf("second MakeExecutable: %v", err)
	}
}

// .sh without the bit runs through /bin/sh; anything else needs it.
func TestNativeTargetShFallback(t *testing.T) {
	t.Parallel()
	sh := NativeTarget("/g/start.sh", ExecScript, false)
	if sh.Exe != "/bin/sh" || len(sh.Args) != 1 || sh.Args[0] != "/g/start.sh" || !sh.Native {
		t.Fatalf("sh fallback = %+v", sh)
	}
	direct := NativeTarget("/g/start.sh", ExecScript, true)
	if direct.Exe != "/g/start.sh" || len(direct.Args) != 0 {
		t.Fatalf("executable sh must run directly: %+v", direct)
	}
}
