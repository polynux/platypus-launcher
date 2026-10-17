package types

import (
	"encoding/json"
	"testing"
)

// A game's settings keys the backend doesn't model reach SettingsExtra, so the
// plugin that declared them reads them; modeled keys stay in Settings.
func TestGameKeepsUnmodeledSettingsKeys(t *testing.T) {
	var g Game
	data := `{"id":"g1","settings":{"use_gamemode":true,"sync_mode":"ntsync","proton_log":true,"gamescope_args":"-f"}}`
	if err := json.Unmarshal([]byte(data), &g); err != nil {
		t.Fatal(err)
	}
	if !g.Settings.UseGameMode || g.Settings.GamescopeArgs != "-f" {
		t.Fatalf("modeled settings = %+v", g.Settings)
	}
	if g.SettingsExtra["sync_mode"] != "ntsync" || g.SettingsExtra["proton_log"] != true {
		t.Fatalf("unmodeled keys = %v, want sync_mode and proton_log kept", g.SettingsExtra)
	}
	if _, ok := g.SettingsExtra["use_gamemode"]; ok {
		t.Fatal("a modeled key was also kept as an extra")
	}
}
