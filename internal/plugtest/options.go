package plugtest

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"platypus/internal/jsonrpc"
	"platypus/internal/types"
)

// The install-options capability (protocol 1.5, docs/plugins/protocol.md,
// "Install options"): the keys game.options may answer with, and the rules
// core relies on. A group of a kind core doesn't know is allowed; core ignores
// it, so the suite only checks the kinds it knows.
var (
	OptionsKeys      = []string{"groups"}
	OptionGroupKeys  = []string{"id", "label", "kind", "dlc", "values", "default"}
	OptionValueKeys  = []string{"id", "label", "sizeBytes", "owned", "installed", "storeUrl"}
	optionGroupKinds = []string{types.OptionSelect, types.OptionMulti, types.OptionBool}
)

// checkOptions holds a plugin offering install-options to those shapes:
// game.options needs an id and answers only documented keys; a select group's
// default is one of its values; a bool default is a bool; every value has an
// id. unknownIDErrors says whether an unknown game is answered with an error
// (the same rule as versions.list).
func (s *Session) checkOptions(t *testing.T, game types.Game, unknownIDErrors bool) {
	t.Helper()
	s.wantCode("game.options", map[string]any{}, jsonrpc.CodeInvalidParams)
	var raw json.RawMessage
	if err := s.call("game.options", map[string]any{"id": game.StoreAppID}, &raw); err != nil {
		t.Fatalf("game.options: %v", err)
	}
	checkKeys(t, "game.options", raw, OptionsKeys)
	var list struct {
		Groups []json.RawMessage `json:"groups"`
	}
	if err := json.Unmarshal(raw, &list); err != nil || list.Groups == nil {
		t.Fatalf("game.options has no groups list: %s", raw)
	}
	for _, g := range list.Groups {
		checkKeys(t, "game.options group", g, OptionGroupKeys)
		var grp struct {
			ID      string            `json:"id"`
			Kind    string            `json:"kind"`
			Values  []json.RawMessage `json:"values"`
			Default any               `json:"default"`
		}
		if err := json.Unmarshal(g, &grp); err != nil {
			t.Fatalf("game.options group: %v", err)
		}
		if grp.ID == "" {
			t.Errorf("game.options group without an id: %s", g)
		}
		if !slices.Contains(optionGroupKinds, grp.Kind) {
			continue // a kind core doesn't know: allowed, and ignored by core
		}
		ids := map[string]bool{}
		for _, v := range grp.Values {
			checkKeys(t, "game.options value", v, OptionValueKeys)
			var val struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(v, &val); err != nil || val.ID == "" {
				t.Errorf("game.options value without an id in group %q: %s", grp.ID, v)
			}
			ids[val.ID] = true
		}
		switch grp.Kind {
		case types.OptionSelect:
			if def, _ := grp.Default.(string); grp.Default != nil && !ids[def] {
				t.Errorf("group %q defaults to %v, which is not one of its values", grp.ID, grp.Default)
			}
		case types.OptionBool:
			if _, ok := grp.Default.(bool); grp.Default != nil && !ok {
				t.Errorf("bool group %q defaults to %v, want true or false", grp.ID, grp.Default)
			}
		}
	}
	var junk json.RawMessage
	err := s.call("game.options", map[string]any{"id": "no-such-game-0xdead"}, &junk)
	if unknownIDErrors {
		var re *jsonrpc.Error
		if err == nil || !errors.As(err, &re) {
			t.Errorf("game.options on an unknown id answered %s (err %v), want an RPC error", junk, err)
		}
	}
}
