package plugtest

import (
	"encoding/json"
	"errors"
	"testing"

	"platypus/internal/jsonrpc"
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
	plain := checkTermList(t, "game.details", raw)
	var rpcErr *jsonrpc.Error
	err := s.call("game.acceptTerms", map[string]any{"id": storeAppID, "terms": []map[string]string{{"id": "no-such-term", "version": "0"}}}, nil)
	if !errors.As(err, &rpcErr) || (rpcErr.Code != jsonrpc.CodeInvalidParams && rpcErr.Code != jsonrpc.CodeMethodNotFound) {
		t.Errorf("game.acceptTerms for a term the game doesn't list: %v, want -32602 (or -32601 when it isn't offered)", err)
	}
	// The terms read (protocol 1.11, "Terms"): with terms:true the plugin
	// answers the terms it lists, and fails rather than omitting what it
	// couldn't read — an empty list would read as "nothing to accept".
	var asked json.RawMessage
	if err := s.call("game.details", map[string]any{"id": storeAppID, "terms": true}, &asked); err != nil {
		if !errors.As(err, &rpcErr) {
			t.Errorf("game.details with terms:true failed without an RPC error: %v", err)
		}
		return
	}
	checkKeys(t, "game.details", asked, DetailsKeys)
	answered := checkTermList(t, "game.details with terms:true", asked)
	for id := range plain {
		if !answered[id] {
			t.Errorf("game.details lists term %q without terms:true but omits it when asked", id)
		}
	}
}

// checkTermList holds one game.details answer's terms to their shape (an id
// and a version, an https url when it has one) and returns the ids it saw.
func checkTermList(t *testing.T, what string, raw json.RawMessage) map[string]bool {
	t.Helper()
	seen := map[string]bool{}
	var d struct {
		Terms []json.RawMessage `json:"terms"`
	}
	if err := json.Unmarshal(raw, &d); err != nil || len(d.Terms) == 0 {
		return seen
	}
	for _, term := range d.Terms {
		checkKeys(t, what+" term", term, TermKeys)
		var v struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			URL     string `json:"url"`
			Version string `json:"version"`
		}
		if err := json.Unmarshal(term, &v); err != nil || v.ID == "" || v.Version == "" {
			t.Errorf("%s term without an id or a version: %s", what, term)
			continue
		}
		seen[v.ID] = true
	}
	return seen
}
