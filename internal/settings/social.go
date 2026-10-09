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
	prev, had := s.data.SocialPlugins[pluginID]
	if s.data.SocialPlugins == nil {
		s.data.SocialPlugins = map[string]bool{}
	}
	s.data.SocialPlugins[pluginID] = enabled
	if err := s.save(); err != nil {
		if had {
			s.data.SocialPlugins[pluginID] = prev
		} else {
			delete(s.data.SocialPlugins, pluginID)
		}
		return err
	}
	return nil
}
