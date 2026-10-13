package settings

// MinimizeToTray says whether the window hides to the tray while a game runs
// (issue #49). Off by default: a plain bool, unset means false.
func (s *Store) MinimizeToTray() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.MinimizeToTray
}

// SetMinimizeToTray saves the "Minimize to tray while a game runs" switch.
func (s *Store) SetMinimizeToTray(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.MinimizeToTray
	s.data.MinimizeToTray = on
	if err := s.save(); err != nil {
		s.data.MinimizeToTray = prev
		return err
	}
	return nil
}
