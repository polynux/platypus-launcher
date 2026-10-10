package settings

import (
	"errors"
	"maps"
)

// PluginGameValues are one game's values for a plugin's per-game settings
// (the `settings` capability). StoreID is the game's id in the plugin, which
// core sends as gameId when it applies the values again on plugin start.
type PluginGameValues struct {
	Source  string         `json:"source"`
	StoreID string         `json:"store_id"`
	Values  map[string]any `json:"values"`
}

// PluginValues returns the saved plugin-wide values of a plugin's settings. A
// copy: the caller may keep it. Nothing saved is an empty map.
func (s *Store) PluginValues(pluginID string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.data.PluginSettings[pluginID])
}

// SetPluginValues replaces the plugin-wide values of a plugin's settings. The
// caller checks them against the plugin's schema first.
func (s *Store) SetPluginValues(pluginID string, values map[string]any) error {
	if pluginID == "" {
		return errors.New("plugin id is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := s.data.PluginSettings[pluginID]
	if len(values) == 0 {
		delete(s.data.PluginSettings, pluginID)
	} else {
		if s.data.PluginSettings == nil {
			s.data.PluginSettings = map[string]map[string]any{}
		}
		s.data.PluginSettings[pluginID] = maps.Clone(values)
	}
	if err := s.save(); err != nil {
		if had {
			s.data.PluginSettings[pluginID] = prev
		} else {
			delete(s.data.PluginSettings, pluginID)
		}
		return err
	}
	return nil
}

// PluginGameSettings returns the saved per-game values of a game, if any.
func (s *Store) PluginGameSettings(gameID string) (PluginGameValues, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data.PluginGameSettings[gameID]
	v.Values = maps.Clone(v.Values)
	return v, ok
}

// PluginGameSettingsFor lists the saved per-game values of one plugin's games,
// so they can be applied again when the plugin starts.
func (s *Store) PluginGameSettingsFor(source string) []PluginGameValues {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []PluginGameValues
	for _, v := range s.data.PluginGameSettings {
		if v.Source == source {
			v.Values = maps.Clone(v.Values)
			out = append(out, v)
		}
	}
	return out
}

// SetPluginGameSettings replaces a game's per-game values. Empty values forget
// the game's entry.
func (s *Store) SetPluginGameSettings(gameID string, v PluginGameValues) error {
	if gameID == "" {
		return errors.New("game id is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := s.data.PluginGameSettings[gameID]
	if len(v.Values) == 0 {
		delete(s.data.PluginGameSettings, gameID)
	} else {
		if s.data.PluginGameSettings == nil {
			s.data.PluginGameSettings = map[string]PluginGameValues{}
		}
		v.Values = maps.Clone(v.Values)
		s.data.PluginGameSettings[gameID] = v
	}
	if err := s.save(); err != nil {
		if had {
			s.data.PluginGameSettings[gameID] = prev
		} else {
			delete(s.data.PluginGameSettings, gameID)
		}
		return err
	}
	return nil
}

// PrunePlugins forgets the settings of the plugins keep rejects, plugin-wide
// and per-game alike: a plugin that is gone takes its values with it.
func (s *Store) PrunePlugins(keep func(pluginID string) bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prevPlugin := maps.Clone(s.data.PluginSettings)
	prevGames := maps.Clone(s.data.PluginGameSettings)
	changed := false
	for id := range s.data.PluginSettings {
		if !keep(id) {
			delete(s.data.PluginSettings, id)
			changed = true
		}
	}
	for id, v := range s.data.PluginGameSettings {
		if !keep(v.Source) {
			delete(s.data.PluginGameSettings, id)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if err := s.save(); err != nil {
		s.data.PluginSettings, s.data.PluginGameSettings = prevPlugin, prevGames
		return err
	}
	return nil
}

// PruneGames forgets the per-game values of one plugin's games that are not in
// keep (Platypus game ids). Call it with a library that was listed in full: a
// game missing from a partial list would lose its values.
func (s *Store) PruneGames(source string, keep map[string]bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := maps.Clone(s.data.PluginGameSettings)
	changed := false
	for id, v := range s.data.PluginGameSettings {
		if v.Source == source && !keep[id] {
			delete(s.data.PluginGameSettings, id)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if err := s.save(); err != nil {
		s.data.PluginGameSettings = prev
		return err
	}
	return nil
}
