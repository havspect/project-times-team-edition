package store

// CreateProject inserts a project.
func (s *Store) CreateProject(p *Project) error {
	return s.db.Create(p).Error
}
