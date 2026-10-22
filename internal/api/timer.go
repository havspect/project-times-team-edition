package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/havspect/project-times/internal/entry"
	"github.com/havspect/project-times/internal/store"
)

// timerConflict is the 409 a second start gets: the RFC 9457 problem fields plus
// the entry that is already running, so a client can offer to stop it.
type timerConflict struct {
	huma.ErrorModel
	RunningEntry entryResponse `json:"running_entry"`
}

// timerRoutes registers the running-entry endpoints.
func (s *Server) timerRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "timer-get",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/timer",
		Tags:        []string{"timer"},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}, s.getTimer)

	huma.Register(api, huma.Operation{
		OperationID:   "timer-start",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/timer/start",
		Tags:          []string{"timer"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnauthorized, http.StatusConflict, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, s.startTimer)

	huma.Register(api, huma.Operation{
		OperationID: "timer-stop",
		Method:      http.MethodPost,
		Path:        apiPrefix + "/timer/stop",
		Tags:        []string{"timer"},
		Errors:      []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, s.stopTimer)
}

// timerOutput carries the running entry, or null when none runs.
type timerOutput struct{ Body *entryResponse }

func (s *Server) getTimer(ctx context.Context, _ *struct{}) (*timerOutput, error) {
	u, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	running, err := s.Store.RunningEntry(u.ID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return &timerOutput{}, nil
	case err != nil:
		return nil, internal(err)
	}
	body := entryResponseOf(running)
	return &timerOutput{Body: &body}, nil
}

type timerStartInput struct {
	Body struct {
		ProjectID uint   `json:"project_id" required:"true" minimum:"1"`
		Note      string `json:"note,omitempty"`
	}
}

func (s *Server) startTimer(ctx context.Context, in *timerStartInput) (*entryOutput, error) {
	u, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireLoggableProject(in.Body.ProjectID); err != nil {
		return nil, err
	}
	if conflict, err := s.runningConflict(u); err != nil {
		return nil, err
	} else if conflict != nil {
		return nil, conflict
	}
	now := s.Clock.Now()
	e := &store.Entry{
		UserID: u.ID, ProjectID: in.Body.ProjectID,
		StartAt: now, Note: in.Body.Note, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.Store.CreateEntry(e); err != nil {
		if errors.Is(err, store.ErrEntryRunning) {
			// The index caught a start that raced this one; report it the same way.
			if conflict, cErr := s.runningConflict(u); cErr == nil && conflict != nil {
				return nil, conflict
			}
			return nil, problem(http.StatusConflict, urnTimerAlreadyRunning, "you already have a running entry")
		}
		return nil, fail(err)
	}
	return &entryOutput{Body: entryResponseOf(e)}, nil
}

type timerStopInput struct {
	Body *struct {
		Note string `json:"note,omitempty"`
	}
}

func (s *Server) stopTimer(ctx context.Context, in *timerStopInput) (*entryOutput, error) {
	u, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	note := ""
	if in.Body != nil {
		note = in.Body.Note
	}
	running, err := s.Store.RunningEntry(u.ID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, problem(http.StatusNotFound, urnNotFound, "no running entry")
	case err != nil:
		return nil, internal(err)
	}
	now := s.Clock.Now()
	if err := entry.ValidateStop(running.StartAt, now, now); err != nil {
		return nil, entryRuleProblem(err)
	}
	if err := s.Store.StopRunning(u.ID, now, note); err != nil {
		return nil, fail(err)
	}
	running.EndAt = &now
	if note != "" {
		running.Note = note
	}
	return &entryOutput{Body: entryResponseOf(running)}, nil
}

// runningConflict returns the 409 for a user who is already timing, or nil when
// the timer is free. A store failure is returned as the second value.
func (s *Server) runningConflict(u *store.User) (*timerConflict, error) {
	running, err := s.Store.RunningEntry(u.ID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, nil
	case err != nil:
		return nil, internal(err)
	}
	return &timerConflict{
		ErrorModel: huma.ErrorModel{
			Type: urnTimerAlreadyRunning, Title: http.StatusText(http.StatusConflict),
			Status: http.StatusConflict, Detail: "you already have a running entry",
		},
		RunningEntry: entryResponseOf(running),
	}, nil
}
