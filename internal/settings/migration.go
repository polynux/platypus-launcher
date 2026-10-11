package settings

// MigrationNoticeDismissed says whether the user dismissed the one-time
// Hydra→Platypus migration notice (issue #14). False until dismissed.
func (s *Store) MigrationNoticeDismissed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.MigrationNoticeDismissed
}

// SetMigrationNoticeDismissed persists the dismissal of the one-time
// Hydra→Platypus migration notice, so it never returns.
func (s *Store) SetMigrationNoticeDismissed(dismissed bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.data.MigrationNoticeDismissed
	s.data.MigrationNoticeDismissed = dismissed
	if err := s.save(); err != nil {
		s.data.MigrationNoticeDismissed = prev
		return err
	}
	return nil
}
