package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/havspect/tstamps/internal/client"
)

// resolveProject matches a reference by name, then by id. Names are compared
// case-insensitively; two projects with the same name need an id.
func (a *App) resolveProject(c Client, ref string) (*client.Project, error) {
	projects, err := c.ListProjects(false)
	if err != nil {
		return nil, err
	}
	for i := range projects {
		if strings.EqualFold(projects[i].Name, ref) {
			return &projects[i], nil
		}
	}
	if id, err := strconv.ParseUint(ref, 10, 64); err == nil {
		for i := range projects {
			if uint64(projects[i].ID) == id {
				return &projects[i], nil
			}
		}
	}
	return nil, fmt.Errorf("unknown project %q", ref)
}

// resolveTeam matches a team the caller belongs to by name first, so a lead
// needs no admin rights, then any team by name as admin, then an id.
func (a *App) resolveTeam(c Client, me *client.Me, ref string) (uint, error) {
	for _, m := range me.Memberships {
		if strings.EqualFold(m.Team, ref) {
			return m.TeamID, nil
		}
	}
	if me.IsAdmin {
		teams, err := c.ListTeams()
		if err != nil {
			return 0, err
		}
		for _, t := range teams {
			if strings.EqualFold(t.Name, ref) {
				return t.ID, nil
			}
		}
	}
	if id, err := strconv.ParseUint(ref, 10, 64); err == nil {
		return uint(id), nil
	}
	return 0, fmt.Errorf("unknown team %q", ref)
}

// resolveUser matches me, an id, or an email. Listing users is admin-only, so a
// non-admin can only name themselves by email; numeric ids still work and the
// server enforces the read scope.
func (a *App) resolveUser(c Client, ref string) (uint, error) {
	if strings.EqualFold(ref, "me") {
		me, err := c.Me()
		if err != nil {
			return 0, err
		}
		return me.ID, nil
	}
	if id, err := strconv.ParseUint(ref, 10, 64); err == nil {
		return uint(id), nil
	}
	users, err := c.ListUsers()
	if err != nil {
		return 0, fmt.Errorf("look up %q: %w (email lookup needs an admin)", ref, err)
	}
	for _, u := range users {
		if strings.EqualFold(u.Email, ref) {
			return u.ID, nil
		}
	}
	return 0, fmt.Errorf("unknown user %q", ref)
}

// projectNames indexes projects for display. Archived projects are in the index
// for an admin only; anyone else sees the id for one.
func (a *App) projectNames(c Client, me *client.Me) map[uint]string {
	projects, err := c.ListProjects(me.IsAdmin)
	if err != nil {
		return nil
	}
	out := make(map[uint]string, len(projects))
	for _, p := range projects {
		out[p.ID] = p.Name
	}
	return out
}

// projectName falls back to the id when the name is unknown.
func projectName(names map[uint]string, id uint) string {
	if name, ok := names[id]; ok {
		return name
	}
	return fmt.Sprintf("project #%d", id)
}

// teamName is the display twin of projectName.
func teamName(names map[uint]string, id uint) string {
	if name, ok := names[id]; ok {
		return name
	}
	return fmt.Sprintf("team #%d", id)
}

// teamNames indexes the teams the caller can name: their memberships, plus
// every team for an admin.
func (a *App) teamNames(c Client, me *client.Me) map[uint]string {
	out := make(map[uint]string, len(me.Memberships))
	for _, m := range me.Memberships {
		out[m.TeamID] = m.Team
	}
	if me.IsAdmin {
		if teams, err := c.ListTeams(); err == nil {
			for _, t := range teams {
				out[t.ID] = t.Name
			}
		}
	}
	return out
}

// parseID reads a positive numeric argument.
func parseID(value string) (uint, error) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid id %q", value)
	}
	return uint(id), nil
}

// alreadyRunning reports whether err is the 409 a second timer start returns, so
// the command can name the entry instead of repeating the server prose.
func alreadyRunning(err error) (*client.Entry, bool) {
	var apiErr *client.Error
	if !errors.As(err, &apiErr) || apiErr.Type != "urn:tstamps:error:timer-already-running" {
		return nil, false
	}
	return apiErr.RunningEntry, true
}
