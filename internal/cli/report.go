package cli

import (
	"fmt"

	"github.com/havspect/project-times/internal/client"
)

func (a *App) report(args []string) int {
	fs := newFlags("report")
	urlFlag, tokenFlag := clientFlags(fs)
	from := fs.String("from", "", "inclusive start date, YYYY-MM-DD")
	to := fs.String("to", "", "exclusive end date, YYYY-MM-DD")
	by := fs.String("by", "week", "bucket: week, day, month or none")
	team := fs.String("team", "", "team name or id")
	project := fs.String("project", "", "project name or id")
	user := fs.String("user", "", "me, an id, or an email (email needs an admin)")
	asJSON := fs.Bool("json", false, "print JSON")
	asCSV := fs.Bool("csv", false, "print the CSV detail export")
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("report", err)
	}
	if code, ok := a.checkNoArgs("report", fs); !ok {
		return code
	}
	if *asJSON && *asCSV {
		return a.fail("report", fmt.Errorf("--json and --csv are mutually exclusive"))
	}
	switch *by {
	case "week", "day", "month", "none":
	default:
		return a.fail("report", fmt.Errorf("invalid --by %q; use week, day, month or none", *by))
	}

	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("report", err)
	}
	q := client.ReportQuery{From: *from, To: *to, By: *by}
	if *user != "" {
		if q.UserID, err = a.resolveUser(c, *user); err != nil {
			return a.fail("report", err)
		}
	}
	if *project != "" {
		p, err := a.resolveProject(c, *project)
		if err != nil {
			return a.fail("report", err)
		}
		q.ProjectID = p.ID
	}
	if *team != "" {
		me, err := c.Me()
		if err != nil {
			return a.fail("report", err)
		}
		if q.TeamID, err = a.resolveTeam(c, me, *team); err != nil {
			return a.fail("report", err)
		}
	}

	if *asCSV {
		body, err := c.ReportCSV(q)
		if err != nil {
			return a.fail("report", err)
		}
		if _, err := a.Out.Write(body); err != nil {
			return a.fail("report", err)
		}
		return 0
	}

	r, err := c.Report(q)
	if err != nil {
		return a.fail("report", err)
	}
	if *asJSON {
		return a.writeJSON(r)
	}
	a.printReport(r)
	return 0
}

// printReport writes the totals table with one row per bucket, person and
// project, then the grand total.
func (a *App) printReport(r *client.Report) {
	w := newTable(a.Out)
	fmt.Fprintln(w, "BUCKET\tUSER\tTEAM\tPROJECT\tHOURS")
	for _, row := range r.Rows {
		bucket := row.Bucket
		if bucket == "" {
			bucket = "-"
		}
		project := row.Project
		if row.Archived {
			project += " (archived)"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			bucket, row.User, row.Team, project, formatSeconds(row.Seconds))
	}
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		"", "total", "", "", formatSeconds(r.TotalSeconds))
	_ = w.Flush()
}
