// TestMain builds hydra-steam-dl once for the whole conformance package,
// with the real environment. The steam suite reaches this binary through
// PATH, and per-test builds under an overridden HOME would redirect Go's
// module cache into read-only temp folders, so it is built here, before
// any test touches the environment.
package plugtest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// prebuiltDLDir holds a hydra-steam-dl binary all tests share via PATH.
var prebuiltDLDir string

func TestMain(m *testing.M) {
	home := os.Getenv("HOME")
	modCache := goEnvValue("GOMODCACHE")
	buildCache := goEnvValue("GOCACHE")
	dir, err := os.MkdirTemp("", "hydra-steam-dl-conformance")
	if err != nil {
		panic(err)
	}
	cmd := exec.Command("go", "build", "-o", filepath.Join(dir, "hydra-steam-dl"),
		"hydra-launcher/plugins/steam-standalone/cmd/hydra-steam-dl")
	cmd.Env = append(os.Environ(),
		"HOME="+home, "GOMODCACHE="+modCache, "GOCACHE="+buildCache)
	if combined, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic("build hydra-steam-dl: " + string(combined))
	}
	prebuiltDLDir = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func goEnvValue(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		panic("go env " + key + ": " + err.Error())
	}
	for len(out) > 0 && (out[len(out)-1] == '\n' || out[len(out)-1] == '\r') {
		out = out[:len(out)-1]
	}
	return string(out)
}
