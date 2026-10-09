package types

import (
	"errors"
	"fmt"
	"slices"
)

// The `install-options` capability (protocol 1.5, docs/plans/launcher-parity.md
// §2.1): what a store lets the user choose before an install (language, add-on
// content, optional packs, toggles). DLC is an option group with `dlc: true`.
// Core only knows the three group kinds below; a group of any other kind is
// ignored, so a newer plugin can add kinds without breaking this Hydra.

const (
	OptionSelect = "select" // one value of values
	OptionMulti  = "multi"  // any number of values
	OptionBool   = "bool"   // on or off; has no values

	maxOptionGroups = 50
	maxOptionValues = 500
	maxOptionField  = 256
)

// OptionValue is one choice of a select or multi group. SizeBytes is what it
// adds to the install. Owned is false for DLC the user doesn't have: it is
// shown, but never installed. Installed says it is already on disk.
type OptionValue struct {
	ID        string `json:"id"`
	Label     string `json:"label,omitempty"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
	Owned     bool   `json:"owned"`
	Installed bool   `json:"installed,omitempty"`
	StoreURL  string `json:"storeUrl,omitempty"`
}

// OptionGroup is one question the install dialog asks. Default is the value
// id of a select group, or true/false for a bool group.
type OptionGroup struct {
	ID      string        `json:"id"`
	Label   string        `json:"label,omitempty"`
	Kind    string        `json:"kind"`
	DLC     bool          `json:"dlc,omitempty"`
	Values  []OptionValue `json:"values,omitempty"`
	Default any           `json:"default,omitempty"`
}

// InstallOptions are the choices for one install or update, keyed by group id:
// a string for a select group, a list of value ids for multi, a bool for bool.
// It travels as `options` on game.install, game.update and game.installSize.
type InstallOptions map[string]any

// GameOptions answers GetGameOptions: the groups the store offers and the
// choices Hydra would use now (the saved ones, or the defaults).
type GameOptions struct {
	Groups []OptionGroup `json:"groups"`
	Chosen InstallOptions `json:"chosen"`
}

// NormalizeGroups makes a plugin's groups safe to show and to resolve:
// unknown kinds, groups without ids or values, and duplicates are dropped;
// labels fall back to ids; a select default that isn't a value falls back to
// the first value; the sizes are never negative.
func NormalizeGroups(in []OptionGroup) []OptionGroup {
	out := []OptionGroup{}
	seen := map[string]bool{}
	for _, g := range in {
		if len(out) >= maxOptionGroups {
			break
		}
		if g.ID == "" || len(g.ID) > maxOptionField || seen[g.ID] {
			continue
		}
		if !slices.Contains([]string{OptionSelect, OptionMulti, OptionBool}, g.Kind) {
			continue // a kind this Hydra doesn't know: ignored, not an error
		}
		g.Label = fallback(g.Label, g.ID)
		if g.Kind == OptionBool {
			g.Values = nil
			on, _ := g.Default.(bool)
			g.Default = on
			seen[g.ID] = true
			out = append(out, g)
			continue
		}
		values := []OptionValue{}
		ids := map[string]bool{}
		for _, v := range g.Values {
			if v.ID == "" || len(v.ID) > maxOptionField || ids[v.ID] || len(values) >= maxOptionValues {
				continue
			}
			ids[v.ID] = true
			v.Label = fallback(v.Label, v.ID)
			if v.SizeBytes < 0 {
				v.SizeBytes = 0
			}
			if v.StoreURL != "" && httpsOnly(v.StoreURL) == "" {
				v.StoreURL = ""
			}
			values = append(values, v)
		}
		if len(values) == 0 {
			continue
		}
		g.Values = values
		if g.Kind == OptionSelect {
			def, _ := g.Default.(string)
			if !ids[def] {
				def = values[0].ID
			}
			g.Default = def
		} else {
			g.Default = nil
		}
		seen[g.ID] = true
		out = append(out, g)
	}
	return out
}

func fallback(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// httpsOnly keeps a store link when it is an https URL, "" otherwise. The UI
// opens it in the user's browser.
func httpsOnly(s string) string {
	if len(s) > 2048 || len(s) < len("https://x") || s[:8] != "https://" {
		return ""
	}
	return s
}

// ResolveOptions turns the user's choices into the complete set of options an
// install sends, checked against the groups the store offers now.
//
// Strict (a choice the user just made): a value that doesn't exist, a wrong
// type, or an unowned DLC that isn't already installed is an error.
// Lenient (choices saved earlier, re-used by a plain install or update): the
// same problems quietly fall back to the default or are left out, so a DLC the
// user no longer owns doesn't block a game update. In both cases a choice for
// a group the store no longer offers is dropped. A group left out of choices
// gets its default; the result has a key for every group.
func ResolveOptions(groups []OptionGroup, choices InstallOptions, strict bool) (InstallOptions, error) {
	out := InstallOptions{}
	for _, g := range groups {
		raw, chosen := choices[g.ID]
		switch g.Kind {
		case OptionBool:
			out[g.ID] = g.Default
			if chosen {
				on, ok := raw.(bool)
				if !ok {
					if strict {
						return nil, fmt.Errorf("%s is on or off", g.Label)
					}
					break
				}
				out[g.ID] = on
			}
		case OptionSelect:
			out[g.ID] = g.Default
			if chosen {
				id, ok := raw.(string)
				if ok && hasValue(g, id) {
					out[g.ID] = id
					break
				}
				if strict {
					return nil, fmt.Errorf("%s has no choice %v", g.Label, raw)
				}
			}
		case OptionMulti:
			picked, err := pickValues(g, raw, chosen, strict)
			if err != nil {
				return nil, err
			}
			out[g.ID] = picked
		}
	}
	return out, nil
}

// pickValues resolves a multi group. Values keep the store's order and appear
// once. An unowned DLC value is allowed only when it is already installed.
func pickValues(g OptionGroup, raw any, chosen, strict bool) ([]string, error) {
	picked := []string{}
	if !chosen {
		return picked, nil
	}
	list, ok := raw.([]any)
	if !ok && raw != nil {
		if strict {
			return nil, fmt.Errorf("%s takes a list of choices", g.Label)
		}
		return picked, nil
	}
	want := map[string]bool{}
	for _, item := range list {
		id, ok := item.(string)
		if !ok || id == "" {
			if strict {
				return nil, fmt.Errorf("%s has a choice that isn't a value", g.Label)
			}
			continue
		}
		want[id] = true
	}
	for _, v := range g.Values {
		if !want[v.ID] {
			continue
		}
		if g.DLC && !v.Owned && !v.Installed {
			if strict {
				return nil, fmt.Errorf("%s is not owned: %w", v.Label, ErrNotOwned)
			}
			continue
		}
		picked = append(picked, v.ID)
	}
	if strict {
		for id := range want {
			if !hasValue(g, id) {
				return nil, fmt.Errorf("%s has no choice %q", g.Label, id)
			}
		}
	}
	return picked, nil
}

// ErrNotOwned is returned for an install that names DLC the user doesn't own.
var ErrNotOwned = errors.New("buy it in the store first")

func hasValue(g OptionGroup, id string) bool {
	for _, v := range g.Values {
		if v.ID == id {
			return true
		}
	}
	return false
}

// OptionsSize is what the chosen values add to a game's base size: the
// SizeBytes of every value a select or multi group has chosen.
func OptionsSize(groups []OptionGroup, options InstallOptions) int64 {
	var total int64
	for _, g := range groups {
		for _, v := range g.Values {
			if isChosen(options[g.ID], v.ID) {
				total += v.SizeBytes
			}
		}
	}
	return total
}

// isChosen says whether a select value (a string) or a multi list (strings,
// or []any after a round trip through settings or the job queue) names id.
func isChosen(choice any, id string) bool {
	switch c := choice.(type) {
	case string:
		return c == id
	case []string:
		return slices.Contains(c, id)
	case []any:
		for _, item := range c {
			if s, ok := item.(string); ok && s == id {
				return true
			}
		}
	}
	return false
}
