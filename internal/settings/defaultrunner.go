package settings

// DefaultRunnerID is the runner "auto" resolves to (issue #146): "" means the
// automatic pick.
func (s *Store) DefaultRunnerID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.DefaultRunnerID
}

// SetDefaultRunnerID saves the runner "auto" resolves to ("" for the automatic
// pick). A runner that is uninstalled later stays saved: the auto pick answers
// for it until it is back.
func (s *Store) SetDefaultRunnerID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.DefaultRunnerID
	s.data.DefaultRunnerID = id
	if err := s.save(); err != nil {
		s.data.DefaultRunnerID = prev
		return err
	}
	return nil
}
