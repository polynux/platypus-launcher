package settings

// Theme is the picked theme's id (issue #93): "" is the built-in default.
func (s *Store) Theme() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Theme
}

// SetTheme saves the picked theme's id ("" for the default). The app checks
// that the theme exists and can be used before it gets here.
func (s *Store) SetTheme(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.Theme
	s.data.Theme = id
	if err := s.save(); err != nil {
		s.data.Theme = prev
		return err
	}
	return nil
}
