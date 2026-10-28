// TestMain builds platypus-steam-dl once for the whole package, with the
// real environment. Tests below override HOME with temp dirs (cell-id
// tests point it at a fake Steam client config); building per-test under
// those would redirect Go's module cache into read-only temp folders and
// break TempDir cleanup, so the binary is built here, before any test
// touches the environment, and setup() only copies it into place.
package steam_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// prebuiltDL is the built platypus-steam-dl binary all tests share.
var prebuiltDL string

// prebuiltPython is the interpreter behind `python3`, captured here for the
// same reason: tests point HOME at an empty dir (so the user's Steam install
// can't answer), which also breaks version-manager shims (asdf, mise) — their
// shims look for a version file under HOME or a cwd below it, and the plugin
// runs with its cwd in the temp root. The plugin's `#!/usr/bin/env python3`
// then exits 126 and it "crashes 5 times in a row" during initialize. setup()
// puts this interpreter first on PATH instead of the shim.
var prebuiltPython string

func TestMain(m *testing.M) {
	home, modCache, buildCache := realGoEnv()
	if out, err := exec.Command("python3", "-c", "import sys; print(sys.executable)").Output(); err == nil {
		prebuiltPython = strings.TrimSpace(string(out))
	}
	dir, err := os.MkdirTemp("", "platypus-steam-dl-test")
	if err != nil {
		panic(err)
	}
	out := filepath.Join(dir, "platypus-steam-dl")
	cmd := exec.Command("go", "build", "-o", out,
		"platypus/plugins/steam-standalone/cmd/platypus-steam-dl")
	cmd.Env = append(os.Environ(),
		"HOME="+home, "GOMODCACHE="+modCache, "GOCACHE="+buildCache)
	if combined, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("build platypus-steam-dl: " + string(combined))
	}
	prebuiltDL = out
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// realGoEnv captures the module/build cache locations and HOME before any
// test overrides them.
func realGoEnv() (home, modCache, buildCache string) {
	home = os.Getenv("HOME")
	modCache = os.Getenv("GOMODCACHE")
	buildCache = os.Getenv("GOCACHE")
	if modCache == "" || buildCache == "" {
		out, err := exec.Command("go", "env", "GOMODCACHE", "GOCACHE").Output()
		if err != nil {
			panic("go env: " + err.Error())
		}
		lines := splitLines(string(out))
		if len(lines) >= 2 {
			if modCache == "" {
				modCache = lines[0]
			}
			if buildCache == "" {
				buildCache = lines[1]
			}
		}
	}
	return home, modCache, buildCache
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
