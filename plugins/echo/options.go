package main

// The fake `install-options` capability (protocol 1.5): a language, add-on
// content (a DLC group, one of its values unowned with a store link), optional
// packs, a subtitles toggle, and a group of a kind core doesn't know, which
// core must ignore. Each installable game offers all of them. The choices of
// the last install are kept in state, so game.options reports what is
// installed, and installSize adds the sizes of the chosen add-ons and packs
// to the game's size. See docs/plugins/protocol.md, "Install options".

import (
	"fmt"
	"slices"
)

type optValue struct {
	id, label string
	size      int64
	owned     bool
	storeURL  string
}

var echoLanguages = []optValue{{id: "en", label: "English"}, {id: "fr", label: "Français"}, {id: "de", label: "Deutsch"}}

var echoDLC = []optValue{
	{"season", "Season Pass", 1 << 20, true, ""},
	{"soundtrack", "Soundtrack", 2 << 20, true, ""},
	{"deluxe", "Deluxe Content", 4 << 20, false, "https://example.com/echo/store/deluxe"},
}

var echoPacks = []optValue{
	{"hd", "HD textures", 3 << 20, true, ""},
	{"docs", "Manual", 512 << 10, true, ""},
}

// optionRec is what echo keeps of an install's choices. Languages and toggles
// change nothing on disk here; the add-ons and packs change the size.
type optionRec struct {
	Language  string   `json:"language"`
	DLC       []string `json:"dlc"`
	Packs     []string `json:"packs"`
	Subtitles bool     `json:"subtitles"`
}

func bad(format string, a ...any) error {
	return &rpcError{Code: -32602, Message: fmt.Sprintf(format, a...)}
}

// listOf reads a JSON list of strings from an options value. ok is false when
// the value is not a list.
func listOf(v any) (list []string, ok bool) {
	items, ok := v.([]any)
	if !ok {
		return nil, false
	}
	for _, item := range items {
		s, isString := item.(string)
		if !isString {
			return nil, false
		}
		list = append(list, s)
	}
	return list, true
}

// parseOptions checks an install's options the way the store would: unknown
// values are refused, and so is DLC the user doesn't own, unless it is already
// installed. A nil map means the defaults.
func parseOptions(gameID string, raw map[string]any) (optionRec, error) {
	rec := optionRec{Language: "en"}
	if raw == nil {
		return rec, nil
	}
	if v, ok := raw["language"]; ok {
		s, _ := v.(string)
		if !slices.ContainsFunc(echoLanguages, func(l optValue) bool { return l.id == s }) {
			return rec, bad("Echo has no language %v", v)
		}
		rec.Language = s
	}
	stMu.Lock()
	installed := st.Options[gameID].DLC
	stMu.Unlock()
	if v, ok := raw["dlc"]; ok {
		list, ok := listOf(v)
		if !ok {
			return rec, bad("dlc is a list of value ids")
		}
		for _, id := range list {
			d, found := findValue(echoDLC, id)
			if !found {
				return rec, bad("Echo has no add-on %q", id)
			}
			if !d.owned && !slices.Contains(installed, id) {
				return rec, bad("%s is not owned: buy it in the store first", d.label)
			}
			rec.DLC = append(rec.DLC, id)
		}
	}
	if v, ok := raw["packs"]; ok {
		list, ok := listOf(v)
		if !ok {
			return rec, bad("packs is a list of value ids")
		}
		for _, id := range list {
			if _, found := findValue(echoPacks, id); !found {
				return rec, bad("Echo has no pack %q", id)
			}
			rec.Packs = append(rec.Packs, id)
		}
	}
	if v, ok := raw["subtitles"]; ok {
		on, isBool := v.(bool)
		if !isBool {
			return rec, bad("subtitles is on or off")
		}
		rec.Subtitles = on
	}
	return rec, nil
}

func findValue(values []optValue, id string) (optValue, bool) {
	for _, v := range values {
		if v.id == id {
			return v, true
		}
	}
	return optValue{}, false
}

// optionsSize is what the chosen add-ons and packs add to the game's size.
func optionsSize(rec optionRec) int64 {
	var total int64
	for _, id := range rec.DLC {
		d, _ := findValue(echoDLC, id)
		total += d.size
	}
	for _, id := range rec.Packs {
		p, _ := findValue(echoPacks, id)
		total += p.size
	}
	return total
}

// valueList renders catalog values for game.options. installed says which
// DLC ids the last install left on disk.
func valueList(values []optValue, installed []string) []map[string]any {
	out := []map[string]any{}
	for _, v := range values {
		e := map[string]any{"id": v.id, "label": v.label, "owned": v.owned}
		if v.size > 0 {
			e["sizeBytes"] = v.size
		}
		if v.storeURL != "" {
			e["storeUrl"] = v.storeURL
		}
		if slices.Contains(installed, v.id) {
			e["installed"] = true
		}
		out = append(out, e)
	}
	return out
}

// gameOptions answers game.options. A game the store doesn't install has no
// groups; the others offer the full set, with what was installed marked.
func gameOptions(p params) (any, error) {
	if p.ID == "" {
		return nil, &rpcError{Code: -32602, Message: "invalid params: id is required"}
	}
	g, ok := games[p.ID]
	if !ok {
		return nil, fmt.Errorf("unknown game %q", p.ID)
	}
	if g.external != "" || g.client {
		return map[string]any{"groups": []any{}}, nil
	}
	stMu.Lock()
	installed := st.Options[p.ID]
	stMu.Unlock()
	languages := valueList(echoLanguages, nil)
	return map[string]any{"groups": []any{
		map[string]any{"id": "language", "label": "Language", "kind": "select",
			"values": languages, "default": "en"},
		map[string]any{"id": "dlc", "label": "Add-ons", "kind": "multi", "dlc": true,
			"values": valueList(echoDLC, installed.DLC)},
		map[string]any{"id": "packs", "label": "Optional packs", "kind": "multi",
			"values": valueList(echoPacks, installed.Packs)},
		map[string]any{"id": "subtitles", "label": "Subtitles", "kind": "bool", "default": false},
		// A kind core doesn't know: it must be ignored, not shown or refused.
		map[string]any{"id": "mystery", "label": "Mystery", "kind": "carousel",
			"values": []map[string]any{{"id": "x", "label": "x"}}},
	}}, nil
}
