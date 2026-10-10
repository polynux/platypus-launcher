package types

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const echoSchema = `{"groups": [
  {"id": "overlay", "label": "Overlay", "fields": [
    {"id": "eos", "kind": "bool", "label": "Enable EOS overlay", "default": false},
    {"id": "region", "kind": "select", "label": "Region", "default": "eu",
     "options": [{"id": "eu", "label": "Europe"}, {"id": "us", "label": "US"}, {"id": "eu", "label": "dup"}]},
    {"id": "note", "kind": "text", "label": "Note", "default": "", "placeholder": "e.g. flags"},
    {"id": "volume", "kind": "number", "label": "Volume", "default": 5, "min": 0, "max": 10},
    {"id": "mystery", "kind": "carousel", "label": "Carousel"}
  ]},
  {"id": "per-game", "label": "Per game", "fields": [
    {"id": "game-overlay", "kind": "bool", "label": "Overlay for this game", "scope": "game", "default": true}
  ]},
  {"id": "junk", "label": "Only unknown kinds", "fields": [{"id": "x", "kind": "carousel", "label": "x"}]},
  "not a group"
]}`

func mustParse(t *testing.T, raw string) SettingsSchema {
	t.Helper()
	s, err := ParseSettingsSchema(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("ParseSettingsSchema: %v", err)
	}
	return s
}

func TestParseSettingsSchemaDropsWhatCoreDoesNotKnow(t *testing.T) {
	s := mustParse(t, echoSchema)
	if len(s.Groups) != 2 {
		t.Fatalf("groups = %d, want 2 (the group of unknown kinds and the junk entry are dropped)", len(s.Groups))
	}
	ids := []string{}
	for _, g := range s.Groups {
		for _, f := range g.Fields {
			ids = append(ids, f.ID)
		}
	}
	want := []string{"eos", "region", "note", "volume", "game-overlay"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("fields = %v, want %v", ids, want)
	}
	region, _ := s.Field("region")
	if len(region.Options) != 2 || region.Default != "eu" {
		t.Fatalf("region = %+v: a repeated option id must be dropped and the default kept", region)
	}
	game, _ := s.Field("game-overlay")
	if game.Scope != ScopeGame || game.Default != true {
		t.Fatalf("game-overlay = %+v, want game scope with default on", game)
	}
	eos, _ := s.Field("eos")
	if eos.Scope != ScopePlugin || eos.Default != false {
		t.Fatalf("eos = %+v, want plugin scope with default off (a false default is kept)", eos)
	}
}

func TestParseSettingsSchemaBadDefaultsAndDuplicates(t *testing.T) {
	s := mustParse(t, `{"groups":[{"id":"g","label":"G","fields":[
	  {"id":"a","kind":"bool","label":"A","default":"yes"},
	  {"id":"b","kind":"number","label":"B","default":null},
	  {"id":"c","kind":"select","label":"C","default":"zz","options":[{"id":"x","label":"X"}]},
	  {"id":"c","kind":"text","label":"again"},
	  {"id":"d","kind":"select","label":"D","options":"nope"},
	  {"label":"no id","kind":"text"}
	]}]}`)
	for _, id := range []string{"a", "b", "c"} {
		f, ok := s.Field(id)
		if !ok || f.Default != nil {
			t.Errorf("%s = %+v: a default of the wrong type or none must be nil", id, f)
		}
	}
	if _, ok := s.Field("d"); ok {
		t.Error("a select with malformed options must be dropped")
	}
	if f, _ := s.Field("c"); f.Kind != SettingSelect {
		t.Errorf("the first field with an id keeps the id, got kind %q", f.Kind)
	}
}

func TestParseSettingsSchemaNeedsGroups(t *testing.T) {
	if _, err := ParseSettingsSchema(json.RawMessage(`{"nope":1}`)); err == nil {
		t.Fatal("an answer without a groups list must be an error")
	}
	if _, err := ParseSettingsSchema(json.RawMessage(`[]`)); err == nil {
		t.Fatal("an answer that is not an object must be an error")
	}
}

func TestViewForKeepsOneScopeAndItsSavedValues(t *testing.T) {
	s := mustParse(t, echoSchema)
	saved := map[string]any{"eos": true, "game-overlay": false, "volume": 3.0}
	plugin := s.ViewFor(ScopePlugin, saved)
	if len(plugin.Groups) != 1 || plugin.Groups[0].ID != "overlay" {
		t.Fatalf("plugin view groups = %+v", plugin.Groups)
	}
	if _, ok := plugin.Values["game-overlay"]; ok {
		t.Error("a game-scope value must not reach the plugin view")
	}
	if plugin.Values["eos"] != true || plugin.Values["volume"] != 3.0 {
		t.Errorf("plugin values = %v", plugin.Values)
	}
	game := s.ViewFor(ScopeGame, saved)
	if len(game.Groups) != 1 || len(game.Groups[0].Fields) != 1 || game.Values["game-overlay"] != false {
		t.Errorf("game view = %+v", game)
	}
}

func TestCleanSettingValues(t *testing.T) {
	s := mustParse(t, echoSchema)
	clean, err := s.CleanSettingValues(map[string]any{
		"eos": true, "region": "us", "note": "-dx11", "volume": 7.0,
		"mystery": "x", "unknown": 1, "game-overlay": false, "note2": nil, "eos2": nil,
	})
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	want := map[string]any{"eos": true, "region": "us", "note": "-dx11", "volume": 7.0, "game-overlay": false}
	if !reflect.DeepEqual(clean, want) {
		t.Fatalf("clean = %v, want %v (unknown keys and nil values dropped)", clean, want)
	}
	nilValue, err := s.CleanSettingValues(map[string]any{"eos": nil})
	if err != nil || len(nilValue) != 0 {
		t.Fatalf("a nil value must reset the field: %v %v", nilValue, err)
	}
}

func TestCleanSettingValuesRefusesWrongValues(t *testing.T) {
	s := mustParse(t, echoSchema)
	for name, v := range map[string]map[string]any{
		"bool as text":      {"eos": "yes"},
		"select not option": {"region": "mars"},
		"number too big":    {"volume": 11.0},
		"number as text":    {"volume": "5"},
		"text as number":    {"note": 4.0},
	} {
		if _, err := s.CleanSettingValues(v); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestParseSettingsSchemaKeepsDefaultsInBounds(t *testing.T) {
	long := strings.Repeat("x", maxSettingText+1)
	s := mustParse(t, `{"groups":[{"id":"g","label":"G","fields":[
	  {"id":"high","kind":"number","label":"High","default":50,"min":0,"max":10},
	  {"id":"low","kind":"number","label":"Low","default":-3,"min":0,"max":10},
	  {"id":"inverted","kind":"number","label":"Inverted","default":7,"min":10,"max":0},
	  {"id":"big","kind":"text","label":"Big","default":"`+long+`"},
	  {"id":"ok","kind":"text","label":"Ok","default":"fine"}
	]}]}`)
	if f, _ := s.Field("high"); f.Default != 10.0 {
		t.Errorf("high default = %v, want clamped to the max 10", f.Default)
	}
	if f, _ := s.Field("low"); f.Default != 0.0 {
		t.Errorf("low default = %v, want clamped to the min 0", f.Default)
	}
	if f, _ := s.Field("inverted"); f.Min != nil || f.Max != nil || f.Default != 7.0 {
		t.Errorf("inverted = %+v, want min and max dropped (they contradict) and the default kept", f)
	}
	if f, _ := s.Field("big"); f.Default != nil {
		t.Errorf("big text default is %d bytes, want it dropped at the %d-byte cap", len(f.Default.(string)), maxSettingText)
	}
	if f, _ := s.Field("ok"); f.Default != "fine" {
		t.Errorf("ok default = %v, want kept", f.Default)
	}
}

func TestDefaultsAreAlwaysAcceptedBySave(t *testing.T) {
	s := mustParse(t, `{"groups":[{"id":"g","label":"G","fields":[
	  {"id":"volume","kind":"number","label":"Volume","default":50,"min":0,"max":10},
	  {"id":"big","kind":"text","label":"Big","default":"`+strings.Repeat("x", maxSettingText+1)+`"}
	]}]}`)
	for _, f := range s.Scoped(ScopePlugin).Groups[0].Fields {
		if _, err := s.CleanSettingValues(map[string]any{f.ID: f.Default}); err != nil && f.Default != nil {
			t.Errorf("default of %s refused by the save: %v", f.ID, err)
		}
	}
}

func TestMergeSettingValuesSendsOnlyTheChangedKeys(t *testing.T) {
	s := mustParse(t, echoSchema).Scoped(ScopePlugin)
	saved := map[string]any{"eos": true, "region": "us", "volume": 3.0}
	merged, err := s.MergeSettingValues(saved, map[string]any{"volume": 7.0})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"eos": true, "region": "us", "volume": 7.0}
	if !reflect.DeepEqual(merged, want) {
		t.Fatalf("merged = %v, want %v: a key not in the patch keeps its saved value", merged, want)
	}
	reset, err := s.MergeSettingValues(saved, map[string]any{"region": nil})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reset["region"]; ok || len(reset) != 2 {
		t.Fatalf("reset = %v, want region back to its default (not stored)", reset)
	}
	if _, err := s.MergeSettingValues(saved, map[string]any{"volume": 11.0}); err == nil {
		t.Fatal("a patch value out of range must be refused, and nothing merged")
	}
}

func TestKeepValidDropsValuesTheSchemaNoLongerTakes(t *testing.T) {
	s := mustParse(t, echoSchema).Scoped(ScopePlugin)
	got := s.KeepValid(map[string]any{
		"region": "mars", "volume": 3.0, "game-overlay": false, "gone": true, "eos": true,
	})
	want := map[string]any{"volume": 3.0, "eos": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kept = %v, want %v", got, want)
	}
}

func TestParseSettingsSchemaCollectsNoSecrets(t *testing.T) {
	// Secrets go through auth; a field of a secret kind is not rendered, so a
	// plugin can't collect a password through settings.
	s := mustParse(t, `{"groups":[{"id":"g","label":"G","fields":[
	  {"id":"pw","kind":"password","label":"Password","default":""},
	  {"id":"token","kind":"secret","label":"Token"},
	  {"id":"user","kind":"text","label":"User","default":"ana"}
	]}]}`)
	for _, id := range []string{"pw", "token"} {
		if _, ok := s.Field(id); ok {
			t.Errorf("a %s field is shown: secret kinds must be dropped", id)
		}
	}
	if _, ok := s.Field("user"); !ok {
		t.Error("a plain text field next to them was dropped")
	}
}
