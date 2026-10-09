package settings

// ChatAlerts says whether the user hears about new messages from friends on a
// plugin (a toast when the conversation isn't open). On unless switched off.
func (s *Store) ChatAlerts(pluginID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	on, ok := s.data.ChatAlerts[pluginID]
	return !ok || on
}

// SetChatAlerts saves whether the user hears about new messages on a plugin.
func (s *Store) SetChatAlerts(pluginID string, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setSwitch(&s.data.ChatAlerts, pluginID, on)
}
