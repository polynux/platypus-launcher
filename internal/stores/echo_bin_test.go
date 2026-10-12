package stores

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// Shared echo plugin binary (issue #40): newEchoEnv used to run
// `go build platypus/plugins/echo` per test (~0.5 s each even warm). The
// build runs once per `go test` run, with the environment as found at
// package load, before any test can override HOME (an overridden HOME
// would give the go command a cold GOCACHE and cost ~8 s).
var (
	storesRealHome     = os.Getenv("HOME")
	storesRealGoCache  = os.Getenv("GOCACHE")
	storesRealModCache = os.Getenv("GOMODCACHE")

	echoBinOnce sync.Once
	echoBinPath string
	echoBinErr  error
	echoBinOut  string
)

func sharedEchoBinary(t *testing.T) string {
	t.Helper()
	echoBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "platypus-echo-")
		if err != nil {
			echoBinErr = err
			return
		}
		echoBinPath = filepath.Join(dir, "platypus-echo")
		cmd := exec.Command("go", "build", "-o", echoBinPath, "platypus/plugins/echo")
		env := os.Environ()
		for i, kv := range env {
			switch {
			case len(kv) > 5 && kv[:5] == "HOME=":
				env[i] = "HOME=" + storesRealHome
			case len(kv) > 8 && kv[:8] == "GOCACHE=":
				env[i] = "GOCACHE=" + storesRealGoCache
			case len(kv) > 11 && kv[:11] == "GOMODCACHE=":
				env[i] = "GOMODCACHE=" + storesRealModCache
			}
		}
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			echoBinErr = err
			echoBinOut = string(out)
			echoBinPath = ""
			os.RemoveAll(dir)
		}
	})
	if echoBinErr != nil {
		t.Fatalf("building echo plugin: %v: %s", echoBinErr, echoBinOut)
	}
	return echoBinPath
}
