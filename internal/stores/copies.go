package stores

import (
	"context"
	"slices"
	"strings"

	"platypus/internal/plugins"
	"platypus/internal/types"
)

// Family copies (protocol 1.8, the `family` capability, docs/plugins/protocol.md
// "Family"): when several members lend the same game, each lends its own copy,
// and the copies can differ (DLC, a note, in use). The user picks one with
// family.setPreferredCopy. Platypus shows the copies and sends the choice; the
// store decides what a copy allows. Protocol 1.11 adds the DLC kinds and the
// exclusion text; Platypus sets each copy against the baseline (copydiff.go).

const (
	maxCopies   = 16
	maxCopyDLC  = 64
	maxCopyText = 200
)

// normalizeCopies makes a plugin's copies safe for the UI: entries without an
// owner id are dropped, names fall back to the id, duplicates and overlong
// text are cut, and a preferred copy that names no listed owner is cleared.
func normalizeCopies(in []plugins.LibraryCopy, preferred string) ([]types.GameCopy, string) {
	var out []types.GameCopy
	for _, c := range in {
		if c.OwnerID == "" || len(out) >= maxCopies || slices.ContainsFunc(out, func(o types.GameCopy) bool { return o.OwnerID == c.OwnerID }) {
			continue
		}
		cp := types.GameCopy{
			OwnerID:   c.OwnerID,
			OwnerName: clipText(c.OwnerName, maxCopyText),
			Note:      clipText(c.Note, maxCopyText),
			Available: c.Available,
			InUse:     c.InUse,
			Exclude:   clipText(c.Exclude, maxCopyText),
		}
		if cp.OwnerName == "" {
			cp.OwnerName = c.OwnerID
		}
		for _, d := range c.DLC {
			if d.ID == "" || len(cp.DLC) >= maxCopyDLC || slices.ContainsFunc(cp.DLC, func(o types.GameDLC) bool { return o.ID == d.ID }) {
				continue
			}
			name := clipText(d.Name, maxCopyText)
			if name == "" {
				name = d.ID
			}
			cp.DLC = append(cp.DLC, types.GameDLC{ID: d.ID, Name: name, Kind: dlcKind(d.Kind)})
		}
		out = append(out, cp)
	}
	if !slices.ContainsFunc(out, func(c types.GameCopy) bool { return c.OwnerID == preferred }) {
		preferred = ""
	}
	return out, preferred
}

// dlcKind is the kind of an add-on the UI knows. Anything else, or none, is
// game DLC: a plugin that sends a kind this Platypus doesn't know is still read.
func dlcKind(k string) string {
	if k == "soundtrack" || k == "other" {
		return k
	}
	return "dlc"
}

// clipText trims s and cuts it to max runes.
func clipText(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > max {
		r = r[:max]
	}
	return string(r)
}

// SetPreferredCopy tells the store which family copy of a borrowed game the
// user wants. The caller has checked that the game is borrowed and that the
// copy is listed and available (see App.SetPreferredCopy).
func (s *Plugin) SetPreferredCopy(ctx context.Context, storeAppID, ownerID string) error {
	return s.m.Call(ctx, s.ID(), "family.setPreferredCopy", map[string]any{"id": storeAppID, "ownerId": ownerID}, nil)
}
