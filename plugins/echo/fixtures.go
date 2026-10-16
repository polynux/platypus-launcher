package main

// Fake game behaviours for crash and exit testing (#67, part of #65):
// installed games whose launch script does one thing, so the core's session
// tracking and crash hints (internal/crashhints, #46) can be driven end to
// end without a real game.
//
//	ECHO_FIXTURES      JSON {"<id>": {"mode", "log", "code", "afterMs", "title", "wine"}}
//	                   extra installable games, read at start (default none)
//
// A fixture is a regular echo game once listed (same install, launch,
// verify and cloud paths as tiny): only its install script differs, plus the
// optional wine flag, which lists it as non-native so it launches through
// the runner (issue #69: the crash game needs a prefix for winetricks).
//   - mode "run" (default): say hello, then sleep until stopped
//     (ECHO_GAME_SECONDS, default 300 s).
//   - mode "exit0": exit 0 at once (a clean, very short run).
//   - mode "fail": sleep afterMs (if set), then exit with code
//     (default 1).
//   - mode "crash": sleep afterMs, write the Wine-style stderr named by log,
//     then exit with code (default 1). The lines mirror
//     internal/crashhints/testdata/<log>.log (read-only there):
//     vcruntime, directx, media, dotnet, vulkan, anticheat, generic
//     (default generic).
//
// Example: ECHO_FIXTURES='{"crash-vcrun":{"mode":"crash","log":"vcruntime"},
// "quick-ok":{"mode":"exit0"},"flaky-fail":{"mode":"fail","code":3}}'.
// Ids must be plain names (letters, digits, dot, dash, underscore), must not
// clash with echo's own games or the bulk- prefix. The default library is
// untouched when the knob is unset.

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// fixtureSpec is one ECHO_FIXTURES entry: how the installed game behaves.
// wine runs it through the Proton/Wine runner (needsWine) instead of native,
// so the core creates a prefix and crash-hint fixes apply (issue #69).
type fixtureSpec struct {
	Mode    string `json:"mode"`
	Log     string `json:"log"`
	Title   string `json:"title"`
	Code    int    `json:"code"`
	AfterMs int    `json:"afterMs"`
	Wine    bool   `json:"wine"`
}

// fixtureSpecs remembers every fixture's behaviour by game id, for the
// install script. Read-only after loadFixtures runs at startup.
var fixtureSpecs = map[string]fixtureSpec{}

var fixtureIDRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// fixtureLogs are the Wine-style stderr lines a "crash" game writes, by log
// name. Each mirrors internal/crashhints/testdata/<name>.log (one or two of
// its err lines), so the core's hints match what that fixture documents.
var fixtureLogs = map[string][]string{
	"vcruntime": {
		`err:module:import_dll Library vcruntime140_1.dll (which is needed by L"C:\games\Foo\Foo.exe") not found`,
		`err:module:import_dll Library MSVCP140.dll (which is needed by L"C:\games\Foo\Foo.exe") not found`,
	},
	"directx": {
		`err:module:import_dll Library d3dx9_43.dll (which is needed by L"C:\games\Bar\Bar.exe") not found`,
		`err:module:import_dll Library D3DCOMPILER_47.dll (which is needed by L"C:\games\Bar\Bar.exe") not found`,
	},
	"media": {
		`err:mfplat:MFCreateSourceReaderFromURL Failed to create source reader, hr 0xc00d36b4`,
	},
	"dotnet": {
		`err:module:import_dll Library mscoree.dll (which is needed by L"C:\games\Net\Net.exe") not found`,
	},
	"vulkan": {
		`info:dxvk: Found no compatible Vulkan device`,
	},
	"anticheat": {
		`info:EasyAntiCheat: EAC launcher started, waiting for the game`,
	},
	"generic": {
		`err:module:import_dll Library libfoo-7.dll (which is needed by L"C:\games\Mod\Mod.exe") not found`,
		`err:module:import_dll Library bar.dll not found`,
	},
}

// loadFixtures reads ECHO_FIXTURES and lists each entry as an installable
// game (like tiny: small, native, no details). Unknown modes, bad ids and
// clashes are skipped. Call once at startup, before serving.
func loadFixtures() {
	v := strings.TrimSpace(os.Getenv("ECHO_FIXTURES"))
	if v == "" {
		return
	}
	var raw map[string]fixtureSpec
	if json.Unmarshal([]byte(v), &raw) != nil {
		return
	}
	for id, spec := range raw {
		if !fixtureIDRe.MatchString(id) {
			continue
		}
		if _, taken := games[id]; taken {
			continue
		}
		if strings.HasPrefix(id, "bulk-") {
			continue
		}
		switch spec.Mode {
		case "", "run", "exit0", "fail", "crash":
		default:
			continue
		}
		if spec.Mode == "" {
			spec.Mode = "run"
		}
		if spec.Log == "" {
			spec.Log = "generic"
		}
		if _, ok := fixtureLogs[spec.Log]; !ok {
			continue
		}
		title := strings.TrimSpace(spec.Title)
		if title == "" {
			title = "Echo " + id
		}
		games[id] = game{title: title, size: 3 << 20, steps: 10, stepMs: 100, native: !spec.Wine}
		order = append(order, id)
		fixtureSpecs[id] = spec
	}
}

// fixtureScript is the game.sh a fixture install writes: the behaviour for
// id with the installed version's build in its hello line. Empty for echo's
// own games (the caller keeps the default script then).
func fixtureScript(id, build string) string {
	spec, ok := fixtureSpecs[id]
	if !ok {
		return ""
	}
	hello := fmt.Sprintf("#!/bin/sh\necho \"echo game %s $*\"\n", build)
	sleep := ""
	if spec.AfterMs > 0 {
		sleep = fmt.Sprintf("sleep %.3f\n", float64(spec.AfterMs)/1000)
	}
	code := spec.Code
	if code == 0 {
		code = 1 // 0 would be a clean run, not a failure: fail/crash exit non-zero
	}
	switch spec.Mode {
	case "exit0":
		return "#!/bin/sh\nexit 0\n"
	case "fail":
		return hello + sleep + fmt.Sprintf("exit %d\n", code)
	case "crash":
		var b strings.Builder
		b.WriteString(hello)
		b.WriteString(sleep)
		for _, line := range fixtureLogs[spec.Log] {
			fmt.Fprintf(&b, "echo '%s' >&2\n", strings.ReplaceAll(line, `'`, `'\"'\"'`))
		}
		fmt.Fprintf(&b, "exit %d\n", code)
		return b.String()
	default: // "run": hello, then sleep until the user stops it
		return hello + "sleep \"${ECHO_GAME_SECONDS:-300}\"\n"
	}
}
