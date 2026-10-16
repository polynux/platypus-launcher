package main

// The fake `achievements`, `playtime` and `redeem` capabilities (protocol 1.7).
// Playtime is fixed per store game, so the library's max(local, store) merge is
// visible: echo keeps no local playtime. Achievements exist for tiny and slow
// and cover every field variant: hidden and locked (masked by core), hidden and
// unlocked, a locked one with no globalPercent, and a plain one. Redeem grants
// a title for the code ECHO-OK and fails with a message for anything else. No
// knob; the knobs of the other capabilities (off, missing, gameFeatures, ...)
// apply here too. See docs/plugins/protocol.md, "Achievements, playtime, redeem".

import "fmt"

// redeemCode is the one code echo grants.
const redeemCode = "ECHO-OK"

// playtimeMinutes is the store's playtime per game, in minutes. Games not
// listed have no report (core keeps their local value).
var playtimeMinutes = map[string]int{"tiny": 150, "slow": 42, "huge": 1200, "flaky": 0}

// achievementDefs are echo's achievements, by game id. unlockedAt is unix
// seconds (0 is locked); a nil globalPercent is left out of the answer.
type achievementDef struct {
	id, name, description, icon string
	hidden                      bool
	unlockedAt                  int64
	globalPercent               *float64
}

func pct(v float64) *float64 { return &v }

var achievementDefs = map[string][]achievementDef{
	"tiny": {
		{id: "installed", name: "Get installed", description: "Install the game.", unlockedAt: 1791000000, globalPercent: pct(100)},
		{id: "played", name: "Play it", description: "Start the game once.", globalPercent: pct(84.1)},
		{id: "secret", name: "Secret door", description: "Find the room behind the wall.", hidden: true, globalPercent: pct(7.5)},
		{id: "ninety", name: "Ninety seconds", description: "Keep it running for ninety seconds.", hidden: true, unlockedAt: 1791100000},
		{id: "fast", name: "Quick start", description: "Start within five seconds of the install.", icon: "https://example.com/echo/fast.png"},
	},
	"slow": {},
}

// achievementsList answers achievements.list for one game. A game echo has no
// list for answers an empty one, like a store with no achievements.
func achievementsList(p params) (any, error) {
	if p.ID == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid params: id is required"}
	}
	g, ok := games[p.ID]
	if !ok {
		return nil, fmt.Errorf("unknown game %q", p.ID)
	}
	if g.external != "" || g.client {
		return nil, &rpcError{Code: -32601, Message: "method not found: achievements.list"}
	}
	list := []map[string]any{}
	unlocked := 0
	for _, a := range achievementDefs[p.ID] {
		e := map[string]any{
			"id": a.id, "name": a.name, "description": a.description, "hidden": a.hidden,
		}
		if a.icon != "" {
			e["icon"] = a.icon
		}
		if a.unlockedAt > 0 {
			e["unlockedAt"] = a.unlockedAt
			unlocked++
		}
		if a.globalPercent != nil {
			e["globalPercent"] = *a.globalPercent
		}
		list = append(list, e)
	}
	return map[string]any{"total": len(list), "unlocked": unlocked, "list": list}, nil
}

// playtimeList answers playtime.list for the whole library in one call,
// plus the seeded bulk minutes when ECHO_SEED is on (seeded.go).
func playtimeList() (any, error) {
	list := []map[string]any{}
	for _, id := range order {
		if minutes, ok := playtimeMinutes[id]; ok {
			list = append(list, map[string]any{"id": id, "minutes": minutes})
		}
	}
	if seed, ok := seededSeed(); ok {
		for i := 0; i < bulkCount(); i++ {
			if minutes, report := seededBulkMinutes(seed, i); report {
				list = append(list, map[string]any{"id": bulkID(i), "minutes": minutes})
			}
		}
	}
	return map[string]any{"games": list}, nil
}

// redeemSubmit answers redeem.submit: ECHO-OK grants a title, anything else
// fails with a readable message (-32000).
func redeemSubmit(p params) (any, error) {
	if p.Code == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid params: code is required"}
	}
	if p.Code != redeemCode {
		return nil, &rpcError{Code: -32000, Message: "Echo doesn't know that code. Try ECHO-OK."}
	}
	return map[string]any{
		"message": "Redeemed. Echo Bonus Pack is in your library.",
		"titles":  []string{"Echo Bonus Pack"},
	}, nil
}
