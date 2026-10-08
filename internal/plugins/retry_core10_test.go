package plugins

import (
	"strings"
	"testing"
	"time"
)

// crashUntilGiveUp crashes the plugin maxCrashes times and waits for the
// crash accounting, which is recorded asynchronously after each call
// returns, to settle.
func crashUntilGiveUp(t *testing.T, e *env) {
	t.Helper()
	for range e.m.t.maxCrashes {
		_ = e.m.Call(ctx(t), "echo", "crash", nil, nil)
	}
	proc, err := e.m.process("echo")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		proc.mu.Lock()
		n := proc.crashes
		proc.mu.Unlock()
		if n >= e.m.t.maxCrashes {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("crash count stuck at %d", n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// CORE-10: a plugin that gave up after repeated crashes must be recoverable
// without a restart: re-approval, an explicit retry, or time passing must
// let the next call restart it.
func TestRecoverAfterGiveUp(t *testing.T) {
	t.Run("approve", func(t *testing.T) {
		e := newEnv(t)
		e.ready(t)
		crashUntilGiveUp(t, e)
		if err := e.m.Call(ctx(t), "echo", "echo", nil, nil); err == nil || !strings.Contains(err.Error(), "crashed 3 times") {
			t.Fatalf("want give-up error, got %v", err)
		}
		if err := e.m.Approve("echo"); err != nil {
			t.Fatalf("re-approve: %v", err)
		}
		var pid int
		if err := e.m.Call(ctx(t), "echo", "pid", nil, &pid); err != nil {
			t.Fatalf("call after re-approve: %v", err)
		}
		if pid == 0 {
			t.Fatal("plugin did not restart after re-approval")
		}
	})

	t.Run("retry", func(t *testing.T) {
		e := newEnv(t)
		e.ready(t)
		crashUntilGiveUp(t, e)
		if err := e.m.RetryPlugin("echo"); err != nil {
			t.Fatalf("retry: %v", err)
		}
		var pid int
		if err := e.m.Call(ctx(t), "echo", "pid", nil, &pid); err != nil {
			t.Fatalf("call after retry: %v", err)
		}
		if pid == 0 {
			t.Fatal("plugin did not restart after retry")
		}
	})

	t.Run("decay", func(t *testing.T) {
		e := newEnv(t)
		e.ready(t)
		crashUntilGiveUp(t, e)
		proc, err := e.m.process("echo")
		if err != nil {
			t.Fatal(err)
		}
		proc.mu.Lock()
		proc.lastCrash = time.Now().Add(-2 * e.m.t.stableAfter)
		proc.mu.Unlock()
		var pid int
		if err := e.m.Call(ctx(t), "echo", "pid", nil, &pid); err != nil {
			t.Fatalf("call after crash count decayed: %v", err)
		}
		if pid == 0 {
			t.Fatal("plugin did not restart after decay")
		}
	})
}
