package plugins

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A plugin that answers initialize, acts on the next request (side-effect
// file), then dies without responding.
const dieAfterWorkScript = `#!/bin/sh
SIDEFILE="__SIDE__"
read -r line
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"name":"Fake","version":"0.0.1","api":1,"capabilities":[],"missing":[]}}'
read -r line
printf '%s\n' "$line" >> "$SIDEFILE"
exit 1
`

func startDieAfterWork(t *testing.T, side string) *process {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-plugin.sh")
	if err := os.WriteFile(script, []byte(strings.ReplaceAll(dieAfterWorkScript, "__SIDE__", side)), 0o755); err != nil {
		t.Fatal(err)
	}
	p := &process{
		id:      "fake",
		dir:     dir,
		exec:    script,
		verify:  func() error { return nil },
		dataDir: filepath.Join(dir, "data"),
		t: timings{
			backoffBase: 20 * time.Millisecond,
			backoffMax:  100 * time.Millisecond,
			stableAfter: time.Minute,
			maxCrashes:  10,
			stopGrace:   time.Second,
			initTimeout: 5 * time.Second,
		},
	}
	t.Cleanup(p.stop)
	return p
}

// Plugin methods are not idempotent, so a request the dead process may
// already have acted on must never be replayed: dying after reading the
// request yields ErrExited with the method executed exactly once.
func TestCallDoesNotReplayAfterDeath(t *testing.T) {
	side := filepath.Join(t.TempDir(), "side.log")
	p := startDieAfterWork(t, side)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err := p.call(ctx, "work", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "exited during work") {
		t.Fatalf("want ErrExited during work, got %v", err)
	}
	raw, readErr := os.ReadFile(side)
	if readErr != nil {
		t.Fatalf("side-effect file missing: %v", readErr)
	}
	if n := strings.Count(string(raw), "\n"); n != 1 {
		t.Fatalf("method executed %d times, want exactly once: %q", n, raw)
	}
}

// A call that lands in the post-crash drain window still recovers: ensure
// waits for cleanup and restarts instead of reporting the stale conn.
func TestCallRecoversAfterCrash(t *testing.T) {
	e := newEnv(t)
	e.ready(t)
	var pid1 int
	if err := e.m.Call(ctx(t), "echo", "pid", nil, &pid1); err != nil {
		t.Fatal(err)
	}
	if err := e.m.Call(ctx(t), "echo", "crash", nil, nil); err == nil {
		t.Fatal("crash returned no error")
	}
	var pid2 int
	if err := e.m.Call(ctx(t), "echo", "pid", nil, &pid2); err != nil {
		t.Fatalf("call right after crash: %v", err)
	}
	if pid2 == 0 || pid2 == pid1 {
		t.Fatalf("not restarted: %d -> %d", pid1, pid2)
	}
}
