package settings

import (
	"path/filepath"
	"testing"
)

// Plugin settings values survive a restart, and an empty set forgets the entry.
func TestPluginSettingsPersistAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetPluginValues("echo", map[string]any{"eos-overlay": true, "volume": 7.0}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPluginGameSettings("p.echo.tiny", PluginGameValues{Source: "echo", StoreID: "tiny", Values: map[string]any{"game-overlay": false}}); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.PluginValues("echo"); got["eos-overlay"] != true || got["volume"] != 7.0 || len(got) != 2 {
		t.Fatalf("plugin values after reopen = %v", got)
	}
	game, ok := reopened.PluginGameSettings("p.echo.tiny")
	if !ok || game.StoreID != "tiny" || game.Source != "echo" || game.Values["game-overlay"] != false {
		t.Fatalf("game values after reopen = %+v %v", game, ok)
	}
	if list := reopened.PluginGameSettingsFor("echo"); len(list) != 1 {
		t.Fatalf("games of echo = %v, want the one saved game", list)
	}
	if list := reopened.PluginGameSettingsFor("gog"); len(list) != 0 {
		t.Fatalf("games of gog = %v, want none", list)
	}

	if err := reopened.SetPluginValues("echo", nil); err != nil {
		t.Fatal(err)
	}
	if err := reopened.SetPluginGameSettings("p.echo.tiny", PluginGameValues{Source: "echo"}); err != nil {
		t.Fatal(err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.PluginValues("echo"); len(got) != 0 {
		t.Fatalf("an empty set must forget the plugin's values, got %v", got)
	}
	if _, ok := again.PluginGameSettings("p.echo.tiny"); ok {
		t.Fatal("an empty set must forget the game's values")
	}
}

// PluginValues hands out a copy: changing it does not change what is saved.
func TestPluginValuesAreCopies(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	in := map[string]any{"region": "us"}
	if err := s.SetPluginValues("echo", in); err != nil {
		t.Fatal(err)
	}
	in["region"] = "asia"
	out := s.PluginValues("echo")
	out["region"] = "eu"
	if got := s.PluginValues("echo"); got["region"] != "us" {
		t.Fatalf("saved values changed through a copy: %v", got)
	}
}

func TestPluginSettingsNeedAnID(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetPluginValues("", map[string]any{"a": true}); err == nil {
		t.Error("an empty plugin id must be refused")
	}
	if err := s.SetPluginGameSettings("", PluginGameValues{Values: map[string]any{"a": true}}); err == nil {
		t.Error("an empty game id must be refused")
	}
}
