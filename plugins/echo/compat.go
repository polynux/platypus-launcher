package main

import (
	"encoding/json"
	"os"
	"sync"
)

// Compat facts (protocol 1.10, docs/plugins/protocol.md, "Compat"): game.details
// carries the game's Steam AppID hint, its platforms and a DRM block. "tiny"
// has all of them by default and covers every DRM field. Any other game gets
// them from the ECHO_COMPAT environment variable (JSON {"<id>": {...}}, read
// at start) or from `config` {compat: {"<id>": {...}}}, which replaces that
// game's facts (an empty object clears them). A game with facts answers
// game.details even without the details flag.
var compatDefaults = map[string]map[string]any{
	"tiny": {
		"steamAppId": "620",
		"platforms":  []string{"linux", "windows"},
		"drm": map[string]any{
			"drmFree":        false,
			"onlineRequired": true,
			"launcher":       "ubisoft-connect",
			"denuvoLikely":   true,
			"notes":          "Echo fixture: store facts for the compat tests.",
		},
	},
}

var (
	compatMu    sync.Mutex
	compatFacts = map[string]map[string]any{}
)

// loadCompat sets up the facts at start: the defaults, then ECHO_COMPAT.
func loadCompat() {
	compatMu.Lock()
	defer compatMu.Unlock()
	for id, f := range compatDefaults {
		compatFacts[id] = f
	}
	if v := os.Getenv("ECHO_COMPAT"); v != "" {
		var env map[string]map[string]any
		if json.Unmarshal([]byte(v), &env) == nil {
			for id, f := range env {
				compatFacts[id] = f
			}
		}
	}
}

// compatFor returns the compat facts of a game, nil when it has none.
func compatFor(id string) map[string]any {
	compatMu.Lock()
	defer compatMu.Unlock()
	f := compatFacts[id]
	if len(f) == 0 {
		return nil
	}
	return f
}

// configureCompat applies a `config` request's compat map.
func configureCompat(raw json.RawMessage) {
	var present struct {
		Compat map[string]map[string]any `json:"compat"`
	}
	if json.Unmarshal(raw, &present) != nil || present.Compat == nil {
		return
	}
	compatMu.Lock()
	defer compatMu.Unlock()
	for id, f := range present.Compat {
		compatFacts[id] = f
	}
}
