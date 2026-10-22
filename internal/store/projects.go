package store

// CreateProject inserts a project.
func (s *Store) CreateProject(p *Project) error {
	return s.db.Create(p).Error
}

// ProjectByID returns the project with id, archived or not; an unknown id returns ErrNotFound.
func (s *Store) ProjectByID(id uint) (*Project, error) {
	var p Project
	if err := s.db.First(&p, id).Error; err != nil {
		return nil, translate(err)
	}
	return &p, nil
}

// Projects lists projects ordered by name; archived ones only when includeArchived.
func (s *Store) Projects(includeArchived bool) ([]Project, error) {
	out := []Project{}
	q := s.db.Order("name")
	if !includeArchived {
		q = q.Where("archived_at IS NULL")
	}
	err := q.Find(&out).Error
	return out, err
}

// UpdateProject writes p's name, color, team and archive state; it never inserts.
func (s *Store) UpdateProject(p *Project) error {
	return s.db.Model(p).Select("Name", "Color", "TeamID", "ArchivedAt").Updates(p).Error
}

// TeamAcceptsProjects reports whether teamID exists and is not archived, the
// condition for creating a project there or moving one into it.
func (s *Store) TeamAcceptsProjects(teamID uint) (bool, error) {
	var n int64
	err := s.db.Model(&Team{}).Where("id = ? AND archived_at IS NULL", teamID).Count(&n).Error
	return n > 0, err
}
