package plugtest

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"platypus/internal/jsonrpc"
	"platypus/internal/types"
)

// checkVerify holds a plugin offering `verify` to docs/plugins/protocol.md
// ("Verify and move"). A check of the installed game reports a verify object
// and changes nothing (repair is false), params are checked before any work,
// and an unknown game is an RPC error.
func (s *Session) checkVerify(t *testing.T, game types.Game) {
	t.Helper()
	s.wantCode("game.verify", map[string]any{}, jsonrpc.CodeInvalidParams)
	var junk json.RawMessage
	if err := s.call("game.verify", map[string]any{"id": "no-such-game-0xdead", "installDir": "/nonexistent", "jobId": "conf-verify-x", "repair": false}, &junk); err == nil {
		t.Errorf("game.verify on an unknown game answered %s, want an RPC error", junk)
	}
	const id = "conf-verify-1"
	if err := s.call("game.verify", map[string]any{"id": game.StoreAppID, "installDir": game.InstallDir, "jobId": id, "repair": false}, nil); err != nil {
		t.Fatalf("game.verify: %v", err)
	}
	done := s.waitNotified(t, id)
	if done.Method != "job.done" || done.Update.Verify == nil {
		t.Fatalf("game.verify ended with %s, want job.done with a verify object: %+v", done.Method, done.Update)
	}
	if v := done.Update.Verify; v.Repaired != 0 || v.Checked < 0 || v.Damaged < 0 || v.Missing < 0 {
		t.Errorf("a check reported %+v", v)
	}
}

// checkMoveParams holds a plugin offering `move` to its shapes without moving
// anything: missing params and a from that is not where the game is are both
// refused before any folder is touched.
func (s *Session) checkMoveParams(t *testing.T, game types.Game) {
	t.Helper()
	s.wantCode("game.move", map[string]any{}, jsonrpc.CodeInvalidParams)
	var junk json.RawMessage
	err := s.call("game.move", map[string]any{"id": game.StoreAppID, "from": "/nonexistent-from", "to": "/nonexistent-to", "jobId": "conf-move-x"}, &junk)
	var re *jsonrpc.Error
	if err == nil || !errors.As(err, &re) {
		t.Errorf("game.move from a folder the game isn't in answered %s (err %v), want an RPC error", junk, err)
	}
}

// waitNotified returns the job.done or job.failed the plugin sent for a job.
func (s *Session) waitNotified(t *testing.T, jobID string) event {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range s.Events(jobID) {
			if e.Method == "job.done" || e.Method == "job.failed" {
				return e
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s never reported done or failed", jobID)
	return event{}
}
