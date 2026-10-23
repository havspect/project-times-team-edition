package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/havspect/project-times/internal/client"
)

func (a *App) add(args []string) int {
	fs := newFlags("add")
	urlFlag, tokenFlag := clientFlags(fs)
	from := fs.String("from", "", "start time, 15:04 or RFC 3339")
	to := fs.String("to", "", "end time, 15:04, a duration like 1h30m, or RFC 3339")
	date := fs.String("date", "", "date for bare times: today, yesterday or YYYY-MM-DD")
	note := fs.String("note", "", "note")
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("add", err)
	}
	if fs.NArg() != 1 || *from == "" || *to == "" {
		return a.usageLine("times add <project> --from 09:00 --to 11:30 [--date today] [--note \"...\"]")
	}
	ref := fs.Arg(0)

	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("add", err)
	}
	me, err := c.Me()
	if err != nil {
		return a.fail("add", err)
	}
	loc := a.local()
	day, err := parseDate(*date, a.Now(), loc)
	if err != nil {
		return a.fail("add", err)
	}
	start, err := parseInstant(*from, day, loc)
	if err != nil {
		return a.fail("add", err)
	}
	end, err := parseEnd(*to, start, day, loc)
	if err != nil {
		return a.fail("add", err)
	}
	project, err := a.resolveProject(c, ref)
	if err != nil {
		return a.fail("add", err)
	}

	e, err := c.CreateEntry(client.EntryInput{
		ProjectID: project.ID, StartAt: start, EndAt: end,
		Timezone: a.zone(me), Note: *note,
	})
	if err != nil {
		return a.fail("add", err)
	}
	shown := entryLocation(*e, loc)
	fmt.Fprintf(a.Out, "logged %s %s-%s (%s) on %s\n",
		formatDate(e.StartAt, shown), formatClock(e.StartAt, shown), formatClock(*e.EndAt, shown),
		formatSeconds(secondsOf(*e, a.Now())), project.Name)
	return 0
}

func (a *App) ls(args []string) int {
	fs := newFlags("ls")
	urlFlag, tokenFlag := clientFlags(fs)
	from := fs.String("from", "", "inclusive start date, YYYY-MM-DD")
	to := fs.String("to", "", "exclusive end date, YYYY-MM-DD")
	user := fs.String("user", "", "me, an id, or an email (email needs an admin)")
	project := fs.String("project", "", "project name or id")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("ls", err)
	}
	if code, ok := a.checkNoArgs("ls", fs); !ok {
		return code
	}

	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("ls", err)
	}
	me, err := c.Me()
	if err != nil {
		return a.fail("ls", err)
	}
	q := client.EntryQuery{From: *from, To: *to}
	if *user != "" {
		if q.UserID, err = a.resolveUser(c, *user); err != nil {
			return a.fail("ls", err)
		}
	}
	if *project != "" {
		p, err := a.resolveProject(c, *project)
		if err != nil {
			return a.fail("ls", err)
		}
		q.ProjectID = p.ID
	}

	entries, err := c.ListEntries(q)
	if err != nil {
		return a.fail("ls", err)
	}
	if *asJSON {
		return a.writeJSON(entries)
	}
	a.printEntries(entries, a.projectNames(c, me), location(me))
	return 0
}

// printEntries writes the table `times ls` shows. A running entry has no end
// and counts up to now. Each entry keeps the zone it was logged in.
func (a *App) printEntries(entries []client.Entry, names map[uint]string, fallback *time.Location) {
	w := newTable(a.Out)
	fmt.Fprintln(w, "DATE\tSTART\tEND\tDURATION\tPROJECT\tNOTE")
	now := a.Now()
	for _, e := range entries {
		loc := entryLocation(e, fallback)
		end := "running"
		if e.EndAt != nil {
			end = formatClock(*e.EndAt, loc)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			formatDate(e.StartAt, loc), formatClock(e.StartAt, loc), end,
			formatSeconds(secondsOf(e, now)), projectName(names, e.ProjectID), e.Note)
	}
	_ = w.Flush()
}

// printEntry is the one-line confirmation a mutation prints.
func (a *App) printEntry(verb string, e *client.Entry, names map[uint]string, fallback *time.Location) {
	loc := entryLocation(*e, fallback)
	end := "running"
	duration := formatSeconds(secondsOf(*e, a.Now()))
	if e.EndAt != nil {
		end = formatClock(*e.EndAt, loc)
	}
	fmt.Fprintf(a.Out, "%s entry %d: %s %s-%s (%s) on %s\n",
		verb, e.ID, formatDate(e.StartAt, loc), formatClock(e.StartAt, loc), end,
		duration, projectName(names, e.ProjectID))
}

func (a *App) edit(args []string) int {
	fs := newFlags("edit")
	urlFlag, tokenFlag := clientFlags(fs)
	project := fs.String("project", "", "project name or id")
	from := fs.String("from", "", "start time, 15:04 or RFC 3339")
	to := fs.String("to", "", "end time, 15:04, a duration, or RFC 3339")
	date := fs.String("date", "", "date for bare times: today, yesterday or YYYY-MM-DD")
	timezone := fs.String("timezone", "", "IANA zone to correct the entry's day, e.g. Europe/Berlin")
	note := fs.String("note", "", "note")
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("edit", err)
	}
	const usage = "times edit <id> [--project ...] [--from ...] [--to ...] [--timezone ...] [--note \"...\"]"
	if fs.NArg() != 1 {
		return a.usageLine(usage)
	}
	id, err := parseID(fs.Arg(0))
	if err != nil {
		return a.fail("edit", err)
	}
	if !isSet(fs, "project") && !isSet(fs, "from") && !isSet(fs, "to") && !isSet(fs, "timezone") && !isSet(fs, "note") {
		return a.usageLine(usage)
	}

	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("edit", err)
	}
	me, err := c.Me()
	if err != nil {
		return a.fail("edit", err)
	}
	// Bare times are read in the machine zone; pass RFC 3339 to retime an entry
	// logged somewhere else.
	loc := a.local()
	day, err := parseDate(*date, a.Now(), loc)
	if err != nil {
		return a.fail("edit", err)
	}

	var patch client.EntryPatch
	if isSet(fs, "project") {
		p, err := a.resolveProject(c, *project)
		if err != nil {
			return a.fail("edit", err)
		}
		patch.ProjectID = &p.ID
	}
	if isSet(fs, "from") {
		t, err := parseInstant(*from, day, loc)
		if err != nil {
			return a.fail("edit", err)
		}
		patch.StartAt = &t
	}
	if isSet(fs, "to") {
		if isDuration(*to) && patch.StartAt == nil {
			return a.fail("edit", fmt.Errorf("a --to duration needs --from"))
		}
		base := day
		if patch.StartAt != nil {
			base = *patch.StartAt
		}
		t, err := parseEnd(*to, base, day, loc)
		if err != nil {
			return a.fail("edit", err)
		}
		patch.EndAt = &t
	}
	if isSet(fs, "timezone") {
		patch.Timezone = timezone
	}
	if isSet(fs, "note") {
		patch.Note = note
	}

	e, err := c.PatchEntry(id, patch)
	if err != nil {
		return a.fail("edit", err)
	}
	a.printEntry("updated", e, a.projectNames(c, me), loc)
	return 0
}

func (a *App) rm(args []string) int {
	fs := newFlags("rm")
	urlFlag, tokenFlag := clientFlags(fs)
	if err := parseFlags(fs, args); err != nil {
		return a.flagError("rm", err)
	}
	if fs.NArg() != 1 {
		return a.usageLine("times rm <id>")
	}
	id, err := parseID(fs.Arg(0))
	if err != nil {
		return a.fail("rm", err)
	}
	c, err := a.connect(*urlFlag, *tokenFlag)
	if err != nil {
		return a.fail("rm", err)
	}
	if err := c.DeleteEntry(id); err != nil {
		return a.fail("rm", err)
	}
	fmt.Fprintf(a.Out, "deleted entry %d\n", id)
	return 0
}

func (a *App) writeJSON(v any) int {
	enc := json.NewEncoder(a.Out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return a.fail("json", err)
	}
	return 0
}

// newTable is the tabwriter every listing shares.
func newTable(out io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
}
