package main

// stubs answer the methods of capabilities Platypus recognises but has no
// orchestrator for yet (docs/plugins/protocol.md, "Capabilities"), so the
// conformance suite can check that a plugin declaring one really answers it.
// Methods about one game want an id and answer -32602 without it, like a real
// plugin must before it acts on anything.
var stubs = map[string]struct {
	perGame bool
	result  any
}{
	"game.options":  {true, map[string]any{"groups": []any{}}},
	"game.verify":   {true, map[string]any{}},
	"game.move":     {true, map[string]any{}},
	"compat.lookup": {false, map[string]any{"facts": []any{}}},
}

func capabilityStub(method string, p params) (res any, err error, ok bool) {
	s, ok := stubs[method]
	if !ok {
		return nil, nil, false
	}
	if s.perGame && p.ID == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid params: id is required"}, true
	}
	if method == "compat.lookup" && len(p.Games) == 0 {
		return nil, &rpcError{Code: -32602, Message: "invalid params: games is required"}, true
	}
	return s.result, nil, true
}
