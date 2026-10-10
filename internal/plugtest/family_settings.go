package plugtest

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"hydra-launcher/internal/jsonrpc"
)

// The family and settings capabilities (protocol 1.8, docs/plugins/protocol.md,
// "Family" and "Plugin settings"): the keys each answer may carry, and the
// rules core relies on. Unknown kinds in a schema are allowed; core drops them.
var (
	FamilyMemberKeys  = []string{"id", "name", "role", "avatarUrl", "state"}
	FamilyCopyKeys    = []string{"ownerId", "ownerName", "dlc", "note", "available", "inUse", "exclude"}
	FamilyCopyDLCKeys = []string{"id", "name", "kind"}
	SettingsKeys      = []string{"groups"}
	SettingGroupKeys  = []string{"id", "label", "description", "fields"}
	SettingFieldKeys  = []string{"id", "kind", "label", "description", "scope", "default", "options", "placeholder", "min", "max"}
	SettingOptionKeys = []string{"id", "label"}
	settingKinds      = []string{"bool", "select", "text", "number"}
	// secretKinds are field kinds a plugin must not describe. Core doesn't render
	// them, so no secret is collected through settings: those go through auth.
	secretKinds = []string{"password", "secret"}
)

// checkFamily holds a plugin offering `family` to its shape: family.members
// answers a list (no params needed), and every member has an id.
func (s *Session) checkFamily(t *testing.T) {
	t.Helper()
	var raw json.RawMessage
	if err := s.call("family.members", map[string]any{}, &raw); err != nil {
		t.Fatalf("family.members: %v", err)
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatalf("family.members isn't a list: %s", raw)
	}
	for _, m := range list {
		checkKeys(t, "family.members member", m, FamilyMemberKeys)
		var v struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(m, &v); err != nil || v.ID == "" {
			t.Errorf("family.members member without an id: %s", m)
		}
	}
}

// checkCopies holds a borrowed game's `copies` (family, protocol 1.8) to its
// shape: each copy names its owner and says whether it is available, its DLC
// has ids. A plugin that lists copies must answer family.setPreferredCopy with
// -32602 for an empty pick (or -32601 when it doesn't implement it: optional).
func (s *Session) checkCopies(t *testing.T, storeAppID string, raw json.RawMessage) {
	t.Helper()
	var g struct {
		Copies []json.RawMessage `json:"copies"`
	}
	if err := json.Unmarshal(raw, &g); err != nil || len(g.Copies) == 0 {
		return
	}
	for _, c := range g.Copies {
		checkKeys(t, "library.list copy", c, FamilyCopyKeys)
		var v struct {
			OwnerID   string            `json:"ownerId"`
			Available *bool             `json:"available"`
			DLC       []json.RawMessage `json:"dlc"`
		}
		if err := json.Unmarshal(c, &v); err != nil || v.OwnerID == "" || v.Available == nil {
			t.Errorf("library.list copy without an ownerId or available: %s", c)
		}
		for _, d := range v.DLC {
			checkKeys(t, "library.list copy dlc", d, FamilyCopyDLCKeys)
		}
	}
	var rpcErr *jsonrpc.Error
	err := s.call("family.setPreferredCopy", map[string]any{"id": storeAppID, "ownerId": ""}, nil)
	if !errors.As(err, &rpcErr) || (rpcErr.Code != jsonrpc.CodeInvalidParams && rpcErr.Code != jsonrpc.CodeMethodNotFound) {
		t.Errorf("family.setPreferredCopy with no owner: %v, want -32602 (or -32601 when not implemented)", err)
	}
}

// checkSettings holds a plugin offering `settings` to its shapes: settings.schema
// answers groups of fields with known keys, a select's default is one of its
// options, and settings.apply refuses a call without values (-32602) and
// accepts one with them.
func (s *Session) checkSettings(t *testing.T) {
	t.Helper()
	var raw json.RawMessage
	if err := s.call("settings.schema", map[string]any{}, &raw); err != nil {
		t.Fatalf("settings.schema: %v", err)
	}
	checkKeys(t, "settings.schema", raw, SettingsKeys)
	var top struct {
		Groups []json.RawMessage `json:"groups"`
	}
	if err := json.Unmarshal(raw, &top); err != nil || top.Groups == nil {
		t.Fatalf("settings.schema has no groups list: %s", raw)
	}
	for _, g := range top.Groups {
		checkKeys(t, "settings.schema group", g, SettingGroupKeys)
		var grp struct {
			Fields []json.RawMessage `json:"fields"`
		}
		if err := json.Unmarshal(g, &grp); err != nil {
			t.Fatalf("settings.schema group: %v", err)
		}
		for _, f := range grp.Fields {
			s.checkSettingField(t, f)
		}
	}
	s.wantCode("settings.apply", map[string]any{}, jsonrpc.CodeInvalidParams)
	var applied json.RawMessage
	if err := s.call("settings.apply", map[string]any{"values": map[string]any{}}, &applied); err != nil {
		t.Fatalf("settings.apply with no values: %v", err)
	}
}

// checkSettingField checks one field of a kind core knows. A field of another
// kind is allowed (core ignores it) and is only checked for its id.
func (s *Session) checkSettingField(t *testing.T, raw json.RawMessage) {
	t.Helper()
	checkKeys(t, "settings.schema field", raw, SettingFieldKeys)
	var f struct {
		ID      string            `json:"id"`
		Kind    string            `json:"kind"`
		Scope   string            `json:"scope"`
		Default any               `json:"default"`
		Options []json.RawMessage `json:"options"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("settings.schema field: %v", err)
	}
	if f.ID == "" {
		t.Errorf("settings.schema field without an id: %s", raw)
	}
	if slices.Contains(secretKinds, f.Kind) {
		t.Errorf("field %q is a %s field: secrets go through auth, not settings", f.ID, f.Kind)
	}
	if !slices.Contains(settingKinds, f.Kind) {
		return
	}
	if f.Scope != "" && f.Scope != "plugin" && f.Scope != "game" {
		t.Errorf("field %q has the scope %q, want plugin or game", f.ID, f.Scope)
	}
	if f.Kind != "select" {
		return
	}
	ids := map[string]bool{}
	for _, o := range f.Options {
		checkKeys(t, "settings.schema option", o, SettingOptionKeys)
		var v struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(o, &v); err != nil || v.ID == "" {
			t.Errorf("select %q has an option without an id: %s", f.ID, o)
		}
		ids[v.ID] = true
	}
	if def, _ := f.Default.(string); f.Default != nil && !ids[def] {
		t.Errorf("select %q defaults to %v, which is not one of its options", f.ID, f.Default)
	}
}
