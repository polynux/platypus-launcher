package main

// The fake `settings` capability (protocol 1.8, docs/plugins/protocol.md,
// "Plugin settings"): one schema with every kind core renders (bool, select,
// text, number), a per-game field, and a kind core doesn't know, which core
// must drop. settings.apply keeps the values in memory only, the way a plugin
// that doesn't persist them relies on Hydra to send them again on start.
// `config` answers what was last applied, so tests can read it back.

import (
	"errors"
	"maps"
	"sync"
	"time"
)

var (
	appliedMu   sync.Mutex
	appliedPlug = map[string]any{}
	appliedGame = map[string]map[string]any{}
)

// settingsSchema answers settings.schema.
func settingsSchema() map[string]any {
	return map[string]any{"groups": []any{
		map[string]any{"id": "overlay", "label": "Overlay", "description": "Echo draws nothing; these only change the values it keeps.",
			"fields": []any{
				map[string]any{"id": "eos-overlay", "kind": "bool", "label": "Enable overlay (example)", "description": "Echo's stand-in for an overlay.", "default": false},
				map[string]any{"id": "region", "kind": "select", "label": "Download region (example)", "default": "eu",
					"options": []any{map[string]any{"id": "eu", "label": "Europe"}, map[string]any{"id": "us", "label": "United States"}, map[string]any{"id": "asia", "label": "Asia"}}},
				map[string]any{"id": "note", "kind": "text", "label": "Extra launch arguments (example)", "default": "", "placeholder": "e.g. -windowed"},
				map[string]any{"id": "volume", "kind": "number", "label": "Bandwidth limit, MB/s (example)", "default": 5, "min": 0, "max": 10},
				// A kind core doesn't know: it must be dropped, not shown.
				map[string]any{"id": "mystery", "kind": "carousel", "label": "Carousel", "default": nil},
			}},
		map[string]any{"id": "per-game", "label": "Per game", "fields": []any{
			map[string]any{"id": "game-overlay", "kind": "bool", "label": "Overlay for this game", "scope": "game", "default": true},
		}},
	}}
}

// settingsApply stores what core sends: the plugin-wide values, or the values
// of one game when gameId is set. Values of the wrong kind are refused.
func settingsApply(p params) (any, error) {
	if p.Values == nil {
		return nil, bad("invalid params: values is required")
	}
	knobMu.Lock()
	refuse, delay := kn.SettingsRefuse, kn.SettingsDelayMs
	knobMu.Unlock()
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	if refuse {
		return nil, errors.New("Echo can't take settings right now")
	}
	if p.GameID != "" {
		if _, ok := games[p.GameID]; !ok {
			return nil, bad("Echo has no game %q", p.GameID)
		}
	}
	for id, v := range p.Values {
		switch id {
		case "eos-overlay", "game-overlay":
			if _, ok := v.(bool); !ok {
				return nil, bad("%s is on or off", id)
			}
		case "region":
			if s, _ := v.(string); s != "eu" && s != "us" && s != "asia" {
				return nil, bad("Echo has no region %v", v)
			}
		case "note":
			if _, ok := v.(string); !ok {
				return nil, bad("note is text")
			}
		case "volume":
			if n, ok := v.(float64); !ok || n < 0 || n > 10 {
				return nil, bad("volume is a number from 0 to 10")
			}
		}
	}
	appliedMu.Lock()
	defer appliedMu.Unlock()
	if p.GameID == "" {
		appliedPlug = maps.Clone(p.Values)
	} else if len(p.Values) == 0 {
		delete(appliedGame, p.GameID)
	} else {
		appliedGame[p.GameID] = maps.Clone(p.Values)
	}
	return map[string]any{}, nil
}

// settingsSnapshot is what `config` answers: the values last applied.
func settingsSnapshot() map[string]any {
	appliedMu.Lock()
	defer appliedMu.Unlock()
	perGame := map[string]any{}
	for id, v := range appliedGame {
		perGame[id] = maps.Clone(v)
	}
	return map[string]any{"plugin": maps.Clone(appliedPlug), "games": perGame}
}
