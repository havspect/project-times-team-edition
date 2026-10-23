package entry

import (
	"errors"
	"time"

	"github.com/havspect/project-times/internal/clock"
	"github.com/havspect/project-times/internal/store"
)

// This file is the write side of entries: every mutation and the rules it must
// obey. Handlers decode, call one method and map the error; nothing here knows
// about HTTP or the database beyond the Store interface.

var (
	// ErrForbidden is returned for an entry that belongs to another user.
	ErrForbidden = errors.New("entry belongs to another user")
	// ErrUnknownProject is returned when an entry points at a project that does not exist.
	ErrUnknownProject = errors.New("unknown project")
	// ErrArchivedProject is returned when an entry points at an archived project.
	ErrArchivedProject = errors.New("project is archived")
	// ErrRetimeRunning is returned when a running entry's start moves without an end.
	ErrRetimeRunning = errors.New("a running entry needs an end_at to be retimed")
)

// Store is the persistence the service needs. *store.Store is the production
// adapter; tests use a map.
type Store interface {
	EntryByID(id uint) (*store.Entry, error)
	CreateEntry(e *store.Entry) error
	UpdateEntry(id uint, at time.Time, changes map[string]any) error
	SoftDeleteEntry(id uint, at time.Time) error
	RestoreEntry(id uint, at time.Time) error
	ProjectByID(id uint) (*store.Project, error)
}

// Service applies the entry rules to the store.
type Service struct {
	store Store
	clock clock.Clock
}

// New returns a Service writing through st, with "now" taken from clk.
func New(st Store, clk clock.Clock) *Service {
	return &Service{store: st, clock: clk}
}

// Create logs a finished entry for userID on projectID.
func (s *Service) Create(userID, projectID uint, start, end time.Time, note string) (*store.Entry, error) {
	if err := s.loggable(projectID); err != nil {
		return nil, err
	}
	now := s.clock.Now()
	if err := Validate(start, end, now); err != nil {
		return nil, err
	}
	e := &store.Entry{
		UserID: userID, ProjectID: projectID,
		StartAt: start, EndAt: &end, Note: note,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.store.CreateEntry(e); err != nil {
		return nil, err
	}
	return e, nil
}

// Patch changes the fields in in that are set, on an entry the user owns. It
// writes only those fields, so a stop that landed since the entry was read is
// not undone by a note edit.
type Patch struct {
	ProjectID *uint
	StartAt   *time.Time
	EndAt     *time.Time
	Note      *string
}

func (s *Service) Patch(userID, id uint, in Patch) (*store.Entry, error) {
	e, err := s.own(userID, id)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now()
	changes := map[string]any{}
	if in.ProjectID != nil && *in.ProjectID != e.ProjectID {
		if err := s.loggable(*in.ProjectID); err != nil {
			return nil, err
		}
		e.ProjectID = *in.ProjectID
		changes["project_id"] = e.ProjectID
	}
	if in.Note != nil {
		e.Note = *in.Note
		changes["note"] = e.Note
	}

	start := e.StartAt
	if in.StartAt != nil {
		start = *in.StartAt
	}
	var end time.Time
	switch {
	case in.EndAt != nil:
		end = *in.EndAt
	case e.EndAt != nil:
		end = *e.EndAt
	default:
		// A running entry keeps running until it is given an end time; that is
		// how a user answers entry-too-long without losing hours.
		if in.StartAt != nil {
			return nil, ErrRetimeRunning
		}
	}
	if !end.IsZero() && (in.StartAt != nil || in.EndAt != nil) {
		if err := Validate(start, end, now); err != nil {
			return nil, err
		}
		e.StartAt, e.EndAt = start, &end
		changes["start_at"], changes["end_at"] = start, end
	}
	if len(changes) > 0 {
		if err := s.store.UpdateEntry(e.ID, now, changes); err != nil {
			return nil, err
		}
	}
	return e, nil
}

// Delete soft-deletes an entry the user owns.
func (s *Service) Delete(userID, id uint) error {
	if _, err := s.own(userID, id); err != nil {
		return err
	}
	return s.store.SoftDeleteEntry(id, s.clock.Now())
}

// Restore brings a deleted entry back. The admin gate belongs to the caller.
func (s *Service) Restore(id uint) (*store.Entry, error) {
	if err := s.store.RestoreEntry(id, s.clock.Now()); err != nil {
		return nil, err
	}
	return s.store.EntryByID(id)
}

// own loads a live entry the user may change; a deleted one is not found.
func (s *Service) own(userID, id uint) (*store.Entry, error) {
	e, err := s.store.EntryByID(id)
	if err != nil {
		return nil, err
	}
	if e.DeletedAt != nil {
		return nil, store.ErrNotFound
	}
	if e.UserID != userID {
		return nil, ErrForbidden
	}
	return e, nil
}

// loggable checks the project an entry points at: unknown or archived is refused.
func (s *Service) loggable(projectID uint) error {
	p, err := s.store.ProjectByID(projectID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return ErrUnknownProject
	case err != nil:
		return err
	case p.ArchivedAt != nil:
		return ErrArchivedProject
	}
	return nil
}
