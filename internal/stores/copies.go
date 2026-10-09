package stores

import (
	"context"
	"slices"
	"strings"

	"hydra-launcher/internal/plugins"
	"hydra-launcher/internal/types"
)

// Family copies (protocol 1.8, the `family` capability, docs/plugins/protocol.md
// "Family"): when several members lend the same game, each lends its own copy,
// and the copies can differ (DLC, a note, in use). The user picks one with
// family.setPreferredCopy. Hydra shows the copies and sends the choice; the
// store decides what a copy allows.

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
			cp.DLC = append(cp.DLC, types.GameDLC{ID: d.ID, Name: name})
		}
		out = append(out, cp)
	}
	if !slices.ContainsFunc(out, func(c types.GameCopy) bool { return c.OwnerID == preferred }) {
		preferred = ""
	}
	return out, preferred
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
