package api

import (
	"context"
	"errors"
	"net/http"
	"reflect"

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
	// The body is an entry or null, but Huma cannot mark an object ref nullable
	// (it panics), so the 200 schema spells the choice out as anyOf.
	entryOrNull := &huma.Schema{AnyOf: []*huma.Schema{
		api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[entryResponse](), true, "TimerEntry"),
		{Type: "null"},
	}}
	huma.Register(api, huma.Operation{
		OperationID: "timer-get",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/timer",
		Tags:        []string{"timer"},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
		Responses: map[string]*huma.Response{
			"200": {
				Content: map[string]*huma.MediaType{
					"application/json": {Schema: entryOrNull},
				},
			},
		},
	}, s.getTimer)

	// The 409 carries the entry that is already running, so a client can offer
	// to stop it; Errors would only describe the plain problem body.
	conflictSchema := api.OpenAPI().Components.Schemas.Schema(reflect.TypeFor[timerConflict](), true, "TimerConflict")
	huma.Register(api, huma.Operation{
		OperationID:   "timer-start",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/timer/start",
		Tags:          []string{"timer"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnauthorized, http.StatusUnprocessableEntity, http.StatusInternalServerError},
		Responses: map[string]*huma.Response{
			"409": {
				Description: "A timer is already running; the body names it.",
				Content: map[string]*huma.MediaType{
					"application/problem+json": {Schema: conflictSchema},
				},
			},
		},
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
	running, err := s.entries.Running(u.ID)
	if err != nil {
		return nil, internal(err)
	}
	if running == nil {
		return &timerOutput{}, nil
	}
	body := entryResponseOf(running)
	return &timerOutput{Body: &body}, nil
}

type timerStartInput struct {
	Body struct {
		ProjectID uint   `json:"project_id" required:"true" minimum:"1"`
		Timezone  string `json:"timezone" required:"true" minLength:"1" doc:"IANA zone the entry is started in, e.g. Europe/Berlin."`
		Note      string `json:"note,omitempty"`
	}
}

func (s *Server) startTimer(ctx context.Context, in *timerStartInput) (*entryOutput, error) {
	u, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateTimezone(in.Body.Timezone); err != nil {
		return nil, err
	}
	e, err := s.entries.Start(u.ID, in.Body.ProjectID, in.Body.Timezone, in.Body.Note)
	if err != nil {
		var running *entry.AlreadyRunning
		if errors.As(err, &running) {
			return nil, &timerConflict{
				ErrorModel:   *problem(http.StatusConflict, urnTimerAlreadyRunning, "you already have a running entry"),
				RunningEntry: entryResponseOf(running.Entry),
			}
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
	e, err := s.entries.Stop(u.ID, note)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, problem(http.StatusNotFound, urnNotFound, "no running entry")
		}
		return nil, fail(err)
	}
	return &entryOutput{Body: entryResponseOf(e)}, nil
}
