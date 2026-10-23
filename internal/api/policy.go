package api

import (
	"context"
	"net/http"

	"github.com/havspect/project-times/internal/store"
)

// The permission vocabulary. Handlers ask these questions instead of re-deriving
// "lead of the team that owns the project" in each one.

// requireUser returns the authenticated user, or 401.
func requireUser(ctx context.Context) (*store.User, error) {
	if u := principal(ctx); u != nil {
		return u, nil
	}
	return nil, problem(http.StatusUnauthorized, urnUnauthorized, "authentication required")
}

// requireAdmin gates users, teams, memberships and restores.
func requireAdmin(ctx context.Context) (*store.User, error) {
	u, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if !u.IsAdmin {
		return nil, problem(http.StatusForbidden, urnForbidden, "admin only")
	}
	return u, nil
}

// canManageTeam reports whether u may create and edit projects of teamID: an
// admin always, a lead of an unarchived team.
func (s *Server) canManageTeam(u *store.User, teamID uint) (bool, error) {
	if u.IsAdmin {
		return true, nil
	}
	return s.Store.LeadsTeam(u.ID, teamID)
}

// requireTeamManager is canManageTeam as a handler guard: 401, then 403.
func (s *Server) requireTeamManager(ctx context.Context, teamID uint) (*store.User, error) {
	u, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	ok, err := s.canManageTeam(u, teamID)
	if err != nil {
		return nil, internal(err)
	}
	if !ok {
		return nil, problem(http.StatusForbidden, urnForbidden, "you do not lead this team")
	}
	return u, nil
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
