package plugtest

import (
	"encoding/json"
	"errors"
	"testing"

	"platypus/internal/jsonrpc"
	"platypus/internal/types"
)

// The achievements, playtime and redeem capabilities (protocol 1.7,
// docs/plugins/protocol.md, "Achievements, playtime and redeem"): the keys
// each answer may carry, and the errors core relies on.
var (
	AchievementsKeys = []string{"total", "unlocked", "list"}
	AchievementKeys  = []string{"id", "name", "description", "icon", "hidden", "unlockedAt", "globalPercent"}
	PlaytimeKeys     = []string{"games"}
	PlaytimeGameKeys = []string{"id", "minutes", "lastPlayed"}
)

// checkStats holds a plugin offering the 1.7 stats capabilities to their
// shapes: each method needs its params (-32602), answers only documented keys,
// and gives achievements an id. unknownIDErrors is the same rule as
// versions.list: an unknown game is an RPC error.
func (s *Session) checkStats(t *testing.T, game types.Game, has func(string) bool, unknownIDErrors bool) {
	t.Helper()
	if has("achievements") {
		s.wantCode("achievements.list", map[string]any{}, jsonrpc.CodeInvalidParams)
		var raw json.RawMessage
		if err := s.call("achievements.list", map[string]any{"id": game.StoreAppID}, &raw); err != nil {
			t.Fatalf("achievements.list: %v", err)
		}
		checkKeys(t, "achievements.list", raw, AchievementsKeys)
		var list struct {
			List []json.RawMessage `json:"list"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			t.Fatalf("achievements.list result: %v", err)
		}
		for _, a := range list.List {
			checkKeys(t, "achievements.list entry", a, AchievementKeys)
			var entry struct {
				ID string `json:"id"`
			}
			_ = json.Unmarshal(a, &entry)
			if entry.ID == "" {
				t.Errorf("achievements.list entry without an id: %s", a)
			}
		}
		if unknownIDErrors {
			var junk json.RawMessage
			err := s.call("achievements.list", map[string]any{"id": "no-such-game-0xdead"}, &junk)
			var re *jsonrpc.Error
			if err == nil || !errors.As(err, &re) {
				t.Errorf("achievements.list on an unknown id answered %s (err %v), want an RPC error", junk, err)
			}
		}
	}
	if has("playtime") {
		var raw json.RawMessage
		if err := s.call("playtime.list", map[string]any{}, &raw); err != nil {
			t.Fatalf("playtime.list: %v", err)
		}
		checkKeys(t, "playtime.list", raw, PlaytimeKeys)
		var list struct {
			Games []json.RawMessage `json:"games"`
		}
		if err := json.Unmarshal(raw, &list); err != nil || list.Games == nil {
			t.Fatalf("playtime.list has no games list: %s", raw)
		}
		for _, g := range list.Games {
			checkKeys(t, "playtime.list game", g, PlaytimeGameKeys)
			var entry struct {
				ID      string `json:"id"`
				Minutes int    `json:"minutes"`
			}
			_ = json.Unmarshal(g, &entry)
			if entry.ID == "" || entry.Minutes < 0 {
				t.Errorf("playtime.list entry needs an id and minutes >= 0: %s", g)
			}
		}
	}
	if has("redeem") {
		// Only the parameter check: a real store's code is never sent as a probe.
		s.wantCode("redeem.submit", map[string]any{}, jsonrpc.CodeInvalidParams)
	}
}
