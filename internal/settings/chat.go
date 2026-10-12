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

// ChatDesktopNotify says whether new chat messages raise a desktop
// notification when the window isn't focused (issue #12). On unless the user
// switched it off. In-app toasts are unaffected.
func (s *Store) ChatDesktopNotify() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.ChatDesktopNotify == nil || *s.data.ChatDesktopNotify
}

// SetChatDesktopNotify saves the "Desktop notifications for chat" switch.
func (s *Store) SetChatDesktopNotify(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.ChatDesktopNotify
	s.data.ChatDesktopNotify = &on
	if err := s.save(); err != nil {
		s.data.ChatDesktopNotify = prev
		return err
	}
	return nil
}
