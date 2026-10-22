package store

// CreateTeam inserts a team.
func (s *Store) CreateTeam(t *Team) error {
	return s.db.Create(t).Error
}

// CreateMembership links a user to a team with role lead or member.
func (s *Store) CreateMembership(m *TeamMembership) error {
	return s.db.Create(m).Error
}
