package settings

// ClientNotices says whether Platypus tells the user what a store's client is
// doing while that client starts a game (issue #149, the #87 launch status).
// On until the user switches it off.
func (s *Store) ClientNotices() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.ClientNotices == nil || *s.data.ClientNotices
}

// SetClientNotices saves the "Store client notices" switch.
func (s *Store) SetClientNotices(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.ClientNotices
	s.data.ClientNotices = &on
	if err := s.save(); err != nil {
		s.data.ClientNotices = prev
		return err
	}
	return nil
}
