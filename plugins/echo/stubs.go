package main

// stubs answer the methods of capabilities Hydra recognises but has no
// orchestrator for yet (docs/plugins/protocol.md, "Capabilities"), so the
// conformance suite can check that a plugin declaring one really answers it.
// Methods about one game want an id and answer -32602 without it, like a real
// plugin must before it acts on anything.
var stubs = map[string]struct {
	perGame bool
	result  any
}{
	"game.options":      {true, map[string]any{"groups": []any{}}},
	"game.verify":       {true, map[string]any{}},
	"game.move":         {true, map[string]any{}},
	"achievements.list": {true, map[string]any{"total": 0, "unlocked": 0, "list": []any{}}},
	"playtime.list":     {false, map[string]any{"games": []any{}}},
	"redeem.submit":     {false, map[string]any{"message": "Echo has nothing to redeem."}},
	"family.members":    {false, []any{}},
	"social.start":      {false, map[string]any{}},
	"social.stop":       {false, map[string]any{}},
	"social.state":      {false, map[string]any{}},
	"social.self":       {false, map[string]any{}},
	"social.friends":    {false, map[string]any{"friends": []any{}}},
	"social.setStatus":  {false, map[string]any{}},
	"social.setPlaying": {false, map[string]any{}},
}

func capabilityStub(method string, p params) (res any, err error, ok bool) {
	s, ok := stubs[method]
	if !ok {
		return nil, nil, false
	}
	if s.perGame && p.ID == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid params: id is required"}, true
	}
	return s.result, nil, true
}
