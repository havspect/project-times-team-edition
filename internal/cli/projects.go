package cli

import (
	"fmt"

	"github.com/havspect/project-times/internal/client"
)

func (a *App) projects(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "add":
			return a.projectsAdd(args[1:])
		case "archive":
			return a.projectsArchive(args[1:])
		}
	}

	fs := newFlags("projects")
	urlFlag, tokenFlag := clientFlags(fs)
	all := fs.Bool("all", false, "include archived projects (admin only)")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("projects", err)
	}
	if code, ok := a.checkNoArgs("projects", fs); !ok {
		return code
	}

	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("projects", err)
	}
	me, err := c.Me()
	if err != nil {
		return a.fail("projects", err)
	}
	projects, err := c.ListProjects(*all)
	if err != nil {
		return a.fail("projects", err)
	}
	if *asJSON {
		return a.writeJSON(projects)
	}

	teams := a.teamNames(c, me)
	w := newTable(a.Out)
	fmt.Fprintln(w, "ID\tNAME\tTEAM\tSTATUS")
	for _, p := range projects {
		status := "active"
		if p.ArchivedAt != nil {
			status = "archived"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", p.ID, p.Name, teamName(teams, p.TeamID), status)
	}
	_ = w.Flush()
	return 0
}

func (a *App) projectsAdd(args []string) int {
	fs := newFlags("projects add")
	urlFlag, tokenFlag := clientFlags(fs)
	team := fs.String("team", "", "team name or id")
	color := fs.String("color", "", "color, e.g. #4f46e5")
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("projects add", err)
	}
	if fs.NArg() != 1 || *team == "" {
		return a.usageLine("times projects add <name> --team <team> [--color #rrggbb]")
	}
	name := fs.Arg(0)

	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("projects add", err)
	}
	me, err := c.Me()
	if err != nil {
		return a.fail("projects add", err)
	}
	teamID, err := a.resolveTeam(c, me, *team)
	if err != nil {
		return a.fail("projects add", err)
	}
	p, err := c.CreateProject(client.ProjectInput{TeamID: teamID, Name: name, Color: *color})
	if err != nil {
		return a.fail("projects add", err)
	}
	fmt.Fprintf(a.Out, "created project %d: %s (%s)\n", p.ID, p.Name, teamName(a.teamNames(c, me), p.TeamID))
	return 0
}

func (a *App) projectsArchive(args []string) int {
	fs := newFlags("projects archive")
	urlFlag, tokenFlag := clientFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("projects archive", err)
	}
	if fs.NArg() != 1 {
		return a.usageLine("times projects archive <id>")
	}
	id, err := parseID(fs.Arg(0))
	if err != nil {
		return a.fail("projects archive", err)
	}

	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("projects archive", err)
	}
	archived := true
	p, err := c.PatchProject(id, client.ProjectPatch{Archived: &archived})
	if err != nil {
		return a.fail("projects archive", err)
	}
	fmt.Fprintf(a.Out, "archived project %d: %s\n", p.ID, p.Name)
	return 0
}
