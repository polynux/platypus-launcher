package plugins

import (
	"context"
	"os"
	"testing"
	"time"

	"hydra-launcher/internal/jsonrpc"
)

// CORE-2: ensure() must not hand out a dead-but-not-yet-cleaned-up
// connection. Simulates the drain window: the process already exited
// (dead closed) but the jsonrpc read loop hasn't observed EOF yet
// (conn.Done() open) and cleanup hasn't finished (exited open).
func TestEnsureDoesNotHandOutDeadConn(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	conn := jsonrpc.NewConn(r, w, nil)
	defer conn.Close()

	ready := make(chan struct{})
	close(ready)
	exited := make(chan struct{}) // open: reaper still draining
	dead := make(chan struct{})
	close(dead) // cmd.Wait() already returned

	p := &process{id: "test-dead", conn: conn, ready: ready, exited: exited, dead: dead, t: defaultTimings}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	got, err := p.ensure(ctx)
	if err == nil || got != nil {
		t.Fatalf("ensure returned stale conn (got=%v, err=%v); want blocked until exited/ctx", got, err)
	}
	if ctx.Err() == nil {
		t.Fatalf("ensure returned %v before ctx done", err)
	}
}
