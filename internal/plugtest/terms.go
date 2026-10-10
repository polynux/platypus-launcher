package plugtest

import (
	"encoding/json"
	"errors"
	"testing"

	"hydra-launcher/internal/jsonrpc"
)

// The terms capability (protocol 1.11, docs/plugins/protocol.md, "Terms"): the
// keys a game.details term may carry, and game.acceptTerms when a plugin
// offers it. Terms are optional per game; a game without any checks nothing.
var TermKeys = []string{"id", "title", "url", "version"}

// checkTerms holds the terms of a game's game.details to their shape: an id
// and a version (acceptance is keyed on both), and an https url when it has
// one. game.acceptTerms refuses a term the game doesn't list with -32602, or
// answers -32601 when the store doesn't keep acceptance (optional).
func (s *Session) checkTerms(t *testing.T, storeAppID string, raw json.RawMessage) {
	t.Helper()
	var d struct {
		Terms []json.RawMessage `json:"terms"`
	}
	if err := json.Unmarshal(raw, &d); err != nil || len(d.Terms) == 0 {
		return
	}
	for _, term := range d.Terms {
		checkKeys(t, "game.details term", term, TermKeys)
		var v struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			URL     string `json:"url"`
			Version string `json:"version"`
		}
		if err := json.Unmarshal(term, &v); err != nil || v.ID == "" || v.Version == "" {
			t.Errorf("game.details term without an id or a version: %s", term)
		}
	}
	var rpcErr *jsonrpc.Error
	err := s.call("game.acceptTerms", map[string]any{"id": storeAppID, "terms": []map[string]string{{"id": "no-such-term", "version": "0"}}}, nil)
	if !errors.As(err, &rpcErr) || (rpcErr.Code != jsonrpc.CodeInvalidParams && rpcErr.Code != jsonrpc.CodeMethodNotFound) {
		t.Errorf("game.acceptTerms for a term the game doesn't list: %v, want -32602 (or -32601 when it isn't offered)", err)
	}
}
