package settings

import (
	"fmt"

	"platypus/internal/types"
)

// CloudPluginMode is the plugin's default cloud sync mode, "" when the user
// never chose one.
func (s *Store) CloudPluginMode(pluginID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.CloudPlugins[pluginID]
}

// CloudGameMode is the game's own mode, "" when it follows its plugin.
func (s *Store) CloudGameMode(gameID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.CloudGames[gameID]
}

// CloudModes returns every saved plugin default and game choice.
func (s *Store) CloudModes() (plugins, games map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copyMap := func(m map[string]string) map[string]string {
		out := make(map[string]string, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out
	}
	return copyMap(s.data.CloudPlugins), copyMap(s.data.CloudGames)
}

// SetCloudPluginMode saves a plugin's default mode; "" forgets the choice.
func (s *Store) SetCloudPluginMode(pluginID, mode string) error {
	return s.setCloudMode(&s.data.CloudPlugins, pluginID, mode)
}

// SetCloudGameMode saves a game's own mode; "" makes it follow its plugin.
func (s *Store) SetCloudGameMode(gameID, mode string) error {
	return s.setCloudMode(&s.data.CloudGames, gameID, mode)
}

func (s *Store) setCloudMode(m *map[string]string, key, mode string) error {
	if key == "" {
		return fmt.Errorf("an id is needed")
	}
	if mode != "" && !types.ValidCloudMode(mode) {
		return fmt.Errorf("cloud mode must be off, ask or auto, got %q", mode)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, had := (*m)[key]
	if mode == "" {
		delete(*m, key)
	} else {
		if *m == nil {
			*m = map[string]string{}
		}
		(*m)[key] = mode
	}
	if err := s.save(); err != nil {
		if had {
			(*m)[key] = prev
		} else {
			delete(*m, key)
		}
		return err
	}
	return nil
}
