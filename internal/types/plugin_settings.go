package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

// Plugin settings (the `settings` capability, protocol 1.8,
// docs/plugins/protocol.md "Plugin settings"): the plugin describes its
// options as groups of fields, core renders them and stores the values.
// Values never carry secrets; those go through auth.

// Field kinds core renders. A field of any other kind is dropped on parse, so
// a plugin can describe kinds an older Hydra doesn't know.
const (
	SettingBool   = "bool"
	SettingSelect = "select"
	SettingText   = "text"
	SettingNumber = "number"
)

// Field scopes: a plugin-wide value (the Plugins view) or one per game (the
// game page). A field without a known scope is plugin-wide.
const (
	ScopePlugin = "plugin"
	ScopeGame   = "game"
)

// maxSettingText bounds a text value; longer ones are refused.
const maxSettingText = 4096

// SettingOption is one choice of a select field.
type SettingOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// SettingField is one option the plugin offers. Default is nil when the
// plugin gave none (or one of the wrong type): the value is then the
// field's zero value, or the first option for a select.
type SettingField struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Label       string          `json:"label"`
	Description string          `json:"description,omitempty"`
	Scope       string          `json:"scope"`
	Default     any             `json:"default"`
	Options     []SettingOption `json:"options,omitempty"`
	Placeholder string          `json:"placeholder,omitempty"`
	Min         *float64        `json:"min,omitempty"`
	Max         *float64        `json:"max,omitempty"`
}

// SettingGroup is a titled list of fields, in the order the plugin sent them.
type SettingGroup struct {
	ID          string         `json:"id"`
	Label       string         `json:"label"`
	Description string         `json:"description,omitempty"`
	Fields      []SettingField `json:"fields"`
}

// SettingsSchema is what settings.schema answers, normalized for core.
type SettingsSchema struct {
	Groups []SettingGroup `json:"groups"`
}

// SettingsView is one scope of a schema with the saved values for it: what
// the Plugins view or the game page shows and edits.
type SettingsView struct {
	Groups []SettingGroup `json:"groups"`
	Values map[string]any `json:"values"`
}

type rawSettingsSchema struct {
	Groups []json.RawMessage `json:"groups"`
}

type rawSettingGroup struct {
	ID          string            `json:"id"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Fields      []json.RawMessage `json:"fields"`
}

type rawSettingField struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
	Scope       string          `json:"scope"`
	Default     json.RawMessage `json:"default"`
	Options     json.RawMessage `json:"options"`
	Placeholder string          `json:"placeholder"`
	Min         *float64        `json:"min"`
	Max         *float64        `json:"max"`
}

// ParseSettingsSchema normalizes a settings.schema answer. Malformed groups
// and fields, fields of a kind core doesn't know, and repeated field ids are
// dropped; a bad default is dropped, not the field. The answer is an error
// only when it isn't an object with a groups list.
func ParseSettingsSchema(raw json.RawMessage) (SettingsSchema, error) {
	var top rawSettingsSchema
	if err := json.Unmarshal(raw, &top); err != nil || top.Groups == nil {
		return SettingsSchema{}, errors.New("settings.schema has no groups list")
	}
	out := SettingsSchema{Groups: []SettingGroup{}}
	seen := map[string]bool{}
	for _, rg := range top.Groups {
		var g rawSettingGroup
		if json.Unmarshal(rg, &g) != nil {
			continue
		}
		group := SettingGroup{ID: g.ID, Label: g.Label, Description: g.Description, Fields: []SettingField{}}
		for _, rf := range g.Fields {
			f, ok := parseField(rf)
			if !ok || seen[f.ID] {
				continue
			}
			seen[f.ID] = true
			group.Fields = append(group.Fields, f)
		}
		if len(group.Fields) > 0 {
			out.Groups = append(out.Groups, group)
		}
	}
	return out, nil
}

// parseField reads one field; ok is false for one core drops.
func parseField(raw json.RawMessage) (SettingField, bool) {
	var rf rawSettingField
	if json.Unmarshal(raw, &rf) != nil || rf.ID == "" {
		return SettingField{}, false
	}
	f := SettingField{
		ID: rf.ID, Kind: rf.Kind, Label: rf.Label, Description: rf.Description,
		Scope: ScopePlugin, Placeholder: rf.Placeholder,
	}
	if rf.Scope == ScopeGame {
		f.Scope = ScopeGame
	}
	switch rf.Kind {
	case SettingBool:
		var b bool
		if json.Unmarshal(rf.Default, &b) == nil && present(rf.Default) {
			f.Default = b
		}
	case SettingText:
		var s string
		// A default longer than a value may be is dropped: sent back with the
		// rest of the scope, it would refuse the whole save.
		if json.Unmarshal(rf.Default, &s) == nil && present(rf.Default) && len(s) <= maxSettingText {
			f.Default = s
		}
	case SettingNumber:
		// Bounds that contradict each other are dropped. A default outside the
		// bounds is clamped into them, so the form never sends a refused value.
		f.Min, f.Max = rf.Min, rf.Max
		if f.Min != nil && f.Max != nil && *f.Min > *f.Max {
			f.Min, f.Max = nil, nil
		}
		var n float64
		if json.Unmarshal(rf.Default, &n) == nil && present(rf.Default) && finite(n) {
			f.Default = clampNumber(n, f.Min, f.Max)
		}
	case SettingSelect:
		var list []SettingOption
		if json.Unmarshal(rf.Options, &list) != nil {
			return SettingField{}, false
		}
		f.Options = uniqueOptions(list)
		if len(f.Options) == 0 {
			return SettingField{}, false
		}
		var s string
		if json.Unmarshal(rf.Default, &s) == nil && hasOption(f.Options, s) {
			f.Default = s
		}
	default:
		return SettingField{}, false
	}
	return f, true
}

// uniqueOptions keeps the options with an id, the first of each id.
func uniqueOptions(list []SettingOption) []SettingOption {
	out := []SettingOption{}
	for _, o := range list {
		if o.ID != "" && !hasOption(out, o.ID) {
			out = append(out, o)
		}
	}
	return out
}

func hasOption(list []SettingOption, id string) bool {
	for _, o := range list {
		if o.ID == id {
			return true
		}
	}
	return false
}

// present says a raw default is there: absent and null both mean none.
func present(raw json.RawMessage) bool { return len(raw) > 0 && string(raw) != "null" }

func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }

// clampNumber moves n into [min, max]; a nil bound doesn't limit.
func clampNumber(n float64, lo, hi *float64) float64 {
	if lo != nil && n < *lo {
		return *lo
	}
	if hi != nil && n > *hi {
		return *hi
	}
	return n
}

// Scoped returns the schema with only the fields of one scope. Groups left
// without fields are dropped.
func (s SettingsSchema) Scoped(scope string) SettingsSchema {
	out := SettingsSchema{Groups: []SettingGroup{}}
	for _, g := range s.Groups {
		fields := []SettingField{}
		for _, f := range g.Fields {
			if f.Scope == scope {
				fields = append(fields, f)
			}
		}
		if len(fields) > 0 {
			g.Fields = fields
			out.Groups = append(out.Groups, g)
		}
	}
	return out
}

// ViewFor returns the groups of one scope with the saved values among their
// fields (values of other scopes, and of fields the schema no longer has, are
// left out).
func (s SettingsSchema) ViewFor(scope string, saved map[string]any) SettingsView {
	scoped := s.Scoped(scope)
	v := SettingsView{Groups: scoped.Groups, Values: map[string]any{}}
	for _, g := range scoped.Groups {
		for _, f := range g.Fields {
			if val, ok := saved[f.ID]; ok {
				v.Values[f.ID] = val
			}
		}
	}
	return v
}

// Field returns the field with the id, of any scope.
func (s SettingsSchema) Field(id string) (SettingField, bool) {
	for _, g := range s.Groups {
		for _, f := range g.Fields {
			if f.ID == id {
				return f, true
			}
		}
	}
	return SettingField{}, false
}

// CleanSettingValues checks values against the schema: a value for a field
// the schema doesn't have is dropped, and so is nil (reset to the default). A
// value of the wrong type, out of range, or not an option is an error that
// names the field.
func (s SettingsSchema) CleanSettingValues(values map[string]any) (map[string]any, error) {
	out := map[string]any{}
	for id, v := range values {
		f, ok := s.Field(id)
		if !ok || v == nil {
			continue
		}
		if err := f.check(v); err != nil {
			return nil, fmt.Errorf("%s: %w", f.Label, err)
		}
		out[id] = v
	}
	return out, nil
}

// check says whether v is a value of the field's kind. JSON numbers arrive as
// float64.
func (f SettingField) check(v any) error {
	switch f.Kind {
	case SettingBool:
		if _, ok := v.(bool); !ok {
			return errors.New("expected on or off")
		}
	case SettingText:
		s, ok := v.(string)
		if !ok {
			return errors.New("expected text")
		}
		if len(s) > maxSettingText {
			return errors.New("text is too long")
		}
	case SettingNumber:
		n, ok := v.(float64)
		if !ok || !finite(n) {
			return errors.New("expected a number")
		}
		if (f.Min != nil && n < *f.Min) || (f.Max != nil && n > *f.Max) {
			return errors.New("number is out of range")
		}
	case SettingSelect:
		s, ok := v.(string)
		if !ok || !hasOption(f.Options, s) {
			return errors.New("not one of the choices")
		}
	}
	return nil
}
