package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/havspect/project-times/internal/entry"
	"github.com/havspect/project-times/internal/store"
)

// entryResponse is an entry as every entry and timer reply shows it. Project and
// user names are deliberately absent: the client already has them, and copying
// them here would go stale.
type entryResponse struct {
	ID        uint       `json:"id"`
	UserID    uint       `json:"user_id"`
	ProjectID uint       `json:"project_id"`
	StartAt   time.Time  `json:"start_at"`
	EndAt     *time.Time `json:"end_at"`
	Note      string     `json:"note"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

func entryResponseOf(e *store.Entry) entryResponse {
	return entryResponse{
		ID: e.ID, UserID: e.UserID, ProjectID: e.ProjectID,
		StartAt: e.StartAt, EndAt: e.EndAt, Note: e.Note, DeletedAt: e.DeletedAt,
	}
}

// entryRoutes registers the entry endpoints.
func (s *Server) entryRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "entries-list",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/entries",
		Tags:        []string{"entries"},
		Errors:      []int{http.StatusUnauthorized, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, s.listEntries)

	huma.Register(api, huma.Operation{
		OperationID:   "entries-create",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/entries",
		Tags:          []string{"entries"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnauthorized, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, s.createEntry)

	huma.Register(api, huma.Operation{
		OperationID: "entries-update",
		Method:      http.MethodPatch,
		Path:        apiPrefix + "/entries/{id}",
		Tags:        []string{"entries"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, s.patchEntry)

	huma.Register(api, huma.Operation{
		OperationID:   "entries-delete",
		Method:        http.MethodDelete,
		Path:          apiPrefix + "/entries/{id}",
		Tags:          []string{"entries"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}, s.deleteEntry)

	huma.Register(api, huma.Operation{
		OperationID: "entries-restore",
		Method:      http.MethodPost,
		Path:        apiPrefix + "/entries/{id}/restore",
		Tags:        []string{"entries"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}, s.restoreEntry)
}

// requireLoggableProject checks the project a new or moved entry points at:
// unknown is a 422, and an archived project takes no new entries.
func (s *Server) requireLoggableProject(id uint) error {
	p, err := s.Store.ProjectByID(id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return problem(http.StatusUnprocessableEntity, urnInvalidInput, "unknown project")
	case err != nil:
		return internal(err)
	case p.ArchivedAt != nil:
		return problem(http.StatusUnprocessableEntity, urnInvalidInput, "project is archived")
	}
	return nil
}

// entryRuleProblem turns an entry-rule error into the reply the spec names.
func entryRuleProblem(err error) *huma.ErrorModel {
	switch {
	case errors.Is(err, entry.ErrTooLong):
		return problem(http.StatusUnprocessableEntity, urnEntryTooLong, "an entry may not be longer than 24 hours")
	case errors.Is(err, entry.ErrFuture):
		return problem(http.StatusUnprocessableEntity, urnInvalidInput, "an entry may not be more than 5 minutes in the future")
	default:
		return problem(http.StatusUnprocessableEntity, urnInvalidInput, "end must be after start")
	}
}

type entryListInput struct {
	From      string `query:"from" doc:"Inclusive start date, YYYY-MM-DD. Defaults to the start of the caller's ISO week."`
	To        string `query:"to" doc:"Exclusive end date, YYYY-MM-DD. Required with from."`
	UserID    uint   `query:"user_id" doc:"Only this user's entries, within what the caller may read."`
	ProjectID uint   `query:"project_id" doc:"Only this project's entries."`
}

type entryListOutput struct{ Body []entryResponse }

func (s *Server) listEntries(ctx context.Context, in *entryListInput) (*entryListOutput, error) {
	u, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	scope, err := s.entryScope(u)
	if err != nil {
		return nil, internal(err)
	}
	rng, err := parseRange(u.Timezone, s.Clock.Now(), in.From, in.To)
	if err != nil {
		return nil, err
	}
	page, err := s.readEntries(scope, rng, entryFilter{UserID: in.UserID, ProjectID: in.ProjectID})
	if err != nil {
		return nil, internal(err)
	}
	out := make([]entryResponse, 0, len(page.entries))
	for i := range page.entries {
		out = append(out, entryResponseOf(&page.entries[i]))
	}
	return &entryListOutput{Body: out}, nil
}

type entryCreateInput struct {
	Body struct {
		ProjectID uint      `json:"project_id" required:"true" minimum:"1"`
		StartAt   time.Time `json:"start_at" required:"true"`
		EndAt     time.Time `json:"end_at" required:"true"`
		Note      string    `json:"note,omitempty"`
	}
}

type entryOutput struct{ Body entryResponse }

func (s *Server) createEntry(ctx context.Context, in *entryCreateInput) (*entryOutput, error) {
	u, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.requireLoggableProject(in.Body.ProjectID); err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	if err := entry.Validate(in.Body.StartAt, in.Body.EndAt, now); err != nil {
		return nil, entryRuleProblem(err)
	}
	e := &store.Entry{
		UserID: u.ID, ProjectID: in.Body.ProjectID,
		StartAt: in.Body.StartAt, EndAt: &in.Body.EndAt, Note: in.Body.Note,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.Store.CreateEntry(e); err != nil {
		return nil, fail(err)
	}
	return &entryOutput{Body: entryResponseOf(e)}, nil
}

type entryPatchInput struct {
	ID   uint `path:"id"`
	Body struct {
		ProjectID *uint      `json:"project_id,omitempty" minimum:"1"`
		StartAt   *time.Time `json:"start_at,omitempty"`
		EndAt     *time.Time `json:"end_at,omitempty"`
		Note      *string    `json:"note,omitempty"`
	}
}

func (s *Server) patchEntry(ctx context.Context, in *entryPatchInput) (*entryOutput, error) {
	u, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	e, err := s.Store.EntryByID(in.ID)
	if err != nil {
		return nil, fail(err)
	}
	if e.DeletedAt != nil {
		return nil, problem(http.StatusNotFound, urnNotFound, "not found")
	}
	if e.UserID != u.ID {
		return nil, problem(http.StatusForbidden, urnForbidden, "you can only edit your own entries")
	}
	changes := map[string]any{}
	if in.Body.ProjectID != nil && *in.Body.ProjectID != e.ProjectID {
		if err := s.requireLoggableProject(*in.Body.ProjectID); err != nil {
			return nil, err
		}
		e.ProjectID = *in.Body.ProjectID
		changes["project_id"] = e.ProjectID
	}
	if in.Body.Note != nil {
		e.Note = *in.Body.Note
		changes["note"] = e.Note
	}

	start := e.StartAt
	if in.Body.StartAt != nil {
		start = *in.Body.StartAt
	}
	var end time.Time
	switch {
	case in.Body.EndAt != nil:
		end = *in.Body.EndAt
	case e.EndAt != nil:
		end = *e.EndAt
	default:
		// A running entry keeps running until it is given an end time; that is
		// how a user answers entry-too-long without losing hours.
		if in.Body.StartAt != nil {
			return nil, problem(http.StatusUnprocessableEntity, urnInvalidInput, "a running entry needs an end_at to be retimed")
		}
	}
	if !end.IsZero() && (in.Body.StartAt != nil || in.Body.EndAt != nil) {
		if err := entry.Validate(start, end, s.Clock.Now()); err != nil {
			return nil, entryRuleProblem(err)
		}
		e.StartAt, e.EndAt = start, &end
		changes["start_at"], changes["end_at"] = start, end
	}
	if len(changes) > 0 {
		if err := s.Store.UpdateEntry(e.ID, s.Clock.Now(), changes); err != nil {
			return nil, internal(err)
		}
	}
	return &entryOutput{Body: entryResponseOf(e)}, nil
}

type entryDeleteInput struct {
	ID uint `path:"id"`
}

func (s *Server) deleteEntry(ctx context.Context, in *entryDeleteInput) (*struct{}, error) {
	u, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	e, err := s.Store.EntryByID(in.ID)
	if err != nil {
		return nil, fail(err)
	}
	if e.DeletedAt != nil {
		return nil, problem(http.StatusNotFound, urnNotFound, "not found")
	}
	if e.UserID != u.ID {
		return nil, problem(http.StatusForbidden, urnForbidden, "you can only delete your own entries")
	}
	if err := s.Store.SoftDeleteEntry(in.ID, s.Clock.Now()); err != nil {
		return nil, fail(err)
	}
	return &struct{}{}, nil
}

type entryRestoreInput struct {
	ID uint `path:"id"`
}

func (s *Server) restoreEntry(ctx context.Context, in *entryRestoreInput) (*entryOutput, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := s.Store.RestoreEntry(in.ID, s.Clock.Now()); err != nil {
		return nil, fail(err)
	}
	e, err := s.Store.EntryByID(in.ID)
	if err != nil {
		return nil, fail(err)
	}
	return &entryOutput{Body: entryResponseOf(e)}, nil
}
