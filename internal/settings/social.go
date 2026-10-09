package settings

// SocialEnabled says whether a plugin's friends list is on. A plugin that
// offers the social capability is on unless the user switched it off.
func (s *Store) SocialEnabled(pluginID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	on, ok := s.data.SocialPlugins[pluginID]
	return !ok || on
}

// SetSocialEnabled saves whether a plugin's friends list is on.
func (s *Store) SetSocialEnabled(pluginID string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setSwitch(&s.data.SocialPlugins, pluginID, enabled)
}

// SocialPlaying says whether friends see the game the user plays on a plugin.
// On unless the user switched it off.
func (s *Store) SocialPlaying(pluginID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	on, ok := s.data.SocialPlaying[pluginID]
	return !ok || on
}

// SetSocialPlaying saves whether friends see the game the user plays on a plugin.
func (s *Store) SetSocialPlaying(pluginID string, show bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setSwitch(&s.data.SocialPlaying, pluginID, show)
}

// setSwitch saves one plugin's switch in sw, and puts the old value back when
// the save fails. The caller holds s.mu.
func (s *Store) setSwitch(sw *map[string]bool, pluginID string, on bool) error {
	prev, had := (*sw)[pluginID]
	if *sw == nil {
		*sw = map[string]bool{}
	}
	(*sw)[pluginID] = on
	if err := s.save(); err != nil {
		if had {
			(*sw)[pluginID] = prev
		} else {
			delete(*sw, pluginID)
		}
		return err
	}
	return nil
}
