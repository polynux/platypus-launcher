package types

import (
	"encoding/json"
	"reflect"
	"strings"
)

// ModeledSettingKeys are the saved settings keys GameSettings models. Every other
// key a game saved is a plugin's setting (the runner's choices, for one): the
// core stores it as it is and hands it to the plugin that declares it.
func ModeledSettingKeys() map[string]struct{} {
	out := map[string]struct{}{}
	t := reflect.TypeOf(GameSettings{})
	for i := 0; i < t.NumField(); i++ {
		if name := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]; name != "" && name != "-" {
			out[name] = struct{}{}
		}
	}
	return out
}

// UnmarshalJSON keeps the settings keys GameSettings doesn't model in
// SettingsExtra. The UI holds every saved key of a game and sends them back with
// it, so the plugin that declared a key reads it from the game as the UI holds
// it, whatever the game's source.
func (g *Game) UnmarshalJSON(data []byte) error {
	type plain Game
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	var raw struct {
		Settings map[string]json.RawMessage `json:"settings"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	modeled := ModeledSettingKeys()
	for k, v := range raw.Settings {
		if _, ok := modeled[k]; ok {
			continue
		}
		var val any
		if err := json.Unmarshal(v, &val); err != nil {
			continue
		}
		if p.SettingsExtra == nil {
			p.SettingsExtra = map[string]any{}
		}
		p.SettingsExtra[k] = val
	}
	*g = Game(p)
	return nil
}
