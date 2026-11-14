package plugins

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"platypus/internal/jsonrpc"
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

// unsentConn simulates the stale-handoff race deterministically: Done stays
// open (the read loop hasn't observed the death yet) while Call reports the
// request never reached the peer.
type unsentConn struct {
	mu    sync.Mutex
	done  chan struct{}
	calls int
	err   error
}

func (c *unsentConn) Call(context.Context, string, any, any) error {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	return c.err
}

func (c *unsentConn) Notify(string, any) error { return nil }
func (c *unsentConn) Done() <-chan struct{}    { return c.done }
func (c *unsentConn) Close()                   {}

func (c *unsentConn) numCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// A call issued right after the plugin exits restarts the plugin instead of
// failing on the dead connection — even when the death hasn't been observed
// yet (Done still open because the read loop wasn't scheduled). Before the
// fix this failed deterministically with "exited during pid".
func TestCallRetriesWhenHandoffRacesDeath(t *testing.T) {
	dir := t.TempDir()
	// Answers initialize, then one request with 4242, then waits for EOF.
	script := "#!/bin/sh\n" +
		"read -r line\n" +
		"id=$(printf '%s' \"$line\" | sed -n 's/.*\"id\"[ ]*:[ ]*\\([0-9][0-9]*\\).*/\\1/p')\n" +
		"printf '%s\\n' \"{\\\"jsonrpc\\\":\\\"2.0\\\",\\\"id\\\":$id,\\\"result\\\":{\\\"name\\\":\\\"Fake\\\",\\\"version\\\":\\\"0.0.1\\\",\\\"api\\\":1,\\\"capabilities\\\":[],\\\"missing\\\":[]}}\"\n" +
		"read -r line\n" +
		"id=$(printf '%s' \"$line\" | sed -n 's/.*\"id\"[ ]*:[ ]*\\([0-9][0-9]*\\).*/\\1/p')\n" +
		"printf '%s\\n' \"{\\\"jsonrpc\\\":\\\"2.0\\\",\\\"id\\\":$id,\\\"result\\\":4242}\"\n" +
		"while read -r line; do :; done\n"
	scriptPath := filepath.Join(dir, "fake-plugin.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	close(ready)
	dead := make(chan struct{})   // open: cmd.Wait hasn't returned yet
	exited := make(chan struct{}) // open: the reaper hasn't finished
	stub := &unsentConn{
		done: make(chan struct{}), // open: the read loop hasn't seen EOF
		err:  fmt.Errorf("%w: %w", jsonrpc.ErrClosed, jsonrpc.ErrNotSent),
	}
	p := &process{
		id:        "fake",
		dir:       dir,
		exec:      scriptPath,
		verify:    func() error { return nil },
		dataDir:   filepath.Join(dir, "data"),
		protocols: []Version{{Major: 1, Minor: 0}},
		t: timings{
			backoffBase: 20 * time.Millisecond,
			backoffMax:  100 * time.Millisecond,
			stableAfter: time.Minute,
			maxCrashes:  10,
			stopGrace:   time.Second,
			initTimeout: 5 * time.Second,
		},
		conn:   stub,
		ready:  ready,
		exited: exited,
		dead:   dead,
	}
	t.Cleanup(p.stop)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	var got int
	go func() {
		var v int
		err := p.call(ctx, "pid", nil, &v)
		got = v
		errCh <- err
	}()
	// Wait until the doomed send happened, then play the reaper: record
	// the crash, clear the run, and close dead and exited.
	deadline := time.Now().Add(5 * time.Second)
	for stub.numCalls() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("stale connection never used")
		}
		time.Sleep(time.Millisecond)
	}
	close(dead)
	p.mu.Lock()
	p.recordCrash(errors.New("exited"))
	p.cmd, p.conn, p.stdin, p.dead, p.info = nil, nil, nil, nil, nil
	p.mu.Unlock()
	close(exited)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("call right after death: %v", err)
		}
	case <-time.After(14 * time.Second):
		t.Fatal("call never returned")
	}
	if got != 4242 {
		t.Fatalf("got %d, want 4242", got)
	}
	if n := stub.numCalls(); n != 1 {
		t.Fatalf("stale connection used %d times, want 1", n)
	}
}
