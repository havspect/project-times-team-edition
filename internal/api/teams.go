package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/havspect/project-times/internal/store"
)

// teamRoutes registers the team and membership endpoints.
func (s *Server) teamRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "teams-list",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/teams",
		Tags:        []string{"teams"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError},
	}, s.listTeams)

	huma.Register(api, huma.Operation{
		OperationID:   "teams-create",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/teams",
		Tags:          []string{"teams"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, s.createTeam)

	huma.Register(api, huma.Operation{
		OperationID: "teams-rename",
		Method:      http.MethodPatch,
		Path:        apiPrefix + "/teams/{id}",
		Tags:        []string{"teams"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, s.renameTeam)

	huma.Register(api, huma.Operation{
		OperationID:   "teams-archive",
		Method:        http.MethodDelete,
		Path:          apiPrefix + "/teams/{id}",
		Tags:          []string{"teams"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, s.archiveTeam)

	huma.Register(api, huma.Operation{
		OperationID: "teams-members-replace",
		Method:      http.MethodPut,
		Path:        apiPrefix + "/teams/{id}/members",
		Tags:        []string{"teams"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, s.replaceMembers)
}

type teamMemberResponse struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

type teamResponse struct {
	ID         uint                 `json:"id"`
	Name       string               `json:"name"`
	ArchivedAt *time.Time           `json:"archived_at"`
	Members    []teamMemberResponse `json:"members"`
}

type teamMemberInput struct {
	UserID uint   `json:"user_id" required:"true"`
	Role   string `json:"role" required:"true" enum:"lead,member"`
}

type teamNameInput struct {
	Body struct {
		Name string `json:"name" required:"true" minLength:"1"`
	}
}

type teamRenameInput struct {
	ID   uint `path:"id"`
	Body struct {
		Name string `json:"name" required:"true" minLength:"1"`
	}
}

type teamIDInput struct {
	ID uint `path:"id"`
}

type teamMembersInput struct {
	ID   uint `path:"id"`
	Body []teamMemberInput
}

type teamsOutput struct{ Body []teamResponse }

type teamOutput struct{ Body teamResponse }

type teamMembersOutput struct{ Body []teamMemberResponse }

func teamOf(t store.Team, members []store.Member) teamResponse {
	out := teamResponse{
		ID: t.ID, Name: t.Name, ArchivedAt: t.ArchivedAt,
		Members: []teamMemberResponse{},
	}
	for _, m := range members {
		out.Members = append(out.Members, teamMemberOf(m))
	}
	return out
}

func teamMemberOf(m store.Member) teamMemberResponse {
	return teamMemberResponse{UserID: m.UserID, Email: m.Email, Name: m.Name, Role: m.Role}
}

func (s *Server) listTeams(ctx context.Context, _ *struct{}) (*teamsOutput, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	teams, err := s.Store.ListTeams()
	if err != nil {
		return nil, internal(err)
	}
	all, err := s.Store.AllMembers()
	if err != nil {
		return nil, internal(err)
	}
	byTeam := make(map[uint][]store.Member, len(teams))
	for _, m := range all {
		byTeam[m.TeamID] = append(byTeam[m.TeamID], m)
	}
	out := make([]teamResponse, 0, len(teams))
	for _, t := range teams {
		out = append(out, teamOf(t, byTeam[t.ID]))
	}
	return &teamsOutput{Body: out}, nil
}

func (s *Server) createTeam(ctx context.Context, in *teamNameInput) (*teamOutput, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	team := &store.Team{Name: in.Body.Name}
	if err := s.Store.CreateTeam(team); err != nil {
		return nil, internal(err)
	}
	return &teamOutput{Body: teamOf(*team, nil)}, nil
}

func (s *Server) renameTeam(ctx context.Context, in *teamRenameInput) (*teamOutput, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	team, err := s.Store.RenameTeam(in.ID, in.Body.Name, s.Clock.Now())
	if errors.Is(err, store.ErrNotFound) {
		return nil, problem(http.StatusNotFound, urnNotFound, "team not found")
	}
	if err != nil {
		return nil, internal(err)
	}
	members, err := s.Store.MembersOf(team.ID)
	if err != nil {
		return nil, internal(err)
	}
	return &teamOutput{Body: teamOf(*team, members)}, nil
}

func (s *Server) archiveTeam(ctx context.Context, in *teamIDInput) (*struct{}, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	err := s.Store.ArchiveTeam(in.ID, s.Clock.Now())
	if errors.Is(err, store.ErrNotFound) {
		return nil, problem(http.StatusNotFound, urnNotFound, "team not found")
	}
	if err != nil {
		return nil, internal(err)
	}
	return nil, nil
}

func (s *Server) replaceMembers(ctx context.Context, in *teamMembersInput) (*teamMembersOutput, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	seen := make(map[uint]bool, len(in.Body))
	members := make([]store.MemberRole, 0, len(in.Body))
	for _, m := range in.Body {
		if seen[m.UserID] {
			return nil, problem(http.StatusUnprocessableEntity, urnInvalidInput, "the same user_id appears twice")
		}
		seen[m.UserID] = true
		members = append(members, store.MemberRole{UserID: m.UserID, Role: m.Role})
	}
	err := s.Store.ReplaceMemberships(in.ID, members, s.Clock.Now())
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, problem(http.StatusNotFound, urnNotFound, "team not found")
	case errors.Is(err, store.ErrUnknownUser):
		return nil, problem(http.StatusUnprocessableEntity, urnInvalidInput, "unknown user_id")
	case err != nil:
		return nil, internal(err)
	}
	rows, err := s.Store.MembersOf(in.ID)
	if err != nil {
		return nil, internal(err)
	}
	out := make([]teamMemberResponse, 0, len(rows))
	for _, m := range rows {
		out = append(out, teamMemberOf(m))
	}
	return &teamMembersOutput{Body: out}, nil
}
