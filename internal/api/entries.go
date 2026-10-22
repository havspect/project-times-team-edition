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

// entryScope is what the caller may read: everything for an admin, own entries
// plus the projects they lead for a lead, own entries for everyone else.
func (s *Server) entryScope(u *store.User) (store.Scope, error) {
	if u.IsAdmin {
		return store.Scope{All: true}, nil
	}
	ids, err := s.Store.ProjectIDsLedBy(u.ID)
	if err != nil {
		return store.Scope{}, err
	}
	return store.Scope{UserID: u.ID, ProjectIDs: ids}, nil
}

// dateRange is a query range: from inclusive, to exclusive, both plain dates.
type dateRange struct{ From, To time.Time }

// padded widens the range by 14h either side, the widest offset a timezone can
// be from UTC, so an entry crossing a local midnight is fetched before it is
// split in Go.
func (r dateRange) padded() (time.Time, time.Time) {
	return r.From.Add(-14 * time.Hour), r.To.Add(14 * time.Hour)
}

// parseRange reads from/to as plain dates, defaulting to the caller's current
// ISO week. An empty one of the pair is a 422, never half a range.
func (s *Server) parseRange(u *store.User, from, to string) (dateRange, error) {
	const layout = "2006-01-02"
	if from == "" && to == "" {
		loc, err := time.LoadLocation(u.Timezone)
		if err != nil {
			return dateRange{}, internal(err)
		}
		local := s.Clock.Now().In(loc)
		monday := time.Date(local.Year(), local.Month(), local.Day()-((int(local.Weekday())+6)%7), 0, 0, 0, 0, loc)
		// from/to are plain dates, so take the local calendar dates, not the
		// instants: a Monday 00:00 in UTC+14 is still Sunday in UTC.
		y, m, d := monday.Date()
		ny, nm, nd := monday.AddDate(0, 0, 7).Date()
		return dateRange{
			time.Date(y, m, d, 0, 0, 0, 0, time.UTC),
			time.Date(ny, nm, nd, 0, 0, 0, 0, time.UTC),
		}, nil
	}
	if from == "" || to == "" {
		return dateRange{}, problem(http.StatusUnprocessableEntity, urnInvalidInput, "from and to must be given together")
	}
	f, err := time.Parse(layout, from)
	if err != nil {
		return dateRange{}, problem(http.StatusUnprocessableEntity, urnInvalidInput, "from must be a YYYY-MM-DD date")
	}
	t, err := time.Parse(layout, to)
	if err != nil {
		return dateRange{}, problem(http.StatusUnprocessableEntity, urnInvalidInput, "to must be a YYYY-MM-DD date")
	}
	if !t.After(f) {
		return dateRange{}, problem(http.StatusUnprocessableEntity, urnInvalidInput, "to must be after from")
	}
	if t.Sub(f) > 366*24*time.Hour {
		return dateRange{}, problem(http.StatusUnprocessableEntity, urnInvalidInput, "the range may not be longer than 366 days")
	}
	return dateRange{f, t}, nil
}

// entryFilter narrows a listing or report on top of the caller's scope.
type entryFilter struct {
	UserID    uint
	ProjectID uint
	TeamID    uint
}

// matching keeps only the entries the filter names; 0 means "no filter". Team
// membership comes from the project's team, so it is applied here, not in SQL.
func (f entryFilter) matching(entries []store.Entry, projects map[uint]store.Project) []store.Entry {
	out := make([]store.Entry, 0, len(entries))
	for _, e := range entries {
		if f.UserID != 0 && e.UserID != f.UserID {
			continue
		}
		if f.ProjectID != 0 && e.ProjectID != f.ProjectID {
			continue
		}
		if f.TeamID != 0 && projects[e.ProjectID].TeamID != f.TeamID {
			continue
		}
		out = append(out, e)
	}
	return out
}

// loadEntries fetches the caller's readable entries for rng, already narrowed by
// filter. It also returns the projects the entries reference, which the report
// and the team filter both need.
func (s *Server) loadEntries(scope store.Scope, rng dateRange, filter entryFilter) ([]store.Entry, map[uint]store.Project, error) {
	from, to := rng.padded()
	entries, err := s.Store.EntriesInRange(from, to, scope)
	if err != nil {
		return nil, nil, err
	}
	ids := make([]uint, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.ProjectID)
	}
	projects, err := s.Store.ProjectsByIDs(ids)
	if err != nil {
		return nil, nil, err
	}
	byID := make(map[uint]store.Project, len(projects))
	for _, p := range projects {
		byID[p.ID] = p
	}
	return filter.matching(entries, byID), byID, nil
}

// requireLoggableProject resolves the project a new or moved entry points at:
// unknown is a 422, and an archived project takes no new entries.
func (s *Server) requireLoggableProject(id uint) (*store.Project, error) {
	p, err := s.Store.ProjectByID(id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, problem(http.StatusUnprocessableEntity, urnInvalidInput, "unknown project")
	case err != nil:
		return nil, internal(err)
	case p.ArchivedAt != nil:
		return nil, problem(http.StatusUnprocessableEntity, urnInvalidInput, "project is archived")
	}
	return p, nil
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
	To        string `query:"to" doc:"Exclusive end date, YYYY-MM-DD. Defaults to a week after from."`
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
	rng, err := s.parseRange(u, in.From, in.To)
	if err != nil {
		return nil, err
	}
	entries, _, err := s.loadEntries(scope, rng, entryFilter{UserID: in.UserID, ProjectID: in.ProjectID})
	if err != nil {
		return nil, internal(err)
	}
	out := make([]entryResponse, 0, len(entries))
	for i := range entries {
		out = append(out, entryResponseOf(&entries[i]))
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
	if _, err := s.requireLoggableProject(in.Body.ProjectID); err != nil {
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
	if in.Body.ProjectID != nil && *in.Body.ProjectID != e.ProjectID {
		if _, err := s.requireLoggableProject(*in.Body.ProjectID); err != nil {
			return nil, err
		}
		e.ProjectID = *in.Body.ProjectID
	}
	if in.Body.Note != nil {
		e.Note = *in.Body.Note
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
	if !end.IsZero() {
		if err := entry.Validate(start, end, s.Clock.Now()); err != nil {
			return nil, entryRuleProblem(err)
		}
		e.StartAt, e.EndAt = start, &end
	}
	e.UpdatedAt = s.Clock.Now()
	if err := s.Store.UpdateEntry(e); err != nil {
		return nil, internal(err)
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
