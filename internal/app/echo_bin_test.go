package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// Shared echo plugin binary (issue #40).
//
// Every App fixture used to run `go build platypus/plugins/echo` per test,
// after overriding HOME with a TempDir. The overridden HOME gives the go
// command a cold GOCACHE, so each build recompiled echo and its dependencies
// (~8 s; the same build is ~0.4 s warm). The binary is now built once per
// `go test` run and copied per test; the build itself runs with the real
// HOME/GOCACHE/GOMODCACHE captured below at package load, before any test
// can override them (the same reason internal/plugtest prebuilds in
// TestMain). Only the build sees the real environment: the plugin process
// still inherits the test's, so isolation is unchanged.

var (
	realHome     = os.Getenv("HOME")
	realGoCache  = os.Getenv("GOCACHE")
	realModCache = os.Getenv("GOMODCACHE")

	echoBinOnce sync.Once
	echoBinPath string
	echoBinErr  error
	echoBinOut  string
)

// sharedEchoBinary builds plugins/echo once per test run and returns the
// binary's path. Per-test installs copy it (see installEchoApp).
func sharedEchoBinary(t *testing.T) string {
	t.Helper()
	echoBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "platypus-echo-app-")
		if err != nil {
			echoBinErr = err
			return
		}
		echoBinPath = filepath.Join(dir, "platypus-echo-app")
		cmd := exec.Command("go", "build", "-o", echoBinPath, "platypus/plugins/echo")
		env := os.Environ()
		for i, kv := range env {
			switch {
			case len(kv) > 5 && kv[:5] == "HOME=":
				env[i] = "HOME=" + realHome
			case len(kv) > 8 && kv[:8] == "GOCACHE=":
				env[i] = "GOCACHE=" + realGoCache
			case len(kv) > 11 && kv[:11] == "GOMODCACHE=":
				env[i] = "GOMODCACHE=" + realModCache
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
