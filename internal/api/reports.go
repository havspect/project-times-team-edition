package api

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/havspect/project-times/internal/report"
	"github.com/havspect/project-times/internal/store"
)

// reportRoutes registers the report and CSV endpoints.
func (s *Server) reportRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "reports-get",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/reports",
		Tags:        []string{"reports"},
		Errors:      []int{http.StatusUnauthorized, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, s.getReport)

	huma.Register(api, huma.Operation{
		OperationID: "reports-csv",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/reports.csv",
		Tags:        []string{"reports"},
		Errors:      []int{http.StatusUnauthorized, http.StatusUnprocessableEntity, http.StatusInternalServerError},
		Responses: map[string]*huma.Response{
			"200": {
				Description: "CSV export: one row per entry per local day.",
				Content: map[string]*huma.MediaType{
					"text/csv": {Schema: &huma.Schema{Type: "string"}},
				},
			},
		},
	}, s.getReportCSV)
}

type reportInput struct {
	From      string `query:"from" doc:"Inclusive start date, YYYY-MM-DD. Defaults to the start of the caller's ISO week."`
	To        string `query:"to" doc:"Exclusive end date, YYYY-MM-DD. Required with from."`
	By        string `query:"by" enum:"week,day,none" default:"week" doc:"Bucket: ISO week, subject-local day, or the whole range."`
	TeamID    uint   `query:"team_id" doc:"Only entries on this team's projects."`
	UserID    uint   `query:"user_id" doc:"Only this user's entries, within what the caller may read."`
	ProjectID uint   `query:"project_id" doc:"Only this project's entries."`
}

type reportRow struct {
	Bucket    string `json:"bucket"`
	UserID    uint   `json:"user_id"`
	User      string `json:"user"`
	Email     string `json:"email"`
	TeamID    uint   `json:"team_id"`
	Team      string `json:"team"`
	ProjectID uint   `json:"project_id"`
	Project   string `json:"project"`
	Archived  bool   `json:"archived"`
	Seconds   int64  `json:"seconds"`
}

type reportOutput struct {
	Body struct {
		Rows         []reportRow `json:"rows"`
		TotalSeconds int64       `json:"total_seconds"`
	}
}

func (s *Server) getReport(ctx context.Context, in *reportInput) (*reportOutput, error) {
	segments, err := s.reportSegments(ctx, in)
	if err != nil {
		return nil, err
	}
	rows, total := report.Aggregate(segments, in.By)
	out := &reportOutput{}
	out.Body.Rows = make([]reportRow, 0, len(rows))
	for _, r := range rows {
		out.Body.Rows = append(out.Body.Rows, reportRow{
			Bucket: r.Bucket, UserID: r.UserID, User: r.User, Email: r.Email,
			TeamID: r.TeamID, Team: r.Team, ProjectID: r.ProjectID, Project: r.Project,
			Archived: r.Archived, Seconds: r.Seconds,
		})
	}
	out.Body.TotalSeconds = total
	return out, nil
}

// reportCSVOutput is raw bytes so huma writes the CSV body untouched.
type reportCSVOutput struct {
	ContentType string `header:"Content-Type"`
	Body        []byte
}

func (s *Server) getReportCSV(ctx context.Context, in *reportInput) (*reportCSVOutput, error) {
	segments, err := s.reportSegments(ctx, in)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := report.WriteCSV(&buf, segments); err != nil {
		return nil, internal(err)
	}
	return &reportCSVOutput{ContentType: "text/csv; charset=utf-8", Body: buf.Bytes()}, nil
}

// reportSegments loads the caller's readable entries for the range and splits
// them into subject-local day segments, the input every report shares.
func (s *Server) reportSegments(ctx context.Context, in *reportInput) ([]report.Segment, error) {
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
	page, err := s.readEntries(scope, rng, entryFilter{
		UserID: in.UserID, ProjectID: in.ProjectID, TeamID: in.TeamID,
	})
	if err != nil {
		return nil, internal(err)
	}

	userIDs := make([]uint, 0, len(page.entries))
	projects := make([]store.Project, 0, len(page.projects))
	teamIDs := make([]uint, 0, len(page.projects))
	for _, e := range page.entries {
		userIDs = append(userIDs, e.UserID)
	}
	for _, p := range page.projects {
		projects = append(projects, p)
		teamIDs = append(teamIDs, p.TeamID)
	}
	users, err := s.Store.UsersByIDs(userIDs)
	if err != nil {
		return nil, internal(err)
	}
	teams, err := s.Store.TeamsByIDs(teamIDs)
	if err != nil {
		return nil, internal(err)
	}

	// A running entry counts up to now; only deleted ones are excluded.
	segments, err := report.Split(toReportInput(page.entries, users, projects, teams, s.Clock.Now()), rng.From, rng.To)
	if err != nil {
		return nil, internal(err)
	}
	return segments, nil
}

// toReportInput narrows the loaded store rows to the fields a report reads.
func toReportInput(entries []store.Entry, users []store.User, projects []store.Project, teams []store.Team, now time.Time) report.Input {
	in := report.Input{
		Entries:  make([]report.Entry, 0, len(entries)),
		Users:    make([]report.User, 0, len(users)),
		Projects: make([]report.Project, 0, len(projects)),
		Teams:    make([]report.Team, 0, len(teams)),
		Now:      now,
	}
	for _, e := range entries {
		in.Entries = append(in.Entries, report.Entry{
			ID: e.ID, UserID: e.UserID, ProjectID: e.ProjectID,
			StartAt: e.StartAt, EndAt: e.EndAt, Note: e.Note, DeletedAt: e.DeletedAt,
		})
	}
	for _, u := range users {
		in.Users = append(in.Users, report.User{
			ID: u.ID, Name: u.Name, Email: u.Email, Timezone: u.Timezone,
		})
	}
	for _, p := range projects {
		in.Projects = append(in.Projects, report.Project{
			ID: p.ID, TeamID: p.TeamID, Name: p.Name, ArchivedAt: p.ArchivedAt,
		})
	}
	for _, t := range teams {
		in.Teams = append(in.Teams, report.Team{ID: t.ID, Name: t.Name})
	}
	return in
}
