package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/havspect/project-times/internal/client"
)

func (a *App) teams(args []string) int {
	if len(args) > 0 && args[0] == "members" {
		return a.teamsMembers(args[1:])
	}

	fs := newFlags("teams")
	urlFlag, tokenFlag := clientFlags(fs)
	asJSON := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("teams", err)
	}
	if code, ok := a.checkNoArgs("teams", fs); !ok {
		return code
	}

	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("teams", err)
	}
	teams, err := c.ListTeams()
	if err != nil {
		return a.fail("teams", err)
	}
	if *asJSON {
		return a.writeJSON(teams)
	}

	w := newTable(a.Out)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tMEMBERS")
	for _, t := range teams {
		status := "active"
		if t.ArchivedAt != nil {
			status = "archived"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%d\n", t.ID, t.Name, status, len(t.Members))
	}
	_ = w.Flush()
	return 0
}

func (a *App) teamsMembers(args []string) int {
	fs := newFlags("teams members")
	urlFlag, tokenFlag := clientFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("teams members", err)
	}
	if fs.NArg() != 1 {
		return a.usageLine("times teams members <team>")
	}
	ref := fs.Arg(0)

	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("teams members", err)
	}
	teams, err := c.ListTeams()
	if err != nil {
		return a.fail("teams members", err)
	}
	team, err := findTeam(teams, ref)
	if err != nil {
		return a.fail("teams members", err)
	}

	w := newTable(a.Out)
	fmt.Fprintf(w, "%s\t%s\n", "USER", "ROLE")
	for _, m := range team.Members {
		fmt.Fprintf(w, "%s <%s>\t%s\n", m.Name, m.Email, m.Role)
	}
	_ = w.Flush()
	return 0
}

// findTeam matches a team by name, then by id.
func findTeam(teams []client.Team, ref string) (*client.Team, error) {
	for i := range teams {
		if strings.EqualFold(teams[i].Name, ref) {
			return &teams[i], nil
		}
	}
	if id, err := strconv.ParseUint(ref, 10, 64); err == nil {
		for i := range teams {
			if uint64(teams[i].ID) == id {
				return &teams[i], nil
			}
		}
	}
	return nil, fmt.Errorf("unknown team %q", ref)
}
