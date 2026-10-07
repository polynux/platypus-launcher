// TestMain builds hydra-steam-dl once for the whole package, with the
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
	"testing"
)

// prebuiltDL is the built hydra-steam-dl binary all tests share.
var prebuiltDL string

func TestMain(m *testing.M) {
	home, modCache, buildCache := realGoEnv()
	dir, err := os.MkdirTemp("", "hydra-steam-dl-test")
	if err != nil {
		panic(err)
	}
	out := filepath.Join(dir, "hydra-steam-dl")
	cmd := exec.Command("go", "build", "-o", out,
		"hydra-launcher/plugins/steam-standalone/cmd/hydra-steam-dl")
	cmd.Env = append(os.Environ(),
		"HOME="+home, "GOMODCACHE="+modCache, "GOCACHE="+buildCache)
	if combined, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("build hydra-steam-dl: " + string(combined))
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
