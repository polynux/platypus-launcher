package main

// The fake `terms` capability (protocol 1.11, docs/plugins/protocol.md,
// "Terms"): the game "licensed" lists two terms in game.details, an end user
// licence and a privacy notice. Platypus asks the user to accept them before an
// install. The ECHO_TERMS_BUMP knob (or `config` termsBump) changes the licence
// version, so it asks again. game.acceptTerms records what the user accepted
// in state.json (see "recorded"); ECHO_MISSING=game.acceptTerms makes echo a
// store that doesn't keep acceptance, which Platypus then keeps alone.

import (
	"encoding/json"
	"fmt"
	"slices"
)

type echoTerm struct {
	id, title, url, version string
}

// licenseVersion is the version of the licence term; the bump knob makes it
// the next one.
func licenseVersion() string {
	knobMu.Lock()
	defer knobMu.Unlock()
	if kn.TermsBump {
		return "2026.2"
	}
	return "2026.1"
}

// termsOf lists the terms of a game (none for most games).
func termsOf(gameID string) []echoTerm {
	if !games[gameID].terms {
		return nil
	}
	return []echoTerm{
		{"eula", "Echo end user licence", "https://example.com/echo/eula", licenseVersion()},
		{"privacy", "Echo privacy notice", "https://example.com/echo/privacy", "3"},
	}
}

// termsList is the terms of a game as game.details carries them.
func termsList(gameID string) []map[string]any {
	var out []map[string]any
	for _, t := range termsOf(gameID) {
		out = append(out, map[string]any{"id": t.id, "title": t.title, "url": t.url, "version": t.version})
	}
	return out
}

// acceptTerms answers game.acceptTerms { id, terms: [{id, version}] }. Only
// terms the game lists, at the version it lists now, are accepted; anything
// else is -32602, the way a store refuses an acceptance it can't match.
func acceptTerms(raw json.RawMessage) (any, error) {
	var p struct {
		ID    string `json:"id"`
		Terms []struct {
			ID      string `json:"id"`
			Version string `json:"version"`
		} `json:"terms"`
	}
	if err := json.Unmarshal(raw, &p); err != nil || p.ID == "" {
		return nil, bad("invalid params: id and terms are required")
	}
	if _, ok := games[p.ID]; !ok {
		return nil, fmt.Errorf("unknown game %q", p.ID)
	}
	listed := termsOf(p.ID)
	var keys []string
	for _, t := range p.Terms {
		if !slices.ContainsFunc(listed, func(l echoTerm) bool { return l.id == t.ID && l.version == t.Version }) {
			return nil, bad("%s has no term %s at version %q", p.ID, t.ID, t.Version)
		}
		keys = append(keys, t.ID+"@"+t.Version)
	}
	stMu.Lock()
	defer stMu.Unlock()
	if st.AcceptedTerms == nil {
		st.AcceptedTerms = map[string][]string{}
	}
	for _, k := range keys {
		if !slices.Contains(st.AcceptedTerms[p.ID], k) {
			st.AcceptedTerms[p.ID] = append(st.AcceptedTerms[p.ID], k)
		}
	}
	save()
	return map[string]any{}, nil
}
