package types

// Terms are the agreements a store asks the user to accept before a game is
// installed (protocol 1.11, the `terms` capability, docs/plugins/protocol.md
// "Terms"). Hydra records each acceptance by term id and version, and asks
// again when the version changes. Acceptance is never automatic.

// GameTerm is one term of a game, with the user's acceptance of its current
// version (AcceptedAt is unix seconds, 0 when it is not accepted).
type GameTerm struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	URL        string `json:"url,omitempty"`
	Version    string `json:"version"`
	AcceptedAt int64  `json:"accepted_at,omitempty"`
}

// TermRef names one term version the user accepts.
type TermRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// TermAcceptance is one record of the user accepting a term version (kept in
// settings.json, per game).
type TermAcceptance struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	At      int64  `json:"at"` // unix seconds
}

// Pending returns the terms whose current version the user hasn't accepted.
func Pending(terms []GameTerm) []GameTerm {
	var out []GameTerm
	for _, t := range terms {
		if t.AcceptedAt == 0 {
			out = append(out, t)
		}
	}
	return out
}

// MarkAccepted fills AcceptedAt on each term that has an acceptance of exactly
// its id and version. An acceptance of an older version doesn't count.
func MarkAccepted(terms []GameTerm, accepted []TermAcceptance) []GameTerm {
	out := make([]GameTerm, len(terms))
	copy(out, terms)
	for i := range out {
		for _, a := range accepted {
			if a.ID == out[i].ID && a.Version == out[i].Version {
				out[i].AcceptedAt = a.At
			}
		}
	}
	return out
}
